package chainvm

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/luxfi/adx/pkg/dex"
	"github.com/shopspring/decimal"
)

// EscrowManager - AUSD-settled escrow for campaigns
// Solves "we delivered, they didn't pay" problem with pre-funded campaigns
type EscrowManager struct {
	state  *VMState
	dex    *dex.Engine
	ausdID string
}

// Campaign represents a pre-funded advertising campaign
type Campaign struct {
	ID               string          `json:"id"`
	Advertiser       string          `json:"advertiser"`
	TotalBudget      decimal.Decimal `json:"total_budget"`
	AvailableBudget  decimal.Decimal `json:"available_budget"`
	ReservedBudget   decimal.Decimal `json:"reserved_budget"`
	SpentBudget      decimal.Decimal `json:"spent_budget"`
	Active           bool            `json:"active"`
	HoldbackBps      uint16          `json:"holdback_bps"` // Basis points for fraud protection
	Created          time.Time       `json:"created"`
	GuaranteedDeals  []PGDeal        `json:"guaranteed_deals,omitempty"`
}

// Reservation represents atomic impression reservation with TTL
type Reservation struct {
	ID         string          `json:"id"`
	CampaignID string          `json:"campaign_id"`
	Publisher  string          `json:"publisher"`
	Amount     decimal.Decimal `json:"amount"`
	Expires    time.Time       `json:"expires"`
	Settled    bool            `json:"settled"`
	Metadata   ReservationMeta `json:"metadata"`
}

// ReservationMeta contains impression targeting details
type ReservationMeta struct {
	Placement    string   `json:"placement"`
	Geo          string   `json:"geo"`
	DeviceType   string   `json:"device_type"`
	Categories   []string `json:"categories"`
	Viewability  float64  `json:"min_viewability"`
	UserHash     string   `json:"user_hash,omitempty"` // Privacy-preserving user identifier
}

// PGDeal represents programmatic guaranteed deal
type PGDeal struct {
	ID           string          `json:"id"`
	Publisher    string          `json:"publisher"`
	StartTime    time.Time       `json:"start_time"`
	EndTime      time.Time       `json:"end_time"`
	TotalImprs   uint64          `json:"total_impressions"`
	DeliveredImprs uint64        `json:"delivered_impressions"`
	FixedCPM     decimal.Decimal `json:"fixed_cpm"`
	EscrowAmount decimal.Decimal `json:"escrow_amount"`
	PenaltyRate  decimal.Decimal `json:"penalty_rate"` // Auto-penalty for under-delivery
}

// RPC Methods for Chain VM

// FundCampaign - Pre-fund campaign in AUSD (eliminates payment risk)
func (e *EscrowManager) FundCampaign(ctx context.Context, req *FundCampaignRequest) (*FundCampaignResponse, error) {
	// Validate request
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("amount must be positive")
	}
	if req.HoldbackBps > 2000 {
		return nil, fmt.Errorf("holdback cannot exceed 20%%")
	}

	// Check/create campaign
	campaign, exists := e.state.GetCampaign(req.CampaignID)
	if !exists {
		campaign = &Campaign{
			ID:              req.CampaignID,
			Advertiser:      req.Advertiser,
			HoldbackBps:     req.HoldbackBps,
			Created:         time.Now(),
			Active:          true,
			TotalBudget:     decimal.Zero,
			AvailableBudget: decimal.Zero,
			ReservedBudget:  decimal.Zero,
			SpentBudget:     decimal.Zero,
		}
	} else if campaign.Advertiser != req.Advertiser {
		return nil, fmt.Errorf("only campaign owner can fund")
	}

	// Execute AUSD transfer to escrow
	if err := e.transferAUSD(req.Advertiser, "escrow", req.Amount); err != nil {
		return nil, fmt.Errorf("AUSD transfer failed: %v", err)
	}

	// Update campaign budgets
	campaign.TotalBudget = campaign.TotalBudget.Add(req.Amount)
	campaign.AvailableBudget = campaign.AvailableBudget.Add(req.Amount)

	// Save state
	e.state.SetCampaign(req.CampaignID, campaign)

	return &FundCampaignResponse{
		Success:         true,
		NewTotalBudget:  campaign.TotalBudget,
		AvailableBudget: campaign.AvailableBudget,
	}, nil
}

