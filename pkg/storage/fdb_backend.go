// +build fdb

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/apple/foundationdb/bindings/go/src/fdb/directory"
	"github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
)

// FDBBackend implements Backend using FoundationDB
type FDBBackend struct {
	db       fdb.Database
	dir      directory.DirectorySubspace
	keyspace string
}

// NewFDBBackend creates a new FoundationDB backend
func NewFDBBackend(clusterFile string, keyspace string) (*FDBBackend, error) {
	// Initialize FDB
	fdb.MustAPIVersion(720) // Latest API version

	// Open database
	db, err := fdb.OpenDatabase(clusterFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open FDB: %w", err)
	}

	// Create directory for our keyspace
	dir, err := directory.CreateOrOpen(db, []string{"adx", keyspace}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return &FDBBackend{
		db:       db,
		dir:      dir,
		keyspace: keyspace,
	}, nil
}

// Store implements Backend.Store
func (b *FDBBackend) Store(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	_, err = b.db.Transact(func(tr fdb.Transaction) (interface{}, error) {
		fdbKey := b.dir.Pack(tuple.Tuple{key})
		tr.Set(fdbKey, data)
		return nil, nil
	})

	return err
}

// Load implements Backend.Load
func (b *FDBBackend) Load(ctx context.Context, key string, dest interface{}) error {
	ret, err := b.db.Transact(func(tr fdb.Transaction) (interface{}, error) {
		fdbKey := b.dir.Pack(tuple.Tuple{key})
		return tr.Get(fdbKey).Get(), nil
	})

	if err != nil {
		return err
	}

	data := ret.([]byte)
	if data == nil {
		return fmt.Errorf("key not found: %s", key)
	}

	return json.Unmarshal(data, dest)
}

// Delete implements Backend.Delete
func (b *FDBBackend) Delete(ctx context.Context, key string) error {
	_, err := b.db.Transact(func(tr fdb.Transaction) (interface{}, error) {
		fdbKey := b.dir.Pack(tuple.Tuple{key})
		tr.Clear(fdbKey)
		return nil, nil
	})

	return err
}

// List implements Backend.List
func (b *FDBBackend) List(ctx context.Context, prefix string) ([]string, error) {
	ret, err := b.db.Transact(func(tr fdb.Transaction) (interface{}, error) {
		prefixKey := b.dir.Pack(tuple.Tuple{prefix})
		rng := fdb.PrefixRange(prefixKey)

		iter := tr.GetRange(rng, fdb.RangeOptions{Limit: 1000}).Iterator()

		var keys []string
		for iter.Advance() {
			kv := iter.MustGet()
			t, err := b.dir.Unpack(kv.Key)
			if err != nil {
				continue
			}
			if len(t) > 0 {
				keys = append(keys, t[0].(string))
			}
		}

		return keys, nil
	})

	if err != nil {
		return nil, err
	}

	return ret.([]string), nil
}

// Atomic implements Backend.Atomic
func (b *FDBBackend) Atomic(ctx context.Context, fn func(tx Transaction) error) error {
	_, err := b.db.Transact(func(tr fdb.Transaction) (interface{}, error) {
		ftx := &FDBTransaction{
			tr:  tr,
			dir: b.dir,
		}
		return nil, fn(ftx)
	})

	return err
}

// Watch implements Backend.Watch
func (b *FDBBackend) Watch(ctx context.Context, key string) (<-chan Event, error) {
	ch := make(chan Event, 10)

	go func() {
		defer close(ch)

		fdbKey := b.dir.Pack(tuple.Tuple{key})

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Get current value and watch for changes
			ret, err := b.db.Transact(func(tr fdb.Transaction) (interface{}, error) {
				future := tr.Watch(fdbKey)
				value := tr.Get(fdbKey).MustGet()
				return struct {
					Future fdb.FutureNil
					Value  []byte
				}{future, value}, nil
			})

			if err != nil {
				ch <- Event{Type: EventError, Key: key, Error: err}
				return
			}

			result := ret.(struct {
				Future fdb.FutureNil
				Value  []byte
			})

			// Send current value
			if result.Value != nil {
				var data interface{}
				if err := json.Unmarshal(result.Value, &data); err == nil {
					ch <- Event{Type: EventSet, Key: key, Value: data}
				}
			}

			// Wait for change
			result.Future.BlockUntilReady()
		}
	}()

	return ch, nil
}

// Close implements Backend.Close
func (b *FDBBackend) Close() error {
	// FDB doesn't need explicit closing
	return nil
}

// FDBTransaction implements Transaction using FDB
type FDBTransaction struct {
	tr  fdb.Transaction
	dir directory.DirectorySubspace
}

