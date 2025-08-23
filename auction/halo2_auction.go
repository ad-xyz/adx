// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auction

import (
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/luxfi/adx/proof/halo2"
	"github.com/luxfi/adx/pkg/ids"
	"github.com/luxfi/adx/pkg/log"
)

var (
	ErrCircuitNotSetup = errors.New("circuit not initialized")
	ErrProofGenFailed  = errors.New("proof generation failed")
)

// Halo2Auction represents an auction with Halo2 ZK proofs
type Halo2Auction struct {
	*Auction
	
	// Halo2 circuits
	circuit *halo2.AuctionCircuit
	pk      *halo2.ProvingKey
	vk      *halo2.VerifyingKey
	
	// Proof storage
	proofs  map[ids.ID]*halo2.Halo2Proof
	mu      sync.RWMutex
}

// NewHalo2Auction creates an auction with Halo2 proof support
func NewHalo2Auction(
	auctionID ids.ID,
	reserve uint64,
	duration time.Duration,
	logger log.Logger,
) (*Halo2Auction, error) {
	// Create base auction
	baseAuction := NewAuction(auctionID, reserve, duration, logger)
	
	// Create Halo2 circuit with estimated max bids
	maxBids := 100
	circuit := halo2.NewAuctionCircuit(maxBids, reserve, logger)
	
	// Setup circuit (generate SRS)
	pk, vk, err := circuit.Setup()
	if err != nil {
		return nil, err
	}
	
	return &Halo2Auction{
		Auction: baseAuction,
		circuit: circuit,
		pk:      pk,
		vk:      vk,
		proofs:  make(map[ids.ID]*halo2.Halo2Proof),
	}, nil
}

// RunAuctionWithHalo2 runs the auction and generates Halo2 proof
func (ha *Halo2Auction) RunAuctionWithHalo2(decryptionKey []byte) (*Halo2AuctionOutcome, error) {
	if ha.circuit == nil || ha.pk == nil {
		return nil, ErrCircuitNotSetup
	}
	
	ha.mu.Lock()
	defer ha.mu.Unlock()
	
	// Run base auction to get outcome
	outcome, err := ha.RunAuction(decryptionKey)
	if err != nil {
		return nil, err
	}
	
	// Prepare witness for Halo2 proof
	witness, err := ha.prepareWitness(outcome)
	if err != nil {
		return nil, err
	}
	
	// Generate Halo2 proof
	proof, err := ha.circuit.Prove(ha.pk, witness)
	if err != nil {
		ha.log.Error("Halo2 proof generation failed", "error", err)
		return nil, ErrProofGenFailed
	}
	
	// Store proof
	proofID := ids.GenerateTestID()
	ha.proofs[proofID] = proof
	
	// Create public inputs for verification
	publicInputs := &halo2.AuctionPublicInputs{
		NumBids:       len(ha.Bids),
		Reserve:       ha.Reserve,
		ClearingPrice: outcome.ClearingPrice,
		WinnerCommit:  proof.WitnessCommitments[len(ha.Bids)],
	}
	
	// Verify proof
	valid := ha.circuit.Verify(ha.vk, publicInputs, proof)
	if !valid {
		ha.log.Error("Halo2 proof verification failed")
		return nil, errors.New("proof verification failed")
	}
	
	ha.log.Info("Halo2 auction completed",
		"auction_id", ha.ID,
		"winner", outcome.WinnerID,
		"price", outcome.ClearingPrice,
		"proof_id", proofID,
		"proof_size", len(proof.OpeningProof))
	
	return &Halo2AuctionOutcome{
		AuctionOutcome: *outcome,
		Halo2Proof:     proof,
		ProofID:        proofID,
		VerifyingKey:   ha.vk,
	}, nil
}

// prepareWitness converts auction data to Halo2 witness format
func (ha *Halo2Auction) prepareWitness(outcome *AuctionOutcome) (*halo2.AuctionWitness, error) {
	// Decrypt all bids
	decryptedBids := make([]*big.Int, 0, len(ha.Bids))
	winnerIndex := -1
	secondHighest := big.NewInt(0)
	
	for i, sealedBid := range ha.Bids {
		// In production, decrypt actual bid values
		// For now, use simulated values
		bidValue := big.NewInt(int64(100 + i*50))
		decryptedBids = append(decryptedBids, bidValue)
		
		if sealedBid.BidderID == outcome.WinnerID {
			winnerIndex = i
		}
		
		// Track second highest
		if bidValue.Cmp(big.NewInt(int64(outcome.WinningBid))) < 0 {
			if bidValue.Cmp(secondHighest) > 0 {
				secondHighest = bidValue
			}
		}
	}
	
	// Pad bids to match circuit size
	for len(decryptedBids) < ha.circuit.NumBids {
		decryptedBids = append(decryptedBids, big.NewInt(0))
	}
	
	// If second highest is less than reserve, use reserve
	if secondHighest.Cmp(big.NewInt(int64(ha.Reserve))) < 0 {
		secondHighest = big.NewInt(int64(ha.Reserve))
	}
	
	return &halo2.AuctionWitness{
		Bids:          decryptedBids,
		WinnerIndex:   winnerIndex,
		WinningBid:    big.NewInt(int64(outcome.WinningBid)),
		SecondPrice:   secondHighest,
		ClearingPrice: big.NewInt(int64(outcome.ClearingPrice)),
	}, nil
}

