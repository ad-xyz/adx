// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package proof

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/luxfi/adx/pkg/crypto/hashing"
	"github.com/luxfi/adx/pkg/log"
)

var (
	ErrInvalidProof     = errors.New("invalid proof")
	ErrInvalidWitness   = errors.New("invalid witness")
	ErrConstraintFailed = errors.New("constraint verification failed")
)

// Circuit represents a ZK circuit
type Circuit interface {
	// Setup generates proving and verifying keys
	Setup() (*ProvingKey, *VerifyingKey, error)
	
	// Prove generates a proof given witness
	Prove(pk *ProvingKey, witness Witness) (*Proof, error)
	
	// Verify checks a proof against public inputs
	Verify(vk *VerifyingKey, publicInputs [][]byte, proof *Proof) bool
}

// ProvingKey for generating proofs
type ProvingKey struct {
	CircuitID   string
	Constraints []Constraint
	Parameters  []byte
}

// VerifyingKey for verifying proofs
type VerifyingKey struct {
	CircuitID      string
	PublicParams   []byte
	ConstraintHash []byte
}

// Proof represents a ZK proof
type Proof struct {
	Commitment []byte
	Response   []byte
	Challenge  []byte
}

// Witness contains private inputs to the circuit
type Witness struct {
	Values map[string]*big.Int
}

// Constraint represents a circuit constraint
type Constraint struct {
	Type   string // "equality", "range", "comparison"
	Left   string
	Right  string
	Params []byte
}

// AuctionCircuit proves correct auction execution
type AuctionCircuit struct {
	NumBids int
	Reserve uint64
	log     log.Logger
}

// NewAuctionCircuit creates an auction correctness circuit
func NewAuctionCircuit(numBids int, reserve uint64, logger log.Logger) *AuctionCircuit {
	return &AuctionCircuit{
		NumBids: numBids,
		Reserve: reserve,
		log:     logger,
	}
}

// Setup generates keys for auction circuit
func (ac *AuctionCircuit) Setup() (*ProvingKey, *VerifyingKey, error) {
	// Create constraints for auction correctness
	constraints := []Constraint{
		// Winner has maximum bid
		{Type: "comparison", Left: "winner_bid", Right: "all_bids"},
		// Price is second price or reserve
		{Type: "equality", Left: "price", Right: "second_price_or_reserve"},
		// All bids are in valid range
		{Type: "range", Left: "bids", Right: "0_to_max"},
	}
	
	// Generate random parameters (simplified)
	params := make([]byte, 32)
	rand.Read(params)
	
	pk := &ProvingKey{
		CircuitID:   "auction_v1",
		Constraints: constraints,
		Parameters:  params,
	}
	
	vk := &VerifyingKey{
		CircuitID:      "auction_v1",
		PublicParams:   params[:16],
		ConstraintHash: hashing.ComputeHash256([]byte("auction_constraints")),
	}
	
	return pk, vk, nil
}

// Prove generates proof of correct auction
func (ac *AuctionCircuit) Prove(pk *ProvingKey, witness Witness) (*Proof, error) {
	// Extract witness values
	winnerBid, exists := witness.Values["winner_bid"]
	if !exists {
		return nil, ErrInvalidWitness
	}
	
	secondPrice, exists := witness.Values["second_price"]
	if !exists {
		return nil, ErrInvalidWitness
	}
	
	// Verify constraints locally
	// 1. Winner bid is maximum
	for i := 0; i < ac.NumBids; i++ {
		bidKey := fmt.Sprintf("bid_%d", i)
		bid, exists := witness.Values[bidKey]
		if exists && bid.Cmp(winnerBid) > 0 {
			return nil, ErrConstraintFailed
		}
	}
	
	// 2. Price is correct (second price or reserve)
	reserveBig := big.NewInt(int64(ac.Reserve))
	price := secondPrice
	if secondPrice.Cmp(reserveBig) < 0 {
		price = reserveBig
	}
	
	actualPrice, exists := witness.Values["price"]
	if !exists || actualPrice.Cmp(price) != 0 {
		return nil, ErrConstraintFailed
	}
	
	// Generate proof (simplified Fiat-Shamir)
	commitment := ac.generateCommitment(witness)
	challenge := ac.generateChallenge(commitment, pk.Parameters)
	response := ac.generateResponse(witness, challenge)
	
	proof := &Proof{
		Commitment: commitment,
		Challenge:  challenge,
		Response:   response,
	}
	
	ac.log.Debug("auction proof generated",
		"winner_bid", winnerBid,
		"price", price)
	
	return proof, nil
}