// Get implements Transaction.Get
func (t *FDBTransaction) Get(key string) (interface{}, error) {
	fdbKey := t.dir.Pack(tuple.Tuple{key})
	data := t.tr.Get(fdbKey).MustGet()
	
	if data == nil {
		return nil, nil
	}

	var value interface{}
	err := json.Unmarshal(data, &value)
	return value, err
}

// Set implements Transaction.Set
func (t *FDBTransaction) Set(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	fdbKey := t.dir.Pack(tuple.Tuple{key})
	t.tr.Set(fdbKey, data)
	return nil
}

// Delete implements Transaction.Delete
func (t *FDBTransaction) Delete(key string) error {
	fdbKey := t.dir.Pack(tuple.Tuple{key})
	t.tr.Clear(fdbKey)
	return nil
}

// Increment implements Transaction.Increment
func (t *FDBTransaction) Increment(key string, delta int64) (int64, error) {
	fdbKey := t.dir.Pack(tuple.Tuple{key})
	
	// Use FDB's atomic add
	deltaBytes := tuple.Tuple{delta}.Pack()
	t.tr.Add(fdbKey, deltaBytes)
	
	// Read back the value
	future := t.tr.Get(fdbKey)
	data := future.MustGet()
	
	if data == nil {
		return delta, nil
	}

	unpacked, err := tuple.Unpack(data)
	if err != nil {
		return 0, err
	}

	if len(unpacked) > 0 {
		if val, ok := unpacked[0].(int64); ok {
			return val, nil
		}
	}

	return 0, fmt.Errorf("invalid counter value")
}

// FDB-specific features for high-performance ad serving

// StoreImpression stores an impression with automatic TTL
func (b *FDBBackend) StoreImpression(ctx context.Context, impression *Impression) error {
	_, err := b.db.Transact(func(tr fdb.Transaction) (interface{}, error) {
		// Store by ID
		idKey := b.dir.Pack(tuple.Tuple{"impressions", impression.ID})
		data, _ := json.Marshal(impression)
		tr.Set(idKey, data)

		// Store by date for analytics
		date := impression.Timestamp.Format("2006-01-02")
		dateKey := b.dir.Pack(tuple.Tuple{"impressions", "by-date", date, impression.ID})
		tr.Set(dateKey, data)

		// Store by publisher for revenue tracking
		pubKey := b.dir.Pack(tuple.Tuple{"impressions", "by-publisher", impression.PublisherID, impression.ID})
		tr.Set(pubKey, data)

		// Update counters
		counterKey := b.dir.Pack(tuple.Tuple{"metrics", "impressions", "count"})
		tr.Add(counterKey, tuple.Tuple{int64(1)}.Pack())

		// Set TTL (FDB doesn't have native TTL, so we track expiry time)
		ttl := time.Now().Add(7 * 24 * time.Hour) // 7 day retention
		ttlKey := b.dir.Pack(tuple.Tuple{"ttl", ttl.Unix(), "impressions", impression.ID})
		tr.Set(ttlKey, []byte{1})

		return nil, nil
	})

	return err
}

// GetImpressionsByDate retrieves impressions for a specific date
func (b *FDBBackend) GetImpressionsByDate(ctx context.Context, date string) ([]*Impression, error) {
	ret, err := b.db.Transact(func(tr fdb.Transaction) (interface{}, error) {
		prefix := b.dir.Pack(tuple.Tuple{"impressions", "by-date", date})
		rng := fdb.PrefixRange(prefix)

		iter := tr.GetRange(rng, fdb.RangeOptions{}).Iterator()

		var impressions []*Impression
		for iter.Advance() {
			kv := iter.MustGet()
			var imp Impression
			if err := json.Unmarshal(kv.Value, &imp); err == nil {
				impressions = append(impressions, &imp)
			}
		}

		return impressions, nil
	})

	if err != nil {
		return nil, err
	}

	return ret.([]*Impression), nil
}

// StoreBid stores a bid with indexing
func (b *FDBBackend) StoreBid(ctx context.Context, bid *Bid) error {
	_, err := b.db.Transact(func(tr fdb.Transaction) (interface{}, error) {
		// Store by ID
		idKey := b.dir.Pack(tuple.Tuple{"bids", bid.ID})
		data, _ := json.Marshal(bid)
		tr.Set(idKey, data)

		// Store by impression for auction history
		impKey := b.dir.Pack(tuple.Tuple{"bids", "by-impression", bid.ImpressionID, bid.ID})
		tr.Set(impKey, data)

		// Store by DSP for performance tracking
		dspKey := b.dir.Pack(tuple.Tuple{"bids", "by-dsp", bid.DSPID, bid.Timestamp.Unix(), bid.ID})
		tr.Set(dspKey, data)

		// Update DSP counters
		dspCounterKey := b.dir.Pack(tuple.Tuple{"metrics", "dsp", bid.DSPID, "bids"})
		tr.Add(dspCounterKey, tuple.Tuple{int64(1)}.Pack())

		// Update revenue if won
		if bid.Won {
			revenueKey := b.dir.Pack(tuple.Tuple{"metrics", "revenue", bid.Timestamp.Format("2006-01-02")})
			revenue := int64(bid.Price * 1000000) // Store as micros for precision
			tr.Add(revenueKey, tuple.Tuple{revenue}.Pack())
		}

		return nil, nil
	})

	return err
}

