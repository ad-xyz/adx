package miner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/luxfi/adx/pkg/vast"
	"github.com/shopspring/decimal"
)

// HomeMiner represents a home-based ad serving node
type HomeMiner struct {
	// Identity
	ID            string
	WalletAddress string
	NodeName      string
	
	// Tunnel configuration
	TunnelType    TunnelType
	PublicURL     string // Public URL from LocalXpose/ngrok
	LocalPort     int
	AuthToken     string // For tunnel service
	
	// Hardware specs
	CPU           CPUInfo
	GPU           GPUInfo
	Memory        MemoryInfo
	Storage       StorageInfo
	Network       NetworkInfo
	
	// Geographic info
	Location      LocationInfo
	
	// Serving capabilities
	ServingStats  ServingStats
	AdCache       *AdCache
	
	// Earnings
	Earnings      *MinerEarnings
	
	// Connection to ADX
	ADXEndpoint   string
	WSConn        *websocket.Conn
	
	// HTTP server for ad serving
	Server        *http.Server
	
	mu            sync.RWMutex
}

// TunnelType for exposing miner to internet
type TunnelType string

const (
	TunnelLocalXpose TunnelType = "localxpose"
	TunnelNgrok      TunnelType = "ngrok"
	TunnelCloudflare TunnelType = "cloudflare"
	TunnelTailscale  TunnelType = "tailscale"
	TunnelDirectIP   TunnelType = "direct" // Public IP
)

// CPUInfo about the miner's CPU
type CPUInfo struct {
	Model         string
	Cores         int
	Threads       int
	FrequencyMHz  int
	Architecture  string // x86_64, arm64, etc
	HasAVX2       bool
	HasAVX512     bool
}

// GPUInfo about available GPUs
type GPUInfo struct {
	Available     bool
	Count         int
	Models        []string
	TotalMemoryMB int
	CUDAVersion   string
	MLXSupported  bool // Apple Silicon
}

// MemoryInfo about RAM
type MemoryInfo struct {
	TotalMB       int
	AvailableMB   int
	SwapMB        int
}

// StorageInfo about disk
type StorageInfo struct {
	TotalGB       int
	AvailableGB   int
	CacheSizeGB   int // Allocated for ad cache
	IOPSRead      int
	IOPSWrite     int
}

// NetworkInfo about connection
type NetworkInfo struct {
	ISP           string
	ASN           string
	UploadMbps    float64
	DownloadMbps  float64
	LatencyMS     int
	PacketLoss    float64
	IPv4          string
	IPv6          string
}

// LocationInfo for geographic targeting
type LocationInfo struct {
	Country       string
	Region        string
	City          string
	PostalCode    string
	Latitude      float64
	Longitude     float64
	Timezone      string
	DMA           int // Designated Market Area for US
}

// ServingStats tracks performance
type ServingStats struct {
	TotalRequests uint64
	TotalServed   uint64
	TotalErrors   uint64
	TotalBytes    uint64
	AverageLatency time.Duration
	Uptime        time.Duration
	StartTime     time.Time
	
	// Real-time metrics
	CurrentQPS    float64
	CurrentBandwidth float64
	
	// Quality metrics
	FillRate      float64
	ErrorRate     float64
	P95Latency    time.Duration
	P99Latency    time.Duration
}

// AdCache for local ad storage
type AdCache struct {
	MaxSizeBytes  int64
	CurrentSize   int64
	Items         map[string]*CachedAd
	LRU           []string
	mu            sync.RWMutex
}

// CachedAd stored locally
type CachedAd struct {
	ID            string
	Creative      []byte
	ContentType   string
	Size          int64
	Format        string // video/mp4, image/jpeg, etc
	Duration      int    // For video
	ExpiresAt     time.Time
	ServeCount    uint64
	LastServed    time.Time
}

// MinerEarnings tracks rewards
type MinerEarnings struct {
	TotalEarned   *big.Int
	PendingPayout *big.Int
	LastPayout    time.Time
	PayoutAddress string
	
	// Detailed earnings
	ImpressionsEarned *big.Int
	BandwidthEarned   *big.Int
	StorageEarned     *big.Int
	QualityBonus      *big.Int
	
	// Rate information
	CPMRate       decimal.Decimal // Earnings per 1000 impressions
	BandwidthRate decimal.Decimal // Earnings per GB
	StorageRate   decimal.Decimal // Earnings per GB/day
}