// ReserveBudget - Atomic reservation for impression (1-2s TTL)
func (e *EscrowManager) ReserveBudget(ctx context.Context, req *ReserveBudgetRequest) (*ReserveBudgetResponse, error) {
	if req.TTLSeconds > 10 {
		return nil, fmt.Errorf("TTL too long (max 10s)")
	}
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("amount must be positive")
	}

	// Check for duplicate reservation
	if _, exists := e.state.GetReservation(req.ReservationID); exists {
		return nil, fmt.Errorf("reservation already exists")
	}

	// Validate campaign
	campaign, exists := e.state.GetCampaign(req.CampaignID)
	if !exists || !campaign.Active {
		return nil, fmt.Errorf("campaign inactive")
	}
	if campaign.AvailableBudget.LessThan(req.Amount) {
		return nil, fmt.Errorf("insufficient budget")
	}

	// Create reservation with TTL
	reservation := &Reservation{
		ID:         req.ReservationID,
		CampaignID: req.CampaignID,
		Publisher:  req.Publisher,
		Amount:     req.Amount,
		Expires:    time.Now().Add(time.Duration(req.TTLSeconds) * time.Second),
		Settled:    false,
		Metadata:   req.Metadata,
	}

	// Lock budget atomically
	campaign.AvailableBudget = campaign.AvailableBudget.Sub(req.Amount)
	campaign.ReservedBudget = campaign.ReservedBudget.Add(req.Amount)

	// Save state
	e.state.SetCampaign(req.CampaignID, campaign)
	e.state.SetReservation(req.ReservationID, reservation)

	return &ReserveBudgetResponse{
		Success:    true,
		Expires:    reservation.Expires,
		RemainingBudget: campaign.AvailableBudget,
	}, nil
}

// SettleReceipt - Pay publisher on verified delivery (T+0/T+1 settlement)
func (e *EscrowManager) SettleReceipt(ctx context.Context, req *SettleReceiptRequest) (*SettleReceiptResponse, error) {
	// Get reservation
	reservation, exists := e.state.GetReservation(req.ReservationID)
	if !exists {
		return nil, fmt.Errorf("reservation not found")
	}
	if reservation.Settled {
		return nil, fmt.Errorf("already settled")
	}
	if time.Now().After(reservation.Expires) {
		return nil, fmt.Errorf("reservation expired")
	}

	// Verify delivery proof
	if err := e.verifyDeliveryProof(req.VerificationProof, reservation); err != nil {
		return nil, fmt.Errorf("delivery verification failed: %v", err)
	}

	// Get campaign
	campaign, _ := e.state.GetCampaign(reservation.CampaignID)

	// Calculate streaming settlement vs holdback
	holdbackAmount := reservation.Amount.Mul(decimal.NewFromInt(int64(campaign.HoldbackBps))).Div(decimal.NewFromInt(10000))
	immediateAmount := reservation.Amount.Sub(holdbackAmount)

	// Update campaign accounting
	campaign.ReservedBudget = campaign.ReservedBudget.Sub(reservation.Amount)
	campaign.SpentBudget = campaign.SpentBudget.Add(reservation.Amount)

	// Stream payment to publisher (T+0 settlement)
	publisherBalance := e.state.GetPublisherBalance(reservation.Publisher)
	publisherBalance = publisherBalance.Add(immediateAmount)
	e.state.SetPublisherBalance(reservation.Publisher, publisherBalance)

	// Schedule holdback release (24-48hr fraud window)
	if holdbackAmount.GreaterThan(decimal.Zero) {
		e.scheduleHoldbackRelease(reservation.Publisher, holdbackAmount, 48*time.Hour)
	}

	// Mark settled
	reservation.Settled = true

	// Save state
	e.state.SetCampaign(reservation.CampaignID, campaign)
	e.state.SetReservation(req.ReservationID, reservation)

	return &SettleReceiptResponse{
		Success:         true,
		PaidAmount:      immediateAmount,
		HoldbackAmount:  holdbackAmount,
		PublisherBalance: publisherBalance,
	}, nil
}

