// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package settlement

import (
	"errors"
	"sync"
	"time"

	"github.com/luxfi/adx/core"
	"github.com/luxfi/adx/crypto"
	"github.com/luxfi/adx/pkg/ids"
	"github.com/luxfi/adx/pkg/log"
)

var (
	ErrInsufficientBudget = errors.New("insufficient budget")
	ErrInvalidProof       = errors.New("invalid settlement proof")
	ErrNegativeDelta      = errors.New("negative budget delta")
)

// BudgetManager manages advertiser budgets with privacy
type BudgetManager struct {
	mu       sync.RWMutex
	budgets  map[ids.ID]*Budget
	pending  map[ids.ID]uint64 // Pending spend not yet settled
	receipts []*SettlementReceipt
	log      log.Logger
}

// Budget represents an advertiser's budget state
type Budget struct {
	AdvertiserID ids.ID
	Total        uint64
	Spent        uint64
	Remaining    uint64
	Commitment   []byte // Commitment to budget state
	LastUpdated  time.Time
}

// SettlementReceipt represents a payment settlement
type SettlementReceipt struct {
	AdvertiserID ids.ID
	PublisherID  ids.ID
	Amount       uint64
	Period       time.Time
	Proof        []byte // ZK proof of correct settlement
}

// NewBudgetManager creates a new budget manager
func NewBudgetManager(logger log.Logger) *BudgetManager {
	return &BudgetManager{
		budgets:  make(map[ids.ID]*Budget),
		pending:  make(map[ids.ID]uint64),
		receipts: make([]*SettlementReceipt, 0),
		log:      logger,
	}
}

// SetBudget sets the initial budget for an advertiser
func (bm *BudgetManager) SetBudget(advertiserID ids.ID, amount uint64) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	budget := &Budget{
		AdvertiserID: advertiserID,
		Total:        amount,
		Spent:        0,
		Remaining:    amount,
		LastUpdated:  time.Now(),
	}
	
	// Create commitment to initial budget
	budget.Commitment = bm.createBudgetCommitment(budget)
	
	bm.budgets[advertiserID] = budget
	
	bm.log.Info("budget set",
		"advertiser", advertiserID,
		"amount", amount)
	
	return nil
}

// DeductBudget deducts from budget with ZK proof
func (bm *BudgetManager) DeductBudget(
	advertiserID ids.ID,
	amount uint64,
	auctionRef ids.ID,
) (*BudgetProof, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	budget, exists := bm.budgets[advertiserID]
	if !exists {
		return nil, errors.New("budget not found")
	}
	
	if budget.Remaining < amount {
		return nil, ErrInsufficientBudget
	}
	
	// Store previous commitment
	prevCommitment := budget.Commitment
	
	// Update budget
	budget.Spent += amount
	budget.Remaining -= amount
	budget.LastUpdated = time.Now()
	
	// Create new commitment
	newCommitment := bm.createBudgetCommitment(budget)
	budget.Commitment = newCommitment
	
	// Generate ZK proof of valid deduction
	proof := bm.generateBudgetProof(
		prevCommitment,
		newCommitment,
		amount,
		budget.Remaining,
	)
	
	// Track pending spend
	bm.pending[advertiserID] += amount
	
	bm.log.Debug("budget deducted",
		"advertiser", advertiserID,
		"amount", amount,
		"remaining", budget.Remaining)
	
	return proof, nil
}

// BudgetProof proves budget operations are valid
type BudgetProof struct {
	CmBudgetPrev []byte `json:"cm_budget_prev"`
	CmBudgetNew  []byte `json:"cm_budget_new"`
	ProofDelta   []byte `json:"proof_delta"`
	Timestamp    time.Time `json:"timestamp"`
}

// createBudgetCommitment creates a commitment to budget state
func (bm *BudgetManager) createBudgetCommitment(budget *Budget) []byte {
	// Commit to (total, spent, remaining)
	data := make([]byte, 24)
	
	// Total (8 bytes)
	for i := 0; i < 8; i++ {
		data[i] = byte(budget.Total >> (8 * (7 - i)))
	}
	
	// Spent (8 bytes)
	for i := 0; i < 8; i++ {
		data[8+i] = byte(budget.Spent >> (8 * (7 - i)))
	}
	
	// Remaining (8 bytes)
	for i := 0; i < 8; i++ {
		data[16+i] = byte(budget.Remaining >> (8 * (7 - i)))
	}
	
	return crypto.CreateCommitment(data)
}

