// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	
	"github.com/luxfi/adx/auction"
	"github.com/luxfi/adx/core"
	"github.com/luxfi/adx/crypto"
	"github.com/luxfi/adx/settlement"
	"github.com/luxfi/adx/pkg/ids"
	"github.com/luxfi/adx/pkg/log"
)

func TestEndToEndAdFlow(t *testing.T) {
	require := require.New(t)
	logger := log.NewLogger("test")
	
	// 1. Setup HPKE encryption
	hpke := crypto.NewHPKE()
	
	// Generate keys for participants
	advertiserPub, advertiserPriv, err := hpke.GenerateKeyPair()
	require.NoError(err)
	
	publisherPub, publisherPriv, err := hpke.GenerateKeyPair()
	require.NoError(err)
	
	// 2. Create and run auction
	auctionID := ids.GenerateTestID()
	auc := auction.NewAuction(auctionID, 100, 5*time.Second, logger)
	
	// Submit bids
	bid1 := &auction.SealedBid{
		BidderID:   ids.GenerateTestID(),
		Commitment: []byte{0, 0, 0, 0, 0, 0, 0x03, 0xE8}, // 1000
		Timestamp:  time.Now(),
	}
	
	bid2 := &auction.SealedBid{
		BidderID:   ids.GenerateTestID(),
		Commitment: []byte{0, 0, 0, 0, 0, 0, 0x02, 0x58}, // 600
		Timestamp:  time.Now(),
	}
	
	bid3 := &auction.SealedBid{
		BidderID:   ids.GenerateTestID(),
		Commitment: []byte{0, 0, 0, 0, 0, 0, 0x00, 0x50}, // 80 (below reserve)
		Timestamp:  time.Now(),
	}
	
	err = auc.SubmitBid(bid1)
	require.NoError(err)
	
	err = auc.SubmitBid(bid2)
	require.NoError(err)
	
	err = auc.SubmitBid(bid3)
	require.NoError(err)
	
	// Run auction
	outcome, err := auc.RunAuction(advertiserPriv)
	require.NoError(err)
	require.NotNil(outcome)
	
	// Verify second-price auction logic
	require.Equal(bid1.BidderID, outcome.WinnerID)
	require.Equal(uint64(600), outcome.ClearingPrice) // Second price
	
	// 3. Create auction header
	auctionHeader := auc.CreateAuctionHeader()
	require.NotNil(auctionHeader)
	require.Equal(core.HeaderTypeAuction, auctionHeader.Type)
	require.Len(auctionHeader.CmInputs, 3)
	
	// 4. Test frequency capping
	freqMgr := core.NewFrequencyManager(logger)
	
	campaignID := ids.GenerateTestID()
	deviceID := "test-device-001"
	
	// Check and increment counter
	freqProof, err := freqMgr.CheckAndIncrementCounter(deviceID, campaignID, 3)
	require.NoError(err)
	require.NotNil(freqProof)
	require.Equal("counter", freqProof.Type)
	
	// Increment again (should work, cap is 3)
	freqProof, err = freqMgr.CheckAndIncrementCounter(deviceID, campaignID, 3)
	require.NoError(err)
	
	// Third time (should work)
	freqProof, err = freqMgr.CheckAndIncrementCounter(deviceID, campaignID, 3)
	require.NoError(err)
	
	// Fourth time (should fail - cap exceeded)
	_, err = freqMgr.CheckAndIncrementCounter(deviceID, campaignID, 3)
	require.Error(err)
	require.Equal(core.ErrFrequencyCapExceeded, err)
	
	// 5. Test Privacy Pass tokens
	tokens, err := freqMgr.IssueTokens(deviceID, campaignID, 5)
	require.NoError(err)
	require.Len(tokens, 5)
	
	// Redeem a token
	tokenProof, err := freqMgr.RedeemPrivacyToken(
		campaignID,
		tokens[0].Token,
		tokens[0].Proof,
	)
	require.NoError(err)
	require.Equal("token", tokenProof.Type)
	
	// Try to redeem same token again (should fail)
	_, err = freqMgr.RedeemPrivacyToken(
		campaignID,
		tokens[0].Token,
		tokens[0].Proof,
	)
	require.Error(err)
	
	// 6. Test budget management
	budgetMgr := settlement.NewBudgetManager(logger)
	
	advertiserID := ids.GenerateTestID()
	err = budgetMgr.SetBudget(advertiserID, 10000)
	require.NoError(err)
	
	// Deduct from budget
	budgetProof, err := budgetMgr.DeductBudget(advertiserID, 600, auctionID)
	require.NoError(err)
	require.NotNil(budgetProof)
	
	// Verify budget proof
	valid := settlement.VerifyBudgetProof(budgetProof)
	require.True(valid)
	
	// Create budget header
	budgetHeader := budgetMgr.CreateBudgetHeader(advertiserID, budgetProof)
	require.NotNil(budgetHeader)
	require.Equal(core.HeaderTypeBudget, budgetHeader.Type)
	
	// 7. Test settlement
	publisherID := ids.GenerateTestID()
	receipt, err := budgetMgr.CreateSettlement(
		advertiserID,
		publisherID,
		time.Now(),
	)
	require.NoError(err)
	require.NotNil(receipt)
	require.Equal(uint64(600), receipt.Amount)
	
	// Verify settlement proof
	valid = settlement.VerifySettlementProof(receipt.Proof)
	require.True(valid)
	
	// Create settlement header
	settlementHeader := budgetMgr.CreateSettlementHeader(receipt)
	require.NotNil(settlementHeader)
	require.Equal(core.HeaderTypeSettlement, settlementHeader.Type)
	
	// 8. Test HPKE encryption for multi-recipient
	plaintext := []byte("sensitive ad data")
	recipients := [][]byte{advertiserPub, publisherPub}
	
	// Create AAD from header hash
	headerData := []byte("header-hash-example")
	aad := crypto.CreateCommitment(headerData)
	
	// Seal for multiple recipients
	envelope, err := hpke.Seal(plaintext, recipients, aad)
	require.NoError(err)
	require.Len(envelope.Encapsulations, 2)
	
	// Advertiser decrypts
	decrypted, err := hpke.Open(envelope, advertiserPriv, 0)
	require.NoError(err)
	require.Equal(plaintext, decrypted)
	
	// Publisher decrypts
	decrypted, err = hpke.Open(envelope, publisherPriv, 1)
	require.NoError(err)
	require.Equal(plaintext, decrypted)
}

