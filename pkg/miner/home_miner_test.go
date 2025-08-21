package miner

import (
	"testing"
	"time"
)

func TestNewHomeMiner(t *testing.T) {
	config := &Config{
		WalletAddress: "0xABCDEF123456",
		LocalPort:     8888,
		CacheSize:     "10GB",
	}

	tunnelConfig := TunnelConfig{
		Type:      TunnelLocalXpose,
		Subdomain: "test-miner",
	}

	miner := NewHomeMiner(config, tunnelConfig)

	if miner == nil {
		t.Fatal("Expected miner to be created")
	}

	if miner.WalletAddress != config.WalletAddress {
		t.Errorf("Expected wallet %s, got %s", config.WalletAddress, miner.WalletAddress)
	}

	if miner.LocalPort != config.LocalPort {
		t.Errorf("Expected port %d, got %d", config.LocalPort, miner.LocalPort)
	}

	if miner.TunnelType != TunnelLocalXpose {
		t.Errorf("Expected tunnel type %s, got %s", TunnelLocalXpose, miner.TunnelType)
	}
}

func TestGenerateMinerID(t *testing.T) {
	id1 := generateMinerID()
	time.Sleep(1 * time.Nanosecond) // Ensure different timestamp
	id2 := generateMinerID()

	if id1 == "" {
		t.Error("Generated ID should not be empty")
	}

	if len(id1) != 16 {
		t.Errorf("Expected ID length 16, got %d", len(id1))
	}

	if id1 == id2 {
		t.Error("Generated IDs should be unique")
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"10GB", 10 * 1024 * 1024 * 1024},
		{"100GB", 100 * 1024 * 1024 * 1024},
		{"unknown", 1024 * 1024 * 1024}, // default 1GB
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSize(tt.input)
			if result != tt.expected {
				t.Errorf("parseSize(%s) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewAdCache(t *testing.T) {
	maxSize := int64(1024 * 1024 * 1024) // 1GB
	cache := NewAdCache(maxSize)

	if cache == nil {
		t.Fatal("Expected cache to be created")
	}

	if cache.maxSize != maxSize {
		t.Errorf("Expected max size %d, got %d", maxSize, cache.maxSize)
	}

	if cache.ads == nil {
		t.Error("Cache ads map should be initialized")
	}
}

func TestNewMinerEarnings(t *testing.T) {
	wallet := "0x123456789"
	earnings := NewMinerEarnings(wallet)

	if earnings == nil {
		t.Fatal("Expected earnings to be created")
	}

	if earnings.WalletAddress != wallet {
		t.Errorf("Expected wallet %s, got %s", wallet, earnings.WalletAddress)
	}

	if earnings.TotalEarnings.Cmp(earnings.PendingWithdrawal) != 0 {
		t.Error("Initial earnings should equal pending withdrawal (both 0)")
	}

	if !earnings.LastPayout.Equal(time.Time{}) {
		t.Error("Initial last payout should be zero time")
	}
}

func TestHardwareInfo_Defaults(t *testing.T) {
	miner := &HomeMiner{}
	hw := miner.DetectHardware()

	if hw == nil {
		t.Fatal("Expected hardware info to be returned")
	}

	if hw.CPUCores <= 0 {
		t.Error("CPU cores should be positive")
	}

	if hw.MemoryGB <= 0 {
		t.Error("Memory should be positive")
	}

	if hw.DiskGB <= 0 {
		t.Error("Disk space should be positive")
	}

	if hw.NetworkMbps <= 0 {
		t.Error("Network speed should be positive")
	}
}

func TestTunnelTypes(t *testing.T) {
	tunnels := []TunnelType{
		TunnelLocalXpose,
		TunnelNgrok,
		TunnelCloudflare,
		TunnelTailscale,
		TunnelDirectIP,
	}

	for _, tunnel := range tunnels {
		if tunnel == "" {
			t.Error("Tunnel type should not be empty")
		}
	}

	// Test specific values
	if TunnelLocalXpose != "localxpose" {
		t.Errorf("Expected localxpose, got %s", TunnelLocalXpose)
	}

	if TunnelNgrok != "ngrok" {
		t.Errorf("Expected ngrok, got %s", TunnelNgrok)
	}
}