// generateBudgetProof generates a ZK proof of valid budget update
func (bm *BudgetManager) generateBudgetProof(
	prevCommitment []byte,
	newCommitment []byte,
	delta uint64,
	remaining uint64,
) *BudgetProof {
	// Simplified proof generation
	// In production, use actual ZK proving system
	// Proves:
	// 1. new_remaining = prev_remaining - delta
	// 2. new_remaining >= 0
	// 3. Commitments are correctly formed
	
	proofData := make([]byte, 16)
	
	// Delta (8 bytes)
	for i := 0; i < 8; i++ {
		proofData[i] = byte(delta >> (8 * (7 - i)))
	}
	
	// Remaining (8 bytes)
	for i := 0; i < 8; i++ {
		proofData[8+i] = byte(remaining >> (8 * (7 - i)))
	}
	
	return &BudgetProof{
		CmBudgetPrev: prevCommitment,
		CmBudgetNew:  newCommitment,
		ProofDelta:   crypto.CreateCommitment(proofData),
		Timestamp:    time.Now(),
	}
}

// CreateSettlement creates a settlement between advertiser and publisher
func (bm *BudgetManager) CreateSettlement(
	advertiserID ids.ID,
	publisherID ids.ID,
	period time.Time,
) (*SettlementReceipt, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	// Get pending amount for advertiser
	amount, exists := bm.pending[advertiserID]
	if !exists || amount == 0 {
		return nil, errors.New("no pending settlement")
	}
	
	// Generate settlement proof
	proof := bm.generateSettlementProof(advertiserID, publisherID, amount, period)
	
	receipt := &SettlementReceipt{
		AdvertiserID: advertiserID,
		PublisherID:  publisherID,
		Amount:       amount,
		Period:       period,
		Proof:        proof,
	}
	
	// Clear pending amount
	delete(bm.pending, advertiserID)
	
	// Store receipt
	bm.receipts = append(bm.receipts, receipt)
	
	bm.log.Info("settlement created",
		"advertiser", advertiserID,
		"publisher", publisherID,
		"amount", amount)
	
	return receipt, nil
}

// generateSettlementProof generates a ZK proof of correct settlement
func (bm *BudgetManager) generateSettlementProof(
	advertiserID ids.ID,
	publisherID ids.ID,
	amount uint64,
	period time.Time,
) []byte {
	// Simplified proof generation
	// In production, use actual ZK proving system
	// Proves: amount == sum of accepted auction prices over period
	
	data := make([]byte, 0)
	data = append(data, advertiserID[:]...)
	data = append(data, publisherID[:]...)
	
	// Amount (8 bytes)
	amountBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		amountBytes[i] = byte(amount >> (8 * (7 - i)))
	}
	data = append(data, amountBytes...)
	
	// Period timestamp (8 bytes)
	periodBytes := make([]byte, 8)
	periodUnix := period.Unix()
	for i := 0; i < 8; i++ {
		periodBytes[i] = byte(periodUnix >> (8 * (7 - i)))
	}
	data = append(data, periodBytes...)
	
	return crypto.CreateCommitment(data)
}

// CreateBudgetHeader creates a header for budget update
func (bm *BudgetManager) CreateBudgetHeader(
	advertiserID ids.ID,
	proof *BudgetProof,
) *core.BudgetHeader {
	return &core.BudgetHeader{
		BaseHeader: core.BaseHeader{
			Type:      core.HeaderTypeBudget,
			ID:        ids.GenerateTestID(),
			Timestamp: proof.Timestamp,
			Height:    1,
		},
		AdvertiserID: advertiserID,
		CmBudgetPrev: proof.CmBudgetPrev,
		CmBudgetNew:  proof.CmBudgetNew,
		ProofDelta:   proof.ProofDelta,
	}
}

// CreateSettlementHeader creates a header for settlement
func (bm *BudgetManager) CreateSettlementHeader(
	receipt *SettlementReceipt,
) *core.SettlementHeader {
	// Create commitment to settlement amount
	amountBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		amountBytes[i] = byte(receipt.Amount >> (8 * (7 - i)))
	}
	cmAmount := crypto.CreateCommitment(amountBytes)
	
	return &core.SettlementHeader{
		BaseHeader: core.BaseHeader{
			Type:      core.HeaderTypeSettlement,
			ID:        ids.GenerateTestID(),
			Timestamp: time.Now(),
			Height:    1,
		},
		AdvertiserID: receipt.AdvertiserID,
		PublisherID:  receipt.PublisherID,
		CmAmount:     cmAmount,
		ProofSettle:  receipt.Proof,
	}
}

// VerifyBudgetProof verifies a budget update proof
func VerifyBudgetProof(proof *BudgetProof) bool {
	// Simplified verification
	// In production, use actual ZK proof verification
	return len(proof.ProofDelta) > 0 &&
		len(proof.CmBudgetPrev) > 0 &&
		len(proof.CmBudgetNew) > 0
}

// VerifySettlementProof verifies a settlement proof
func VerifySettlementProof(proof []byte) bool {
	// Simplified verification
	// In production, use actual ZK proof verification
	return len(proof) > 0
}