// VerifyHalo2Proof verifies a Halo2 auction proof
func (ha *Halo2Auction) VerifyHalo2Proof(
	proofID ids.ID,
	publicInputs *halo2.AuctionPublicInputs,
) bool {
	ha.mu.RLock()
	defer ha.mu.RUnlock()
	
	proof, exists := ha.proofs[proofID]
	if !exists {
		ha.log.Debug("Proof not found", "proof_id", proofID)
		return false
	}
	
	return ha.circuit.Verify(ha.vk, publicInputs, proof)
}

// GetVerifyingKey returns the verifying key for external verification
func (ha *Halo2Auction) GetVerifyingKey() *halo2.VerifyingKey {
	return ha.vk
}

// Halo2AuctionOutcome represents auction outcome with Halo2 proof
type Halo2AuctionOutcome struct {
	AuctionOutcome
	Halo2Proof   *halo2.Halo2Proof
	ProofID      ids.ID
	VerifyingKey *halo2.VerifyingKey
}

// Halo2BudgetManager manages budgets with Halo2 proofs
type Halo2BudgetManager struct {
	mu sync.RWMutex
	
	// Budget storage
	budgets map[ids.ID]*big.Int
	
	// Halo2 circuit
	circuit *halo2.BudgetCircuit
	pk      *halo2.ProvingKey
	vk      *halo2.VerifyingKey
	
	// Proof storage
	proofs map[ids.ID]*halo2.Halo2Proof
	
	log log.Logger
}

// NewHalo2BudgetManager creates a budget manager with Halo2 proofs
func NewHalo2BudgetManager(logger log.Logger) (*Halo2BudgetManager, error) {
	circuit := halo2.NewBudgetCircuit(logger)
	
	// Setup circuit
	pk, vk, err := circuit.Setup()
	if err != nil {
		return nil, err
	}
	
	return &Halo2BudgetManager{
		budgets: make(map[ids.ID]*big.Int),
		circuit: circuit,
		pk:      pk,
		vk:      vk,
		proofs:  make(map[ids.ID]*halo2.Halo2Proof),
		log:     logger,
	}, nil
}

// DeductBudgetWithProof deducts budget and generates Halo2 proof
func (bm *Halo2BudgetManager) DeductBudgetWithProof(
	advertiserID ids.ID,
	amount uint64,
) (*Halo2BudgetProof, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	// Get current budget
	currentBudget, exists := bm.budgets[advertiserID]
	if !exists {
		currentBudget = big.NewInt(10000) // Default budget
		bm.budgets[advertiserID] = currentBudget
	}
	
	// Calculate new budget
	delta := big.NewInt(int64(amount))
	newBudget := new(big.Int).Sub(currentBudget, delta)
	
	// Check if budget would go negative
	if newBudget.Sign() < 0 {
		return nil, errors.New("insufficient budget")
	}
	
	// Create witness
	witness := &halo2.BudgetWitness{
		OldBudget: new(big.Int).Set(currentBudget),
		Delta:     delta,
		NewBudget: newBudget,
	}
	
	// Generate proof
	proof, err := bm.circuit.Prove(bm.pk, witness)
	if err != nil {
		return nil, err
	}
	
	// Update budget
	bm.budgets[advertiserID] = newBudget
	
	// Store proof
	proofID := ids.GenerateTestID()
	bm.proofs[proofID] = proof
	
	bm.log.Info("Budget deducted with Halo2 proof",
		"advertiser", advertiserID,
		"amount", amount,
		"new_budget", newBudget,
		"proof_id", proofID)
	
	return &Halo2BudgetProof{
		ProofID:         proofID,
		AdvertiserID:    advertiserID,
		Delta:           amount,
		NewBudget:       newBudget.Uint64(),
		Halo2Proof:      proof,
		OldBudgetCommit: proof.WitnessCommitments[0],
		NewBudgetCommit: proof.WitnessCommitments[2],
	}, nil
}

// VerifyBudgetProof verifies a Halo2 budget proof
func (bm *Halo2BudgetManager) VerifyBudgetProof(proofData *Halo2BudgetProof) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	
	publicInputs := &halo2.BudgetPublicInputs{
		Delta:           proofData.Delta,
		OldBudgetCommit: proofData.OldBudgetCommit,
		NewBudgetCommit: proofData.NewBudgetCommit,
	}
	
	return bm.circuit.Verify(bm.vk, publicInputs, proofData.Halo2Proof)
}

