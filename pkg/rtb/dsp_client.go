package rtb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prebid/openrtb/v20/openrtb2"
)

// DSPClientFactory creates DSP clients for different protocols
type DSPClientFactory struct {
	HTTPClient *http.Client
}

// NewDSPClientFactory creates a new DSP client factory
func NewDSPClientFactory() *DSPClientFactory {
	return &DSPClientFactory{
		HTTPClient: &http.Client{
			Timeout: 100 * time.Millisecond, // Strict timeout for RTB
			Transport: &http.Transport{
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  true, // Faster for small payloads
			},
		},
	}
}

// CreateClient creates a DSP client based on the connection config
func (f *DSPClientFactory) CreateClient(conn *DSPConnection) DSPClient {
	switch conn.Protocol {
	case "prebid":
		return &PrebidDSPClient{
			conn:       conn,
			httpClient: f.HTTPClient,
		}
	case "amazon_uap":
		return &AmazonUAPClient{
			conn:       conn,
			httpClient: f.HTTPClient,
		}
	case "google_adx":
		return &GoogleADXClient{
			conn:       conn,
			httpClient: f.HTTPClient,
		}
	case "thetradedesk":
		return &TradeDeskClient{
			conn:       conn,
			httpClient: f.HTTPClient,
		}
	default:
		return &StandardDSPClient{
			conn:       conn,
			httpClient: f.HTTPClient,
		}
	}
}

// DSPClient interface for different DSP protocols
type DSPClient interface {
	SendBidRequest(ctx context.Context, req *openrtb2.BidRequest) (*Bid, error)
	NotifyWin(ctx context.Context, bid *Bid, clearingPrice float64) error
	NotifyLoss(ctx context.Context, bid *Bid, reason int) error
}

// StandardDSPClient implements standard OpenRTB 2.5 protocol
type StandardDSPClient struct {
	conn       *DSPConnection
	httpClient *http.Client
}

