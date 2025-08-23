package chainvm

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// AdSlotManager - Semi-Fungible Tokens for perishable ad inventory
// Implements high-performance DEX primitives with time-decay pricing
type AdSlotManager struct {
	state  *VMState
	dex    *dex.Engine
	nextID uint64
}

// AdSlot represents perishable ad inventory with time-decay pricing
type AdSlot struct {
	ID               uint64                 `json:"id"`
	Publisher        string                 `json:"publisher"`
	Placement        string                 `json:"placement"`        // "ctv-preroll", "banner-300x250"
	TargetingHash    string                 `json:"targeting_hash"`   // Hash of targeting predicate
	StartTime        time.Time              `json:"start_time"`       // Delivery window start
	EndTime          time.Time              `json:"end_time"`         // Perishable expiration!
	MaxImpressions   uint64                 `json:"max_impressions"`  // Total supply
	DeliveredImprs   uint64                 `json:"delivered_imprs"`  // Already served
	MinViewability   float64                `json:"min_viewability"`  // Quality floor %
	FloorCPM         decimal.Decimal        `json:"floor_cpm"`        // Minimum price
	Active           bool                   `json:"active"`
	Targeting        TargetingPredicate     `json:"targeting"`
	SecondaryMarkets []SecondaryListing     `json:"secondary_markets,omitempty"`
}

