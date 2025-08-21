package analytics

import (
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/shopspring/decimal"
)

// AnalyticsTracker tracks all ADX metrics and events
type AnalyticsTracker struct {
	// Real-time counters
	TotalRequests      atomic.Uint64
	TotalImpressions   atomic.Uint64
	TotalClicks        atomic.Uint64
	TotalCompletions   atomic.Uint64
	TotalRevenue       atomic.Uint64 // In microcents
	
	// Performance metrics
	AverageLatency     atomic.Uint64 // In microseconds
	P95Latency         atomic.Uint64
	P99Latency         atomic.Uint64
	
	// Fill rates
	FillRate           atomic.Uint64 // Percentage * 100
	ViewabilityRate    atomic.Uint64
	CompletionRate     atomic.Uint64
	
	// Pod metrics for CTV
	PodMetrics         *PodMetrics
	
	// Time series data
	TimeSeries         *TimeSeriesData
	
	// Publisher metrics
	PublisherMetrics   map[string]*PublisherStats
	
	// DSP metrics
	DSPMetrics         map[string]*DSPStats
	
	// Miner metrics
	MinerMetrics       map[string]*MinerStats
	
	// Mutex for maps
	mu                 sync.RWMutex
	
	// Event stream for real-time analytics
	EventStream        chan *Event
	
	// Storage backend (FoundationDB when ready)
	storage            StorageBackend
}

// PodMetrics tracks CTV ad pod performance
type PodMetrics struct {
	TotalPods          atomic.Uint64
	AveragePodSize     atomic.Uint64
	PodFillRate        atomic.Uint64
	PodCompletionRate  atomic.Uint64
	AveragePodCPM      atomic.Uint64
	
	// Break type distribution
	PrerollCount       atomic.Uint64
	MidrollCount       atomic.Uint64
	PostrollCount      atomic.Uint64
}

// TimeSeriesData stores time-bucketed metrics
type TimeSeriesData struct {
	Buckets    map[int64]*MetricBucket
	BucketSize time.Duration
	mu         sync.RWMutex
}

// MetricBucket represents metrics for a time period
type MetricBucket struct {
	Timestamp    time.Time
	Requests     uint64
	Impressions  uint64
	Revenue      decimal.Decimal
	FillRate     float64
	AvgLatency   time.Duration
	UniqueUsers  map[string]bool
	TopDomains   map[string]uint64
}

// PublisherStats tracks per-publisher metrics
type PublisherStats struct {
	PublisherID      string
	Name             string
	TotalRequests    uint64
	TotalImpressions uint64
	TotalRevenue     *big.Int
	FillRate         float64
	eCPM             decimal.Decimal
	TopPlacements    map[string]*PlacementStats
}

// PlacementStats tracks individual placement performance
type PlacementStats struct {
	PlacementID  string
	Impressions  uint64
	Clicks       uint64
	CTR          float64
	Revenue      decimal.Decimal
}

// DSPStats tracks demand-side platform performance
type DSPStats struct {
	DSPID            string
	Name             string
	TotalBids        uint64
	WinningBids      uint64
	WinRate          float64
	AverageBid       decimal.Decimal
	TotalSpend       decimal.Decimal
	ResponseTime     time.Duration
	TimeoutRate      float64
	Categories       map[string]uint64
}

// MinerStats tracks home miner performance
type MinerStats struct {
	MinerID          string
	WalletAddress    string
	Location         string
	TotalServed      uint64
	TotalBandwidth   uint64 // In bytes
	Uptime           time.Duration
	Earnings         *big.Int
	QualityScore     float64
	AverageLatency   time.Duration
	ErrorRate        float64
}

// Event represents an analytics event
type Event struct {
	Type         EventType
	Timestamp    time.Time
	PublisherID  string
	PlacementID  string
	ImpressionID string
	DSPID        string
	MinerID      string
	UserID       string
	DeviceType   string
	GeoCountry   string
	Price        decimal.Decimal
	Metadata     map[string]interface{}
}

// EventType represents the type of analytics event
type EventType string

const (
	EventRequest     EventType = "request"
	EventBid         EventType = "bid"
	EventWin         EventType = "win"
	EventImpression  EventType = "impression"
	EventClick       EventType = "click"
	EventComplete    EventType = "complete"
	EventPodStart    EventType = "pod_start"
	EventPodEnd      EventType = "pod_end"
	EventError       EventType = "error"
	EventTimeout     EventType = "timeout"
	EventMinerJoin   EventType = "miner_join"
	EventMinerLeave  EventType = "miner_leave"
	EventPayout      EventType = "payout"
)

