//go:build !fdb
// +build !fdb

package storage

import (
	"context"
	"sync"
	"time"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/shopspring/decimal"
)

// Mock FDB types for when FDB is not available
type mockDatabase struct{}
type mockTransaction struct{}
type mockDirectorySubspace struct{}

// FDBBackend provides storage backend (mock version)
type FDBBackend struct {
	// Mock storage
	impressionStore map[string]*ImpressionRecord
	bidStore        map[string]*BidRecord
	minerEarnings   map[string]decimal.Decimal
	
	// Write buffer for batch operations
	writeBuffer chan WriteOp
	bufferSize  int
	
	// Metrics
	writeCount  uint64
	readCount   uint64
	errorCount  uint64
	
	mu          sync.RWMutex
}

// WriteOp represents a write operation
type WriteOp struct {
	Type      string
	Key       []byte
	Value     []byte
	Timestamp time.Time
}

// ImpressionRecord stores impression data
type ImpressionRecord struct {
	ID           string          `json:"id"`
	Timestamp    time.Time       `json:"timestamp"`
	PublisherID  string          `json:"publisher_id"`
	PlacementID  string          `json:"placement_id"`
	MinerID      string          `json:"miner_id"`
	UserID       string          `json:"user_id"`
	DeviceType   string          `json:"device_type"`
	GeoCountry   string          `json:"geo_country"`
	GeoRegion    string          `json:"geo_region"`
	Price        decimal.Decimal `json:"price"`
	WinningDSP   string          `json:"winning_dsp"`
	AdPodID      string          `json:"ad_pod_id"`
	VideoURL     string          `json:"video_url"`
	Duration     int             `json:"duration"`
	Completed    bool            `json:"completed"`
}

// BidRecord stores bid data
type BidRecord struct {
	ID          string          `json:"id"`
	Timestamp   time.Time       `json:"timestamp"`
	BidRequest  string          `json:"bid_request"`
	DSPID       string          `json:"dsp_id"`
	BidPrice    decimal.Decimal `json:"bid_price"`
	Won         bool            `json:"won"`
	ResponseTime time.Duration  `json:"response_time"`
	CreativeID  string          `json:"creative_id"`
}

// NewFDBBackend creates a new mock backend
func NewFDBBackend(clusterFile string) (*FDBBackend, error) {
	backend := &FDBBackend{
		impressionStore: make(map[string]*ImpressionRecord),
		bidStore:        make(map[string]*BidRecord),
		minerEarnings:   make(map[string]decimal.Decimal),
		writeBuffer:     make(chan WriteOp, 100000),
		bufferSize:      100000,
	}
	
	// Start background writer
	go backend.backgroundWriter()
	
	return backend, nil
}

// StoreImpression stores an impression record
func (f *FDBBackend) StoreImpression(ctx context.Context, impression *ImpressionRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.impressionStore[impression.ID] = impression
	f.writeCount++
	
	return nil
}

// StoreBid stores a bid record
func (f *FDBBackend) StoreBid(ctx context.Context, bid *BidRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.bidStore[bid.ID] = bid
	f.writeCount++
	
	return nil
}

// backgroundWriter processes batched writes (mock)
func (f *FDBBackend) backgroundWriter() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	batch := make([]WriteOp, 0, 1000)
	
	for {
		select {
		case op := <-f.writeBuffer:
			batch = append(batch, op)
			
			if len(batch) >= 1000 {
				f.flushBatch(batch)
				batch = batch[:0]
			}
			
		case <-ticker.C:
			if len(batch) > 0 {
				f.flushBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// flushBatch writes a batch of operations (mock)
func (f *FDBBackend) flushBatch(batch []WriteOp) {
	f.mu.Lock()
	f.writeCount += uint64(len(batch))
	f.mu.Unlock()
}

// GetImpressions retrieves impressions for a time range
func (f *FDBBackend) GetImpressions(ctx context.Context, start, end time.Time, limit int) ([]*ImpressionRecord, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	impressions := make([]*ImpressionRecord, 0, limit)
	
	for _, imp := range f.impressionStore {
		if imp.Timestamp.After(start) && imp.Timestamp.Before(end) {
			impressions = append(impressions, imp)
			if len(impressions) >= limit {
				break
			}
		}
	}
	
	f.readCount += uint64(len(impressions))
	
	return impressions, nil
}

// GetPublisherStats retrieves aggregated stats for a publisher
func (f *FDBBackend) GetPublisherStats(ctx context.Context, publisherID string, start, end time.Time) (*PublisherStats, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	stats := &PublisherStats{
		PublisherID:      publisherID,
		TotalImpressions: 0,
		TotalRevenue:     decimal.Zero,
		UniqueUsers:      make(map[string]bool),
	}
	
	for _, imp := range f.impressionStore {
		if imp.PublisherID == publisherID && 
		   imp.Timestamp.After(start) && 
		   imp.Timestamp.Before(end) {
			stats.TotalImpressions++
			stats.TotalRevenue = stats.TotalRevenue.Add(imp.Price)
			stats.UniqueUsers[imp.UserID] = true
		}
	}
	
	if stats.TotalImpressions > 0 {
		stats.FillRate = 0.85 // Mock fill rate
		stats.eCPM = stats.TotalRevenue.Div(decimal.NewFromInt(int64(stats.TotalImpressions))).Mul(decimal.NewFromInt(1000))
	}
	
	return stats, nil
}

// StoreBidRequest stores an OpenRTB bid request
func (f *FDBBackend) StoreBidRequest(ctx context.Context, request *openrtb2.BidRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.writeCount++
	return nil
}

// GetMinerEarnings retrieves earnings for a miner
func (f *FDBBackend) GetMinerEarnings(ctx context.Context, minerID string) (decimal.Decimal, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	if earnings, ok := f.minerEarnings[minerID]; ok {
		return earnings, nil
	}
	
	return decimal.Zero, nil
}

// UpdateMinerEarnings updates miner earnings
func (f *FDBBackend) UpdateMinerEarnings(ctx context.Context, minerID string, amount decimal.Decimal) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	current := f.minerEarnings[minerID]
	f.minerEarnings[minerID] = current.Add(amount)
	f.writeCount++
	
	return nil
}

// GetMetrics returns storage metrics
func (f *FDBBackend) GetMetrics() map[string]uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	return map[string]uint64{
		"writes": f.writeCount,
		"reads":  f.readCount,
		"errors": f.errorCount,
	}
}

// PublisherStats represents aggregated publisher statistics
type PublisherStats struct {
	PublisherID      string
	TotalImpressions uint64
	TotalRevenue     decimal.Decimal
	UniqueUsers      map[string]bool
	FillRate         float64
	eCPM             decimal.Decimal
}

// Close closes the backend
func (f *FDBBackend) Close() error {
	close(f.writeBuffer)
	return nil
}

// Optimize runs optimization on the database
func (f *FDBBackend) Optimize(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Clear old data beyond retention period
	retentionDays := 30
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	
	// Remove old impressions
	for id, imp := range f.impressionStore {
		if imp.Timestamp.Before(cutoff) {
			delete(f.impressionStore, id)
		}
	}
	
	// Remove old bids
	for id, bid := range f.bidStore {
		if bid.Timestamp.Before(cutoff) {
			delete(f.bidStore, id)
		}
	}
	
	return nil
}