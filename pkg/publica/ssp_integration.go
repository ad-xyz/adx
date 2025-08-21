package publica

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/luxfi/adx/pkg/analytics"
	"github.com/luxfi/adx/pkg/rtb"
	"github.com/luxfi/adx/pkg/vast"
	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/shopspring/decimal"
)

// PublicaSSP integrates with Publica's CTV SSP
type PublicaSSP struct {
	// Publica endpoints
	BidEndpoint   string
	VASTEndpoint  string
	ReportEndpoint string
	
	// Authentication
	PublisherID string
	APIKey      string
	
	// Configuration
	Timeout     time.Duration
	MaxQPS      int
	
	// Demand sources connected through ADX
	DSPConnections map[string]*DSPConfig
	
	// Analytics
	Analytics *analytics.AnalyticsTracker
	
	// Cache for pod assembly
	PodCache *PodCache
}

// DSPConfig represents a demand-side platform configuration
type DSPConfig struct {
	ID          string
	Name        string
	Endpoint    string
	BidderCode  string
	Priority    int
	MaxBid      decimal.Decimal
	Categories  []string // Content categories this DSP handles
	GeoTargets  []string // Geographic targets
	DeviceTypes []string // CTV, Mobile, Desktop
}

// NewPublicaSSP creates a new Publica SSP integration
func NewPublicaSSP(publisherID, apiKey string) *PublicaSSP {
	return &PublicaSSP{
		BidEndpoint:    "https://ssp.publica.com/openrtb2/bid",
		VASTEndpoint:   "https://ssp.publica.com/vast/4.0",
		ReportEndpoint: "https://analytics.publica.com/v1/events",
		PublisherID:    publisherID,
		APIKey:         apiKey,
		Timeout:        100 * time.Millisecond,
		MaxQPS:         1000,
		DSPConnections: make(map[string]*DSPConfig),
		Analytics:      analytics.NewAnalyticsTracker(),
		PodCache:       NewPodCache(1000), // Cache 1000 pods
	}
}

// AddDSP adds a demand-side platform to route bids through
func (p *PublicaSSP) AddDSP(config *DSPConfig) {
	p.DSPConnections[config.ID] = config
}

// HandleBidRequest processes incoming SSP bid requests and routes to DSPs
func (p *PublicaSSP) HandleBidRequest(ctx context.Context, request *openrtb2.BidRequest) (*openrtb2.BidResponse, error) {
	startTime := time.Now()
	
	// Track request
	p.Analytics.TrackRequest(request)
	
	// Enrich request with Publica-specific extensions
	enrichedRequest := p.enrichRequest(request)
	
	// Route to connected DSPs in parallel
	bidResponses := p.routeToDSPs(ctx, enrichedRequest)
	
	// Run auction to select winning bids
	winningBids := p.runAuction(bidResponses)
	
	// Assemble into ad pods for CTV
	podResponse := p.assemblePods(winningBids, request)
	
	// Track metrics
	p.Analytics.TrackResponse(podResponse, time.Since(startTime))
	
	return podResponse, nil
}

// enrichRequest adds Publica-specific data
func (p *PublicaSSP) enrichRequest(request *openrtb2.BidRequest) *openrtb2.BidRequest {
	// Add CTV-specific signals
	for i := range request.Imp {
		if request.Imp[i].Video != nil {
			// Add pod information
			ext := map[string]interface{}{
				"pod": map[string]interface{}{
					"id":       fmt.Sprintf("pod-%d", i),
					"sequence": i + 1,
					"maxseq":   len(request.Imp),
					"maxduration": 120, // 2 minute pod max
					"minduration": 15,  // 15 second minimum
				},
				"publica": map[string]interface{}{
					"publisher_id": p.PublisherID,
					"ctv_enabled":  true,
					"vast_version": "4.0",
				},
			}
			
			request.Imp[i].Ext, _ = json.Marshal(ext)
		}
	}
	
	return request
}