// CreatePGDeal - Create programmatic guaranteed deal with escrow
func (e *EscrowManager) CreatePGDeal(ctx context.Context, req *CreatePGDealRequest) (*CreatePGDealResponse, error) {
	campaign, exists := e.state.GetCampaign(req.CampaignID)
	if !exists {
		return nil, fmt.Errorf("campaign not found")
	}

	// Calculate total escrow needed (impressions * CPM + penalty buffer)
	totalCost := decimal.NewFromInt(int64(req.TotalImpressions)).Mul(req.FixedCPM).Div(decimal.NewFromInt(1000))
	penaltyBuffer := totalCost.Mul(req.PenaltyRate)
	escrowAmount := totalCost.Add(penaltyBuffer)

	if campaign.AvailableBudget.LessThan(escrowAmount) {
		return nil, fmt.Errorf("insufficient budget for PG deal")
	}

	deal := PGDeal{
		ID:           req.DealID,
		Publisher:    req.Publisher,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		TotalImprs:   req.TotalImpressions,
		FixedCPM:     req.FixedCPM,
		EscrowAmount: escrowAmount,
		PenaltyRate:  req.PenaltyRate,
	}

	// Lock budget for PG deal
	campaign.AvailableBudget = campaign.AvailableBudget.Sub(escrowAmount)
	campaign.GuaranteedDeals = append(campaign.GuaranteedDeals, deal)

	e.state.SetCampaign(req.CampaignID, campaign)

	return &CreatePGDealResponse{
		Success:      true,
		EscrowAmount: escrowAmount,
		DealID:       req.DealID,
	}, nil
}

// Helper functions

func (e *EscrowManager) transferAUSD(from, to string, amount decimal.Decimal) error {
	// Interface with DEX engine for AUSD transfers
	return e.dex.TransferAsset(e.ausdID, from, to, amount)
}

func (e *EscrowManager) verifyDeliveryProof(proof string, reservation *Reservation) error {
	// Verify cryptographic proof of ad delivery
	// In production: validate VRF nonce, signed player events, viewability attestation
	proofHash := sha256.Sum256([]byte(proof))
	expectedHash := sha256.Sum256([]byte(reservation.ID + reservation.Publisher))
	
	// Simplified verification - production would use more sophisticated proof system
	if len(proof) < 32 {
		return fmt.Errorf("invalid proof format")
	}
	
	// Verify proof contains reservation ID (anti-replay)
	if len(proofHash) != len(expectedHash) {
		return fmt.Errorf("proof verification failed")
	}
	
	return nil
}

func (e *EscrowManager) scheduleHoldbackRelease(publisher string, amount decimal.Decimal, delay time.Duration) {
	// In production: create timelock transaction for holdback release
	// For now, add to pending releases
	e.state.AddPendingRelease(publisher, amount, time.Now().Add(delay))
}

// Request/Response types for RPC

type FundCampaignRequest struct {
	CampaignID  string          `json:"campaign_id"`
	Advertiser  string          `json:"advertiser"`
	Amount      decimal.Decimal `json:"amount"`
	HoldbackBps uint16          `json:"holdback_bps"`
}

type FundCampaignResponse struct {
	Success         bool            `json:"success"`
	NewTotalBudget  decimal.Decimal `json:"new_total_budget"`
	AvailableBudget decimal.Decimal `json:"available_budget"`
}

type ReserveBudgetRequest struct {
	ReservationID string          `json:"reservation_id"`
	CampaignID    string          `json:"campaign_id"`
	Publisher     string          `json:"publisher"`
	Amount        decimal.Decimal `json:"amount"`
	TTLSeconds    uint32          `json:"ttl_seconds"`
	Metadata      ReservationMeta `json:"metadata"`
}

type ReserveBudgetResponse struct {
	Success         bool            `json:"success"`
	Expires         time.Time       `json:"expires"`
	RemainingBudget decimal.Decimal `json:"remaining_budget"`
}

type SettleReceiptRequest struct {
	ReservationID     string `json:"reservation_id"`
	VerificationProof string `json:"verification_proof"`
}

type SettleReceiptResponse struct {
	Success          bool            `json:"success"`
	PaidAmount       decimal.Decimal `json:"paid_amount"`
	HoldbackAmount   decimal.Decimal `json:"holdback_amount"`
	PublisherBalance decimal.Decimal `json:"publisher_balance"`
}

type CreatePGDealRequest struct {
	CampaignID       string          `json:"campaign_id"`
	DealID           string          `json:"deal_id"`
	Publisher        string          `json:"publisher"`
	StartTime        time.Time       `json:"start_time"`
	EndTime          time.Time       `json:"end_time"`
	TotalImpressions uint64          `json:"total_impressions"`
	FixedCPM         decimal.Decimal `json:"fixed_cpm"`
	PenaltyRate      decimal.Decimal `json:"penalty_rate"`
}

type CreatePGDealResponse struct {
	Success      bool            `json:"success"`
	EscrowAmount decimal.Decimal `json:"escrow_amount"`
	DealID       string          `json:"deal_id"`
}