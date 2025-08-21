package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/luxfi/adx/pkg/analytics"
	"github.com/luxfi/adx/pkg/publica"
	"github.com/luxfi/adx/pkg/rtb"
	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/shopspring/decimal"
)

// Example: Complete Publica SSP Integration with ADX
func main() {
	// Initialize analytics tracking
	tracker := analytics.NewAnalyticsTracker()
	
	// Start analytics event processor
	go processAnalyticsEvents(tracker)
	
	// Initialize Publica SSP integration
	publicaSSP := publica.NewPublicaSSP("pub-123456", "api-key-secret")
	publicaSSP.Analytics = tracker
	
	// Configure DSP connections (demand sources)
	configureDSPs(publicaSSP)
	
	// Initialize RTB exchange
	_ = initializeExchange(tracker)
	
	// Example 1: Handle CTV bid request from Publica
	handleCTVBidRequest(publicaSSP)
	
	// Example 2: Assemble ad pod for streaming break
	assembleAdPod(publicaSSP)
	
	// Example 3: Track analytics and generate reports
	generateAnalyticsReport(tracker)
	
	// Example 4: Miner earnings calculation
	calculateMinerEarnings(tracker)
	
	log.Println("ADX-Publica integration example completed")
}

// configureDSPs sets up demand-side platform connections
func configureDSPs(ssp *publica.PublicaSSP) {
	// Add premium CTV DSP
	ssp.AddDSP(&publica.DSPConfig{
		ID:          "dsp-ctv-premium",
		Name:        "Premium CTV Network",
		Endpoint:    "https://dsp.premium-ctv.com/bid",
		BidderCode:  "premium",
		Priority:    1,
		MaxBid:      decimal.NewFromFloat(50.0), // $50 CPM max
		Categories:  []string{"IAB1", "IAB2", "IAB3"}, // Entertainment, Auto, Business
		GeoTargets:  []string{"US", "CA", "GB"},
		DeviceTypes: []string{"CTV"},
	})
	
	// Add programmatic DSP
	ssp.AddDSP(&publica.DSPConfig{
		ID:          "dsp-programmatic",
		Name:        "Programmatic Exchange",
		Endpoint:    "https://rtb.programmatic.com/bid",
		BidderCode:  "prog",
		Priority:    2,
		MaxBid:      decimal.NewFromFloat(30.0), // $30 CPM max
		Categories:  []string{"IAB1", "IAB4", "IAB5"},
		GeoTargets:  []string{"US"},
		DeviceTypes: []string{"CTV", "Mobile", "Desktop"},
	})
	
	// Add performance DSP
	ssp.AddDSP(&publica.DSPConfig{
		ID:          "dsp-performance",
		Name:        "Performance Marketing DSP",
		Endpoint:    "https://perf.dsp.com/bid",
		BidderCode:  "perf",
		Priority:    3,
		MaxBid:      decimal.NewFromFloat(20.0), // $20 CPM max
		Categories:  []string{"IAB22"}, // Shopping
		GeoTargets:  []string{"US", "CA"},
		DeviceTypes: []string{"CTV", "Mobile"},
	})
	
	log.Printf("Configured %d DSP connections", len(ssp.DSPConnections))
}

// initializeExchange sets up the RTB exchange
func initializeExchange(tracker *analytics.AnalyticsTracker) *rtb.RTBExchange {
	exchange := &rtb.RTBExchange{
		DSPs:           make(map[string]*rtb.DSPConnection),
		SSPs:           make(map[string]*rtb.SSPConnection),
		AuctionTimeout: 100 * time.Millisecond,
		FloorPrice:     decimal.NewFromFloat(2.0), // $2 CPM floor
		CTVOptimizer: &rtb.CTVOptimizer{
			PublicaEnabled:            true,
			PodFillRate:               0.85,
			OptimalPodSize:            6,
			CreativeQualityThreshold: 0.8,
			MinVideoBitrate:          4000000, // 4 Mbps for CTV
			RequiredFormats:          []string{"video/mp4", "video/webm"},
		},
		PodAssembler: &rtb.AdPodAssembler{
			MaxPodDuration:        120 * time.Second,
			MinPodDuration:        15 * time.Second,
			TargetCPM:             decimal.NewFromFloat(25.0),
			CompetitiveSeparation: true,
			FrequencyCapping:      true,
			CreativeDeduping:      true,
		},
		MinerRegistry: &rtb.MinerRegistry{
			Miners: make(map[string]*rtb.HomeMiner),
		},
	}
	
	return exchange
}