// MinerConfig for initialization
type MinerConfig struct {
	WalletAddress string
	ADXEndpoint   string
	TunnelType    TunnelType
	TunnelAuth    string
	LocalPort     int
	CacheSizeGB   int
	MaxBandwidth  float64 // Mbps limit
}

// NewHomeMiner creates a new home miner instance
func NewHomeMiner(config MinerConfig) (*HomeMiner, error) {
	miner := &HomeMiner{
		ID:            generateMinerID(config.WalletAddress),
		WalletAddress: config.WalletAddress,
		NodeName:      fmt.Sprintf("miner-%s", config.WalletAddress[:8]),
		TunnelType:    config.TunnelType,
		AuthToken:     config.TunnelAuth,
		LocalPort:     config.LocalPort,
		ADXEndpoint:   config.ADXEndpoint,
		AdCache: &AdCache{
			MaxSizeBytes: int64(config.CacheSizeGB) * 1024 * 1024 * 1024,
			Items:        make(map[string]*CachedAd),
		},
		Earnings: &MinerEarnings{
			TotalEarned:   big.NewInt(0),
			PendingPayout: big.NewInt(0),
			PayoutAddress: config.WalletAddress,
			ImpressionsEarned: big.NewInt(0),
			BandwidthEarned: big.NewInt(0),
			StorageEarned: big.NewInt(0),
			QualityBonus: big.NewInt(0),
		},
	}
	
	// Detect hardware
	miner.detectHardware()
	
	// Detect location
	miner.detectLocation()
	
	// Setup tunnel
	if err := miner.setupTunnel(); err != nil {
		return nil, fmt.Errorf("failed to setup tunnel: %w", err)
	}
	
	// Start HTTP server
	miner.startHTTPServer()
	
	// Connect to ADX
	if err := miner.connectToADX(); err != nil {
		return nil, fmt.Errorf("failed to connect to ADX: %w", err)
	}
	
	// Start metrics collection
	go miner.collectMetrics()
	
	return miner, nil
}

// detectHardware analyzes system capabilities
func (m *HomeMiner) detectHardware() {
	// CPU detection
	m.CPU = CPUInfo{
		Cores:        runtime.NumCPU(),
		Architecture: runtime.GOARCH,
	}
	
	// GPU detection (simplified)
	m.detectGPU()
	
	// Memory detection
	m.detectMemory()
	
	// Storage detection
	m.detectStorage()
	
	// Network speed test
	m.testNetworkSpeed()
}

// detectGPU checks for GPU availability
func (m *HomeMiner) detectGPU() {
	// Check for NVIDIA GPU
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		output, err := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader").Output()
		if err == nil {
			m.GPU.Available = true
			m.GPU.Count = 1
			m.GPU.Models = []string{string(output)}
		}
	}
	
	// Check for Apple Silicon
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		m.GPU.Available = true
		m.GPU.MLXSupported = true
		m.GPU.Models = []string{"Apple Silicon GPU"}
	}
}

// detectMemory gets system memory info
func (m *HomeMiner) detectMemory() {
	// Simplified - would use system calls in production
	m.Memory = MemoryInfo{
		TotalMB:     16384, // 16GB default
		AvailableMB: 8192,
	}
}

// detectStorage gets disk info
func (m *HomeMiner) detectStorage() {
	// Simplified - would use system calls in production
	m.Storage = StorageInfo{
		TotalGB:     500,
		AvailableGB: 100,
		CacheSizeGB: 10,
	}
}

// testNetworkSpeed performs bandwidth test
func (m *HomeMiner) testNetworkSpeed() {
	// Simplified - would use actual speed test in production
	m.Network = NetworkInfo{
		UploadMbps:   100.0,
		DownloadMbps: 500.0,
		LatencyMS:    20,
	}
}