// SendBidRequest sends OpenRTB bid request
func (c *StandardDSPClient) SendBidRequest(ctx context.Context, req *openrtb2.BidRequest) (*Bid, error) {
	// Add ADXYZ extensions if supported
	if c.conn.SupportsADXYZ {
		ext := map[string]interface{}{
			"adxyz": map[string]interface{}{
				"settlement":       "ausd",
				"escrow_enabled":   true,
				"ttl_ms":           2000,
				"delivery_proof":   "required",
				"wallet":           c.conn.AUSDWallet,
				"min_quality_score": 0.85,
			},
		}
		extBytes, _ := json.Marshal(ext)
		req.Ext = extBytes
	}

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.conn.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-openrtb-version", "2.5")
	if c.conn.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer " + c.conn.APIKey)
	}

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.conn.Stats.RequestErrors++
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle no-bid
	if resp.StatusCode == http.StatusNoContent {
		c.conn.Stats.NoBids++
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		c.conn.Stats.RequestErrors++
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// Parse response
	var bidResp openrtb2.BidResponse
	if err := json.NewDecoder(resp.Body).Decode(&bidResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract bid
	if len(bidResp.SeatBid) == 0 || len(bidResp.SeatBid[0].Bid) == 0 {
		c.conn.Stats.NoBids++
		return nil, nil
	}

	return c.convertToBid(&bidResp.SeatBid[0].Bid[0]), nil
}

// NotifyWin notifies DSP of winning bid
func (c *StandardDSPClient) NotifyWin(ctx context.Context, bid *Bid, clearingPrice float64) error {
	if bid.WinURL == "" {
		return nil
	}

	// Macro substitution
	winURL := bid.WinURL
	winURL = replaceMacro(winURL, "${AUCTION_PRICE}", fmt.Sprintf("%.2f", clearingPrice))
	winURL = replaceMacro(winURL, "${AUCTION_ID}", bid.ID)

	req, err := http.NewRequestWithContext(ctx, "GET", winURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	c.conn.Stats.WinsNotified++
	return nil
}

// NotifyLoss notifies DSP of lost bid
func (c *StandardDSPClient) NotifyLoss(ctx context.Context, bid *Bid, reason int) error {
	if bid.LossURL == "" {
		return nil
	}

	lossURL := bid.LossURL
	lossURL = replaceMacro(lossURL, "${LOSS_REASON}", fmt.Sprintf("%d", reason))

	req, err := http.NewRequestWithContext(ctx, "GET", lossURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// convertToBid converts OpenRTB bid to internal Bid
func (c *StandardDSPClient) convertToBid(ortbBid *openrtb2.Bid) *Bid {
	bid := &Bid{
		ID:            ortbBid.ID,
		ImpID:         ortbBid.ImpID,
		Price:         ortbBid.Price,
		AdID:          ortbBid.AdID,
		CrID:          ortbBid.CrID,
		DealID:        ortbBid.DealID,
		DSPID:         c.conn.ID,
		SeatID:        c.conn.SeatID,
		Timestamp:     time.Now(),
		CreativeURL:   ortbBid.NURL,
		WinURL:        ortbBid.NURL,
		LossURL:       ortbBid.LURL,
		Width:         &ortbBid.W,
		Height:        &ortbBid.H,
		Categories:    ortbBid.Cat,
		AdvertiserDom: ortbBid.ADomain,
	}

	// Check for ADXYZ confirmation
	if ortbBid.Ext != nil {
		var ext map[string]interface{}
		if err := json.Unmarshal(ortbBid.Ext, &ext); err == nil {
			if adxyz, ok := ext["adxyz"].(map[string]interface{}); ok {
				bid.ADXYZEnabled = adxyz["confirmed"] == true
				if escrowID, ok := adxyz["escrow_id"].(string); ok {
					bid.EscrowID = escrowID
				}
			}
		}
	}

	// Extract VAST
	if ortbBid.AdM != "" {
		bid.VAST = ortbBid.AdM
	}

	return bid
}

// PrebidDSPClient implements Prebid-specific protocol
type PrebidDSPClient struct {
	conn       *DSPConnection
	httpClient *http.Client
}

// SendBidRequest sends Prebid-formatted request
func (c *PrebidDSPClient) SendBidRequest(ctx context.Context, req *openrtb2.BidRequest) (*Bid, error) {
	// Add Prebid-specific extensions
	if req.Ext == nil {
		req.Ext = json.RawMessage(`{}`)
	}

	var ext map[string]interface{}
	json.Unmarshal(req.Ext, &ext)
	
	ext["prebid"] = map[string]interface{}{
		"bidder": c.conn.BidderCode,
		"is_debug": false,
		"aliases": map[string]string{
			"adxyz": "adxyz-prebid-adapter",
		},
	}

	if c.conn.SupportsADXYZ {
		ext["adxyz"] = map[string]interface{}{
			"settlement": "ausd",
			"escrow":     true,
		}
	}

	extBytes, _ := json.Marshal(ext)
	req.Ext = extBytes

	// Use standard client for actual request
	stdClient := &StandardDSPClient{
		conn:       c.conn,
		httpClient: c.httpClient,
	}

	return stdClient.SendBidRequest(ctx, req)
}

// NotifyWin for Prebid
func (c *PrebidDSPClient) NotifyWin(ctx context.Context, bid *Bid, clearingPrice float64) error {
	stdClient := &StandardDSPClient{
		conn:       c.conn,
		httpClient: c.httpClient,
	}
	return stdClient.NotifyWin(ctx, bid, clearingPrice)
}

// NotifyLoss for Prebid
func (c *PrebidDSPClient) NotifyLoss(ctx context.Context, bid *Bid, reason int) error {
	stdClient := &StandardDSPClient{
		conn:       c.conn,
		httpClient: c.httpClient,
	}
	return stdClient.NotifyLoss(ctx, bid, reason)
}

// AmazonUAPClient implements Amazon UAP protocol
type AmazonUAPClient struct {
	conn       *DSPConnection
	httpClient *http.Client
}

// SendBidRequest for Amazon UAP
func (c *AmazonUAPClient) SendBidRequest(ctx context.Context, req *openrtb2.BidRequest) (*Bid, error) {
	// Transform to Amazon UAP format
	uapReq := map[string]interface{}{
		"id":        req.ID,
		"imp":       req.Imp,
		"site":      req.Site,
		"device":    req.Device,
		"user":      req.User,
		"at":        2, // First price auction
		"tmax":      req.TMax,
		"cur":       req.Cur,
		"source":    map[string]interface{}{"tid": req.ID},
		"regs":      req.Regs,
		"ext": map[string]interface{}{
			"aps": map[string]interface{}{
				"sd": "1234", // Slot ID
				"ac": "12345", // Account ID
			},
		},
	}

	// Add ADXYZ if supported
	if c.conn.SupportsADXYZ {
		uapReq["ext"].(map[string]interface{})["adxyz"] = map[string]interface{}{
			"enabled": true,
		}
	}

	reqBody, _ := json.Marshal(uapReq)
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.conn.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-amzn-request-id", req.ID)
	if c.conn.APIKey != "" {
		httpReq.Header.Set("x-api-key", c.conn.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("UAP error: %d", resp.StatusCode)
	}

	// Parse UAP response
	var uapResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&uapResp); err != nil {
		return nil, err
	}

	// Convert to standard bid
	return c.convertUAPToBid(uapResp), nil
}

// convertUAPToBid converts Amazon UAP response to Bid
func (c *AmazonUAPClient) convertUAPToBid(uapResp map[string]interface{}) *Bid {
	// Extract bid from UAP response structure
	seatbid := uapResp["seatbid"].([]interface{})
	if len(seatbid) == 0 {
		return nil
	}

	seat := seatbid[0].(map[string]interface{})
	bids := seat["bid"].([]interface{})
	if len(bids) == 0 {
		return nil
	}

	uapBid := bids[0].(map[string]interface{})

	return &Bid{
		ID:          uapBid["id"].(string),
		ImpID:       uapBid["impid"].(string),
		Price:       uapBid["price"].(float64),
		AdID:        uapBid["adid"].(string),
		DSPID:       c.conn.ID,
		Timestamp:   time.Now(),
	}
}

// NotifyWin for Amazon UAP
func (c *AmazonUAPClient) NotifyWin(ctx context.Context, bid *Bid, clearingPrice float64) error {
	// Amazon handles win notification through their reporting API
	return nil
}

// NotifyLoss for Amazon UAP
func (c *AmazonUAPClient) NotifyLoss(ctx context.Context, bid *Bid, reason int) error {
	// Amazon handles loss notification through their reporting API
	return nil
}

// GoogleADXClient implements Google Ad Exchange protocol
type GoogleADXClient struct {
	conn       *DSPConnection
	httpClient *http.Client
}

// SendBidRequest for Google ADX
func (c *GoogleADXClient) SendBidRequest(ctx context.Context, req *openrtb2.BidRequest) (*Bid, error) {
	// Google ADX uses Protocol Buffers, simplified to JSON here
	adxReq := map[string]interface{}{
		"id":          req.ID,
		"is_test":     req.Test,
		"impression":  c.convertToADXImpressions(req.Imp),
		"site":        req.Site,
		"device":      req.Device,
		"user":        req.User,
		"at":          2,
		"tmax":        req.TMax,
		"cur":         req.Cur,
		"bcat":        req.BCat,
		"badv":        req.BAdv,
	}

	reqBody, _ := json.Marshal(adxReq)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.conn.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.conn.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer " + c.conn.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ADX error: %d", resp.StatusCode)
	}

	var adxResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&adxResp); err != nil {
		return nil, err
	}

	return c.convertADXToBid(adxResp), nil
}

// convertToADXImpressions converts OpenRTB impressions to ADX format
func (c *GoogleADXClient) convertToADXImpressions(imps []openrtb2.Imp) []map[string]interface{} {
	adxImps := make([]map[string]interface{}, len(imps))
	for i, imp := range imps {
		adxImps[i] = map[string]interface{}{
			"id":          imp.ID,
			"banner":      imp.Banner,
			"video":       imp.Video,
			"native":      imp.Native,
			"bidfloor":    imp.BidFloor,
			"bidfloorcur": imp.BidFloorCur,
			"secure":      imp.Secure,
		}
	}
	return adxImps
}

// convertADXToBid converts Google ADX response to Bid
func (c *GoogleADXClient) convertADXToBid(adxResp map[string]interface{}) *Bid {
	// Parse ADX response format
	if adxResp["no_bid"] == true {
		return nil
	}

	adGroups := adxResp["ad_groups"].([]interface{})
	if len(adGroups) == 0 {
		return nil
	}

	adGroup := adGroups[0].(map[string]interface{})

	return &Bid{
		ID:        adGroup["id"].(string),
		Price:     adGroup["max_cpm_micros"].(float64) / 1000000.0, // Convert micros to dollars
		DSPID:     c.conn.ID,
		Timestamp: time.Now(),
	}
}

// NotifyWin for Google ADX
func (c *GoogleADXClient) NotifyWin(ctx context.Context, bid *Bid, clearingPrice float64) error {
	// Google handles through their reporting API
	return nil
}

// NotifyLoss for Google ADX
func (c *GoogleADXClient) NotifyLoss(ctx context.Context, bid *Bid, reason int) error {
	// Google handles through their reporting API
	return nil
}

// TradeDeskClient implements The Trade Desk protocol
type TradeDeskClient struct {
	conn       *DSPConnection
	httpClient *http.Client
}

// SendBidRequest for The Trade Desk
func (c *TradeDeskClient) SendBidRequest(ctx context.Context, req *openrtb2.BidRequest) (*Bid, error) {
	// TTD uses standard OpenRTB with custom extensions
	if req.Ext == nil {
		req.Ext = json.RawMessage(`{}`)
	}

	var ext map[string]interface{}
	json.Unmarshal(req.Ext, &ext)

	ext["ttd"] = map[string]interface{}{
		"partner_id": c.conn.BidderCode,
		"site_id":    "adxyz",
	}

	if c.conn.SupportsADXYZ {
		ext["adxyz"] = map[string]interface{}{
			"enabled":     true,
			"settlement":  "ausd",
			"escrow":      true,
			"quality_req": 0.9,
		}
	}

	extBytes, _ := json.Marshal(ext)
	req.Ext = extBytes

	// Use standard client
	stdClient := &StandardDSPClient{
		conn:       c.conn,
		httpClient: c.httpClient,
	}

	return stdClient.SendBidRequest(ctx, req)
}

// NotifyWin for The Trade Desk
func (c *TradeDeskClient) NotifyWin(ctx context.Context, bid *Bid, clearingPrice float64) error {
	// TTD-specific win notification
	winData := map[string]interface{}{
		"bid_id":         bid.ID,
		"clearing_price": clearingPrice,
		"timestamp":      time.Now().Unix(),
		"partner_id":     c.conn.BidderCode,
	}

	reqBody, _ := json.Marshal(winData)

	req, err := http.NewRequestWithContext(ctx, "POST", 
		fmt.Sprintf("%s/win", c.conn.Endpoint), bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.conn.APIKey != "" {
		req.Header.Set("X-TTD-ApiKey", c.conn.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// NotifyLoss for The Trade Desk
func (c *TradeDeskClient) NotifyLoss(ctx context.Context, bid *Bid, reason int) error {
	// TTD loss notification
	lossData := map[string]interface{}{
		"bid_id":     bid.ID,
		"loss_reason": reason,
		"timestamp":  time.Now().Unix(),
	}

	reqBody, _ := json.Marshal(lossData)

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/loss", c.conn.Endpoint), bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.conn.APIKey != "" {
		req.Header.Set("X-TTD-ApiKey", c.conn.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// replaceMacro replaces macro in URL
func replaceMacro(url, macro, value string) string {
	return string(bytes.ReplaceAll([]byte(url), []byte(macro), []byte(value)))
}

// DSPProtocol enum
const (
	ProtocolOpenRTB    = "openrtb"
	ProtocolPrebid     = "prebid"
	ProtocolAmazonUAP  = "amazon_uap"
	ProtocolGoogleADX  = "google_adx"
	ProtocolTradeDesk  = "thetradedesk"
)