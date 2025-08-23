// +build cgo

package dex

/*
#cgo CXXFLAGS: -std=c++17 -O3 -march=native
#cgo LDFLAGS: -L. -ladx_matching_engine -lstdc++ -lm
#cgo linux LDFLAGS: -lcuda -lcudart -lnvrtc
#cgo darwin LDFLAGS: -framework Metal -framework MetalPerformanceShaders

#include <stdlib.h>
#include <stdint.h>

// C wrapper for C++ ADX Matching Engine
typedef struct {
    char* id;
    char* ad_slot_id;
    double price;
    uint64_t quantity;
    uint64_t timestamp_ms;
    uint8_t is_buy;
    char* advertiser_id;
    char* targeting_json;
} COrder;

typedef struct {
    char* slot_id;
    double clearing_price;
    uint64_t total_volume;
    uint32_t matched_orders;
    char* winners_json;
} CBatchResult;

// Forward declarations of C++ wrapper functions
extern void* create_adx_engine();
extern void destroy_adx_engine(void* engine);
extern int add_order(void* engine, COrder* order);
extern CBatchResult* run_batch_auction(void* engine, const char* slot_id, uint32_t batch_ms);
extern void free_batch_result(CBatchResult* result);
extern double calculate_time_decay(void* engine, const char* slot_id, uint64_t timestamp_ms);
extern int cancel_order(void* engine, const char* order_id);
extern char* get_order_book_json(void* engine, const char* slot_id);
extern char* get_metrics_json(void* engine);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

// CGOEngine wraps the C++ ADX matching engine
type CGOEngine struct {
	engine unsafe.Pointer
	mu     sync.RWMutex
}

// NewCGOEngine creates a new C++ matching engine wrapper
func NewCGOEngine() (*CGOEngine, error) {
	engine := C.create_adx_engine()
	if engine == nil {
		return nil, fmt.Errorf("failed to create C++ engine")
	}

	e := &CGOEngine{
		engine: engine,
	}

	// Ensure cleanup on GC
	runtime.SetFinalizer(e, (*CGOEngine).destroy)

	return e, nil
}

// destroy cleans up the C++ engine
func (e *CGOEngine) destroy() {
	if e.engine != nil {
		C.destroy_adx_engine(e.engine)
		e.engine = nil
	}
}

// Close explicitly destroys the engine
func (e *CGOEngine) Close() {
	e.destroy()
	runtime.SetFinalizer(e, nil)
}

// AddOrder adds an order to the matching engine
func (e *CGOEngine) AddOrder(order *Order) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.engine == nil {
		return fmt.Errorf("engine is closed")
	}

	// Convert targeting to JSON
	targetingJSON, err := json.Marshal(order.Targeting)
	if err != nil {
		return fmt.Errorf("failed to marshal targeting: %w", err)
	}

	// Create C order
	cOrder := C.COrder{
		id:             C.CString(order.ID),
		ad_slot_id:     C.CString(order.AdSlotID),
		price:          C.double(order.Price),
		quantity:       C.uint64_t(order.Quantity),
		timestamp_ms:   C.uint64_t(order.Timestamp.UnixMilli()),
		is_buy:         boolToUint8(order.IsBuy),
		advertiser_id:  C.CString(order.AdvertiserID),
		targeting_json: C.CString(string(targetingJSON)),
	}

	// Clean up C strings
	defer func() {
		C.free(unsafe.Pointer(cOrder.id))
		C.free(unsafe.Pointer(cOrder.ad_slot_id))
		C.free(unsafe.Pointer(cOrder.advertiser_id))
		C.free(unsafe.Pointer(cOrder.targeting_json))
	}()

	// Add order to engine
	result := C.add_order(e.engine, &cOrder)
	if result != 0 {
		return fmt.Errorf("failed to add order: error code %d", result)
	}

	return nil
}

// RunBatchAuction runs a batch auction for a specific ad slot
func (e *CGOEngine) RunBatchAuction(slotID string, batchSizeMS uint32) (*BatchAuctionResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.engine == nil {
		return nil, fmt.Errorf("engine is closed")
	}

	cSlotID := C.CString(slotID)
	defer C.free(unsafe.Pointer(cSlotID))

	// Run auction
	cResult := C.run_batch_auction(e.engine, cSlotID, C.uint32_t(batchSizeMS))
	if cResult == nil {
		return nil, fmt.Errorf("batch auction failed")
	}
	defer C.free_batch_result(cResult)

	// Parse winners JSON
	var winners []MatchedOrder
	if cResult.winners_json != nil {
		winnersJSON := C.GoString(cResult.winners_json)
		if err := json.Unmarshal([]byte(winnersJSON), &winners); err != nil {
			return nil, fmt.Errorf("failed to parse winners: %w", err)
		}
	}

	return &BatchAuctionResult{
		SlotID:        C.GoString(cResult.slot_id),
		ClearingPrice: float64(cResult.clearing_price),
		TotalVolume:   uint64(cResult.total_volume),
		MatchedOrders: uint32(cResult.matched_orders),
		Winners:       winners,
	}, nil
}

// CalculateTimeDecay calculates time decay price for an ad slot
func (e *CGOEngine) CalculateTimeDecay(slotID string, timestamp time.Time) (float64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.engine == nil {
		return 0, fmt.Errorf("engine is closed")
	}

	cSlotID := C.CString(slotID)
	defer C.free(unsafe.Pointer(cSlotID))

	decay := C.calculate_time_decay(e.engine, cSlotID, C.uint64_t(timestamp.UnixMilli()))
	return float64(decay), nil
}

// CancelOrder cancels an existing order
func (e *CGOEngine) CancelOrder(orderID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.engine == nil {
		return fmt.Errorf("engine is closed")
	}

	cOrderID := C.CString(orderID)
	defer C.free(unsafe.Pointer(cOrderID))

	result := C.cancel_order(e.engine, cOrderID)
	if result != 0 {
		return fmt.Errorf("failed to cancel order: error code %d", result)
	}

	return nil
}

// GetOrderBook returns the current order book for an ad slot
func (e *CGOEngine) GetOrderBook(slotID string) (*OrderBook, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.engine == nil {
		return nil, fmt.Errorf("engine is closed")
	}

	cSlotID := C.CString(slotID)
	defer C.free(unsafe.Pointer(cSlotID))

	cJSON := C.get_order_book_json(e.engine, cSlotID)
	if cJSON == nil {
		return nil, fmt.Errorf("failed to get order book")
	}
	defer C.free(unsafe.Pointer(cJSON))

	var orderBook OrderBook
	if err := json.Unmarshal([]byte(C.GoString(cJSON)), &orderBook); err != nil {
		return nil, fmt.Errorf("failed to parse order book: %w", err)
	}

	return &orderBook, nil
}

// GetMetrics returns engine performance metrics
func (e *CGOEngine) GetMetrics() (*EngineMetrics, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.engine == nil {
		return nil, fmt.Errorf("engine is closed")
	}

	cJSON := C.get_metrics_json(e.engine)
	if cJSON == nil {
		return nil, fmt.Errorf("failed to get metrics")
	}
	defer C.free(unsafe.Pointer(cJSON))

	var metrics EngineMetrics
	if err := json.Unmarshal([]byte(C.GoString(cJSON)), &metrics); err != nil {
		return nil, fmt.Errorf("failed to parse metrics: %w", err)
	}

	return &metrics, nil
}

// Order represents an order in the matching engine
type Order struct {
	ID           string                 `json:"id"`
	AdSlotID     string                 `json:"ad_slot_id"`
	Price        float64                `json:"price"`
	Quantity     uint64                 `json:"quantity"`
	Timestamp    time.Time              `json:"timestamp"`
	IsBuy        bool                   `json:"is_buy"`
	AdvertiserID string                 `json:"advertiser_id"`
	Targeting    map[string]interface{} `json:"targeting"`
}

// BatchAuctionResult contains results from a batch auction
type BatchAuctionResult struct {
	SlotID        string         `json:"slot_id"`
	ClearingPrice float64        `json:"clearing_price"`
	TotalVolume   uint64         `json:"total_volume"`
	MatchedOrders uint32         `json:"matched_orders"`
	Winners       []MatchedOrder `json:"winners"`
}

// MatchedOrder represents a matched order in an auction
type MatchedOrder struct {
	OrderID      string  `json:"order_id"`
	AdvertiserID string  `json:"advertiser_id"`
	Price        float64 `json:"price"`
	Quantity     uint64  `json:"quantity"`
}

// OrderBook represents the order book for an ad slot
type OrderBook struct {
	SlotID     string        `json:"slot_id"`
	Bids       []OrderLevel  `json:"bids"`
	Asks       []OrderLevel  `json:"asks"`
	LastUpdate time.Time     `json:"last_update"`
}

// OrderLevel represents a price level in the order book
type OrderLevel struct {
	Price    float64 `json:"price"`
	Quantity uint64  `json:"quantity"`
	Orders   int     `json:"orders"`
}

// EngineMetrics contains performance metrics
type EngineMetrics struct {
	TotalOrders      uint64        `json:"total_orders"`
	ActiveOrders     uint64        `json:"active_orders"`
	TotalAuctions    uint64        `json:"total_auctions"`
	TotalVolume      uint64        `json:"total_volume"`
	AvgLatencyUS     uint64        `json:"avg_latency_us"`
	P99LatencyUS     uint64        `json:"p99_latency_us"`
	GPUUtilization   float64       `json:"gpu_utilization"`
	OrdersPerSecond  float64       `json:"orders_per_second"`
	AuctionsPerSecond float64      `json:"auctions_per_second"`
}

// Helper function
func boolToUint8(b bool) C.uint8_t {
	if b {
		return 1
	}
	return 0
}