// StorageBackend interface for persisting analytics
type StorageBackend interface {
	Store(event *Event) error
	Query(filter QueryFilter) ([]*Event, error)
	Aggregate(metric string, groupBy []string, timeRange TimeRange) (map[string]interface{}, error)
}

// QueryFilter for retrieving events
type QueryFilter struct {
	StartTime    time.Time
	EndTime      time.Time
	EventTypes   []EventType
	PublisherIDs []string
	DSPIDs       []string
	MinerIDs     []string
	Limit        int
}

// TimeRange for aggregations
type TimeRange struct {
	Start       time.Time
	End         time.Time
	Granularity time.Duration
}

// NewAnalyticsTracker creates a new analytics tracker
func NewAnalyticsTracker() *AnalyticsTracker {
	return &AnalyticsTracker{
		PodMetrics: &PodMetrics{},
		TimeSeries: &TimeSeriesData{
			Buckets:    make(map[int64]*MetricBucket),
			BucketSize: 1 * time.Minute,
		},
		PublisherMetrics: make(map[string]*PublisherStats),
		DSPMetrics:       make(map[string]*DSPStats),
		MinerMetrics:     make(map[string]*MinerStats),
		EventStream:      make(chan *Event, 10000),
		storage:          NewInMemoryStorage(), // Default to in-memory
	}
}

// TrackRequest tracks an incoming bid request
func (a *AnalyticsTracker) TrackRequest(request *openrtb2.BidRequest) {
	a.TotalRequests.Add(1)
	
	event := &Event{
		Type:        EventRequest,
		Timestamp:   time.Now(),
		PublisherID: a.extractPublisherID(request),
		PlacementID: a.extractPlacementID(request),
		DeviceType:  a.extractDeviceType(request),
		GeoCountry:  a.extractGeoCountry(request),
		Metadata: map[string]interface{}{
			"imp_count": len(request.Imp),
			"video":     a.hasVideo(request),
			"pod":       a.isPod(request),
		},
	}
	
	// Send to event stream
	select {
	case a.EventStream <- event:
	default:
		// Buffer full, drop event
	}
	
	// Update time series
	a.updateTimeSeries(event)
}

// TrackResponse tracks bid response and latency
func (a *AnalyticsTracker) TrackResponse(response *openrtb2.BidResponse, latency time.Duration) {
	if response != nil && len(response.SeatBid) > 0 {
		a.TotalImpressions.Add(1)
		
		// Update fill rate
		totalReq := a.TotalRequests.Load()
		totalImp := a.TotalImpressions.Load()
		if totalReq > 0 {
			newFillRate := (totalImp * 10000) / totalReq // Percentage * 100
			a.FillRate.Store(newFillRate)
		}
		
		// Track revenue
		for _, seatBid := range response.SeatBid {
			for _, bid := range seatBid.Bid {
				revenue := uint64(bid.Price * 1000000) // Convert to microcents
				a.TotalRevenue.Add(revenue)
			}
		}
	}
	
	// Update latency metrics
	latencyMicros := uint64(latency.Microseconds())
	a.updateLatencyMetrics(latencyMicros)
}

// TrackImpression tracks an ad impression
func (a *AnalyticsTracker) TrackImpression(impressionID, publisherID, minerID string, price decimal.Decimal) {
	event := &Event{
		Type:         EventImpression,
		Timestamp:    time.Now(),
		ImpressionID: impressionID,
		PublisherID:  publisherID,
		MinerID:      minerID,
		Price:        price,
	}
	
	// Update publisher metrics
	a.mu.Lock()
	if pub, ok := a.PublisherMetrics[publisherID]; ok {
		pub.TotalImpressions++
		pub.TotalRevenue.Add(pub.TotalRevenue, price.BigInt())
	}
	a.mu.Unlock()
	
	// Update miner metrics
	if minerID != "" {
		a.updateMinerMetrics(minerID, event)
	}
	
	// Store event
	a.storage.Store(event)
}

// TrackPodMetrics tracks CTV pod performance
func (a *AnalyticsTracker) TrackPodMetrics(podID string, podSize int, completed bool) {
	a.PodMetrics.TotalPods.Add(1)
	a.PodMetrics.AveragePodSize.Store(uint64(podSize))
	
	if completed {
		currentRate := a.PodMetrics.PodCompletionRate.Load()
		totalPods := a.PodMetrics.TotalPods.Load()
		newRate := ((currentRate * (totalPods - 1)) + 100) / totalPods
		a.PodMetrics.PodCompletionRate.Store(newRate)
	}
}

