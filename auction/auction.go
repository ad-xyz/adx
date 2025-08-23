// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auction

import (
	"errors"
	"sort"
	"time"

	"github.com/luxfi/adx/core"
	"github.com/luxfi/adx/crypto"
	"github.com/luxfi/adx/pkg/ids"
	"github.com/luxfi/adx/pkg/log"
)

var (
	ErrInvalidBid       = errors.New("invalid bid")
	ErrNoValidBids      = errors.New("no valid bids")
	ErrAuctionClosed    = errors.New("auction closed")
	ErrInvalidThreshold = errors.New("viewability threshold not met")
)

// Auction represents a second-price sealed-bid auction
type Auction struct {
	ID           ids.ID
	StartTime    time.Time
	EndTime      time.Time
	Reserve      uint64
	PolicyRoot   []byte
	Bids         []*SealedBid
	Outcome      *AuctionOutcome
	log          log.Logger
}

// SealedBid represents an encrypted bid
type SealedBid struct {
	BidderID      ids.ID
	Commitment    []byte // Commitment to bid value
	EncryptedBid  []byte // HPKE-encrypted bid details
	RangeProof    []byte // ZK proof that bid is in valid range
	Timestamp     time.Time
}

// AuctionOutcome represents the result of an auction
type AuctionOutcome struct {
	WinnerID     ids.ID
	WinningBid   uint64
	ClearingPrice uint64 // Second price
	ProofCorrect []byte // ZK proof of correct auction execution
}

// NewAuction creates a new auction instance
func NewAuction(id ids.ID, reserve uint64, duration time.Duration, logger log.Logger) *Auction {
	now := time.Now()
	return &Auction{
		ID:        id,
		StartTime: now,
		EndTime:   now.Add(duration),
		Reserve:   reserve,
		Bids:      make([]*SealedBid, 0),
		log:       logger,
	}
}

// SubmitBid adds a sealed bid to the auction
func (a *Auction) SubmitBid(bid *SealedBid) error {
	if time.Now().After(a.EndTime) {
		return ErrAuctionClosed
	}
	
	// Verify range proof if provided
	if len(bid.RangeProof) > 0 {
		if !a.verifyRangeProof(bid.Commitment, bid.RangeProof) {
			return ErrInvalidBid
		}
	}
	
	a.Bids = append(a.Bids, bid)
	a.log.Debug("bid submitted", "bidder", bid.BidderID, "auction", a.ID)
	
	return nil
}

// RunAuction executes the auction and determines the winner
func (a *Auction) RunAuction(decryptionKey []byte) (*AuctionOutcome, error) {
	if len(a.Bids) == 0 {
		return nil, ErrNoValidBids
	}
	
	// Decrypt and validate bids
	validBids := make([]*DecryptedBid, 0)
	hpke := crypto.NewHPKE()
	
	for _, sealedBid := range a.Bids {
		// In production, this would be done in a TEE or with MPC
		decrypted, err := a.decryptBid(sealedBid, decryptionKey, hpke)
		if err != nil {
			a.log.Debug("failed to decrypt bid", "bidder", sealedBid.BidderID, "error", err)
			continue
		}
		
		if decrypted.Value >= a.Reserve {
			validBids = append(validBids, decrypted)
		}
	}
	
	if len(validBids) == 0 {
		return nil, ErrNoValidBids
	}
	
	// Sort bids by value (descending)
	sort.Slice(validBids, func(i, j int) bool {
		return validBids[i].Value > validBids[j].Value
	})
	
	// Determine winner and second price
	winner := validBids[0]
	clearingPrice := a.Reserve
	
	if len(validBids) > 1 {
		clearingPrice = validBids[1].Value
	}
	
	// Generate proof of correct auction execution
	proof := a.generateAuctionProof(validBids, winner, clearingPrice)
	
	outcome := &AuctionOutcome{
		WinnerID:      winner.BidderID,
		WinningBid:    winner.Value,
		ClearingPrice: clearingPrice,
		ProofCorrect:  proof,
	}
	
	a.Outcome = outcome
	a.log.Info("auction completed", 
		"auction", a.ID,
		"winner", winner.BidderID,
		"price", clearingPrice,
		"bids", len(validBids))
	
	return outcome, nil
}

// DecryptedBid represents a decrypted bid
type DecryptedBid struct {
	BidderID   ids.ID
	Value      uint64
	CreativeID ids.ID
	Targeting  []byte
}

// decryptBid decrypts a sealed bid (in production, done in TEE)
func (a *Auction) decryptBid(sealed *SealedBid, key []byte, hpke *crypto.HPKE) (*DecryptedBid, error) {
	// Simplified decryption - in production use proper HPKE
	// This would be done inside a TEE to keep bids private
	
	// For now, use the commitment as a simplified bid value
	if len(sealed.Commitment) < 8 {
		return nil, ErrInvalidBid
	}
	
	value := uint64(0)
	for i := 0; i < 8; i++ {
		value = (value << 8) | uint64(sealed.Commitment[i])
	}
	
	return &DecryptedBid{
		BidderID:   sealed.BidderID,
		Value:      value % 10000, // Simplified for testing
		CreativeID: ids.GenerateTestID(),
	}, nil
}

// verifyRangeProof verifies that a bid is within valid range
func (a *Auction) verifyRangeProof(commitment, proof []byte) bool {
	// Simplified verification - in production use actual ZK proof verification
	// This would verify: 0 <= bid <= MAX_BID
	return len(proof) > 0 && len(commitment) > 0
}

// generateAuctionProof generates a ZK proof of correct auction execution
func (a *Auction) generateAuctionProof(bids []*DecryptedBid, winner *DecryptedBid, price uint64) []byte {
	// Simplified proof generation - in production use actual ZK proving system
	// Proves:
	// 1. winner.Value = max(all bid values)
	// 2. price = max(reserve, second highest bid)
	// 3. All operations performed correctly
	
	proof := crypto.CreateCommitment([]byte{
		byte(winner.Value >> 8),
		byte(winner.Value),
		byte(price >> 8),
		byte(price),
		byte(len(bids)),
	})
	
	return proof
}

// CreateAuctionHeader creates a header for the auction outcome
func (a *Auction) CreateAuctionHeader() *core.AuctionHeader {
	if a.Outcome == nil {
		return nil
	}
	
	// Create commitments for all bids
	cmInputs := make([][]byte, len(a.Bids))
	for i, bid := range a.Bids {
		cmInputs[i] = bid.Commitment
	}
	
	// Create winner commitment
	cmWinner := crypto.CreateCommitment([]byte{
		byte(a.Outcome.WinnerID[0]),
		byte(a.Outcome.WinningBid >> 8),
		byte(a.Outcome.WinningBid),
		byte(a.Outcome.ClearingPrice >> 8),
		byte(a.Outcome.ClearingPrice),
	})
	
	return &core.AuctionHeader{
		BaseHeader: core.BaseHeader{
			Type:      core.HeaderTypeAuction,
			ID:        ids.GenerateTestID(),
			Timestamp: time.Now(),
			Height:    1,
		},
		AuctionID:    a.ID,
		CmInputs:     cmInputs,
		CmWinner:     cmWinner,
		ProofAuction: a.Outcome.ProofCorrect,
		PolicyRoot:   a.PolicyRoot,
	}
}