// handleCTVBidRequest demonstrates handling a CTV bid request
func handleCTVBidRequest(ssp *publica.PublicaSSP) {
	log.Println("\n=== Handling CTV Bid Request ===")
	
	// Create sample CTV bid request
	bidRequest := &openrtb2.BidRequest{
		ID: fmt.Sprintf("bid-%d", time.Now().Unix()),
		Imp: []openrtb2.Imp{
			{
				ID: "imp-1",
				Video: &openrtb2.Video{
					W:           ptrInt64(1920),
					H:           ptrInt64(1080),
					MinDuration: 15,
					MaxDuration: 30,
					MIMEs:       []string{"video/mp4"},
					// Protocols: VAST 2.0, 3.0, 4.0, 4.1
				},
				BidFloor: 5.0, // $5 CPM floor
			},
			{
				ID: "imp-2",
				Video: &openrtb2.Video{
					W:           ptrInt64(1920),
					H:           ptrInt64(1080),
					MinDuration: 15,
					MaxDuration: 30,
					MIMEs:       []string{"video/mp4"},
				},
				BidFloor: 5.0,
			},
		},
		Site: &openrtb2.Site{
			Publisher: &openrtb2.Publisher{
				ID:   "pub-123456",
				Name: "Premium Streaming Service",
			},
			Content: &openrtb2.Content{
				Title:   "Premium Show",
				Series:  "Hit Series",
				Season:  "1",
				Genre:   "Drama",
			},
		},
		Device: &openrtb2.Device{
			DeviceType: 3, // Connected TV
			Make:       "Samsung",
			Model:      "Smart TV",
			OS:         "Tizen",
			IP:         "192.168.1.100",
			Geo: &openrtb2.Geo{
				Country: "US",
				Region:  "CA",
				City:    "Los Angeles",
				ZIP:     "90001",
			},
		},
		User: &openrtb2.User{
			ID: "user-abc-123",
		},
	}
	
	// Process bid request through Publica SSP
	ctx := context.Background()
	response, err := ssp.HandleBidRequest(ctx, bidRequest)
	if err != nil {
		log.Printf("Error handling bid request: %v", err)
		return
	}
	
	// Log results
	if response != nil && len(response.SeatBid) > 0 {
		totalBids := 0
		totalValue := 0.0
		
		for _, seatBid := range response.SeatBid {
			for _, bid := range seatBid.Bid {
				totalBids++
				totalValue += bid.Price
				log.Printf("  Bid %s: $%.2f CPM from %s", bid.ID, bid.Price, seatBid.Seat)
			}
		}
		
		log.Printf("Received %d bids totaling $%.2f CPM", totalBids, totalValue)
	}
}

// assembleAdPod demonstrates creating an ad pod for CTV
func assembleAdPod(ssp *publica.PublicaSSP) {
	log.Println("\n=== Assembling Ad Pod for CTV ===")
	
	// Create sample bids for pod assembly
	bids := []openrtb2.Bid{
		{ID: "ad1", ImpID: "imp1", Price: 25.50, CrID: "creative-1", AdM: "<VAST>...</VAST>"},
		{ID: "ad2", ImpID: "imp2", Price: 22.00, CrID: "creative-2", AdM: "<VAST>...</VAST>"},
		{ID: "ad3", ImpID: "imp3", Price: 20.00, CrID: "creative-3", AdM: "<VAST>...</VAST>"},
		{ID: "ad4", ImpID: "imp4", Price: 18.50, CrID: "creative-4", AdM: "<VAST>...</VAST>"},
		{ID: "ad5", ImpID: "imp5", Price: 15.00, CrID: "creative-5", AdM: "<VAST>...</VAST>"},
		{ID: "ad6", ImpID: "imp6", Price: 12.00, CrID: "creative-6", AdM: "<VAST>...</VAST>"},
	}
	
	// Generate VAST response for the pod
	podID := fmt.Sprintf("pod-%d", time.Now().Unix())
	vast, err := ssp.GenerateVAST(podID, bids)
	if err != nil {
		log.Printf("Error generating VAST: %v", err)
		return
	}
	
	log.Printf("Generated VAST 4.0 pod with %d ads", len(vast.Ads))
	log.Printf("Pod ID: %s", podID)
	log.Printf("Total pod value: $%.2f CPM", calculatePodValue(bids))
	
	// Track pod metrics
	ssp.Analytics.TrackPodMetrics(podID, len(bids), true)
}