// detectLocation gets geographic info
func (m *HomeMiner) detectLocation() {
	// Use IP geolocation service
	resp, err := http.Get("https://ipapi.co/json/")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	
	var loc struct {
		Country    string  `json:"country_name"`
		Region     string  `json:"region"`
		City       string  `json:"city"`
		Postal     string  `json:"postal"`
		Latitude   float64 `json:"latitude"`
		Longitude  float64 `json:"longitude"`
		Timezone   string  `json:"timezone"`
		ASN        string  `json:"asn"`
		Org        string  `json:"org"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&loc); err == nil {
		m.Location = LocationInfo{
			Country:    loc.Country,
			Region:     loc.Region,
			City:       loc.City,
			PostalCode: loc.Postal,
			Latitude:   loc.Latitude,
			Longitude:  loc.Longitude,
			Timezone:   loc.Timezone,
		}
		m.Network.ISP = loc.Org
		m.Network.ASN = loc.ASN
	}
}

// setupTunnel establishes public access
func (m *HomeMiner) setupTunnel() error {
	switch m.TunnelType {
	case TunnelLocalXpose:
		return m.setupLocalXpose()
	case TunnelNgrok:
		return m.setupNgrok()
	case TunnelCloudflare:
		return m.setupCloudflare()
	case TunnelTailscale:
		return m.setupTailscale()
	case TunnelDirectIP:
		// No tunnel needed, using public IP
		m.PublicURL = fmt.Sprintf("http://%s:%d", m.Network.IPv4, m.LocalPort)
		return nil
	default:
		return fmt.Errorf("unsupported tunnel type: %s", m.TunnelType)
	}
}

// setupLocalXpose creates LocalXpose tunnel
func (m *HomeMiner) setupLocalXpose() error {
	// Install LocalXpose if not present
	if _, err := exec.LookPath("loclx"); err != nil {
		// Download and install LocalXpose
		cmd := exec.Command("curl", "-sSL", "https://localxpose.io/install.sh", "|", "bash")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install LocalXpose: %w", err)
		}
	}
	
	// Start LocalXpose tunnel
	cmd := exec.Command("loclx", "tunnel", "http",
		"--port", fmt.Sprintf("%d", m.LocalPort),
		"--subdomain", m.NodeName,
	)
	
	if m.AuthToken != "" {
		cmd.Args = append(cmd.Args, "--auth-token", m.AuthToken)
	}
	
	// Start tunnel in background
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start LocalXpose: %w", err)
	}
	
	// Wait for tunnel to be ready
	time.Sleep(3 * time.Second)
	
	// Get public URL
	output, err := exec.Command("loclx", "list").Output()
	if err != nil {
		return fmt.Errorf("failed to get LocalXpose URL: %w", err)
	}
	
	// Parse URL from output
	m.PublicURL = extractURLFromOutput(string(output))
	
	return nil
}

// setupNgrok creates ngrok tunnel
func (m *HomeMiner) setupNgrok() error {
	// Similar to LocalXpose but using ngrok
	cmd := exec.Command("ngrok", "http", fmt.Sprintf("%d", m.LocalPort))
	
	if m.AuthToken != "" {
		exec.Command("ngrok", "authtoken", m.AuthToken).Run()
	}
	
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ngrok: %w", err)
	}
	
	// Get public URL from ngrok API
	time.Sleep(3 * time.Second)
	resp, err := http.Get("http://localhost:4040/api/tunnels")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	var result struct {
		Tunnels []struct {
			PublicURL string `json:"public_url"`
		} `json:"tunnels"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	
	if len(result.Tunnels) > 0 {
		m.PublicURL = result.Tunnels[0].PublicURL
	}
	
	return nil
}

// setupCloudflare creates Cloudflare Tunnel
func (m *HomeMiner) setupCloudflare() error {
	// Use cloudflared for zero-trust tunnel
	cmd := exec.Command("cloudflared", "tunnel", "--url",
		fmt.Sprintf("http://localhost:%d", m.LocalPort))
	
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start cloudflared: %w", err)
	}
	
	// Cloudflare will output the URL
	// In production, would parse this properly
	m.PublicURL = fmt.Sprintf("https://%s.trycloudflare.com", m.NodeName)
	
	return nil
}

// setupTailscale creates Tailscale tunnel
func (m *HomeMiner) setupTailscale() error {
	// Tailscale funnel for public access
	cmd := exec.Command("tailscale", "funnel", fmt.Sprintf("%d", m.LocalPort))
	
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Tailscale funnel: %w", err)
	}
	
	// Get Tailscale hostname
	output, err := exec.Command("tailscale", "status", "--json").Output()
	if err != nil {
		return err
	}
	
	var status struct {
		Self struct {
			DNSName string `json:"DNSName"`
		} `json:"Self"`
	}
	
	if err := json.NewDecoder(bytes.NewReader(output)).Decode(&status); err != nil {
		return err
	}
	
	m.PublicURL = fmt.Sprintf("https://%s:%d", status.Self.DNSName, m.LocalPort)
	
	return nil
}

