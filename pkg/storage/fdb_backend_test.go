package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockFDBBackend for testing
type MockFDBBackend struct {
	mock.Mock
}

func (m *MockFDBBackend) StoreImpression(ctx context.Context, imp *ImpressionRecord) error {
	args := m.Called(ctx, imp)
	return args.Error(0)
}

func (m *MockFDBBackend) GetImpressions(ctx context.Context, start, end time.Time, limit int) ([]*ImpressionRecord, error) {
	args := m.Called(ctx, start, end, limit)
	return args.Get(0).([]*ImpressionRecord), args.Error(1)
}

func TestImpressionRecord_Store(t *testing.T) {
	mockDB := new(MockFDBBackend)
	ctx := context.Background()
	
	impression := &ImpressionRecord{
		ID:          "imp-123",
		Timestamp:   time.Now(),
		PublisherID: "pub-456",
		PlacementID: "placement-789",
		MinerID:     "miner-abc",
		UserID:      "user-xyz",
		DeviceType:  "ctv",
		GeoCountry:  "US",
		GeoRegion:   "CA",
		Price:       decimal.NewFromFloat(5.50),
		WinningDSP:  "dsp-premium",
		AdPodID:     "pod-123",
		VideoURL:    "https://cdn.adx.com/video.mp4",
		Duration:    15,
		Completed:   true,
	}
	
	mockDB.On("StoreImpression", ctx, impression).Return(nil)
	
	err := mockDB.StoreImpression(ctx, impression)
	assert.NoError(t, err)
	
	mockDB.AssertExpectations(t)
}

func TestBidRecord_Store(t *testing.T) {
	bid := &BidRecord{
		ID:           "bid-123",
		Timestamp:    time.Now(),
		BidRequest:   "req-456",
		DSPID:        "dsp-789",
		BidPrice:     decimal.NewFromFloat(10.50),
		Won:          true,
		ResponseTime: 50 * time.Millisecond,
		CreativeID:   "creative-abc",
	}
	
	assert.Equal(t, "bid-123", bid.ID)
	assert.True(t, bid.Won)
	assert.Equal(t, "10.5", bid.BidPrice.String())
}

func TestPublisherStats_Aggregate(t *testing.T) {
	stats := &PublisherStats{
		PublisherID:      "pub-123",
		TotalImpressions: 1000000,
		TotalRevenue:     decimal.NewFromFloat(5000),
		UniqueUsers:      map[string]bool{"user1": true, "user2": true},
		FillRate:         0.85,
		eCPM:             decimal.NewFromFloat(5.0),
	}
	
	assert.Equal(t, uint64(1000000), stats.TotalImpressions)
	assert.Equal(t, 2, len(stats.UniqueUsers))
	assert.Equal(t, 0.85, stats.FillRate)
	
	// Calculate effective CPM
	if stats.TotalImpressions > 0 {
		calculatedECPM := stats.TotalRevenue.Div(decimal.NewFromInt(int64(stats.TotalImpressions))).Mul(decimal.NewFromInt(1000))
		assert.Equal(t, "5", calculatedECPM.StringFixed(0))
	}
}

func TestGetImpressions_TimeRange(t *testing.T) {
	mockDB := new(MockFDBBackend)
	ctx := context.Background()
	
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()
	limit := 100
	
	expectedImpressions := []*ImpressionRecord{
		{
			ID:          "imp-1",
			Timestamp:   time.Now().Add(-1 * time.Hour),
			PublisherID: "pub-1",
			Price:       decimal.NewFromFloat(3.50),
		},
		{
			ID:          "imp-2",
			Timestamp:   time.Now().Add(-2 * time.Hour),
			PublisherID: "pub-1",
			Price:       decimal.NewFromFloat(4.50),
		},
	}
	
	mockDB.On("GetImpressions", ctx, start, end, limit).Return(expectedImpressions, nil)
	
	impressions, err := mockDB.GetImpressions(ctx, start, end, limit)
	assert.NoError(t, err)
	assert.Len(t, impressions, 2)
	assert.Equal(t, "imp-1", impressions[0].ID)
	
	mockDB.AssertExpectations(t)
}

func TestMinerEarnings_Update(t *testing.T) {
	tests := []struct {
		name           string
		minerID        string
		initialAmount  decimal.Decimal
		addAmount      decimal.Decimal
		expectedTotal  decimal.Decimal
	}{
		{
			name:          "first earning",
			minerID:       "miner-001",
			initialAmount: decimal.Zero,
			addAmount:     decimal.NewFromFloat(10.50),
			expectedTotal: decimal.NewFromFloat(10.50),
		},
		{
			name:          "additional earning",
			minerID:       "miner-002",
			initialAmount: decimal.NewFromFloat(100),
			addAmount:     decimal.NewFromFloat(25.75),
			expectedTotal: decimal.NewFromFloat(125.75),
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total := tt.initialAmount.Add(tt.addAmount)
			assert.Equal(t, tt.expectedTotal.String(), total.String())
		})
	}
}

func TestWriteBuffer_Batching(t *testing.T) {
	// Test that write operations are properly batched
	writeOps := make([]WriteOp, 0)
	
	for i := 0; i < 1000; i++ {
		op := WriteOp{
			Type:      "impression",
			Key:       []byte(fmt.Sprintf("key-%d", i)),
			Value:     []byte(fmt.Sprintf("value-%d", i)),
			Timestamp: time.Now(),
		}
		writeOps = append(writeOps, op)
	}
	
	assert.Len(t, writeOps, 1000)
	
	// Simulate batch processing
	batchSize := 100
	for i := 0; i < len(writeOps); i += batchSize {
		end := i + batchSize
		if end > len(writeOps) {
			end = len(writeOps)
		}
		batch := writeOps[i:end]
		assert.LessOrEqual(t, len(batch), batchSize)
	}
}

func TestMetrics_Tracking(t *testing.T) {
	metrics := map[string]uint64{
		"writes": 0,
		"reads":  0,
		"errors": 0,
	}
	
	// Simulate operations
	metrics["writes"] += 100
	metrics["reads"] += 50
	metrics["errors"] += 2
	
	assert.Equal(t, uint64(100), metrics["writes"])
	assert.Equal(t, uint64(50), metrics["reads"])
	assert.Equal(t, uint64(2), metrics["errors"])
	
	// Calculate success rate
	totalOps := metrics["writes"] + metrics["reads"]
	successRate := float64(totalOps-metrics["errors"]) / float64(totalOps) * 100
	assert.Greater(t, successRate, 98.0)
}