// routeToDSPs sends bid requests to all connected DSPs
func (p *PublicaSSP) routeToDSPs(ctx context.Context, request *openrtb2.BidRequest) []*openrtb2.BidResponse {
	responses := make([]*openrtb2.BidResponse, 0)
	respChan := make(chan *openrtb2.BidResponse, len(p.DSPConnections))
	
	// Send to each DSP in parallel
	for _, dsp := range p.DSPConnections {
		go func(d *DSPConfig) {
			resp, err := p.sendToDSP(ctx, d, request)
			if err == nil && resp != nil {
				respChan <- resp
			}
		}(dsp)
	}
	
	// Collect responses with timeout
	timeout := time.After(p.Timeout)
	for i := 0; i < len(p.DSPConnections); i++ {
		select {
		case resp := <-respChan:
			responses = append(responses, resp)
		case <-timeout:
			break
		}
	}
	
	return responses
}

// sendToDSP sends request to a specific DSP
func (p *PublicaSSP) sendToDSP(ctx context.Context, dsp *DSPConfig, request *openrtb2.BidRequest) (*openrtb2.BidResponse, error) {
	// In production, would make actual HTTP request
	// For now, return mock response
	return &openrtb2.BidResponse{
		ID:      request.ID,
		BidID:   fmt.Sprintf("bid-%s-%d", dsp.ID, time.Now().Unix()),
		SeatBid: []openrtb2.SeatBid{
			{
				Seat: dsp.BidderCode,
				Bid: []openrtb2.Bid{
					{
						ID:    fmt.Sprintf("%s-bid-1", dsp.ID),
						ImpID: request.Imp[0].ID,
						Price: 5.50,
						AdM:   "<VAST>...</VAST>",
						CrID:  "creative-123",
					},
				},
			},
		},
	}, nil
}

// runAuction selects winning bids
func (p *PublicaSSP) runAuction(responses []*openrtb2.BidResponse) []openrtb2.Bid {
	allBids := make([]openrtb2.Bid, 0)
	
	for _, resp := range responses {
		for _, seatBid := range resp.SeatBid {
			allBids = append(allBids, seatBid.Bid...)
		}
	}
	
	// Sort by price (simple first-price auction)
	// In production, would implement more sophisticated auction logic
	return allBids
}

// assemblePods creates ad pods for CTV
func (p *PublicaSSP) assemblePods(bids []openrtb2.Bid, request *openrtb2.BidRequest) *openrtb2.BidResponse {
	response := &openrtb2.BidResponse{
		ID:    request.ID,
		BidID: fmt.Sprintf("pod-%d", time.Now().Unix()),
	}
	
	// Group bids into pods
	podSize := 6 // Standard CTV pod size
	for i := 0; i < len(bids); i += podSize {
		end := i + podSize
		if end > len(bids) {
			end = len(bids)
		}
		
		seatBid := openrtb2.SeatBid{
			Seat: "publica-pod",
			Bid:  bids[i:end],
		}
		
		response.SeatBid = append(response.SeatBid, seatBid)
	}
	
	return response
}

