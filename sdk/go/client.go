package adxsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/shopspring/decimal"
)

// Client is the ADX client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	wsConn     *websocket.Conn
}

// NewClient creates a new ADX client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// BidRequest sends an OpenRTB bid request
func (c *Client) BidRequest(ctx context.Context, request *openrtb2.BidRequest) (*openrtb2.BidResponse, error) {
	url := fmt.Sprintf("%s/rtb/bid", c.baseURL)
	
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bid request failed: %s", resp.Status)
	}
	
	var bidResponse openrtb2.BidResponse
	if err := json.NewDecoder(resp.Body).Decode(&bidResponse); err != nil {
		return nil, err
	}
	
	return &bidResponse, nil
}

// GetVAST retrieves a VAST ad
func (c *Client) GetVAST(ctx context.Context, params VASTParams) (string, error) {
	url := fmt.Sprintf("%s/vast?w=%d&h=%d&dur=%d", 
		c.baseURL, params.Width, params.Height, params.Duration)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	
	req.Header.Set("X-API-Key", c.apiKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("VAST request failed: %s", resp.Status)
	}
	
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	
	return buf.String(), nil
}

// ConnectWebSocket establishes a WebSocket connection for real-time updates
func (c *Client) ConnectWebSocket(ctx context.Context) error {
	wsURL := fmt.Sprintf("ws%s/ws", c.baseURL[4:]) // Convert http to ws
	
	header := http.Header{}
	header.Set("X-API-Key", c.apiKey)
	
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return err
	}
	
	c.wsConn = conn
	return nil
}

// Subscribe subscribes to real-time events
func (c *Client) Subscribe(events []string) error {
	if c.wsConn == nil {
		return fmt.Errorf("WebSocket not connected")
	}
	
	msg := map[string]interface{}{
		"type":   "subscribe",
		"events": events,
	}
	
	return c.wsConn.WriteJSON(msg)
}

// GetAnalytics retrieves analytics data
func (c *Client) GetAnalytics(ctx context.Context, params AnalyticsParams) (*AnalyticsResponse, error) {
	url := fmt.Sprintf("%s/analytics?publisher_id=%s&start=%s&end=%s",
		c.baseURL, params.PublisherID, 
		params.StartTime.Format(time.RFC3339),
		params.EndTime.Format(time.RFC3339))
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("X-API-Key", c.apiKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var analytics AnalyticsResponse
	if err := json.NewDecoder(resp.Body).Decode(&analytics); err != nil {
		return nil, err
	}
	
	return &analytics, nil
}

// RegisterMiner registers a home miner
func (c *Client) RegisterMiner(ctx context.Context, config MinerConfig) (*MinerRegistration, error) {
	url := fmt.Sprintf("%s/miner/register", c.baseURL)
	
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var registration MinerRegistration
	if err := json.NewDecoder(resp.Body).Decode(&registration); err != nil {
		return nil, err
	}
	
	return &registration, nil
}

// GetMinerEarnings retrieves miner earnings
func (c *Client) GetMinerEarnings(ctx context.Context, minerID string) (*MinerEarnings, error) {
	url := fmt.Sprintf("%s/miner/%s/earnings", c.baseURL, minerID)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("X-API-Key", c.apiKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var earnings MinerEarnings
	if err := json.NewDecoder(resp.Body).Decode(&earnings); err != nil {
		return nil, err
	}
	
	return &earnings, nil
}

// Close closes the client connections
func (c *Client) Close() error {
	if c.wsConn != nil {
		return c.wsConn.Close()
	}
	return nil
}

// Types

// VASTParams parameters for VAST request
type VASTParams struct {
	Width    int
	Height   int
	Duration int
}

// AnalyticsParams parameters for analytics query
type AnalyticsParams struct {
	PublisherID string
	StartTime   time.Time
	EndTime     time.Time
}

// AnalyticsResponse analytics data response
type AnalyticsResponse struct {
	PublisherID      string          `json:"publisher_id"`
	TotalImpressions uint64          `json:"total_impressions"`
	TotalRevenue     decimal.Decimal `json:"total_revenue"`
	FillRate         float64         `json:"fill_rate"`
	eCPM             decimal.Decimal `json:"ecpm"`
	TimeRange        struct {
		Start time.Time `json:"start"`
		End   time.Time `json:"end"`
	} `json:"time_range"`
	DailyStats []DailyStat `json:"daily_stats"`
}

// DailyStat daily statistics
type DailyStat struct {
	Date        string          `json:"date"`
	Impressions uint64          `json:"impressions"`
	Revenue     decimal.Decimal `json:"revenue"`
	FillRate    float64         `json:"fill_rate"`
}

// MinerConfig miner configuration
type MinerConfig struct {
	WalletAddress string `json:"wallet_address"`
	PublicURL     string `json:"public_url"`
	CacheSize     string `json:"cache_size"`
	Location      struct {
		Country string  `json:"country"`
		Region  string  `json:"region"`
		City    string  `json:"city"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
	} `json:"location"`
	Hardware struct {
		CPUCores    int `json:"cpu_cores"`
		MemoryGB    int `json:"memory_gb"`
		DiskGB      int `json:"disk_gb"`
		NetworkMbps int `json:"network_mbps"`
	} `json:"hardware"`
}

// MinerRegistration miner registration response
type MinerRegistration struct {
	MinerID       string    `json:"miner_id"`
	Status        string    `json:"status"`
	RegisteredAt  time.Time `json:"registered_at"`
	WebSocketURL  string    `json:"websocket_url"`
}

// MinerEarnings miner earnings data
type MinerEarnings struct {
	MinerID          string          `json:"miner_id"`
	TotalEarnings    decimal.Decimal `json:"total_earnings"`
	PendingPayout    decimal.Decimal `json:"pending_payout"`
	LastPayout       time.Time       `json:"last_payout"`
	TotalImpressions uint64          `json:"total_impressions"`
	TotalBandwidth   uint64          `json:"total_bandwidth"`
	Period           string          `json:"period"`
}