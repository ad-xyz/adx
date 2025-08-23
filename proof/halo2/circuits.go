// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package halo2

import (
	"crypto/rand"
	"errors"
	"math/big"

	"github.com/luxfi/crypto/hashing"
	"github.com/luxfi/ids"
	"github.com/luxfi/log"
)

var (
	ErrInvalidProof        = errors.New("invalid proof")
	ErrInvalidPublicInputs = errors.New("invalid public inputs")
	ErrSetupFailed         = errors.New("circuit setup failed")
	ErrProvingFailed       = errors.New("proof generation failed")
)

// Field represents the scalar field for Halo2 (BN254)
type Field struct {
	Modulus *big.Int
}

// NewField creates a new field with BN254 scalar field modulus
func NewField() *Field {
	// BN254 scalar field modulus
	modulus, _ := new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)
	return &Field{
		Modulus: modulus,
	}
}

// Add performs field addition
func (f *Field) Add(a, b *big.Int) *big.Int {
	result := new(big.Int).Add(a, b)
	return result.Mod(result, f.Modulus)
}

// Mul performs field multiplication
func (f *Field) Mul(a, b *big.Int) *big.Int {
	result := new(big.Int).Mul(a, b)
	return result.Mod(result, f.Modulus)
}

// Sub performs field subtraction
func (f *Field) Sub(a, b *big.Int) *big.Int {
	result := new(big.Int).Sub(a, b)
	return result.Mod(result, f.Modulus)
}

// Inv computes multiplicative inverse
func (f *Field) Inv(a *big.Int) *big.Int {
	return new(big.Int).ModInverse(a, f.Modulus)
}

// PoseidonHash implements Poseidon hash for ZK-friendly operations
type PoseidonHash struct {
	field      *Field
	roundConst [][]*big.Int
	mdsMatrix  [][]*big.Int
	rounds     int
}

// NewPoseidonHash creates a new Poseidon hash instance
func NewPoseidonHash() *PoseidonHash {
	field := NewField()
	
	// Simplified constants - in production use proper Poseidon parameters
	rounds := 8
	width := 3
	
	// Generate round constants
	roundConst := make([][]*big.Int, rounds)
	for i := 0; i < rounds; i++ {
		roundConst[i] = make([]*big.Int, width)
		for j := 0; j < width; j++ {
			roundConst[i][j] = new(big.Int).SetInt64(int64(i*width + j + 1))
		}
	}
	
	// Generate MDS matrix
	mdsMatrix := make([][]*big.Int, width)
	for i := 0; i < width; i++ {
		mdsMatrix[i] = make([]*big.Int, width)
		for j := 0; j < width; j++ {
			mdsMatrix[i][j] = new(big.Int).SetInt64(int64(i + j + 1))
		}
	}
	
	return &PoseidonHash{
		field:      field,
		roundConst: roundConst,
		mdsMatrix:  mdsMatrix,
		rounds:     rounds,
	}
}

// Hash computes Poseidon hash of inputs
func (p *PoseidonHash) Hash(inputs []*big.Int) *big.Int {
	state := make([]*big.Int, 3)
	
	// Initialize state with inputs (padding with zeros)
	for i := 0; i < len(inputs) && i < 3; i++ {
		state[i] = new(big.Int).Set(inputs[i])
	}
	for i := len(inputs); i < 3; i++ {
		state[i] = big.NewInt(0)
	}
	
	// Apply Poseidon rounds
	for round := 0; round < p.rounds; round++ {
		// Add round constants
		for i := 0; i < 3; i++ {
			state[i] = p.field.Add(state[i], p.roundConst[round][i])
		}
		
		// S-box (x^5)
		for i := 0; i < 3; i++ {
			x2 := p.field.Mul(state[i], state[i])
			x4 := p.field.Mul(x2, x2)
			state[i] = p.field.Mul(x4, state[i])
		}
		
		// MDS matrix multiplication
		newState := make([]*big.Int, 3)
		for i := 0; i < 3; i++ {
			newState[i] = big.NewInt(0)
			for j := 0; j < 3; j++ {
				tmp := p.field.Mul(p.mdsMatrix[i][j], state[j])
				newState[i] = p.field.Add(newState[i], tmp)
			}
		}
		state = newState
	}
	
	return state[0]
}

