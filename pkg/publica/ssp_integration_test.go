package publica

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/luxfi/adx/pkg/analytics"
	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicaSSP_HandleBidRequest(t *testing.T) {
	// Create mock DSP server
	mockDSP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var bidReq openrtb2.BidRequest
		json.NewDecoder(r.Body).Decode(&bidReq)
		
		// Return mock bid response
		bidResp := openrtb2.BidResponse{
			ID: bidReq.ID,
			SeatBid: []openrtb2.SeatBid{{
				Bid: []openrtb2.Bid{{
					ID:    "bid-1",
					ImpID: bidReq.Imp[0].ID,
					Price: 5.50,
					AdM:   "<VAST>mock ad</VAST>",
				}},
			}},
			Cur: "USD",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bidResp)
	}))
	defer mockDSP.Close()

	// Create SSP with mock DSP
	ssp := &PublicaSSP{
		BidEndpoint: "/rtb/bid",
		DSPConnections: map[string]*DSPConfig{
			"test-dsp": {
				ID:       "test-dsp",
				Name:     "Test DSP",
				Endpoint: mockDSP.URL,
				Timeout:  100 * time.Millisecond,
				QPS:      100,
			},
		},
		Analytics: analytics.NewAnalyticsTracker(),
		FloorCPM:  decimal.NewFromFloat(1.0),
		Config: &Config{
			AuctionTimeout: 100 * time.Millisecond,
			MaxDSPs:        10,
		},
	}

	// Create bid request
	bidReq := &openrtb2.BidRequest{
		ID: "req-123",
		Imp: []openrtb2.Imp{{
			ID: "imp-1",
			Video: &openrtb2.Video{
				MIMEs:       []string{"video/mp4"},
				MinDuration: ptrInt64(5),
				MaxDuration: ptrInt64(30),
				W:           ptrInt64(1920),
				H:           ptrInt64(1080),
			},
			BidFloor: 2.0,
		}},
		Device: &openrtb2.Device{
			DeviceType: &openrtb2.DeviceType3, // CTV
		},
	}

	// Handle bid request
	ctx := context.Background()
	resp, err := ssp.HandleBidRequest(ctx, bidReq)
	
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "req-123", resp.ID)
	assert.Len(t, resp.SeatBid, 1)
	assert.Len(t, resp.SeatBid[0].Bid, 1)
	assert.Equal(t, 5.50, resp.SeatBid[0].Bid[0].Price)
}

func TestPublicaSSP_FilterDSPs(t *testing.T) {
	ssp := &PublicaSSP{
		DSPConnections: map[string]*DSPConfig{
			"premium-dsp": {
				ID:           "premium-dsp",
				Name:         "Premium DSP",
				TargetingCats: []string{"IAB1", "IAB2"},
				MinBidFloor:  decimal.NewFromFloat(5.0),
			},
			"standard-dsp": {
				ID:           "standard-dsp",
				Name:         "Standard DSP",
				TargetingCats: []string{"IAB3", "IAB4"},
				MinBidFloor:  decimal.NewFromFloat(1.0),
			},
		},
		Config: &Config{
			MaxDSPs: 10,
		},
	}

	bidReq := &openrtb2.BidRequest{
		ID: "req-456",
		Site: &openrtb2.Site{
			Cat: []string{"IAB1"},
		},
		Imp: []openrtb2.Imp{{
			BidFloor: 6.0,
		}},
	}

	dsps := ssp.filterDSPs(bidReq)
	
	assert.Len(t, dsps, 1)
	assert.Equal(t, "premium-dsp", dsps[0].ID)
}

func TestPublicaSSP_AssembleAdPod(t *testing.T) {
	ssp := &PublicaSSP{
		Analytics: analytics.NewAnalyticsTracker(),
	}

	bids := []openrtb2.Bid{
		{
			ID:    "bid-1",
			Price: 10.0,
			AdM:   "<VAST>ad1</VAST>",
			Dur:   15,
		},
		{
			ID:    "bid-2",
			Price: 8.0,
			AdM:   "<VAST>ad2</VAST>",
			Dur:   30,
		},
		{
			ID:    "bid-3",
			Price: 6.0,
			AdM:   "<VAST>ad3</VAST>",
			Dur:   15,
		},
	}

	pod := ssp.AssembleAdPod(bids, 60)
	
	assert.NotNil(t, pod)
	assert.Equal(t, "60", pod.MaxDuration)
	assert.Len(t, pod.Ads, 3)
	assert.Equal(t, 1, pod.Ads[0].Sequence)
	assert.Equal(t, 2, pod.Ads[1].Sequence)
	assert.Equal(t, 3, pod.Ads[2].Sequence)
}

func TestPublicaSSP_Metrics(t *testing.T) {
	ssp := &PublicaSSP{
		Analytics: analytics.NewAnalyticsTracker(),
	}

	// Track some metrics
	ssp.Analytics.TrackBidRequest("pub-123", "dsp-456")
	ssp.Analytics.TrackBidRequest("pub-123", "dsp-789")
	ssp.Analytics.TrackImpression("pub-123", decimal.NewFromFloat(5.50))
	ssp.Analytics.TrackImpression("pub-123", decimal.NewFromFloat(3.50))

	metrics := ssp.Analytics.GetMetrics()
	
	assert.Equal(t, uint64(2), metrics.TotalRequests)
	assert.Equal(t, uint64(2), metrics.TotalImpressions)
	assert.Equal(t, "9", metrics.TotalRevenue.String())
}

func TestDSPConfig_ShouldBid(t *testing.T) {
	tests := []struct {
		name     string
		dsp      *DSPConfig
		bidReq   *openrtb2.BidRequest
		expected bool
	}{
		{
			name: "meets all criteria",
			dsp: &DSPConfig{
				TargetingCats: []string{"IAB1"},
				MinBidFloor:   decimal.NewFromFloat(1.0),
			},
			bidReq: &openrtb2.BidRequest{
				Site: &openrtb2.Site{Cat: []string{"IAB1"}},
				Imp:  []openrtb2.Imp{{BidFloor: 2.0}},
			},
			expected: true,
		},
		{
			name: "floor too low",
			dsp: &DSPConfig{
				MinBidFloor: decimal.NewFromFloat(5.0),
			},
			bidReq: &openrtb2.BidRequest{
				Imp: []openrtb2.Imp{{BidFloor: 2.0}},
			},
			expected: false,
		},
		{
			name: "category mismatch",
			dsp: &DSPConfig{
				TargetingCats: []string{"IAB1"},
			},
			bidReq: &openrtb2.BidRequest{
				Site: &openrtb2.Site{Cat: []string{"IAB5"}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dsp.ShouldBid(tt.bidReq)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAdPod_Duration(t *testing.T) {
	pod := &AdPod{
		MaxDuration: "90",
		Ads: []Ad{
			{Duration: 15},
			{Duration: 30},
			{Duration: 15},
		},
	}

	totalDuration := 0
	for _, ad := range pod.Ads {
		totalDuration += ad.Duration
	}

	assert.Equal(t, 60, totalDuration)
	assert.LessOrEqual(t, totalDuration, 90)
}

// Helper functions
func ptrInt64(v int64) *int64 {
	return &v
}

func ptrString(v string) *string {
	return &v
}