func TestImpressionFlow(t *testing.T) {
	require := require.New(t)
	
	// Create impression header
	impressionHeader := &core.ImpressionHeader{
		BaseHeader: core.BaseHeader{
			Type:      core.HeaderTypeImpression,
			ID:        ids.GenerateTestID(),
			Timestamp: time.Now(),
			Height:    1,
		},
		OutcomeRef: ids.GenerateTestID(),
		CmView:     crypto.CreateCommitment([]byte("viewability-metrics")),
		ProofView:  []byte("viewability-proof"),
		PSTTag:     []byte("privacy-state-token"),
		FreqRoot:   []byte("frequency-root"),
		ProofFreq:  []byte("frequency-proof"),
		DAPtr:      []byte("data-availability-pointer"),
	}
	
	require.Equal(core.HeaderTypeImpression, impressionHeader.Type)
	require.NotNil(impressionHeader.CmView)
	require.NotNil(impressionHeader.PSTTag)
}

func TestConversionFlow(t *testing.T) {
	require := require.New(t)
	
	clickRef := ids.GenerateTestID()
	
	// Create conversion header
	conversionHeader := &core.ConversionHeader{
		BaseHeader: core.BaseHeader{
			Type:      core.HeaderTypeConversion,
			ID:        ids.GenerateTestID(),
			Timestamp: time.Now(),
			Height:    1,
		},
		ClickRef:  &clickRef,
		CmAttr:    crypto.CreateCommitment([]byte("attribution-data")),
		ProofAttr: []byte("attribution-proof"),
		DAPtr:     []byte("encrypted-conversion-data"),
	}
	
	require.Equal(core.HeaderTypeConversion, conversionHeader.Type)
	require.NotNil(conversionHeader.ClickRef)
	require.NotNil(conversionHeader.CmAttr)
}

func TestViewabilityMetrics(t *testing.T) {
	require := require.New(t)
	
	metrics := &core.ViewabilityMetrics{
		PixelPercent: 75,  // 75% pixels in view
		TimeInView:   2000, // 2 seconds
	}
	
	// Check thresholds (MRC standard: 50% pixels, 1 second)
	require.True(metrics.PixelPercent >= 50)
	require.True(metrics.TimeInView >= 1000)
}

func BenchmarkAuction(b *testing.B) {
	logger := log.NewLogger("bench")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auc := auction.NewAuction(ids.GenerateTestID(), 100, time.Second, logger)
		
		// Submit 10 bids
		for j := 0; j < 10; j++ {
			bid := &auction.SealedBid{
				BidderID:   ids.GenerateTestID(),
				Commitment: []byte{0, 0, 0, 0, 0, 0, byte(j), byte(100 + j*10)},
				Timestamp:  time.Now(),
			}
			_ = auc.SubmitBid(bid)
		}
		
		// Run auction
		_, _ = auc.RunAuction(nil)
	}
}

func BenchmarkHPKE(b *testing.B) {
	hpke := crypto.NewHPKE()
	
	pub1, priv1, _ := hpke.GenerateKeyPair()
	pub2, _, _ := hpke.GenerateKeyPair()
	pub3, _, _ := hpke.GenerateKeyPair()
	
	plaintext := []byte("test message for benchmarking HPKE encryption")
	recipients := [][]byte{pub1, pub2, pub3}
	aad := []byte("additional-authenticated-data")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		envelope, _ := hpke.Seal(plaintext, recipients, aad)
		_, _ = hpke.Open(envelope, priv1, 0)
	}
}