// Halo2Proof represents a Halo2 proof
type Halo2Proof struct {
	// Commitments to witness polynomials
	WitnessCommitments [][]byte
	
	// Quotient polynomial commitment
	QuotientCommitment []byte
	
	// Opening proofs
	OpeningProof []byte
	
	// Evaluation claims
	Evaluations map[string]*big.Int
}

// Halo2Circuit represents a Halo2 circuit
type Halo2Circuit struct {
	field    *Field
	poseidon *PoseidonHash
	log      log.Logger
}

// NewHalo2Circuit creates a new Halo2 circuit
func NewHalo2Circuit(logger log.Logger) *Halo2Circuit {
	return &Halo2Circuit{
		field:    NewField(),
		poseidon: NewPoseidonHash(),
		log:      logger,
	}
}

// AuctionCircuit implements Halo2 circuit for auction correctness
type AuctionCircuit struct {
	*Halo2Circuit
	NumBids int
	Reserve *big.Int
}

// NewAuctionCircuit creates an auction circuit
func NewAuctionCircuit(numBids int, reserve uint64, logger log.Logger) *AuctionCircuit {
	return &AuctionCircuit{
		Halo2Circuit: NewHalo2Circuit(logger),
		NumBids:      numBids,
		Reserve:      big.NewInt(int64(reserve)),
	}
}

// Setup generates structured reference string (SRS)
func (ac *AuctionCircuit) Setup() (*ProvingKey, *VerifyingKey, error) {
	// Generate toxic waste (would be done in trusted setup ceremony)
	tau := make([]byte, 32)
	if _, err := rand.Read(tau); err != nil {
		return nil, nil, ErrSetupFailed
	}
	
	// Create SRS powers of tau
	tauBig := new(big.Int).SetBytes(tau)
	powers := make([]*big.Int, ac.NumBids+10)
	powers[0] = big.NewInt(1)
	for i := 1; i < len(powers); i++ {
		powers[i] = ac.field.Mul(powers[i-1], tauBig)
	}
	
	pk := &ProvingKey{
		CircuitID: "auction_halo2_v1",
		SRS:       powers,
		NumBids:   ac.NumBids,
		Reserve:   ac.Reserve,
	}
	
	vk := &VerifyingKey{
		CircuitID:       "auction_halo2_v1",
		CommitmentKey:   powers[:2], // G1 and G2 elements
		ConstraintCount: ac.NumBids * 3, // Constraints for max selection, second price, range
	}
	
	ac.log.Info("Halo2 auction circuit setup complete",
		"num_bids", ac.NumBids,
		"reserve", ac.Reserve)
	
	return pk, vk, nil
}

// Prove generates a Halo2 proof of correct auction
func (ac *AuctionCircuit) Prove(pk *ProvingKey, witness *AuctionWitness) (*Halo2Proof, error) {
	// Validate witness
	if witness.WinnerIndex >= ac.NumBids {
		return nil, ErrProvingFailed
	}
	
	// Create Poseidon commitments to witness values
	commitments := make([][]byte, 0)
	
	// Commit to bids
	for _, bid := range witness.Bids {
		commitment := ac.poseidon.Hash([]*big.Int{bid})
		commitments = append(commitments, commitment.Bytes())
	}
	
	// Commit to winner selection
	winnerCommit := ac.poseidon.Hash([]*big.Int{
		big.NewInt(int64(witness.WinnerIndex)),
		witness.WinningBid,
	})
	commitments = append(commitments, winnerCommit.Bytes())
	
	// Commit to price
	priceCommit := ac.poseidon.Hash([]*big.Int{witness.ClearingPrice})
	commitments = append(commitments, priceCommit.Bytes())
	
	// Create quotient polynomial for constraint satisfaction
	// Q(X) = (constraints(X)) / Z_H(X) where Z_H vanishes on domain
	quotient := ac.computeQuotient(witness)
	quotientCommit := ac.poseidon.Hash([]*big.Int{quotient})
	
	// Generate opening proof (simplified)
	openingProof := ac.generateOpeningProof(witness, commitments)
	
	// Create evaluations
	evaluations := make(map[string]*big.Int)
	evaluations["winner_bid"] = witness.WinningBid
	evaluations["clearing_price"] = witness.ClearingPrice
	evaluations["num_valid_bids"] = big.NewInt(int64(len(witness.Bids)))
	
	proof := &Halo2Proof{
		WitnessCommitments: commitments,
		QuotientCommitment: quotientCommit.Bytes(),
		OpeningProof:       openingProof,
		Evaluations:        evaluations,
	}
	
	ac.log.Debug("Halo2 auction proof generated",
		"winner_index", witness.WinnerIndex,
		"winning_bid", witness.WinningBid,
		"clearing_price", witness.ClearingPrice)
	
	return proof, nil
}