// GenerateVAST creates VAST 4.0 response for CTV
func (p *PublicaSSP) GenerateVAST(podID string, ads []openrtb2.Bid) (*vast.VAST, error) {
	vastResponse := &vast.VAST{
		Version: "4.0",
		Ads:     make([]vast.Ad, 0),
	}
	
	for i, bid := range ads {
		ad := vast.Ad{
			ID:       fmt.Sprintf("ad-%s-%d", podID, i),
			Sequence: i + 1,
			InLine: &vast.InLine{
				AdSystem: vast.AdSystem{
					Name:    "ADX-Publica",
					Version: "1.0",
				},
				AdTitle:     fmt.Sprintf("CTV Ad %d", i+1),
				Description: "Connected TV Advertisement",
				Impression: []vast.Impression{
					{
						ID:  fmt.Sprintf("imp-%s-%d", podID, i),
						URL: fmt.Sprintf("%s?pod=%s&ad=%d&price=%.2f", p.ReportEndpoint, podID, i, bid.Price),
					},
				},
				Creatives: vast.Creatives{
					Creative: []vast.Creative{
						{
							ID:       bid.CrID,
							Sequence: i + 1,
							Linear: &vast.Linear{
								Duration: "00:00:15",
								MediaFiles: vast.MediaFiles{
									MediaFile: []vast.MediaFile{
										{
											Delivery: "progressive",
											Type:     "video/mp4",
											Width:    1920,
											Height:   1080,
											Bitrate:  4000,
											URL:      fmt.Sprintf("https://cdn.adx.com/video/%s.mp4", bid.CrID),
										},
									},
								},
								TrackingEvents: &vast.TrackingEvents{
									Tracking: []vast.Tracking{
										{Event: "start", URL: fmt.Sprintf("%s?event=start&creative=%s", p.ReportEndpoint, bid.CrID)},
										{Event: "firstQuartile", URL: fmt.Sprintf("%s?event=q1&creative=%s", p.ReportEndpoint, bid.CrID)},
										{Event: "midpoint", URL: fmt.Sprintf("%s?event=mid&creative=%s", p.ReportEndpoint, bid.CrID)},
										{Event: "thirdQuartile", URL: fmt.Sprintf("%s?event=q3&creative=%s", p.ReportEndpoint, bid.CrID)},
										{Event: "complete", URL: fmt.Sprintf("%s?event=complete&creative=%s", p.ReportEndpoint, bid.CrID)},
									},
								},
							},
						},
					},
				},
			},
		}
		
		vastResponse.Ads = append(vastResponse.Ads, ad)
	}
	
	return vastResponse, nil
}

// PodCache caches assembled ad pods
type PodCache struct {
	capacity int
	pods     map[string]*CachedPod
}

// CachedPod represents a cached ad pod
type CachedPod struct {
	ID        string
	VAST      *vast.VAST
	CreatedAt time.Time
	TTL       time.Duration
}

// NewPodCache creates a new pod cache
func NewPodCache(capacity int) *PodCache {
	return &PodCache{
		capacity: capacity,
		pods:     make(map[string]*CachedPod),
	}
}

// DSPRouter manages demand-side platform connections
type DSPRouter struct {
	DSPs       map[string]*rtb.DSPConnection
	Priorities map[string]int
	Filters    map[string]*DSPFilter
}

// DSPFilter determines which DSPs to use
type DSPFilter struct {
	GeoTargets    []string
	ContentTypes  []string
	MinCPM        decimal.Decimal
	MaxCPM        decimal.Decimal
	BlockedDomains []string
}

// Route selects appropriate DSPs for a request
func (r *DSPRouter) Route(request *openrtb2.BidRequest) []*rtb.DSPConnection {
	selected := make([]*rtb.DSPConnection, 0)
	
	for id, dsp := range r.DSPs {
		if filter, ok := r.Filters[id]; ok {
			if r.matchesFilter(request, filter) {
				selected = append(selected, dsp)
			}
		}
	}
	
	return selected
}

// matchesFilter checks if request matches DSP filter criteria
func (r *DSPRouter) matchesFilter(request *openrtb2.BidRequest, filter *DSPFilter) bool {
	// Check geo targeting
	if request.Device != nil && request.Device.Geo != nil {
		// Match country code
		for _, target := range filter.GeoTargets {
			if request.Device.Geo.Country == target {
				return true
			}
		}
	}
	
	// Check price floors
	for _, imp := range request.Imp {
		if imp.BidFloor < float64(filter.MinCPM.InexactFloat64()) ||
		   imp.BidFloor > float64(filter.MaxCPM.InexactFloat64()) {
			return false
		}
	}
	
	return true
}