// TargetingPredicate defines audience and quality constraints
type TargetingPredicate struct {
	GeoTargets   []string `json:"geo_targets"`   // ["US", "CA", "UK"]
	DeviceTypes  []string `json:"device_types"`  // ["CTV", "mobile", "desktop"]
	Categories   []string `json:"categories"`    // ["IAB1", "IAB2"] 
	MinAge       uint32   `json:"min_age,omitempty"`
	MaxAge       uint32   `json:"max_age,omitempty"`
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`
}

// SecondaryListing represents resale of unused ad slots
type SecondaryListing struct {
	SellerID     string          `json:"seller_id"`
	Quantity     uint64          `json:"quantity"`
	AskPrice     decimal.Decimal `json:"ask_price"`
	ListedAt     time.Time       `json:"listed_at"`
	FlashLoanOK  bool            `json:"flash_loan_ok"` // Allow flash borrows
}

// AdMM_Pool for continuous liquidity (AMM for ad slots)
type AdMM_Pool struct {
	SlotID        uint64          `json:"slot_id"`
	ReserveAUSD   decimal.Decimal `json:"reserve_ausd"`   // AUSD liquidity
	ReserveSlots  uint64          `json:"reserve_slots"`  // Ad slot supply
	LastPrice     decimal.Decimal `json:"last_price"`
	LPTokenSupply decimal.Decimal `json:"lp_token_supply"`
	TimeDecayRate decimal.Decimal `json:"time_decay_rate"` // λ in pricing formula
	CreatedAt     time.Time       `json:"created_at"`
}

// Order represents limit/market orders for ad slots
type AdSlotOrder struct {
	ID            string          `json:"id"`
	TraderID      string          `json:"trader_id"`
	SlotID        uint64          `json:"slot_id"`
	IsBuy         bool            `json:"is_buy"`
	OrderType     string          `json:"order_type"`     // "limit", "market", "commit-reveal"
	LimitPrice    decimal.Decimal `json:"limit_price"`    // CPM in AUSD
	Quantity      uint64          `json:"quantity"`       // Number of impressions
	Filled        uint64          `json:"filled"`
	Status        string          `json:"status"`         // "active", "filled", "canceled", "expired"
	CreatedAt     time.Time       `json:"created_at"`
	ExpiresAt     time.Time       `json:"expires_at,omitempty"`
	CommitHash    string          `json:"commit_hash,omitempty"`    // For sealed bids
	RevealedPrice decimal.Decimal `json:"revealed_price,omitempty"` // After reveal
	Revealed      bool            `json:"revealed,omitempty"`
}

// RPC Methods

// CreateAdSlot - Mint new perishable ad inventory tokens
func (a *AdSlotManager) CreateAdSlot(ctx context.Context, req *CreateAdSlotRequest) (*CreateAdSlotResponse, error) {
	// Validate time window
	if req.StartTime.After(req.EndTime) {
		return nil, fmt.Errorf("invalid time window")
	}
	if req.EndTime.Before(time.Now()) {
		return nil, fmt.Errorf("window already ended")
	}
	if req.MaxImpressions == 0 {
		return nil, fmt.Errorf("no impressions")
	}

	// Generate deterministic targeting hash
	targetingHash := a.hashTargeting(req.Targeting)

	// Create slot
	slotID := a.nextID
	a.nextID++

	slot := &AdSlot{
		ID:             slotID,
		Publisher:      req.Publisher,
		Placement:      req.Placement,
		TargetingHash:  targetingHash,
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		MaxImpressions: req.MaxImpressions,
		MinViewability: req.MinViewability,
		FloorCPM:       req.FloorCPM,
		Active:         true,
		Targeting:      req.Targeting,
	}

	// Store in state
	a.state.SetAdSlot(slotID, slot)

	// Mint SFT to publisher (using DEX engine as registry)
	if err := a.dex.MintAsset(fmt.Sprintf("adslot-%d", slotID), req.Publisher, req.MaxImpressions); err != nil {
		return nil, fmt.Errorf("failed to mint SFT: %v", err)
	}

	return &CreateAdSlotResponse{
		Success: true,
		SlotID:  slotID,
		TokenID: fmt.Sprintf("adslot-%d", slotID),
	}, nil
}

// PlaceOrder - Place limit/market order for ad slots
func (a *AdSlotManager) PlaceOrder(ctx context.Context, req *PlaceOrderRequest) (*PlaceOrderResponse, error) {
	// Validate slot exists and is active
	slot, exists := a.state.GetAdSlot(req.SlotID)
	if !exists {
		return nil, fmt.Errorf("slot not found")
	}
	if !slot.Active {
		return nil, fmt.Errorf("slot inactive")
	}
	if time.Now().After(slot.EndTime) {
		return nil, fmt.Errorf("slot expired")
	}

	// Validate order
	if req.Quantity == 0 {
		return nil, fmt.Errorf("invalid quantity")
	}

	// Check price constraints
	currentPrice := a.calculateCurrentPrice(slot)
	if req.IsBuy && req.LimitPrice.LessThan(currentPrice) {
		return nil, fmt.Errorf("bid below current price")
	}

	// Create order
	order := &AdSlotOrder{
		ID:         req.OrderID,
		TraderID:   req.TraderID,
		SlotID:     req.SlotID,
		IsBuy:      req.IsBuy,
		OrderType:  req.OrderType,
		LimitPrice: req.LimitPrice,
		Quantity:   req.Quantity,
		Status:     "active",
		CreatedAt:  time.Now(),
		ExpiresAt:  req.ExpiresAt,
	}

	// Handle commit-reveal orders
	if req.OrderType == "commit-reveal" {
		if req.CommitHash == "" {
			return nil, fmt.Errorf("commit hash required")
		}
		order.CommitHash = req.CommitHash
	}

	// Store order
	a.state.SetAdSlotOrder(req.OrderID, order)

	// Add to matching engine via DEX
	dexOrder := convertToGDexOrder(order, slot)
	if err := a.dex.AddOrder(dexOrder); err != nil {
		return nil, fmt.Errorf("failed to add order: %v", err)
	}

	return &PlaceOrderResponse{
		Success:      true,
		OrderID:      req.OrderID,
		CurrentPrice: currentPrice,
		EstimatedFill: a.estimateOrderFill(order, slot),
	}, nil
}

// RevealBid - Reveal sealed bid in commit-reveal auction
func (a *AdSlotManager) RevealBid(ctx context.Context, req *RevealBidRequest) (*RevealBidResponse, error) {
	order, exists := a.state.GetAdSlotOrder(req.OrderID)
	if !exists {
		return nil, fmt.Errorf("order not found")
	}
	if order.OrderType != "commit-reveal" {
		return nil, fmt.Errorf("not a commit-reveal order")
	}
	if order.Revealed {
		return nil, fmt.Errorf("already revealed")
	}

	// Validate commitment
	expectedHash := a.hashCommitment(req.RevealedPrice, req.Nonce)
	if expectedHash != order.CommitHash {
		return nil, fmt.Errorf("invalid reveal")
	}

	// Update order with revealed price
	order.RevealedPrice = req.RevealedPrice
	order.Revealed = true
	order.LimitPrice = req.RevealedPrice // Use revealed price for matching

	a.state.SetAdSlotOrder(req.OrderID, order)

	return &RevealBidResponse{
		Success:       true,
		RevealedPrice: req.RevealedPrice,
	}, nil
}

// CreateAdMM_Pool - Create AMM pool for continuous liquidity
func (a *AdSlotManager) CreateAdMM_Pool(ctx context.Context, req *CreateAdMM_PoolRequest) (*CreateAdMM_PoolResponse, error) {
	// Validate slot
	slot, exists := a.state.GetAdSlot(req.SlotID)
	if !exists {
		return nil, fmt.Errorf("slot not found")
	}

	// Check for existing pool
	if _, exists := a.state.GetAdMM_Pool(req.SlotID); exists {
		return nil, fmt.Errorf("pool already exists")
	}

	// Calculate initial price and LP tokens
	initialPrice := req.InitialAUSD.Div(decimal.NewFromInt(int64(req.InitialSlots)))
	lpTokens := req.InitialAUSD.Mul(decimal.NewFromInt(int64(req.InitialSlots))).Sqrt() // Geometric mean

	pool := &AdMM_Pool{
		SlotID:        req.SlotID,
		ReserveAUSD:   req.InitialAUSD,
		ReserveSlots:  req.InitialSlots,
		LastPrice:     initialPrice,
		LPTokenSupply: lpTokens,
		TimeDecayRate: req.TimeDecayRate,
		CreatedAt:     time.Now(),
	}

	a.state.SetAdMM_Pool(req.SlotID, pool)

	// Transfer initial liquidity
	if err := a.transferAUSD(req.LiquidityProvider, "pool", req.InitialAUSD); err != nil {
		return nil, fmt.Errorf("AUSD transfer failed: %v", err)
	}

	return &CreateAdMM_PoolResponse{
		Success:   true,
		PoolID:    req.SlotID,
		LPTokens:  lpTokens,
		InitialPrice: initialPrice,
	}, nil
}

// SwapAdMM - Execute AMM swap (continuous liquidity)
func (a *AdSlotManager) SwapAdMM(ctx context.Context, req *SwapAdMM_Request) (*SwapAdMM_Response, error) {
	pool, exists := a.state.GetAdMM_Pool(req.SlotID)
	if !exists {
		return nil, fmt.Errorf("pool not found")
	}

	slot, _ := a.state.GetAdSlot(req.SlotID)
	
	// Calculate swap with time decay
	swapAmount := a.calculateAMM_Swap(pool, slot, req.AmountIn, req.BuyAUSD)
	if swapAmount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("insufficient liquidity")
	}

	// Execute swap
	if req.BuyAUSD {
		// Selling slots for AUSD
		pool.ReserveSlots += req.AmountIn
		pool.ReserveAUSD = pool.ReserveAUSD.Sub(swapAmount)
	} else {
		// Buying slots with AUSD
		pool.ReserveAUSD = pool.ReserveAUSD.Add(decimal.NewFromInt(int64(req.AmountIn)))
		pool.ReserveSlots -= uint64(swapAmount.IntPart())
	}

	// Update pool price
	if pool.ReserveSlots > 0 {
		pool.LastPrice = pool.ReserveAUSD.Div(decimal.NewFromInt(int64(pool.ReserveSlots)))
	}

	a.state.SetAdMM_Pool(req.SlotID, pool)

	return &SwapAdMM_Response{
		Success:    true,
		AmountOut:  swapAmount,
		NewPrice:   pool.LastPrice,
		SlippageActual: calculateSlippage(req.ExpectedAmountOut, swapAmount),
	}, nil
}

// RecordDelivery - Record impression delivery (burns tokens)
func (a *AdSlotManager) RecordDelivery(ctx context.Context, req *RecordDeliveryRequest) (*RecordDeliveryResponse, error) {
	slot, exists := a.state.GetAdSlot(req.SlotID)
	if !exists {
		return nil, fmt.Errorf("slot not found")
	}

	// Validate delivery window
	now := time.Now()
	if now.Before(slot.StartTime) || now.After(slot.EndTime) {
		return nil, fmt.Errorf("outside delivery window")
	}

	// Check capacity
	if slot.DeliveredImprs+req.Count > slot.MaxImpressions {
		return nil, fmt.Errorf("exceeds capacity")
	}

	// Record delivery
	slot.DeliveredImprs += req.Count
	a.state.SetAdSlot(req.SlotID, slot)

	// Burn delivered tokens from circulation
	if err := a.dex.BurnAsset(fmt.Sprintf("adslot-%d", req.SlotID), slot.Publisher, req.Count); err != nil {
		return nil, fmt.Errorf("failed to burn tokens: %v", err)
	}

	return &RecordDeliveryResponse{
		Success:           true,
		DeliveredCount:    req.Count,
		TotalDelivered:    slot.DeliveredImprs,
		RemainingSupply:   slot.MaxImpressions - slot.DeliveredImprs,
	}, nil
}

// Helper functions

func (a *AdSlotManager) hashTargeting(targeting TargetingPredicate) string {
	h := sha256.New()
	
	// Hash all targeting fields deterministically
	for _, geo := range targeting.GeoTargets {
		h.Write([]byte(geo))
	}
	for _, device := range targeting.DeviceTypes {
		h.Write([]byte(device))
	}
	for _, cat := range targeting.Categories {
		h.Write([]byte(cat))
	}
	
	// Add age constraints
	ageBuf := make([]byte, 8)
	binary.LittleEndian.PutUint32(ageBuf[0:4], targeting.MinAge)
	binary.LittleEndian.PutUint32(ageBuf[4:8], targeting.MaxAge)
	h.Write(ageBuf)
	
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (a *AdSlotManager) calculateCurrentPrice(slot *AdSlot) decimal.Decimal {
	now := time.Now()
	
	// Expired = worthless
	if now.After(slot.EndTime) || !slot.Active {
		return decimal.Zero
	}
	
	// Not started = full price
	if now.Before(slot.StartTime) {
		return slot.FloorCPM
	}
	
	// Linear time decay
	timeRemaining := slot.EndTime.Sub(now).Seconds()
	totalWindow := slot.EndTime.Sub(slot.StartTime).Seconds()
	
	if totalWindow <= 0 {
		return slot.FloorCPM
	}
	
	// Price = floor + (50% premium * time_remaining / total_window)
	premium := slot.FloorCPM.Div(decimal.NewFromInt(2))
	timeRatio := decimal.NewFromFloat(timeRemaining / totalWindow)
	
	return slot.FloorCPM.Add(premium.Mul(timeRatio))
}

func (a *AdSlotManager) calculateAMM_Swap(pool *AdMM_Pool, slot *AdSlot, amountIn uint64, buyAUSD bool) decimal.Decimal {
	// Constant product AMM with time decay: k = reserves_ausd * reserves_slots * time_factor
	if pool.ReserveAUSD.LessThanOrEqual(decimal.Zero) || pool.ReserveSlots == 0 {
		return decimal.Zero
	}
	
	// Apply time decay to effective reserves
	timeDecay := a.calculateTimeDecay(slot, pool.TimeDecayRate)
	effectiveK := pool.ReserveAUSD.Mul(decimal.NewFromInt(int64(pool.ReserveSlots))).Mul(timeDecay)
	
	if buyAUSD {
		// Selling slots for AUSD: new_slots = old_slots + amount_in
		newSlots := decimal.NewFromInt(int64(pool.ReserveSlots + amountIn))
		newAUSD := effectiveK.Div(newSlots)
		return pool.ReserveAUSD.Sub(newAUSD)
	} else {
		// Buying slots with AUSD: new_ausd = old_ausd + amount_in
		newAUSD := pool.ReserveAUSD.Add(decimal.NewFromInt(int64(amountIn)))
		newSlots := effectiveK.Div(newAUSD)
		return decimal.NewFromInt(int64(pool.ReserveSlots)).Sub(newSlots)
	}
}

func (a *AdSlotManager) calculateTimeDecay(slot *AdSlot, decayRate decimal.Decimal) decimal.Decimal {
	now := time.Now()
	if now.After(slot.EndTime) {
		return decimal.Zero
	}
	
	timeRemaining := slot.EndTime.Sub(now).Seconds()
	totalWindow := slot.EndTime.Sub(slot.StartTime).Seconds()
	
	if totalWindow <= 0 {
		return decimal.NewFromInt(1)
	}
	
	// Exponential decay: e^(-λ * (1 - time_remaining/total_window))
	normalizedTime := decimal.NewFromFloat(1.0 - (timeRemaining / totalWindow))
	exponent := decayRate.Mul(normalizedTime).Neg()
	
	// Approximate e^x for small x
	return decimal.NewFromInt(1).Add(exponent)
}

// Request/Response types

type CreateAdSlotRequest struct {
	Publisher      string             `json:"publisher"`
	Placement      string             `json:"placement"`
	Targeting      TargetingPredicate `json:"targeting"`
	StartTime      time.Time          `json:"start_time"`
	EndTime        time.Time          `json:"end_time"`
	MaxImpressions uint64             `json:"max_impressions"`
	MinViewability float64            `json:"min_viewability"`
	FloorCPM       decimal.Decimal    `json:"floor_cpm"`
}

type CreateAdSlotResponse struct {
	Success bool   `json:"success"`
	SlotID  uint64 `json:"slot_id"`
	TokenID string `json:"token_id"`
}

type PlaceOrderRequest struct {
	OrderID    string          `json:"order_id"`
	TraderID   string          `json:"trader_id"`
	SlotID     uint64          `json:"slot_id"`
	IsBuy      bool            `json:"is_buy"`
	OrderType  string          `json:"order_type"`
	LimitPrice decimal.Decimal `json:"limit_price"`
	Quantity   uint64          `json:"quantity"`
	ExpiresAt  time.Time       `json:"expires_at,omitempty"`
	CommitHash string          `json:"commit_hash,omitempty"`
}

type PlaceOrderResponse struct {
	Success       bool            `json:"success"`
	OrderID       string          `json:"order_id"`
	CurrentPrice  decimal.Decimal `json:"current_price"`
	EstimatedFill decimal.Decimal `json:"estimated_fill"`
}

// Additional request/response types would follow similar patterns...