// GetDSPMetrics retrieves metrics for a DSP
func (b *FDBBackend) GetDSPMetrics(ctx context.Context, dspID string) (*DSPMetrics, error) {
	ret, err := b.db.Transact(func(tr fdb.Transaction) (interface{}, error) {
		metrics := &DSPMetrics{
			DSPID: dspID,
		}

		// Get bid count
		bidCountKey := b.dir.Pack(tuple.Tuple{"metrics", "dsp", dspID, "bids"})
		if data := tr.Get(bidCountKey).MustGet(); data != nil {
			if unpacked, err := tuple.Unpack(data); err == nil && len(unpacked) > 0 {
				metrics.BidCount = unpacked[0].(int64)
			}
		}

		// Get win count
		winCountKey := b.dir.Pack(tuple.Tuple{"metrics", "dsp", dspID, "wins"})
		if data := tr.Get(winCountKey).MustGet(); data != nil {
			if unpacked, err := tuple.Unpack(data); err == nil && len(unpacked) > 0 {
				metrics.WinCount = unpacked[0].(int64)
			}
		}

		// Get total spend
		spendKey := b.dir.Pack(tuple.Tuple{"metrics", "dsp", dspID, "spend"})
		if data := tr.Get(spendKey).MustGet(); data != nil {
			if unpacked, err := tuple.Unpack(data); err == nil && len(unpacked) > 0 {
				metrics.TotalSpend = float64(unpacked[0].(int64)) / 1000000.0
			}
		}

		return metrics, nil
	})

	if err != nil {
		return nil, err
	}

	return ret.(*DSPMetrics), nil
}

// CleanupExpired removes expired entries based on TTL
func (b *FDBBackend) CleanupExpired(ctx context.Context) error {
	now := time.Now().Unix()

	_, err := b.db.Transact(func(tr fdb.Transaction) (interface{}, error) {
		// Find expired entries
		prefix := b.dir.Pack(tuple.Tuple{"ttl"})
		endKey := b.dir.Pack(tuple.Tuple{"ttl", now})
		rng := fdb.KeyRange{Begin: fdb.Key(prefix), End: fdb.Key(endKey)}

		iter := tr.GetRange(rng, fdb.RangeOptions{Limit: 1000}).Iterator()

		for iter.Advance() {
			kv := iter.MustGet()
			
			// Parse TTL key to get the actual data key
			t, err := b.dir.Unpack(kv.Key)
			if err != nil || len(t) < 4 {
				continue
			}

			// Delete the TTL entry
			tr.Clear(kv.Key)

			// Delete the actual data
			dataType := t[2].(string)
			dataID := t[3].(string)
			dataKey := b.dir.Pack(tuple.Tuple{dataType, dataID})
			tr.Clear(dataKey)
		}

		return nil, nil
	})

	return err
}

// Data structures

// Impression represents an ad impression
type Impression struct {
	ID          string    `json:"id"`
	PublisherID string    `json:"publisher_id"`
	AdSlotID    string    `json:"ad_slot_id"`
	Timestamp   time.Time `json:"timestamp"`
	Device      Device    `json:"device"`
	Geo         Geo       `json:"geo"`
}

// Bid represents a bid in the system
type Bid struct {
	ID           string    `json:"id"`
	ImpressionID string    `json:"impression_id"`
	DSPID        string    `json:"dsp_id"`
	Price        float64   `json:"price"`
	Timestamp    time.Time `json:"timestamp"`
	Won          bool      `json:"won"`
}

// Device information
type Device struct {
	Type string `json:"type"`
	OS   string `json:"os"`
	UA   string `json:"ua"`
}

// Geo location
type Geo struct {
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
}

// DSPMetrics for performance tracking
type DSPMetrics struct {
	DSPID      string  `json:"dsp_id"`
	BidCount   int64   `json:"bid_count"`
	WinCount   int64   `json:"win_count"`
	TotalSpend float64 `json:"total_spend"`
	WinRate    float64 `json:"win_rate"`
}