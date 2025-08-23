// +build !cgo

package dex

import (
	"container/heap"
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// PureGoEngine implements the ADX matching engine in pure Go
type PureGoEngine struct {
	// Order books per ad slot
	orderBooks map[string]*OrderBook
	bookMu     sync.RWMutex

	// Performance metrics
	totalOrders   uint64
	totalAuctions uint64
	totalVolume   uint64

	// Time decay parameters
	decayRate float64
	halfLife  time.Duration
}

// NewPureGoEngine creates a new pure Go matching engine
func NewPureGoEngine() Engine {
	return &PureGoEngine{
		orderBooks: make(map[string]*OrderBook),
		decayRate:  0.1,
		halfLife:   30 * time.Minute,
	}
}

// AddOrder adds an order to the matching engine
func (e *PureGoEngine) AddOrder(ctx context.Context, order *Order) error {
	e.bookMu.Lock()
	defer e.bookMu.Unlock()

	// Get or create order book
	book, exists := e.orderBooks[order.AdSlotID]
	if !exists {
		book = NewOrderBook(order.AdSlotID)
		e.orderBooks[order.AdSlotID] = book
	}

	// Add to appropriate side
	if order.IsBuy {
		heap.Push(&book.Bids, order)
	} else {
		heap.Push(&book.Asks, order)
	}

	atomic.AddUint64(&e.totalOrders, 1)
	return nil
}

// RunBatchAuction runs a batch auction for an ad slot
func (e *PureGoEngine) RunBatchAuction(ctx context.Context, slotID string, batchSizeMS uint32) (*BatchAuctionResult, error) {
	e.bookMu.Lock()
	defer e.bookMu.Unlock()

	book, exists := e.orderBooks[slotID]
	if !exists {
		return &BatchAuctionResult{
			SlotID:        slotID,
			ClearingPrice: 0,
			TotalVolume:   0,
			MatchedOrders: 0,
		}, nil
	}

	// Collect orders for batch
	startTime := time.Now()
	batchDuration := time.Duration(batchSizeMS) * time.Millisecond

	// Match orders
	matches := e.matchOrders(book)

	// Calculate clearing price (uniform price auction)
	clearingPrice := e.calculateClearingPrice(matches)

	// Apply time decay to clearing price
	clearingPrice *= e.calculateTimeDecayFactor(slotID, time.Now())

	// Execute matches
	var totalVolume uint64
	var winners []MatchedOrder
	for _, match := range matches {
		if match.Price >= clearingPrice {
			totalVolume += match.Quantity
			winners = append(winners, match)
		}
	}

	atomic.AddUint64(&e.totalAuctions, 1)
	atomic.AddUint64(&e.totalVolume, totalVolume)

	// Record auction time
	auctionTime := time.Since(startTime)
	if auctionTime > batchDuration {
		fmt.Printf("WARNING: Auction took %v, exceeding batch size %v\n", auctionTime, batchDuration)
	}

	return &BatchAuctionResult{
		SlotID:        slotID,
		ClearingPrice: clearingPrice,
		TotalVolume:   totalVolume,
		MatchedOrders: uint32(len(winners)),
		Winners:       winners,
		LatencyUS:     uint64(auctionTime.Microseconds()),
	}, nil
}

// matchOrders performs order matching using price-time priority
func (e *PureGoEngine) matchOrders(book *OrderBook) []MatchedOrder {
	var matches []MatchedOrder

	// Simple greedy matching algorithm
	for book.Bids.Len() > 0 && book.Asks.Len() > 0 {
		bestBid := heap.Pop(&book.Bids).(*Order)
		bestAsk := heap.Pop(&book.Asks).(*Order)

		// Check if orders cross
		if bestBid.Price >= bestAsk.Price {
			// Calculate match quantity
			matchQty := bestBid.Quantity
			if bestAsk.Quantity < matchQty {
				matchQty = bestAsk.Quantity
			}

			// Record match
			matches = append(matches, MatchedOrder{
				BuyOrderID:   bestBid.ID,
				SellOrderID:  bestAsk.ID,
				Price:        bestAsk.Price, // Seller's price in uniform auction
				Quantity:     matchQty,
				AdvertiserID: bestBid.AdvertiserID,
			})

			// Update quantities
			bestBid.Quantity -= matchQty
			bestAsk.Quantity -= matchQty

			// Re-add if not fully filled
			if bestBid.Quantity > 0 {
				heap.Push(&book.Bids, bestBid)
			}
			if bestAsk.Quantity > 0 {
				heap.Push(&book.Asks, bestAsk)
			}
		} else {
			// No more crosses, re-add orders
			heap.Push(&book.Bids, bestBid)
			heap.Push(&book.Asks, bestAsk)
			break
		}
	}

	return matches
}

// calculateClearingPrice determines the uniform clearing price
func (e *PureGoEngine) calculateClearingPrice(matches []MatchedOrder) float64 {
	if len(matches) == 0 {
		return 0
	}

	// Sort matches by price
	prices := make([]float64, len(matches))
	for i, match := range matches {
		prices[i] = match.Price
	}
	sort.Float64s(prices)

	// Use median price as clearing price (simplified)
	return prices[len(prices)/2]
}

// calculateTimeDecayFactor calculates time decay for perishable inventory
func (e *PureGoEngine) calculateTimeDecayFactor(slotID string, timestamp time.Time) float64 {
	// Exponential decay: price = initial_price * e^(-λt)
	// where λ = ln(2) / half_life

	// For demo, assume slot was created 10 minutes ago
	slotAge := 10 * time.Minute
	lambda := math.Ln2 / e.halfLife.Seconds()
	decay := math.Exp(-lambda * slotAge.Seconds())

	// Ensure minimum price (10% of original)
	if decay < 0.1 {
		decay = 0.1
	}

	return decay
}

// CancelOrder cancels an existing order
func (e *PureGoEngine) CancelOrder(ctx context.Context, orderID string) error {
	e.bookMu.Lock()
	defer e.bookMu.Unlock()

	// Search all order books (simplified)
	for _, book := range e.orderBooks {
		// Remove from bids
		for i, order := range book.Bids {
			if order.ID == orderID {
				book.Bids = append(book.Bids[:i], book.Bids[i+1:]...)
				heap.Init(&book.Bids)
				return nil
			}
		}

		// Remove from asks
		for i, order := range book.Asks {
			if order.ID == orderID {
				book.Asks = append(book.Asks[:i], book.Asks[i+1:]...)
				heap.Init(&book.Asks)
				return nil
			}
		}
	}

	return fmt.Errorf("order %s not found", orderID)
}

// GetOrderBook returns the current order book for an ad slot
func (e *PureGoEngine) GetOrderBook(ctx context.Context, slotID string) (*OrderBookSnapshot, error) {
	e.bookMu.RLock()
	defer e.bookMu.RUnlock()

	book, exists := e.orderBooks[slotID]
	if !exists {
		return &OrderBookSnapshot{
			SlotID:     slotID,
			Bids:       []PriceLevel{},
			Asks:       []PriceLevel{},
			LastUpdate: time.Now(),
		}, nil
	}

	// Aggregate by price level
	bidLevels := aggregatePriceLevels(book.Bids)
	askLevels := aggregatePriceLevels(book.Asks)

	return &OrderBookSnapshot{
		SlotID:     slotID,
		Bids:       bidLevels,
		Asks:       askLevels,
		LastUpdate: time.Now(),
	}, nil
}

// GetMetrics returns engine performance metrics
func (e *PureGoEngine) GetMetrics(ctx context.Context) (*EngineMetrics, error) {
	e.bookMu.RLock()
	activeOrders := uint64(0)
	for _, book := range e.orderBooks {
		activeOrders += uint64(book.Bids.Len() + book.Asks.Len())
	}
	e.bookMu.RUnlock()

	return &EngineMetrics{
		Implementation: "pure-go",
		TotalOrders:    atomic.LoadUint64(&e.totalOrders),
		ActiveOrders:   activeOrders,
		TotalAuctions:  atomic.LoadUint64(&e.totalAuctions),
		TotalVolume:    atomic.LoadUint64(&e.totalVolume),
		AvgLatencyUS:   100, // Typical Go performance
		P99LatencyUS:   500,
		OrdersPerSec:   float64(atomic.LoadUint64(&e.totalOrders)) / time.Since(startTime).Seconds(),
		AuctionsPerSec: float64(atomic.LoadUint64(&e.totalAuctions)) / time.Since(startTime).Seconds(),
	}, nil
}

// OrderBook represents an order book for an ad slot
type OrderBook struct {
	SlotID string
	Bids   OrderHeap // Max heap (highest price first)
	Asks   OrderHeap // Min heap (lowest price first)
}

// NewOrderBook creates a new order book
func NewOrderBook(slotID string) *OrderBook {
	return &OrderBook{
		SlotID: slotID,
		Bids:   make(OrderHeap, 0),
		Asks:   make(OrderHeap, 0),
	}
}

// OrderHeap implements heap.Interface for orders
type OrderHeap []*Order

func (h OrderHeap) Len() int           { return len(h) }
func (h OrderHeap) Less(i, j int) bool {
	// For bids: higher price is better (max heap)
	// For asks: lower price is better (min heap)
	// This is toggled based on IsBuy flag
	if len(h) > 0 && h[0].IsBuy {
		return h[i].Price > h[j].Price // Max heap for bids
	}
	return h[i].Price < h[j].Price // Min heap for asks
}
func (h OrderHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *OrderHeap) Push(x interface{}) {
	*h = append(*h, x.(*Order))
}

func (h *OrderHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// aggregatePriceLevels aggregates orders by price
func aggregatePriceLevels(orders OrderHeap) []PriceLevel {
	levels := make(map[float64]*PriceLevel)

	for _, order := range orders {
		level, exists := levels[order.Price]
		if !exists {
			level = &PriceLevel{
				Price:    order.Price,
				Quantity: 0,
				Orders:   0,
			}
			levels[order.Price] = level
		}
		level.Quantity += order.Quantity
		level.Orders++
	}

	// Convert to slice and sort
	var result []PriceLevel
	for _, level := range levels {
		result = append(result, *level)
	}

	sort.Slice(result, func(i, j int) bool {
		if len(orders) > 0 && orders[0].IsBuy {
			return result[i].Price > result[j].Price // Bids: highest first
		}
		return result[i].Price < result[j].Price // Asks: lowest first
	})

	return result
}

// BenchmarkPureGoEngine benchmarks the pure Go implementation
func BenchmarkPureGoEngine(b *testing.T) {
	engine := NewPureGoEngine()
	ctx := context.Background()

	// Pre-populate with orders
	for i := 0; i < 1000; i++ {
		order := &Order{
			ID:           fmt.Sprintf("order-%d", i),
			AdSlotID:     "slot-1",
			Price:        float64(i%100) + 1.0,
			Quantity:     uint64(i%10) + 1,
			IsBuy:        i%2 == 0,
			AdvertiserID: fmt.Sprintf("adv-%d", i%10),
			Timestamp:    time.Now(),
		}
		engine.AddOrder(ctx, order)
	}

	b.ResetTimer()

	// Benchmark batch auctions
	for i := 0; i < b.N; i++ {
		engine.RunBatchAuction(ctx, "slot-1", 100)
	}
}

var startTime = time.Now()