// GetRealTimeMetrics returns current real-time metrics
func (a *AnalyticsTracker) GetRealTimeMetrics() map[string]interface{} {
	return map[string]interface{}{
		"total_requests":    a.TotalRequests.Load(),
		"total_impressions": a.TotalImpressions.Load(),
		"total_clicks":      a.TotalClicks.Load(),
		"total_revenue":     float64(a.TotalRevenue.Load()) / 1000000.0, // Convert from microcents
		"fill_rate":         float64(a.FillRate.Load()) / 100.0,
		"avg_latency_ms":    float64(a.AverageLatency.Load()) / 1000.0,
		"p95_latency_ms":    float64(a.P95Latency.Load()) / 1000.0,
		"p99_latency_ms":    float64(a.P99Latency.Load()) / 1000.0,
		"pod_metrics": map[string]interface{}{
			"total_pods":       a.PodMetrics.TotalPods.Load(),
			"avg_pod_size":     a.PodMetrics.AveragePodSize.Load(),
			"completion_rate":  float64(a.PodMetrics.PodCompletionRate.Load()) / 100.0,
		},
	}
}

// GetPublisherReport generates a publisher performance report
func (a *AnalyticsTracker) GetPublisherReport(publisherID string, timeRange TimeRange) (*PublisherReport, error) {
	a.mu.RLock()
	stats, ok := a.PublisherMetrics[publisherID]
	a.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("publisher %s not found", publisherID)
	}
	
	// Query historical data
	events, err := a.storage.Query(QueryFilter{
		StartTime:    timeRange.Start,
		EndTime:      timeRange.End,
		PublisherIDs: []string{publisherID},
	})
	if err != nil {
		return nil, err
	}
	
	report := &PublisherReport{
		PublisherID:      publisherID,
		TimeRange:        timeRange,
		TotalImpressions: stats.TotalImpressions,
		TotalRevenue:     decimal.NewFromBigInt(stats.TotalRevenue, 0),
		FillRate:         stats.FillRate,
		eCPM:             stats.eCPM,
		Events:           events,
	}
	
	return report, nil
}

// Helper methods

func (a *AnalyticsTracker) extractPublisherID(request *openrtb2.BidRequest) string {
	if request.Site != nil {
		return request.Site.Publisher.ID
	}
	if request.App != nil {
		return request.App.Publisher.ID
	}
	return ""
}

func (a *AnalyticsTracker) extractPlacementID(request *openrtb2.BidRequest) string {
	if len(request.Imp) > 0 {
		return request.Imp[0].ID
	}
	return ""
}

func (a *AnalyticsTracker) extractDeviceType(request *openrtb2.BidRequest) string {
	if request.Device != nil {
		switch request.Device.DeviceType {
		case 3:
			return "ctv"
		case 4, 5:
			return "mobile"
		default:
			return "desktop"
		}
	}
	return "unknown"
}

func (a *AnalyticsTracker) extractGeoCountry(request *openrtb2.BidRequest) string {
	if request.Device != nil && request.Device.Geo != nil {
		return request.Device.Geo.Country
	}
	return ""
}

func (a *AnalyticsTracker) hasVideo(request *openrtb2.BidRequest) bool {
	for _, imp := range request.Imp {
		if imp.Video != nil {
			return true
		}
	}
	return false
}

func (a *AnalyticsTracker) isPod(request *openrtb2.BidRequest) bool {
	return len(request.Imp) > 1 && a.hasVideo(request)
}

func (a *AnalyticsTracker) updateTimeSeries(event *Event) {
	bucket := time.Now().Unix() / int64(a.TimeSeries.BucketSize.Seconds())
	
	a.TimeSeries.mu.Lock()
	defer a.TimeSeries.mu.Unlock()
	
	if _, ok := a.TimeSeries.Buckets[bucket]; !ok {
		a.TimeSeries.Buckets[bucket] = &MetricBucket{
			Timestamp:   time.Unix(bucket*int64(a.TimeSeries.BucketSize.Seconds()), 0),
			UniqueUsers: make(map[string]bool),
			TopDomains:  make(map[string]uint64),
		}
	}
	
	a.TimeSeries.Buckets[bucket].Requests++
}