// startHTTPServer for serving ads
func (m *HomeMiner) startHTTPServer() {
	mux := http.NewServeMux()
	
	// Ad serving endpoint
	mux.HandleFunc("/serve", m.handleServeAd)
	
	// VAST endpoint for video ads
	mux.HandleFunc("/vast", m.handleVAST)
	
	// Health check
	mux.HandleFunc("/health", m.handleHealth)
	
	// Metrics endpoint
	mux.HandleFunc("/metrics", m.handleMetrics)
	
	m.Server = &http.Server{
		Addr:    fmt.Sprintf(":%d", m.LocalPort),
		Handler: mux,
	}
	
	go func() {
		if err := m.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()
}

// handleServeAd serves cached ads
func (m *HomeMiner) handleServeAd(w http.ResponseWriter, r *http.Request) {
	adID := r.URL.Query().Get("id")
	if adID == "" {
		http.Error(w, "Missing ad ID", http.StatusBadRequest)
		return
	}
	
	// Get from cache
	m.AdCache.mu.RLock()
	cached, exists := m.AdCache.Items[adID]
	m.AdCache.mu.RUnlock()
	
	if !exists {
		// Try to fetch from ADX
		if err := m.fetchAndCacheAd(adID); err != nil {
			http.Error(w, "Ad not found", http.StatusNotFound)
			return
		}
		
		m.AdCache.mu.RLock()
		cached = m.AdCache.Items[adID]
		m.AdCache.mu.RUnlock()
	}
	
	// Update serve count
	cached.ServeCount++
	cached.LastServed = time.Now()
	
	// Set content type
	w.Header().Set("Content-Type", cached.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	
	// Serve the ad
	w.Write(cached.Creative)
	
	// Report impression
	go m.reportImpression(adID, r)
	
	// Update stats
	m.ServingStats.TotalServed++
	m.ServingStats.TotalBytes += uint64(len(cached.Creative))
}

// handleVAST serves VAST video ads
func (m *HomeMiner) handleVAST(w http.ResponseWriter, r *http.Request) {
	// Parse request parameters
	width := r.URL.Query().Get("w")
	height := r.URL.Query().Get("h")
	duration := r.URL.Query().Get("dur")
	
	// Get appropriate video ad from cache
	vastXML := m.getVASTAd(width, height, duration)
	
	if vastXML == "" {
		// Return empty VAST
		vastXML = `<?xml version="1.0" encoding="UTF-8"?>
<VAST version="4.0"></VAST>`
	}
	
	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(vastXML))
	
	m.ServingStats.TotalServed++
}

// handleHealth returns miner health status
func (m *HomeMiner) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"uptime":    time.Since(m.ServingStats.StartTime).String(),
		"served":    m.ServingStats.TotalServed,
		"cache_size": m.AdCache.CurrentSize,
		"public_url": m.PublicURL,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleMetrics returns performance metrics
func (m *HomeMiner) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"hardware": map[string]interface{}{
			"cpu_cores":   m.CPU.Cores,
			"gpu_available": m.GPU.Available,
			"memory_mb":   m.Memory.TotalMB,
			"storage_gb":  m.Storage.TotalGB,
		},
		"network": map[string]interface{}{
			"upload_mbps":   m.Network.UploadMbps,
			"download_mbps": m.Network.DownloadMbps,
			"latency_ms":    m.Network.LatencyMS,
		},
		"serving": map[string]interface{}{
			"total_requests": m.ServingStats.TotalRequests,
			"total_served":   m.ServingStats.TotalServed,
			"total_errors":   m.ServingStats.TotalErrors,
			"fill_rate":      m.ServingStats.FillRate,
			"current_qps":    m.ServingStats.CurrentQPS,
		},
		"earnings": map[string]interface{}{
			"total_earned":   m.Earnings.TotalEarned.String(),
			"pending_payout": m.Earnings.PendingPayout.String(),
			"cpm_rate":       m.Earnings.CPMRate.String(),
		},
		"location": map[string]interface{}{
			"country": m.Location.Country,
			"region":  m.Location.Region,
			"city":    m.Location.City,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// connectToADX establishes WebSocket connection
func (m *HomeMiner) connectToADX() error {
	// Create registration message
	reg := MinerRegistration{
		ID:            m.ID,
		WalletAddress: m.WalletAddress,
		PublicURL:     m.PublicURL,
		CPU:           m.CPU,
		GPU:           m.GPU,
		Memory:        m.Memory,
		Storage:       m.Storage,
		Network:       m.Network,
		Location:      m.Location,
	}
	
	// Connect to ADX WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(m.ADXEndpoint+"/miner/connect", nil)
	if err != nil {
		return err
	}
	
	m.WSConn = conn
	
	// Send registration
	if err := conn.WriteJSON(reg); err != nil {
		return err
	}
	
	// Start message handler
	go m.handleADXMessages()
	
	// Start heartbeat
	go m.sendHeartbeat()
	
	return nil
}

// MinerRegistration message
type MinerRegistration struct {
	ID            string
	WalletAddress string
	PublicURL     string
	CPU           CPUInfo
	GPU           GPUInfo
	Memory        MemoryInfo
	Storage       StorageInfo
	Network       NetworkInfo
	Location      LocationInfo
}

// handleADXMessages processes commands from ADX
func (m *HomeMiner) handleADXMessages() {
	for {
		var msg ADXMessage
		if err := m.WSConn.ReadJSON(&msg); err != nil {
			fmt.Printf("WebSocket error: %v\n", err)
			// Reconnect
			time.Sleep(5 * time.Second)
			m.connectToADX()
			return
		}
		
		switch msg.Type {
		case "cache_ad":
			go m.handleCacheCommand(msg)
		case "purge_ad":
			go m.handlePurgeCommand(msg)
		case "update_config":
			go m.handleConfigUpdate(msg)
		case "payout":
			go m.handlePayout(msg)
		}
	}
}

// ADXMessage from exchange
type ADXMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// fetchAndCacheAd downloads ad from ADX
func (m *HomeMiner) fetchAndCacheAd(adID string) error {
	// TODO: Implement ad fetching from ADX
	return nil
}

// reportImpression to ADX for payment
func (m *HomeMiner) reportImpression(adID string, r *http.Request) {
	// TODO: Report impression to ADX for earnings
}

// getVASTAd returns appropriate VAST XML
func (m *HomeMiner) getVASTAd(width, height, duration string) string {
	// TODO: Return cached VAST ad
	return ""
}

// collectMetrics continuously
func (m *HomeMiner) collectMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		// Calculate QPS
		// Update bandwidth usage
		// Check cache size
		// etc.
	}
}