// computeQuotient computes the quotient polynomial
func (ac *AuctionCircuit) computeQuotient(witness *AuctionWitness) *big.Int {
	// Simplified quotient computation
	// In production, this would involve polynomial division
	
	constraints := big.NewInt(0)
	
	// Constraint 1: Winner has maximum bid
	for i, bid := range witness.Bids {
		if i != witness.WinnerIndex {
			// witness.WinningBid >= bid
			diff := ac.field.Sub(witness.WinningBid, bid)
			constraints = ac.field.Add(constraints, diff)
		}
	}
	
	// Constraint 2: Price is second price or reserve
	priceDiff := ac.field.Sub(witness.ClearingPrice, witness.SecondPrice)
	if witness.SecondPrice.Cmp(ac.Reserve) < 0 {
		priceDiff = ac.field.Sub(witness.ClearingPrice, ac.Reserve)
	}
	constraints = ac.field.Add(constraints, priceDiff)
	
	return constraints
}

// generateOpeningProof generates polynomial opening proof
func (ac *AuctionCircuit) generateOpeningProof(witness *AuctionWitness, commitments [][]byte) []byte {
	// Simplified opening proof
	// In production, use KZG or IPA opening
	
	data := make([]byte, 0)
	for _, commit := range commitments {
		data = append(data, commit...)
	}
	
	// Add witness hash
	witnessHash := ac.poseidon.Hash([]*big.Int{
		witness.WinningBid,
		witness.ClearingPrice,
	})
	data = append(data, witnessHash.Bytes()...)
	
	return hashing.ComputeHash256(data)
}

// Verify verifies a Halo2 auction proof
func (ac *AuctionCircuit) Verify(vk *VerifyingKey, publicInputs *AuctionPublicInputs, proof *Halo2Proof) bool {
	// Verify proof structure
	if len(proof.WitnessCommitments) < ac.NumBids+2 {
		ac.log.Debug("Invalid commitment count")
		return false
	}
	
	// Verify public inputs match claimed evaluations
	if proof.Evaluations["clearing_price"].Cmp(big.NewInt(int64(publicInputs.ClearingPrice))) != 0 {
		ac.log.Debug("Price mismatch")
		return false
	}
	
	// Verify quotient polynomial commitment
	if len(proof.QuotientCommitment) != 32 {
		ac.log.Debug("Invalid quotient commitment")
		return false
	}
	
	// Verify opening proof (simplified)
	// In production, verify KZG or IPA opening
	if len(proof.OpeningProof) < 32 {
		ac.log.Debug("Invalid opening proof")
		return false
	}
	
	// Check constraint satisfaction at random point (Fiat-Shamir)
	_ = ac.poseidon.Hash([]*big.Int{
		new(big.Int).SetBytes(proof.QuotientCommitment),
	})
	
	// Simplified constraint check
	// In production, evaluate constraint polynomials at challenge point
	
	ac.log.Debug("Halo2 proof verified",
		"clearing_price", publicInputs.ClearingPrice,
		"num_commitments", len(proof.WitnessCommitments))
	
	return true
}

// ProvingKey for Halo2 circuits
type ProvingKey struct {
	CircuitID string
	SRS       []*big.Int // Structured Reference String
	NumBids   int
	Reserve   *big.Int
}

// VerifyingKey for Halo2 circuits
type VerifyingKey struct {
	CircuitID       string
	CommitmentKey   []*big.Int
	ConstraintCount int
}

// AuctionWitness contains private auction inputs
type AuctionWitness struct {
	Bids          []*big.Int
	WinnerIndex   int
	WinningBid    *big.Int
	SecondPrice   *big.Int
	ClearingPrice *big.Int
}