func (a *AnalyticsTracker) updateLatencyMetrics(latencyMicros uint64) {
	// Simple moving average for demo
	current := a.AverageLatency.Load()
	count := a.TotalRequests.Load()
	if count > 0 {
		newAvg := ((current * (count - 1)) + latencyMicros) / count
		a.AverageLatency.Store(newAvg)
	}
	
	// Update P95/P99 (simplified)
	if latencyMicros > a.P95Latency.Load() {
		a.P95Latency.Store(latencyMicros)
	}
	if latencyMicros > a.P99Latency.Load() {
		a.P99Latency.Store(latencyMicros)
	}
}

func (a *AnalyticsTracker) updateMinerMetrics(minerID string, event *Event) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if _, ok := a.MinerMetrics[minerID]; !ok {
		a.MinerMetrics[minerID] = &MinerStats{
			MinerID:  minerID,
			Earnings: big.NewInt(0),
		}
	}
	
	a.MinerMetrics[minerID].TotalServed++
	if event.Price.GreaterThan(decimal.Zero) {
		earnings := event.Price.Mul(decimal.NewFromFloat(0.1)) // 10% to miner
		a.MinerMetrics[minerID].Earnings.Add(
			a.MinerMetrics[minerID].Earnings,
			earnings.BigInt(),
		)
	}
}

// PublisherReport represents a publisher performance report
type PublisherReport struct {
	PublisherID      string
	TimeRange        TimeRange
	TotalImpressions uint64
	TotalRevenue     decimal.Decimal
	FillRate         float64
	eCPM             decimal.Decimal
	Events           []*Event
	TopPlacements    []*PlacementStats
	DailyBreakdown   map[string]*DailyStats
}

// DailyStats represents daily statistics
type DailyStats struct {
	Date        time.Time
	Impressions uint64
	Revenue     decimal.Decimal
	FillRate    float64
}

// InMemoryStorage provides in-memory storage for analytics
type InMemoryStorage struct {
	events []Event
	mu     sync.RWMutex
}

// NewInMemoryStorage creates new in-memory storage
func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		events: make([]Event, 0),
	}
}

// Store saves an event
func (s *InMemoryStorage) Store(event *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, *event)
	return nil
}

// Query retrieves events matching filter
func (s *InMemoryStorage) Query(filter QueryFilter) ([]*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	results := make([]*Event, 0)
	for i := range s.events {
		if s.matchesFilter(&s.events[i], filter) {
			results = append(results, &s.events[i])
			if len(results) >= filter.Limit && filter.Limit > 0 {
				break
			}
		}
	}
	
	return results, nil
}

// Aggregate performs aggregation queries
func (s *InMemoryStorage) Aggregate(metric string, groupBy []string, timeRange TimeRange) (map[string]interface{}, error) {
	// Simplified aggregation
	return map[string]interface{}{
		"metric":    metric,
		"timeRange": timeRange,
		"result":    0,
	}, nil
}

func (s *InMemoryStorage) matchesFilter(event *Event, filter QueryFilter) bool {
	if !event.Timestamp.After(filter.StartTime) || !event.Timestamp.Before(filter.EndTime) {
		return false
	}
	
	if len(filter.EventTypes) > 0 {
		found := false
		for _, t := range filter.EventTypes {
			if event.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	return true
}

// ExportMetrics exports metrics in Prometheus format
func (a *AnalyticsTracker) ExportMetrics() string {
	metrics := fmt.Sprintf(`# HELP adx_requests_total Total number of ad requests
# TYPE adx_requests_total counter
adx_requests_total %d

# HELP adx_impressions_total Total number of ad impressions
# TYPE adx_impressions_total counter
adx_impressions_total %d

# HELP adx_revenue_total Total revenue in dollars
# TYPE adx_revenue_total counter
adx_revenue_total %.2f

# HELP adx_fill_rate Current fill rate
# TYPE adx_fill_rate gauge
adx_fill_rate %.4f

# HELP adx_latency_milliseconds Average latency in milliseconds
# TYPE adx_latency_milliseconds gauge
adx_latency_milliseconds %.2f

# HELP adx_pods_total Total number of ad pods served
# TYPE adx_pods_total counter
adx_pods_total %d

# HELP adx_pod_completion_rate Pod completion rate
# TYPE adx_pod_completion_rate gauge
adx_pod_completion_rate %.4f
`,
		a.TotalRequests.Load(),
		a.TotalImpressions.Load(),
		float64(a.TotalRevenue.Load())/1000000.0,
		float64(a.FillRate.Load())/10000.0,
		float64(a.AverageLatency.Load())/1000.0,
		a.PodMetrics.TotalPods.Load(),
		float64(a.PodMetrics.PodCompletionRate.Load())/100.0,
	)
	
	return metrics
}