// Halo2BudgetProof represents a budget update with Halo2 proof
type Halo2BudgetProof struct {
	ProofID         ids.ID
	AdvertiserID    ids.ID
	Delta           uint64
	NewBudget       uint64
	Halo2Proof      *halo2.Halo2Proof
	OldBudgetCommit []byte
	NewBudgetCommit []byte
}

// Halo2FrequencyManager manages frequency caps with Halo2 proofs
type Halo2FrequencyManager struct {
	mu sync.RWMutex
	
	// Counter storage
	counters map[string]*big.Int
	caps     map[ids.ID]uint32
	
	// Halo2 circuits (one per cap value)
	circuits map[uint32]*halo2.FrequencyCircuit
	pks      map[uint32]*halo2.ProvingKey
	vks      map[uint32]*halo2.VerifyingKey
	
	// Proof storage
	proofs map[ids.ID]*halo2.Halo2Proof
	
	log log.Logger
}

// NewHalo2FrequencyManager creates a frequency manager with Halo2 proofs
func NewHalo2FrequencyManager(logger log.Logger) *Halo2FrequencyManager {
	return &Halo2FrequencyManager{
		counters: make(map[string]*big.Int),
		caps:     make(map[ids.ID]uint32),
		circuits: make(map[uint32]*halo2.FrequencyCircuit),
		pks:      make(map[uint32]*halo2.ProvingKey),
		vks:      make(map[uint32]*halo2.VerifyingKey),
		proofs:   make(map[ids.ID]*halo2.Halo2Proof),
		log:      logger,
	}
}

// CheckAndIncrementWithProof checks frequency cap and generates Halo2 proof
func (fm *Halo2FrequencyManager) CheckAndIncrementWithProof(
	deviceID string,
	campaignID ids.ID,
	cap uint32,
) (*Halo2FrequencyProof, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	
	// Get or create circuit for this cap value
	circuit, exists := fm.circuits[cap]
	if !exists {
		circuit = halo2.NewFrequencyCircuit(cap, fm.log)
		pk, vk, err := circuit.Setup()
		if err != nil {
			return nil, err
		}
		fm.circuits[cap] = circuit
		fm.pks[cap] = pk
		fm.vks[cap] = vk
	}
	
	// Get current counter
	key := deviceID + "_" + campaignID.String()
	counter, exists := fm.counters[key]
	if !exists {
		counter = big.NewInt(0)
		fm.counters[key] = counter
	}
	
	// Check if cap would be exceeded
	if counter.Cmp(big.NewInt(int64(cap))) >= 0 {
		return nil, errors.New("frequency cap exceeded")
	}
	
	// Create witness
	newCounter := new(big.Int).Add(counter, big.NewInt(1))
	witness := &halo2.FrequencyWitness{
		CounterBefore: new(big.Int).Set(counter),
		CounterAfter:  newCounter,
		CampaignID:    campaignID,
	}
	
	// Generate proof
	proof, err := circuit.Prove(fm.pks[cap], witness)
	if err != nil {
		return nil, err
	}
	
	// Update counter
	fm.counters[key] = newCounter
	fm.caps[campaignID] = cap
	
	// Store proof
	proofID := ids.GenerateTestID()
	fm.proofs[proofID] = proof
	
	fm.log.Debug("Frequency incremented with Halo2 proof",
		"device", deviceID,
		"campaign", campaignID,
		"new_count", newCounter,
		"cap", cap,
		"proof_id", proofID)
	
	return &Halo2FrequencyProof{
		ProofID:     proofID,
		CampaignID:  campaignID,
		Cap:         cap,
		NewCounter:  uint32(newCounter.Int64()),
		Halo2Proof:  proof,
		CounterRoot: proof.WitnessCommitments[1],
	}, nil
}

// VerifyFrequencyProof verifies a Halo2 frequency proof
func (fm *Halo2FrequencyManager) VerifyFrequencyProof(proofData *Halo2FrequencyProof) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	
	// Get verifying key for this cap
	vk, exists := fm.vks[proofData.Cap]
	if !exists {
		fm.log.Debug("No verifying key for cap", "cap", proofData.Cap)
		return false
	}
	
	circuit, exists := fm.circuits[proofData.Cap]
	if !exists {
		return false
	}
	
	publicInputs := &halo2.FrequencyPublicInputs{
		Cap:         proofData.Cap,
		CampaignID:  proofData.CampaignID,
		CounterRoot: proofData.CounterRoot,
	}
	
	return circuit.Verify(vk, publicInputs, proofData.Halo2Proof)
}

// Halo2FrequencyProof represents a frequency update with Halo2 proof
type Halo2FrequencyProof struct {
	ProofID     ids.ID
	CampaignID  ids.ID
	Cap         uint32
	NewCounter  uint32
	Halo2Proof  *halo2.Halo2Proof
	CounterRoot []byte
}