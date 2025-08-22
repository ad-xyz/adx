package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestFDBBackend_StoreImpression(t *testing.T) {
	backend, err := NewFDBBackend("")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()
	impression := &ImpressionRecord{
		ID:          "imp-test-123",
		Timestamp:   time.Now(),
		PublisherID: "pub-456",
		Price:       decimal.NewFromFloat(5.50),
		MinerID:     "miner-789",
		UserID:      "user-abc",
		Completed:   true,
	}

	err = backend.StoreImpression(ctx, impression)
	if err != nil {
		t.Errorf("Failed to store impression: %v", err)
	}

	// Verify it was stored
	if backend.impressionStore[impression.ID] == nil {
		t.Error("Impression was not stored in memory")
	}
}

func TestFDBBackend_GetImpressions(t *testing.T) {
	backend, err := NewFDBBackend("")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()
	now := time.Now()

	// Store test impressions
	for i := 0; i < 5; i++ {
		imp := &ImpressionRecord{
			ID:          fmt.Sprintf("imp-%d", i),
			Timestamp:   now.Add(time.Duration(i) * time.Hour),
			PublisherID: "pub-123",
			Price:       decimal.NewFromFloat(float64(i) + 1.50),
		}
		backend.StoreImpression(ctx, imp)
	}

	// Get impressions for time range
	start := now.Add(-1 * time.Hour)
	end := now.Add(6 * time.Hour)
	impressions, err := backend.GetImpressions(ctx, start, end, 10)

	if err != nil {
		t.Errorf("Failed to get impressions: %v", err)
	}

	if len(impressions) == 0 {
		t.Error("No impressions returned")
	}
}

func TestFDBBackend_MinerEarnings(t *testing.T) {
	backend, err := NewFDBBackend("")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()
	minerID := "miner-test-123"

	// Initially should be zero
	earnings, err := backend.GetMinerEarnings(ctx, minerID)
	if err != nil {
		t.Errorf("Failed to get miner earnings: %v", err)
	}
	if !earnings.IsZero() {
		t.Errorf("Expected zero earnings, got %s", earnings.String())
	}

	// Update earnings
	amount := decimal.NewFromFloat(10.50)
	err = backend.UpdateMinerEarnings(ctx, minerID, amount)
	if err != nil {
		t.Errorf("Failed to update miner earnings: %v", err)
	}

	// Check updated earnings
	earnings, err = backend.GetMinerEarnings(ctx, minerID)
	if err != nil {
		t.Errorf("Failed to get updated earnings: %v", err)
	}
	if !earnings.Equal(amount) {
		t.Errorf("Expected earnings %s, got %s", amount.String(), earnings.String())
	}

	// Add more earnings
	additionalAmount := decimal.NewFromFloat(5.25)
	err = backend.UpdateMinerEarnings(ctx, minerID, additionalAmount)
	if err != nil {
		t.Errorf("Failed to add more earnings: %v", err)
	}

	earnings, err = backend.GetMinerEarnings(ctx, minerID)
	if err != nil {
		t.Errorf("Failed to get total earnings: %v", err)
	}
	expectedTotal := amount.Add(additionalAmount)
	if !earnings.Equal(expectedTotal) {
		t.Errorf("Expected total earnings %s, got %s", expectedTotal.String(), earnings.String())
	}
}

func TestFDBBackend_GetMetrics(t *testing.T) {
	backend, err := NewFDBBackend("")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Initial metrics should be zero
	metrics := backend.GetMetrics()
	if metrics["writes"] != 0 {
		t.Errorf("Expected 0 writes, got %d", metrics["writes"])
	}

	// Store some data
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		backend.StoreImpression(ctx, &ImpressionRecord{
			ID:        fmt.Sprintf("imp-%d", i),
			Timestamp: time.Now(),
		})
	}

	// Check metrics updated
	metrics = backend.GetMetrics()
	if metrics["writes"] != 10 {
		t.Errorf("Expected 10 writes, got %d", metrics["writes"])
	}
}

func TestFDBBackend_Optimize(t *testing.T) {
	backend, err := NewFDBBackend("")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()
	now := time.Now()

	// Store old and new impressions
	oldImp := &ImpressionRecord{
		ID:        "old-imp",
		Timestamp: now.AddDate(0, -2, 0), // 2 months old
	}
	newImp := &ImpressionRecord{
		ID:        "new-imp",
		Timestamp: now, // Current
	}

	backend.StoreImpression(ctx, oldImp)
	backend.StoreImpression(ctx, newImp)

	// Run optimization
	err = backend.Optimize(ctx)
	if err != nil {
		t.Errorf("Failed to optimize: %v", err)
	}

	// Old impression should be removed, new should remain
	if _, exists := backend.impressionStore["old-imp"]; exists {
		t.Error("Old impression should have been removed")
	}
	if _, exists := backend.impressionStore["new-imp"]; !exists {
		t.Error("New impression should still exist")
	}
}

func TestPublisherStats(t *testing.T) {
	stats := &PublisherStats{
		PublisherID:      "pub-123",
		TotalImpressions: 1000,
		TotalRevenue:     decimal.NewFromFloat(500),
		UniqueUsers:      map[string]bool{"user1": true, "user2": true},
	}

	// Calculate fill rate
	stats.FillRate = 0.85

	// Calculate eCPM
	if stats.TotalImpressions > 0 {
		stats.eCPM = stats.TotalRevenue.Div(decimal.NewFromInt(int64(stats.TotalImpressions))).Mul(decimal.NewFromInt(1000))
	}

	if stats.eCPM.String() != "500" {
		t.Errorf("Expected eCPM 500, got %s", stats.eCPM.String())
	}

	if len(stats.UniqueUsers) != 2 {
		t.Errorf("Expected 2 unique users, got %d", len(stats.UniqueUsers))
	}
}

