// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package halo2

import (
	"math/big"
	"testing"

	"github.com/luxfi/adx/pkg/ids"
	"github.com/luxfi/adx/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestPoseidonHash(t *testing.T) {
	require := require.New(t)
	
	poseidon := NewPoseidonHash()
	
	// Test single input
	input1 := []*big.Int{big.NewInt(42)}
	hash1 := poseidon.Hash(input1)
	require.NotNil(hash1)
	require.Greater(hash1.BitLen(), 0)
	
	// Test multiple inputs
	input2 := []*big.Int{
		big.NewInt(1),
		big.NewInt(2),
		big.NewInt(3),
	}
	hash2 := poseidon.Hash(input2)
	require.NotNil(hash2)
	
	// Different inputs should produce different hashes
	require.NotEqual(hash1, hash2)
	
	// Same input should produce same hash
	hash1Again := poseidon.Hash(input1)
	require.Equal(hash1, hash1Again)
}

func TestAuctionCircuit(t *testing.T) {
	require := require.New(t)
	logger := log.NoLog{}
	
	numBids := 5
	reserve := uint64(100)
	
	// Create circuit
	circuit := NewAuctionCircuit(numBids, reserve, logger)
	
	// Setup
	pk, vk, err := circuit.Setup()
	require.NoError(err)
	require.NotNil(pk)
	require.NotNil(vk)
	require.Equal("auction_halo2_v1", pk.CircuitID)
	require.Equal(numBids, pk.NumBids)
	
	// Create witness
	bids := []*big.Int{
		big.NewInt(150),
		big.NewInt(200), // Second highest
		big.NewInt(250), // Winner
		big.NewInt(120),
		big.NewInt(180),
	}
	
	witness := &AuctionWitness{
		Bids:          bids,
		WinnerIndex:   2,
		WinningBid:    big.NewInt(250),
		SecondPrice:   big.NewInt(200),
		ClearingPrice: big.NewInt(200), // Second price
	}
	
	// Generate proof
	proof, err := circuit.Prove(pk, witness)
	require.NoError(err)
	require.NotNil(proof)
	require.Len(proof.WitnessCommitments, numBids+2) // bids + winner + price
	require.NotEmpty(proof.QuotientCommitment)
	require.NotEmpty(proof.OpeningProof)
	require.Equal(big.NewInt(200), proof.Evaluations["clearing_price"])
	
	// Create public inputs
	publicInputs := &AuctionPublicInputs{
		NumBids:       numBids,
		Reserve:       reserve,
		ClearingPrice: 200,
		WinnerCommit:  proof.WitnessCommitments[numBids], // Winner commitment
	}
	
	// Verify proof
	valid := circuit.Verify(vk, publicInputs, proof)
	require.True(valid)
	
	// Test with reserve price winning
	witnessReserve := &AuctionWitness{
		Bids:          []*big.Int{big.NewInt(90), big.NewInt(95), big.NewInt(80)},
		WinnerIndex:   1,
		WinningBid:    big.NewInt(95),
		SecondPrice:   big.NewInt(90),
		ClearingPrice: big.NewInt(100), // Reserve price
	}
	
	// Need to pad bids to match circuit size
	for len(witnessReserve.Bids) < numBids {
		witnessReserve.Bids = append(witnessReserve.Bids, big.NewInt(0))
	}
	
	proofReserve, err := circuit.Prove(pk, witnessReserve)
	require.NoError(err)
	
	publicInputsReserve := &AuctionPublicInputs{
		NumBids:       numBids,
		Reserve:       reserve,
		ClearingPrice: 100, // Reserve price
		WinnerCommit:  proofReserve.WitnessCommitments[numBids],
	}
	
	validReserve := circuit.Verify(vk, publicInputsReserve, proofReserve)
	require.True(validReserve)
}

func TestBudgetCircuit(t *testing.T) {
	require := require.New(t)
	logger := log.NoLog{}
	
	// Create circuit
	circuit := NewBudgetCircuit(logger)
	
	// Setup
	pk, vk, err := circuit.Setup()
	require.NoError(err)
	require.NotNil(pk)
	require.NotNil(vk)
	require.Equal("budget_halo2_v1", pk.CircuitID)
	
	// Create witness
	witness := &BudgetWitness{
		OldBudget: big.NewInt(1000),
		Delta:     big.NewInt(250),
		NewBudget: big.NewInt(750),
	}
	
	// Generate proof
	proof, err := circuit.Prove(pk, witness)
	require.NoError(err)
	require.NotNil(proof)
	require.Len(proof.WitnessCommitments, 3) // old, delta, new
	require.Equal(big.NewInt(750), proof.Evaluations["new_budget"])
	require.Equal(big.NewInt(250), proof.Evaluations["delta"])
	
	// Create public inputs
	publicInputs := &BudgetPublicInputs{
		Delta:           250,
		OldBudgetCommit: proof.WitnessCommitments[0],
		NewBudgetCommit: proof.WitnessCommitments[2],
	}
	
	// Verify proof
	valid := circuit.Verify(vk, publicInputs, proof)
	require.True(valid)
	
	// Test invalid proof (budget goes negative)
	invalidWitness := &BudgetWitness{
		OldBudget: big.NewInt(100),
		Delta:     big.NewInt(200),
		NewBudget: big.NewInt(-100),
	}
	
	_, err = circuit.Prove(pk, invalidWitness)
	require.Error(err)
	require.Equal(ErrProvingFailed, err)
	
	// Test invalid proof (incorrect arithmetic)
	invalidWitness2 := &BudgetWitness{
		OldBudget: big.NewInt(1000),
		Delta:     big.NewInt(200),
		NewBudget: big.NewInt(700), // Should be 800
	}
	
	_, err = circuit.Prove(pk, invalidWitness2)
	require.Error(err)
	require.Equal(ErrProvingFailed, err)
}