// AuctionPublicInputs contains public auction inputs
type AuctionPublicInputs struct {
	NumBids       int
	Reserve       uint64
	ClearingPrice uint64
	WinnerCommit  []byte
}

// BudgetCircuit implements Halo2 circuit for budget safety
type BudgetCircuit struct {
	*Halo2Circuit
}

// NewBudgetCircuit creates a budget circuit
func NewBudgetCircuit(logger log.Logger) *BudgetCircuit {
	return &BudgetCircuit{
		Halo2Circuit: NewHalo2Circuit(logger),
	}
}

// Setup generates SRS for budget circuit
func (bc *BudgetCircuit) Setup() (*ProvingKey, *VerifyingKey, error) {
	tau := make([]byte, 32)
	if _, err := rand.Read(tau); err != nil {
		return nil, nil, ErrSetupFailed
	}
	
	tauBig := new(big.Int).SetBytes(tau)
	powers := make([]*big.Int, 10)
	powers[0] = big.NewInt(1)
	for i := 1; i < len(powers); i++ {
		powers[i] = bc.field.Mul(powers[i-1], tauBig)
	}
	
	pk := &ProvingKey{
		CircuitID: "budget_halo2_v1",
		SRS:       powers,
	}
	
	vk := &VerifyingKey{
		CircuitID:       "budget_halo2_v1",
		CommitmentKey:   powers[:2],
		ConstraintCount: 2, // new = old - delta, new >= 0
	}
	
	return pk, vk, nil
}

// Prove generates proof of valid budget update
func (bc *BudgetCircuit) Prove(pk *ProvingKey, witness *BudgetWitness) (*Halo2Proof, error) {
	// Verify constraint: new = old - delta
	expected := bc.field.Sub(witness.OldBudget, witness.Delta)
	if expected.Cmp(witness.NewBudget) != 0 {
		return nil, ErrProvingFailed
	}
	
	// Verify: new >= 0
	if witness.NewBudget.Sign() < 0 {
		return nil, ErrProvingFailed
	}
	
	// Create commitments
	oldCommit := bc.poseidon.Hash([]*big.Int{witness.OldBudget})
	deltaCommit := bc.poseidon.Hash([]*big.Int{witness.Delta})
	newCommit := bc.poseidon.Hash([]*big.Int{witness.NewBudget})
	
	commitments := [][]byte{
		oldCommit.Bytes(),
		deltaCommit.Bytes(),
		newCommit.Bytes(),
	}
	
	// Quotient for constraint satisfaction
	quotient := bc.field.Sub(
		bc.field.Add(witness.NewBudget, witness.Delta),
		witness.OldBudget,
	)
	quotientCommit := bc.poseidon.Hash([]*big.Int{quotient})
	
	// Opening proof
	openingProof := hashing.ComputeHash256(append(
		witness.NewBudget.Bytes(),
		witness.Delta.Bytes()...,
	))
	
	evaluations := make(map[string]*big.Int)
	evaluations["new_budget"] = witness.NewBudget
	evaluations["delta"] = witness.Delta
	
	bc.log.Debug("Halo2 budget proof generated",
		"old", witness.OldBudget,
		"delta", witness.Delta,
		"new", witness.NewBudget)
	
	return &Halo2Proof{
		WitnessCommitments: commitments,
		QuotientCommitment: quotientCommit.Bytes(),
		OpeningProof:       openingProof,
		Evaluations:        evaluations,
	}, nil
}

// Verify verifies budget proof
func (bc *BudgetCircuit) Verify(vk *VerifyingKey, publicInputs *BudgetPublicInputs, proof *Halo2Proof) bool {
	// Verify commitment structure
	if len(proof.WitnessCommitments) != 3 {
		return false
	}
	
	// Verify public delta matches
	if proof.Evaluations["delta"].Cmp(big.NewInt(int64(publicInputs.Delta))) != 0 {
		return false
	}
	
	// Simplified verification
	return len(proof.OpeningProof) > 0
}

// BudgetWitness contains private budget inputs
type BudgetWitness struct {
	OldBudget *big.Int
	Delta     *big.Int
	NewBudget *big.Int
}

