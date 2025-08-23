// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auction

import (
	"math/big"
	"testing"
	"time"

	"github.com/luxfi/adx/proof/halo2"
	"github.com/luxfi/adx/pkg/ids"
	"github.com/luxfi/adx/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestHalo2Auction(t *testing.T) {
	require := require.New(t)
	logger := log.NoLog{}
	
	auctionID := ids.GenerateTestID()
	reserve := uint64(100)
	duration := 5 * time.Minute
	
	// Create Halo2 auction
	auction, err := NewHalo2Auction(auctionID, reserve, duration, logger)
	require.NoError(err)
	require.NotNil(auction)
	require.NotNil(auction.circuit)
	require.NotNil(auction.pk)
	require.NotNil(auction.vk)
	
	// Submit some bids
	for i := 0; i < 5; i++ {
		bid := &SealedBid{
			BidderID:     ids.GenerateTestID(),
			Commitment:   []byte("commitment"),
			EncryptedBid: []byte("encrypted"),
			RangeProof:   []byte("range_proof"),
		}
		err := auction.SubmitBid(bid)
		require.NoError(err)
	}
	
	// Run auction with Halo2 proof
	decryptionKey := []byte("test_key")
	outcome, err := auction.RunAuctionWithHalo2(decryptionKey)
	require.NoError(err)
	require.NotNil(outcome)
	require.NotNil(outcome.Halo2Proof)
	require.NotEmpty(outcome.ProofID)
	require.NotNil(outcome.VerifyingKey)
	
	// Verify the proof
	publicInputs := &halo2.AuctionPublicInputs{
		NumBids:       5,
		Reserve:       reserve,
		ClearingPrice: outcome.ClearingPrice,
		WinnerCommit:  outcome.Halo2Proof.WitnessCommitments[5], // After 5 bid commitments
	}
	
	valid := auction.VerifyHalo2Proof(outcome.ProofID, publicInputs)
	require.True(valid)
	
	// Test with invalid proof ID
	invalidID := ids.GenerateTestID()
	valid = auction.VerifyHalo2Proof(invalidID, publicInputs)
	require.False(valid)
}

func TestHalo2BudgetManager(t *testing.T) {
	require := require.New(t)
	logger := log.NoLog{}
	
	// Create budget manager
	manager, err := NewHalo2BudgetManager(logger)
	require.NoError(err)
	require.NotNil(manager)
	require.NotNil(manager.circuit)
	require.NotNil(manager.pk)
	require.NotNil(manager.vk)
	
	advertiserID := ids.GenerateTestID()
	
	// Deduct budget with proof
	amount := uint64(500)
	proof, err := manager.DeductBudgetWithProof(advertiserID, amount)
	require.NoError(err)
	require.NotNil(proof)
	require.NotNil(proof.Halo2Proof)
	require.Equal(amount, proof.Delta)
	require.Equal(uint64(9500), proof.NewBudget) // 10000 - 500
	
	// Verify the proof
	valid := manager.VerifyBudgetProof(proof)
	require.True(valid)
	
	// Deduct more budget
	amount2 := uint64(1500)
	proof2, err := manager.DeductBudgetWithProof(advertiserID, amount2)
	require.NoError(err)
	require.Equal(uint64(8000), proof2.NewBudget) // 9500 - 1500
	
	// Verify second proof
	valid2 := manager.VerifyBudgetProof(proof2)
	require.True(valid2)
	
	// Try to exceed budget
	hugeAmount := uint64(20000)
	_, err = manager.DeductBudgetWithProof(advertiserID, hugeAmount)
	require.Error(err)
	require.Contains(err.Error(), "insufficient budget")
}