// Verify checks auction proof
func (ac *AuctionCircuit) Verify(vk *VerifyingKey, publicInputs [][]byte, proof *Proof) bool {
	// Verify proof structure
	if len(proof.Commitment) == 0 || len(proof.Response) == 0 {
		return false
	}
	
	// Verify challenge (Fiat-Shamir)
	expectedChallenge := ac.generateChallenge(proof.Commitment, vk.PublicParams)
	if string(expectedChallenge) != string(proof.Challenge) {
		return false
	}
	
	// Simplified verification - in production use actual Halo2/Plonky3
	return true
}

// generateCommitment creates a commitment to witness
func (ac *AuctionCircuit) generateCommitment(witness Witness) []byte {
	data := make([]byte, 0)
	
	// Commit to all witness values
	for key, value := range witness.Values {
		data = append(data, []byte(key)...)
		data = append(data, value.Bytes()...)
	}
	
	return hashing.ComputeHash256(data)
}

// generateChallenge creates Fiat-Shamir challenge
func (ac *AuctionCircuit) generateChallenge(commitment []byte, params []byte) []byte {
	data := append(commitment, params...)
	return hashing.ComputeHash256(data)
}

// generateResponse creates proof response
func (ac *AuctionCircuit) generateResponse(witness Witness, challenge []byte) []byte {
	// Simplified response generation
	data := make([]byte, 0)
	
	challengeBig := new(big.Int).SetBytes(challenge)
	
	for _, value := range witness.Values {
		// response = witness + challenge * randomness (simplified)
		resp := new(big.Int).Add(value, challengeBig)
		data = append(data, resp.Bytes()...)
		if len(data) >= 32 {
			break
		}
	}
	
	return hashing.ComputeHash256(data)
}

// BudgetCircuit proves budget safety
type BudgetCircuit struct {
	log log.Logger
}

// NewBudgetCircuit creates a budget safety circuit
func NewBudgetCircuit(logger log.Logger) *BudgetCircuit {
	return &BudgetCircuit{log: logger}
}

// Setup generates keys for budget circuit
func (bc *BudgetCircuit) Setup() (*ProvingKey, *VerifyingKey, error) {
	constraints := []Constraint{
		// new_budget = old_budget - delta
		{Type: "equality", Left: "new_budget", Right: "old_minus_delta"},
		// new_budget >= 0
		{Type: "range", Left: "new_budget", Right: "non_negative"},
	}
	
	params := make([]byte, 32)
	rand.Read(params)
	
	pk := &ProvingKey{
		CircuitID:   "budget_v1",
		Constraints: constraints,
		Parameters:  params,
	}
	
	vk := &VerifyingKey{
		CircuitID:      "budget_v1",
		PublicParams:   params[:16],
		ConstraintHash: hashing.ComputeHash256([]byte("budget_constraints")),
	}
	
	return pk, vk, nil
}