func TestFrequencyCircuit(t *testing.T) {
	require := require.New(t)
	logger := log.NoLog{}
	
	cap := uint32(5)
	
	// Create circuit
	circuit := NewFrequencyCircuit(cap, logger)
	
	// Setup
	pk, vk, err := circuit.Setup()
	require.NoError(err)
	require.NotNil(pk)
	require.NotNil(vk)
	require.Equal("frequency_halo2_v1", pk.CircuitID)
	
	// Create witness
	witness := &FrequencyWitness{
		CounterBefore: big.NewInt(2),
		CounterAfter:  big.NewInt(3),
		CampaignID:    ids.GenerateTestID(),
	}
	
	// Generate proof
	proof, err := circuit.Prove(pk, witness)
	require.NoError(err)
	require.NotNil(proof)
	require.Len(proof.WitnessCommitments, 2) // before, after
	require.Equal(big.NewInt(3), proof.Evaluations["counter_after"])
	
	// Create public inputs
	publicInputs := &FrequencyPublicInputs{
		Cap:         cap,
		CampaignID:  witness.CampaignID,
		CounterRoot: proof.WitnessCommitments[1],
	}
	
	// Verify proof
	valid := circuit.Verify(vk, publicInputs, proof)
	require.True(valid)
	
	// Test at cap limit
	witnessAtCap := &FrequencyWitness{
		CounterBefore: big.NewInt(4),
		CounterAfter:  big.NewInt(5),
		CampaignID:    ids.GenerateTestID(),
	}
	
	proofAtCap, err := circuit.Prove(pk, witnessAtCap)
	require.NoError(err)
	
	publicInputsAtCap := &FrequencyPublicInputs{
		Cap:         cap,
		CampaignID:  witnessAtCap.CampaignID,
		CounterRoot: proofAtCap.WitnessCommitments[1],
	}
	
	validAtCap := circuit.Verify(vk, publicInputsAtCap, proofAtCap)
	require.True(validAtCap)
	
	// Test exceeding cap
	witnessOverCap := &FrequencyWitness{
		CounterBefore: big.NewInt(5),
		CounterAfter:  big.NewInt(6),
		CampaignID:    ids.GenerateTestID(),
	}
	
	_, err = circuit.Prove(pk, witnessOverCap)
	require.Error(err)
	require.Equal(ErrProvingFailed, err)
	
	// Test invalid increment (not +1)
	witnessInvalidIncrement := &FrequencyWitness{
		CounterBefore: big.NewInt(2),
		CounterAfter:  big.NewInt(4), // Should be 3
		CampaignID:    ids.GenerateTestID(),
	}
	
	_, err = circuit.Prove(pk, witnessInvalidIncrement)
	require.Error(err)
	require.Equal(ErrProvingFailed, err)
}

func TestFieldOperations(t *testing.T) {
	require := require.New(t)
	
	field := NewField()
	
	// Test addition
	a := big.NewInt(10)
	b := big.NewInt(20)
	sum := field.Add(a, b)
	require.Equal(big.NewInt(30), sum)
	
	// Test subtraction
	diff := field.Sub(b, a)
	require.Equal(big.NewInt(10), diff)
	
	// Test multiplication
	prod := field.Mul(a, b)
	require.Equal(big.NewInt(200), prod)
	
	// Test modular reduction
	large := new(big.Int).Add(field.Modulus, big.NewInt(5))
	reduced := field.Add(large, big.NewInt(0))
	require.Equal(big.NewInt(5), reduced)
	
	// Test inverse
	inv := field.Inv(big.NewInt(7))
	require.NotNil(inv)
	
	// Verify inverse property: a * a^-1 = 1 (mod p)
	one := field.Mul(big.NewInt(7), inv)
	require.Equal(big.NewInt(1), one)
}

func BenchmarkHalo2AuctionProof(b *testing.B) {
	logger := log.NoLog{}
	numBids := 10
	reserve := uint64(100)
	
	circuit := NewAuctionCircuit(numBids, reserve, logger)
	pk, vk, _ := circuit.Setup()
	
	// Create witness
	bids := make([]*big.Int, numBids)
	for i := 0; i < numBids; i++ {
		bids[i] = big.NewInt(int64(100 + i*50))
	}
	
	witness := &AuctionWitness{
		Bids:          bids,
		WinnerIndex:   numBids - 1,
		WinningBid:    bids[numBids-1],
		SecondPrice:   bids[numBids-2],
		ClearingPrice: bids[numBids-2],
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		proof, _ := circuit.Prove(pk, witness)
		
		publicInputs := &AuctionPublicInputs{
			NumBids:       numBids,
			Reserve:       reserve,
			ClearingPrice: uint64(witness.ClearingPrice.Int64()),
			WinnerCommit:  proof.WitnessCommitments[numBids],
		}
		
		circuit.Verify(vk, publicInputs, proof)
	}
}

func BenchmarkPoseidonHash(b *testing.B) {
	poseidon := NewPoseidonHash()
	inputs := []*big.Int{
		big.NewInt(42),
		big.NewInt(123),
		big.NewInt(456),
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = poseidon.Hash(inputs)
	}
}