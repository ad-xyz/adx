package dex

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

// Engine interface for different matching engine implementations
type Engine interface {
	// Core operations
	AddOrder(ctx context.Context, order *Order) error
	RunBatchAuction(ctx context.Context, slotID string, batchSizeMS uint32) (*BatchAuctionResult, error)
	CancelOrder(ctx context.Context, orderID string) error
	
	// Query operations
	GetOrderBook(ctx context.Context, slotID string) (*OrderBookSnapshot, error)
	GetMetrics(ctx context.Context) (*EngineMetrics, error)
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
	PublisherID  string                 `json:"publisher_id"`
	Targeting    map[string]interface{} `json:"targeting"`
}

// BatchAuctionResult contains results from a batch auction
type BatchAuctionResult struct {
	SlotID        string         `json:"slot_id"`
	ClearingPrice float64        `json:"clearing_price"`
	TotalVolume   uint64         `json:"total_volume"`
	MatchedOrders uint32         `json:"matched_orders"`
	Winners       []MatchedOrder `json:"winners"`
	LatencyUS     uint64         `json:"latency_us"`
}

// MatchedOrder represents a matched order in an auction
type MatchedOrder struct {
	BuyOrderID   string  `json:"buy_order_id"`
	SellOrderID  string  `json:"sell_order_id"`
	OrderID      string  `json:"order_id"`
	AdvertiserID string  `json:"advertiser_id"`
	PublisherID  string  `json:"publisher_id"`
	Price        float64 `json:"price"`
	Quantity     uint64  `json:"quantity"`
}

// OrderBookSnapshot represents the current state of an order book
type OrderBookSnapshot struct {
	SlotID     string       `json:"slot_id"`
	Bids       []PriceLevel `json:"bids"`
	Asks       []PriceLevel `json:"asks"`
	LastUpdate time.Time    `json:"last_update"`
}

// PriceLevel represents a price level in the order book
type PriceLevel struct {
	Price    float64 `json:"price"`
	Quantity uint64  `json:"quantity"`
	Orders   int     `json:"orders"`
}

// EngineMetrics contains performance metrics
type EngineMetrics struct {
	Implementation   string  `json:"implementation"`
	TotalOrders      uint64  `json:"total_orders"`
	ActiveOrders     uint64  `json:"active_orders"`
	TotalAuctions    uint64  `json:"total_auctions"`
	TotalVolume      uint64  `json:"total_volume"`
	AvgLatencyUS     uint64  `json:"avg_latency_us"`
	P99LatencyUS     uint64  `json:"p99_latency_us"`
	OrdersPerSec     float64 `json:"orders_per_second"`
	AuctionsPerSec   float64 `json:"auctions_per_second"`
	
	// Hardware utilization
	CPUCoresUsed     int     `json:"cpu_cores_used,omitempty"`
	SIMDEnabled      bool    `json:"simd_enabled,omitempty"`
	GPUEnabled       bool    `json:"gpu_enabled,omitempty"`
	GPUDevice        string  `json:"gpu_device,omitempty"`
	GPUMemoryUsed    uint64  `json:"gpu_memory_used,omitempty"`
	GPUUtilization   float64 `json:"gpu_utilization,omitempty"`
}

// EngineType represents the type of matching engine
type EngineType string

const (
	EngineTypePureGo EngineType = "pure-go"
	EngineTypeCPU    EngineType = "cpu-optimized"
	EngineTypeGPU    EngineType = "gpu-accelerated"
	EngineTypeAuto   EngineType = "auto"
)

// NewEngine creates a new matching engine based on the specified type
func NewEngine(engineType EngineType) (Engine, error) {
	switch engineType {
	case EngineTypePureGo:
		return NewPureGoEngine(), nil
		
	case EngineTypeCPU:
		// Try CGO engine if available
		if engine := tryCreateCGOEngine(); engine != nil {
			return engine, nil
		}
		// Fall back to pure Go
		fmt.Println("C++ engine not available, using pure Go implementation")
		return NewPureGoEngine(), nil
		
	case EngineTypeGPU:
		// Try GPU engine if available
		if engine := tryCreateGPUEngine(); engine != nil {
			return engine, nil
		}
		// Fall back to CPU
		if engine := tryCreateCGOEngine(); engine != nil {
			fmt.Println("GPU not available, using CPU-optimized implementation")
			return engine, nil
		}
		// Fall back to pure Go
		fmt.Println("GPU and C++ engines not available, using pure Go implementation")
		return NewPureGoEngine(), nil
		
	case EngineTypeAuto:
		// Auto-detect best available engine
		return autoDetectEngine(), nil
		
	default:
		return nil, fmt.Errorf("unknown engine type: %s", engineType)
	}
}

// autoDetectEngine automatically selects the best available engine
func autoDetectEngine() Engine {
	// Try GPU first (fastest)
	if engine := tryCreateGPUEngine(); engine != nil {
		fmt.Println("Auto-detected: GPU-accelerated engine")
		return engine
	}
	
	// Try CPU-optimized with C++
	if engine := tryCreateCGOEngine(); engine != nil {
		fmt.Println("Auto-detected: CPU-optimized C++ engine")
		return engine
	}
	
	// Fall back to pure Go
	fmt.Println("Auto-detected: Pure Go engine")
	return NewPureGoEngine()
}

// tryCreateCGOEngine attempts to create a CGO-based engine
func tryCreateCGOEngine() Engine {
	// This function is implemented in engine_cgo.go when CGO is enabled
	// Returns nil when CGO is disabled
	return createCGOEngineIfAvailable()
}

// tryCreateGPUEngine attempts to create a GPU-accelerated engine
func tryCreateGPUEngine() Engine {
	// Check for GPU availability
	if runtime.GOOS == "darwin" {
		// Try Metal on macOS
		return createMetalEngineIfAvailable()
	} else if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		// Try CUDA on Linux/Windows
		return createCUDAEngineIfAvailable()
	}
	return nil
}

// These functions are implemented in their respective build-tagged files
var (
	createCGOEngineIfAvailable   = func() Engine { return nil }
	createMetalEngineIfAvailable  = func() Engine { return nil }
	createCUDAEngineIfAvailable   = func() Engine { return nil }
)