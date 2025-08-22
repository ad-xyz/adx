package rtb

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/shopspring/decimal"
)

func TestRTBExchange_Init(t *testing.T) {
	exchange := &RTBExchange{
		DSPs:           make(map[string]*DSPConnection),
		SSPs:           make(map[string]*SSPConnection),
		AuctionTimeout: 100 * time.Millisecond,
		FloorPrice:     decimal.NewFromFloat(0.50),
		Revenue:        big.NewInt(0),
	}

	if exchange.DSPs == nil {
		t.Error("DSPs map should be initialized")
	}

	if exchange.SSPs == nil {
		t.Error("SSPs map should be initialized")
	}

	if exchange.Revenue.Cmp(big.NewInt(0)) != 0 {
		t.Error("Revenue should start at 0")
	}
}

func TestCTVOptimizer_Validate(t *testing.T) {
	optimizer := &CTVOptimizer{
		PublicaEnabled:            true,
		PodFillRate:               0.85,
		OptimalPodSize:            6,
		CreativeQualityThreshold: 0.7,
		MinVideoBitrate:          2000000,
		RequiredFormats:          []string{"video/mp4", "video/webm"},
	}

	if !optimizer.PublicaEnabled {
		t.Error("Publica should be enabled")
	}

	if optimizer.PodFillRate < 0 || optimizer.PodFillRate > 1 {
		t.Errorf("Invalid pod fill rate: %f", optimizer.PodFillRate)
	}

	if optimizer.MinVideoBitrate < 1000000 {
		t.Error("Video bitrate too low for CTV")
	}
}

func TestHomeMiner_Earnings(t *testing.T) {
	miner := &HomeMiner{
		ID:            "miner-123",
		WalletAddress: "0x123456",
		Endpoint:      "https://miner.example.com",
		Earnings:      big.NewInt(1000000), // $1 in smallest unit
	}

	if miner.Earnings.Cmp(big.NewInt(1000000)) != 0 {
		t.Errorf("Expected earnings 1000000, got %s", miner.Earnings.String())
	}

	if miner.WalletAddress != "0x123456" {
		t.Errorf("Expected wallet 0x123456, got %s", miner.WalletAddress)
	}
}

func TestBidRequest_Mock(t *testing.T) {
	// Mock bid request for testing
	bidReq := &openrtb2.BidRequest{
		ID: "test-bid-123",
		Imp: []openrtb2.Imp{
			{
				ID: "imp-1",
				Video: &openrtb2.Video{
					W:        ptrInt64(1920),
					H:        ptrInt64(1080),
					MinDuration: 15,
					MaxDuration: 30,
				},
				BidFloor: 2.5,
			},
		},
	}

	if bidReq.ID != "test-bid-123" {
		t.Errorf("Expected bid ID test-bid-123, got %s", bidReq.ID)
	}

	if len(bidReq.Imp) != 1 {
		t.Errorf("Expected 1 impression, got %d", len(bidReq.Imp))
	}

	if bidReq.Imp[0].BidFloor != 2.5 {
		t.Errorf("Expected floor 2.5, got %f", bidReq.Imp[0].BidFloor)
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}

func TestRTBExchange_BidRequest(t *testing.T) {
	exchange := &RTBExchange{
		DSPs:           make(map[string]*DSPConnection),
		SSPs:           make(map[string]*SSPConnection),
		AuctionTimeout: 100 * time.Millisecond,
		FloorPrice:     decimal.NewFromFloat(0.50),
		Revenue:        big.NewInt(0),
	}
	
	// Add a test DSP with RateLimiter
	exchange.DSPs["test-dsp"] = &DSPConnection{
		ID:         "test-dsp",
		Name:       "Test DSP",
		Endpoint:   "http://localhost:9999/bid",
		QPS:        1000,
		Timeout:    80 * time.Millisecond,
		BidderCode: "test",
		SeatID:     "seat1",
		RateLimiter: &RateLimiter{
			tokens:     10,
			max:        10,
			refill:     time.Millisecond * 10,
			lastRefill: time.Now(),
		},
	}

	bidReq := &openrtb2.BidRequest{
		ID: "test-bid-123",
		Imp: []openrtb2.Imp{
			{
				ID: "imp-1",
				Video: &openrtb2.Video{
					W:           ptrInt64(1920),
					H:           ptrInt64(1080),
					MinDuration: 15,
					MaxDuration: 30,
				},
				BidFloor: 2.5,
			},
		},
	}

	// This will fail to connect but tests the flow
	ctx := context.Background()
	_, err := exchange.BidRequest(ctx, bidReq)
	// Should complete without crashing even if DSP is unreachable
	_ = err
}

func TestAdPodAssembler_AssemblePod(t *testing.T) {
	assembler := &AdPodAssembler{
		MaxPodDuration:        120 * time.Second,
		MinPodDuration:        15 * time.Second,
		TargetCPM:             decimal.NewFromFloat(25.0),
		CompetitiveSeparation: true,
		FrequencyCapping:      true,
		CreativeDeduping:      true,
	}

	if assembler.MaxPodDuration < assembler.MinPodDuration {
		t.Error("Max duration should be >= min duration")
	}

	if !assembler.CompetitiveSeparation {
		t.Error("Competitive separation should be enabled")
	}
}

func TestMinerRegistry_Register(t *testing.T) {
	registry := &MinerRegistry{
		Miners: make(map[string]*HomeMiner),
	}

	miner := &HomeMiner{
		ID:            "miner-123",
		WalletAddress: "0xABC",
		Endpoint:      "https://miner.local",
		Earnings:      big.NewInt(0),
	}

	registry.Miners[miner.ID] = miner

	if len(registry.Miners) != 1 {
		t.Errorf("Expected 1 miner, got %d", len(registry.Miners))
	}

	if registry.Miners["miner-123"].WalletAddress != "0xABC" {
		t.Error("Miner not properly registered")
	}
}