// sendHeartbeat to ADX
func (m *HomeMiner) sendHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		heartbeat := map[string]interface{}{
			"type": "heartbeat",
			"timestamp": time.Now().Unix(),
			"stats": m.ServingStats,
		}
		
		if err := m.WSConn.WriteJSON(heartbeat); err != nil {
			fmt.Printf("Failed to send heartbeat: %v\n", err)
		}
	}
}

// handleCacheCommand caches new ad
func (m *HomeMiner) handleCacheCommand(msg ADXMessage) {
	// TODO: Parse and cache ad
}

// handlePurgeCommand removes ad from cache
func (m *HomeMiner) handlePurgeCommand(msg ADXMessage) {
	// TODO: Remove ad from cache
}

// handleConfigUpdate updates miner config
func (m *HomeMiner) handleConfigUpdate(msg ADXMessage) {
	// TODO: Update configuration
}

// handlePayout processes earnings payout
func (m *HomeMiner) handlePayout(msg ADXMessage) {
	// TODO: Update earnings record
}

// generateMinerID creates unique ID
func generateMinerID(wallet string) string {
	hash := sha256.Sum256([]byte(wallet + time.Now().String()))
	return hex.EncodeToString(hash[:])[:16]
}

// extractURLFromOutput parses tunnel URL from output
func extractURLFromOutput(output string) string {
	// TODO: Parse URL from tunnel service output
	return "https://example.localxpose.io"
}

// bytes package import
var bytes = struct {
	NewReader func([]byte) io.Reader
}{
	NewReader: func(b []byte) io.Reader {
		return &bytesReader{b: b}
	},
}

type bytesReader struct {
	b []byte
	i int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n = copy(p, r.b[r.i:])
	r.i += n
	return
}