// generateAnalyticsReport shows analytics capabilities
func generateAnalyticsReport(tracker *analytics.AnalyticsTracker) {
	log.Println("\n=== Analytics Report ===")
	
	// Simulate some activity
	simulateActivity(tracker)
	
	// Get real-time metrics
	metrics := tracker.GetRealTimeMetrics()
	
	log.Println("Real-Time Metrics:")
	log.Printf("  Total Requests: %v", metrics["total_requests"])
	log.Printf("  Total Impressions: %v", metrics["total_impressions"])
	log.Printf("  Total Revenue: $%.2f", metrics["total_revenue"])
	log.Printf("  Fill Rate: %.2f%%", metrics["fill_rate"].(float64)*100)
	log.Printf("  Avg Latency: %.2f ms", metrics["avg_latency_ms"])
	
	podMetrics := metrics["pod_metrics"].(map[string]interface{})
	log.Println("Pod Metrics:")
	log.Printf("  Total Pods: %v", podMetrics["total_pods"])
	log.Printf("  Avg Pod Size: %v", podMetrics["avg_pod_size"])
	log.Printf("  Completion Rate: %.2f%%", podMetrics["completion_rate"].(float64)*100)
	
	// Export Prometheus metrics
	prometheusMetrics := tracker.ExportMetrics()
	log.Println("\nPrometheus Metrics (sample):")
	fmt.Println(prometheusMetrics[:500] + "...")
}

// calculateMinerEarnings demonstrates miner earnings calculation
func calculateMinerEarnings(tracker *analytics.AnalyticsTracker) {
	log.Println("\n=== Miner Earnings Report ===")
	
	// Simulate miner activity
	minerID := "miner-home-123"
	for i := 0; i < 100; i++ {
		tracker.TrackImpression(
			fmt.Sprintf("imp-%d", i),
			"pub-123",
			minerID,
			decimal.NewFromFloat(5.0), // $5 CPM
		)
	}
	
	// Calculate earnings (10% of impression value goes to miner)
	totalImpressions := 100
	avgCPM := 5.0
	minerShare := 0.10 // 10%
	
	dailyEarnings := float64(totalImpressions) * avgCPM / 1000.0 * minerShare
	monthlyEarnings := dailyEarnings * 30
	
	log.Printf("Miner ID: %s", minerID)
	log.Printf("Impressions Served: %d", totalImpressions)
	log.Printf("Average CPM: $%.2f", avgCPM)
	log.Printf("Miner Share: %.0f%%", minerShare*100)
	log.Printf("Daily Earnings: $%.2f", dailyEarnings)
	log.Printf("Monthly Earnings: $%.2f", monthlyEarnings)
	log.Printf("Annual Earnings: $%.2f", monthlyEarnings*12)
}

// Helper functions

func ptrInt64(v int64) *int64 {
	return &v
}

func calculatePodValue(bids []openrtb2.Bid) float64 {
	total := 0.0
	for _, bid := range bids {
		total += bid.Price
	}
	return total
}

func simulateActivity(tracker *analytics.AnalyticsTracker) {
	// Simulate bid requests
	for i := 0; i < 1000; i++ {
		request := &openrtb2.BidRequest{
			ID: fmt.Sprintf("req-%d", i),
			Imp: []openrtb2.Imp{
				{ID: fmt.Sprintf("imp-%d", i)},
			},
			Site: &openrtb2.Site{
				Publisher: &openrtb2.Publisher{ID: "pub-123"},
			},
			Device: &openrtb2.Device{
				DeviceType: 3, // CTV
				Geo:        &openrtb2.Geo{Country: "US"},
			},
		}
		tracker.TrackRequest(request)
		
		// Simulate 85% fill rate
		if i%100 < 85 {
			response := &openrtb2.BidResponse{
				ID: request.ID,
				SeatBid: []openrtb2.SeatBid{
					{
						Bid: []openrtb2.Bid{
							{Price: 5.0 + float64(i%20)},
						},
					},
				},
			}
			tracker.TrackResponse(response, 50*time.Millisecond)
		}
	}
}

func processAnalyticsEvents(tracker *analytics.AnalyticsTracker) {
	for event := range tracker.EventStream {
		// Process events (e.g., store to FoundationDB, send to data pipeline)
		_ = event // Process event
	}
}