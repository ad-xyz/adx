package main

import (
	// "context"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// "github.com/apple/foundationdb/bindings/go/src/fdb" // TODO: Add FDB support
	"github.com/gorilla/websocket"
	"github.com/luxfi/adx/pkg/rtb"
	"github.com/luxfi/adx/pkg/vast"
	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/shopspring/decimal"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Parse flags
	var (
		port          = flag.Int("port", 8080, "HTTP server port")
		wsPort        = flag.Int("ws-port", 8081, "WebSocket server port")
		fdbCluster    = flag.String("fdb-cluster", "", "FoundationDB cluster file")
		floorCPM      = flag.Float64("floor-cpm", 0.50, "Floor price CPM")
		auctionTimeout = flag.Duration("auction-timeout", 100*time.Millisecond, "Auction timeout")
		version       = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *version {
		fmt.Printf("ADX Exchange v%s (commit: %s, built: %s)\n", Version, GitCommit, BuildTime)
		os.Exit(0)
	}

	log.Printf("Starting ADX Exchange v%s", Version)

	// Initialize FoundationDB
	// TODO: Add FoundationDB support
	// var fdbDatabase fdb.Database
	if *fdbCluster != "" {
		log.Println("FoundationDB support coming soon. Running in-memory mode.")
		// fdb.MustAPIVersion(730)
		// var err error
		// fdbDatabase, err = fdb.OpenDatabase(*fdbCluster)
		// if err != nil {
		// 	log.Fatalf("Failed to open FoundationDB: %v", err)
		// }
		// log.Println("Connected to FoundationDB")
	} else {
		log.Println("Warning: Running without FoundationDB (in-memory mode)")
	}

	// Create RTB exchange
	exchange := &rtb.RTBExchange{
		DSPs:           make(map[string]*rtb.DSPConnection),
		SSPs:           make(map[string]*rtb.SSPConnection),
		AuctionTimeout: *auctionTimeout,
		FloorPrice:     decimal.NewFromFloat(*floorCPM),
		Revenue:        big.NewInt(0),
		CTVOptimizer: &rtb.CTVOptimizer{
			PublicaEnabled:            true,
			PodFillRate:               0.85,
			OptimalPodSize:            6,
			CreativeQualityThreshold: 0.7,
			MinVideoBitrate:          2000000, // 2 Mbps
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

	// if fdbDatabase != nil {
	// 	exchange.fdb = fdbDatabase
	// 	exchange.fdbSpace = "adx"
	// }

	// Initialize sample DSPs
	exchange.DSPs["dsp1"] = &rtb.DSPConnection{
		ID:         "dsp1",
		Name:       "Sample DSP 1",
		Endpoint:   "https://dsp1.example.com/bid",
		QPS:        1000,
		Timeout:    80 * time.Millisecond,
		BidderCode: "dsp1",
		SeatID:     "seat1",
		// RateLimiter: &rtb.RateLimiter{
		// 	tokens: 1000,
		// 	max:    1000,
		// 	refill: time.Second,
		// },
	}

	// HTTP handlers
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/rtb/bid", makeBidHandler(exchange))
	http.HandleFunc("/vast", makeVASTHandler())
	http.HandleFunc("/miner/connect", makeMinerHandler(exchange))

	// Start HTTP server
	go func() {
		addr := fmt.Sprintf(":%d", *port)
		log.Printf("HTTP server listening on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start WebSocket server for miners
	go startWebSocketServer(*wsPort, exchange)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down ADX Exchange...")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","version":"%s"}`, Version)
}

func makeBidHandler(exchange *rtb.RTBExchange) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var bidRequest openrtb2.BidRequest
		if err := json.NewDecoder(r.Body).Decode(&bidRequest); err != nil {
			http.Error(w, "Invalid bid request", http.StatusBadRequest)
			return
		}

		// ctx := r.Context()
		bidResponse, err := exchange.BidRequest(nil, &bidRequest) // TODO: Pass context
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bidResponse)
	}
}

func makeVASTHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse query parameters
		width := r.URL.Query().Get("w")
		height := r.URL.Query().Get("h")
		
		// Create sample VAST response
		vastResp := &vast.VAST{
			Version: "4.0",
			Ads: []vast.Ad{
				{
					ID: "ad123",
					InLine: &vast.InLine{
						AdSystem: vast.AdSystem{
							Name:    "ADX",
							Version: "1.0",
						},
						AdTitle:     "Sample Video Ad",
						Description: "High-quality CTV advertisement",
						Impression: []vast.Impression{
							{URL: "https://track.adx.com/impression/123"},
						},
						Creatives: vast.Creatives{
							Creative: []vast.Creative{
								{
									ID: "creative1",
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
													URL:      fmt.Sprintf("https://cdn.adx.com/video/sample_%sx%s.mp4", width, height),
												},
											},
										},
										TrackingEvents: &vast.TrackingEvents{
											Tracking: []vast.Tracking{
												{Event: "start", URL: "https://track.adx.com/start/123"},
												{Event: "complete", URL: "https://track.adx.com/complete/123"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/xml")
		xml.NewEncoder(w).Encode(vastResp)
	}
}

func makeMinerHandler(exchange *rtb.RTBExchange) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Upgrade to WebSocket
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		log.Printf("New miner connected from %s", r.RemoteAddr)
		
		// Handle miner connection
		for {
			var msg map[string]interface{}
			if err := conn.ReadJSON(&msg); err != nil {
				log.Printf("Miner disconnected: %v", err)
				break
			}
			
			// Process miner messages
			log.Printf("Miner message: %v", msg)
		}
	}
}

func startWebSocketServer(port int, exchange *rtb.RTBExchange) {
	addr := fmt.Sprintf(":%d", port)
	log.Printf("WebSocket server listening on %s", addr)
	
	http.HandleFunc("/ws", makeMinerHandler(exchange))
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("WebSocket server failed: %v", err)
	}
}