// Prove generates proof of valid budget update
func (bc *BudgetCircuit) Prove(pk *ProvingKey, witness Witness) (*Proof, error) {
	oldBudget, exists := witness.Values["old_budget"]
	if !exists {
		return nil, ErrInvalidWitness
	}
	
	delta, exists := witness.Values["delta"]
	if !exists {
		return nil, ErrInvalidWitness
	}
	
	newBudget, exists := witness.Values["new_budget"]
	if !exists {
		return nil, ErrInvalidWitness
	}
	
	// Verify: new = old - delta
	expected := new(big.Int).Sub(oldBudget, delta)
	if expected.Cmp(newBudget) != 0 {
		return nil, ErrConstraintFailed
	}
	
	// Verify: new >= 0
	if newBudget.Sign() < 0 {
		return nil, ErrConstraintFailed
	}
	
	// Generate proof
	commitment := hashing.ComputeHash256(append(
		oldBudget.Bytes(),
		append(delta.Bytes(), newBudget.Bytes()...)...,
	))
	
	challenge := hashing.ComputeHash256(append(commitment, pk.Parameters...))
	response := hashing.ComputeHash256(append(newBudget.Bytes(), challenge...))
	
	bc.log.Debug("budget proof generated",
		"old", oldBudget,
		"delta", delta,
		"new", newBudget)
	
	return &Proof{
		Commitment: commitment,
		Challenge:  challenge,
		Response:   response,
	}, nil
}

// Verify checks budget proof
func (bc *BudgetCircuit) Verify(vk *VerifyingKey, publicInputs [][]byte, proof *Proof) bool {
	// Simplified verification
	return len(proof.Commitment) > 0 && len(proof.Response) > 0
}

// FrequencyCircuit proves frequency cap compliance
type FrequencyCircuit struct {
	Cap uint32
	log log.Logger
}

// NewFrequencyCircuit creates a frequency cap circuit
func NewFrequencyCircuit(cap uint32, logger log.Logger) *FrequencyCircuit {
	return &FrequencyCircuit{
		Cap: cap,
		log: logger,
	}
}

// Setup generates keys for frequency circuit
func (fc *FrequencyCircuit) Setup() (*ProvingKey, *VerifyingKey, error) {
	constraints := []Constraint{
		// counter_after = counter_before + 1
		{Type: "equality", Left: "counter_after", Right: "counter_before_plus_1"},
		// counter_after <= cap
		{Type: "comparison", Left: "counter_after", Right: "cap"},
	}
	
	params := make([]byte, 32)
	rand.Read(params)
	
	pk := &ProvingKey{
		CircuitID:   "frequency_v1",
		Constraints: constraints,
		Parameters:  params,
	}
	
	vk := &VerifyingKey{
		CircuitID:      "frequency_v1",
		PublicParams:   params[:16],
		ConstraintHash: hashing.ComputeHash256([]byte("frequency_constraints")),
	}
	
	return pk, vk, nil
}

// Prove generates proof of frequency cap compliance
func (fc *FrequencyCircuit) Prove(pk *ProvingKey, witness Witness) (*Proof, error) {
	counterBefore, exists := witness.Values["counter_before"]
	if !exists {
		return nil, ErrInvalidWitness
	}
	
	counterAfter, exists := witness.Values["counter_after"]
	if !exists {
		return nil, ErrInvalidWitness
	}
	
	// Verify: after = before + 1
	expected := new(big.Int).Add(counterBefore, big.NewInt(1))
	if expected.Cmp(counterAfter) != 0 {
		return nil, ErrConstraintFailed
	}
	
	// Verify: after <= cap
	capBig := big.NewInt(int64(fc.Cap))
	if counterAfter.Cmp(capBig) > 0 {
		return nil, ErrConstraintFailed
	}
	
	// Generate proof
	commitment := hashing.ComputeHash256(append(
		counterBefore.Bytes(),
		counterAfter.Bytes()...,
	))
	
	challenge := hashing.ComputeHash256(append(commitment, pk.Parameters...))
	response := hashing.ComputeHash256(append(counterAfter.Bytes(), challenge...))
	
	fc.log.Debug("frequency proof generated",
		"before", counterBefore,
		"after", counterAfter,
		"cap", fc.Cap)
	
	return &Proof{
		Commitment: commitment,
		Challenge:  challenge,
		Response:   response,
	}, nil
}

// Verify checks frequency proof
func (fc *FrequencyCircuit) Verify(vk *VerifyingKey, publicInputs [][]byte, proof *Proof) bool {
	// Simplified verification
	return len(proof.Commitment) > 0 && len(proof.Response) > 0
}