func TestHalo2FrequencyManager(t *testing.T) {
	require := require.New(t)
	logger := log.NoLog{}
	
	// Create frequency manager
	manager := NewHalo2FrequencyManager(logger)
	require.NotNil(manager)
	
	deviceID := "device_123"
	campaignID := ids.GenerateTestID()
	cap := uint32(3)
	
	// First increment
	proof1, err := manager.CheckAndIncrementWithProof(deviceID, campaignID, cap)
	require.NoError(err)
	require.NotNil(proof1)
	require.Equal(uint32(1), proof1.NewCounter)
	require.NotNil(proof1.Halo2Proof)
	
	// Verify first proof
	valid1 := manager.VerifyFrequencyProof(proof1)
	require.True(valid1)
	
	// Second increment
	proof2, err := manager.CheckAndIncrementWithProof(deviceID, campaignID, cap)
	require.NoError(err)
	require.Equal(uint32(2), proof2.NewCounter)
	
	// Verify second proof
	valid2 := manager.VerifyFrequencyProof(proof2)
	require.True(valid2)
	
	// Third increment (at cap)
	proof3, err := manager.CheckAndIncrementWithProof(deviceID, campaignID, cap)
	require.NoError(err)
	require.Equal(uint32(3), proof3.NewCounter)
	
	// Fourth increment should fail (exceeds cap)
	_, err = manager.CheckAndIncrementWithProof(deviceID, campaignID, cap)
	require.Error(err)
	require.Contains(err.Error(), "frequency cap exceeded")
	
	// Different campaign should work
	campaign2ID := ids.GenerateTestID()
	proof4, err := manager.CheckAndIncrementWithProof(deviceID, campaign2ID, cap)
	require.NoError(err)
	require.Equal(uint32(1), proof4.NewCounter)
	
	// Verify proof for different campaign
	valid4 := manager.VerifyFrequencyProof(proof4)
	require.True(valid4)
}

func TestHalo2AuctionWithManyBids(t *testing.T) {
	require := require.New(t)
	logger := log.NoLog{}
	
	auctionID := ids.GenerateTestID()
	reserve := uint64(50)
	duration := 10 * time.Minute
	
	// Create auction
	auction, err := NewHalo2Auction(auctionID, reserve, duration, logger)
	require.NoError(err)
	
	// Submit many bids (up to circuit limit)
	numBids := 50
	for i := 0; i < numBids; i++ {
		bid := &SealedBid{
			BidderID:     ids.GenerateTestID(),
			Commitment:   []byte("commitment"),
			EncryptedBid: []byte("encrypted"),
			RangeProof:   []byte("range_proof"),
		}
		err := auction.SubmitBid(bid)
		require.NoError(err)
	}
	
	// Run auction with Halo2 proof
	outcome, err := auction.RunAuctionWithHalo2([]byte("key"))
	require.NoError(err)
	require.NotNil(outcome)
	
	// Verify proof with correct parameters
	publicInputs := &halo2.AuctionPublicInputs{
		NumBids:       numBids,
		Reserve:       reserve,
		ClearingPrice: outcome.ClearingPrice,
		WinnerCommit:  outcome.Halo2Proof.WitnessCommitments[100], // Circuit has 100 slots
	}
	
	valid := auction.VerifyHalo2Proof(outcome.ProofID, publicInputs)
	require.True(valid)
}

func BenchmarkHalo2AuctionWithProof(b *testing.B) {
	logger := log.NoLog{}
	
	auctionID := ids.GenerateTestID()
	reserve := uint64(100)
	duration := 5 * time.Minute
	
	auction, _ := NewHalo2Auction(auctionID, reserve, duration, logger)
	
	// Submit bids
	for i := 0; i < 10; i++ {
		bid := &SealedBid{
			BidderID:     ids.GenerateTestID(),
			Commitment:   []byte("commitment"),
			EncryptedBid: []byte("encrypted"),
			RangeProof:   []byte("range_proof"),
		}
		auction.SubmitBid(bid)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		outcome, _ := auction.RunAuctionWithHalo2([]byte("key"))
		
		publicInputs := &halo2.AuctionPublicInputs{
			NumBids:       10,
			Reserve:       reserve,
			ClearingPrice: outcome.ClearingPrice,
			WinnerCommit:  outcome.Halo2Proof.WitnessCommitments[10],
		}
		
		auction.VerifyHalo2Proof(outcome.ProofID, publicInputs)
	}
}

func BenchmarkHalo2BudgetProof(b *testing.B) {
	logger := log.NoLog{}
	manager, _ := NewHalo2BudgetManager(logger)
	advertiserID := ids.GenerateTestID()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		amount := uint64(100 + i%500)
		proof, err := manager.DeductBudgetWithProof(advertiserID, amount)
		if err == nil && proof != nil {
			manager.VerifyBudgetProof(proof)
		}
		
		// Reset budget to avoid running out
		if i%50 == 0 {
			manager.budgets[advertiserID] = big.NewInt(10000)
		}
	}
}

func BenchmarkHalo2FrequencyProof(b *testing.B) {
	logger := log.NoLog{}
	manager := NewHalo2FrequencyManager(logger)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		deviceID := "device_" + string(rune(i%100))
		campaignID := ids.GenerateTestID()
		cap := uint32(5)
		
		proof, err := manager.CheckAndIncrementWithProof(deviceID, campaignID, cap)
		if err == nil {
			manager.VerifyFrequencyProof(proof)
		}
	}
}