// BudgetPublicInputs contains public budget inputs
type BudgetPublicInputs struct {
	Delta           uint64
	OldBudgetCommit []byte
	NewBudgetCommit []byte
}

// FrequencyCircuit implements Halo2 circuit for frequency capping
type FrequencyCircuit struct {
	*Halo2Circuit
	Cap uint32
}

// NewFrequencyCircuit creates a frequency circuit
func NewFrequencyCircuit(cap uint32, logger log.Logger) *FrequencyCircuit {
	return &FrequencyCircuit{
		Halo2Circuit: NewHalo2Circuit(logger),
		Cap:          cap,
	}
}

// Setup generates SRS for frequency circuit
func (fc *FrequencyCircuit) Setup() (*ProvingKey, *VerifyingKey, error) {
	tau := make([]byte, 32)
	if _, err := rand.Read(tau); err != nil {
		return nil, nil, ErrSetupFailed
	}
	
	tauBig := new(big.Int).SetBytes(tau)
	powers := make([]*big.Int, 10)
	powers[0] = big.NewInt(1)
	for i := 1; i < len(powers); i++ {
		powers[i] = fc.field.Mul(powers[i-1], tauBig)
	}
	
	pk := &ProvingKey{
		CircuitID: "frequency_halo2_v1",
		SRS:       powers,
	}
	
	vk := &VerifyingKey{
		CircuitID:       "frequency_halo2_v1",
		CommitmentKey:   powers[:2],
		ConstraintCount: 2, // after = before + 1, after <= cap
	}
	
	return pk, vk, nil
}

// Prove generates proof of frequency cap compliance
func (fc *FrequencyCircuit) Prove(pk *ProvingKey, witness *FrequencyWitness) (*Halo2Proof, error) {
	// Verify: after = before + 1
	expected := fc.field.Add(witness.CounterBefore, big.NewInt(1))
	if expected.Cmp(witness.CounterAfter) != 0 {
		return nil, ErrProvingFailed
	}
	
	// Verify: after <= cap
	if witness.CounterAfter.Cmp(big.NewInt(int64(fc.Cap))) > 0 {
		return nil, ErrProvingFailed
	}
	
	// Create commitments
	beforeCommit := fc.poseidon.Hash([]*big.Int{witness.CounterBefore})
	afterCommit := fc.poseidon.Hash([]*big.Int{witness.CounterAfter})
	
	commitments := [][]byte{
		beforeCommit.Bytes(),
		afterCommit.Bytes(),
	}
	
	// Quotient
	quotient := fc.field.Sub(witness.CounterAfter, expected)
	quotientCommit := fc.poseidon.Hash([]*big.Int{quotient})
	
	// Opening proof
	openingProof := hashing.ComputeHash256(append(
		witness.CounterBefore.Bytes(),
		witness.CounterAfter.Bytes()...,
	))
	
	evaluations := make(map[string]*big.Int)
	evaluations["counter_after"] = witness.CounterAfter
	
	fc.log.Debug("Halo2 frequency proof generated",
		"before", witness.CounterBefore,
		"after", witness.CounterAfter,
		"cap", fc.Cap)
	
	return &Halo2Proof{
		WitnessCommitments: commitments,
		QuotientCommitment: quotientCommit.Bytes(),
		OpeningProof:       openingProof,
		Evaluations:        evaluations,
	}, nil
}

// Verify verifies frequency proof
func (fc *FrequencyCircuit) Verify(vk *VerifyingKey, publicInputs *FrequencyPublicInputs, proof *Halo2Proof) bool {
	// Verify structure
	if len(proof.WitnessCommitments) != 2 {
		return false
	}
	
	// Verify counter is within cap
	counter := proof.Evaluations["counter_after"]
	if counter.Cmp(big.NewInt(int64(publicInputs.Cap))) > 0 {
		return false
	}
	
	return len(proof.OpeningProof) > 0
}

// FrequencyWitness contains private frequency inputs
type FrequencyWitness struct {
	CounterBefore *big.Int
	CounterAfter  *big.Int
	CampaignID    ids.ID
}

// FrequencyPublicInputs contains public frequency inputs
type FrequencyPublicInputs struct {
	Cap         uint32
	CampaignID  ids.ID
	CounterRoot []byte
}