// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package core

import (
	"time"

	"github.com/luxfi/ids"
)

// HeaderType defines the type of header in the ad network
type HeaderType string

const (
	HeaderTypeBid        HeaderType = "bid"
	HeaderTypeAuction    HeaderType = "auction_outcome"
	HeaderTypeImpression HeaderType = "impression"
	HeaderTypeConversion HeaderType = "conversion"
	HeaderTypeBudget     HeaderType = "budget_delta"
	HeaderTypeSettlement HeaderType = "settlement_receipt"
)

// BaseHeader contains common fields for all headers
type BaseHeader struct {
	Type      HeaderType `json:"type"`
	ID        ids.ID     `json:"id"`
	Timestamp time.Time  `json:"timestamp"`
	Signature []byte     `json:"signature"`
	
	// Blocklace fields for Byzantine-repelling
	Predecessors []ids.ID `json:"predecessors"`
	Height       uint64   `json:"height"`
}

// BidHeader represents a bid commitment without revealing the actual bid
type BidHeader struct {
	BaseHeader
	
	AuctionID   ids.ID `json:"auction_id"`
	CmBid       []byte `json:"cm_bid"`        // Commitment to (bid, creative_id, constraints)
	BidderPub   []byte `json:"bidder_pub"`    // Public key of bidder
	ProofRange  []byte `json:"proof_range"`   // ZK range proof: 0 <= bid <= max
	Meta        []byte `json:"meta"`          // Safe public metadata
}

// AuctionHeader represents the outcome of an auction with privacy
type AuctionHeader struct {
	BaseHeader
	
	AuctionID     ids.ID   `json:"auction_id"`
	CmInputs      [][]byte `json:"cm_inputs"`      // Commitments of all accepted bids
	CmWinner      []byte   `json:"cm_winner"`      // Commitment to (winner_id, winning_bid, price)
	ProofAuction  []byte   `json:"proof_auction"`  // ZK proof of correct auction
	PolicyRoot    []byte   `json:"policy_root"`    // Commitment to policy snapshot
	DAPtr         []byte   `json:"da_ptr"`         // Pointer to data availability layer
}

// ImpressionHeader logs an ad impression with viewability proof
type ImpressionHeader struct {
	BaseHeader
	
	OutcomeRef  ids.ID `json:"outcome_ref"`   // Reference to AuctionHeader
	CmView      []byte `json:"cm_view"`       // Commitment to viewability metrics
	ProofView   []byte `json:"proof_view"`    // ZK proof: metrics meet threshold
	PSTTag      []byte `json:"pst_tag"`       // Private State Token for fraud prevention
	FreqRoot    []byte `json:"freq_root"`     // Commitment to frequency counter state
	ProofFreq   []byte `json:"proof_freq"`    // ZK proof: cap not exceeded
	DAPtr       []byte `json:"da_ptr"`        // Encrypted impression payload
}

// ConversionHeader represents a conversion event with privacy
type ConversionHeader struct {
	BaseHeader
	
	ClickRef   *ids.ID `json:"click_ref,omitempty"`  // Optional click reference
	CmAttr     []byte  `json:"cm_attr"`               // Commitment to attribution fields
	ProofAttr  []byte  `json:"proof_attr"`            // ZK proof of attribution policy
	DAPtr      []byte  `json:"da_ptr"`                // Encrypted conversion payload
}

// BudgetHeader tracks budget changes with proofs
type BudgetHeader struct {
	BaseHeader
	
	AdvertiserID  ids.ID `json:"advertiser_id"`
	CmBudgetPrev  []byte `json:"cm_budget_prev"`  // Previous budget commitment
	CmBudgetNew   []byte `json:"cm_budget_new"`   // New budget commitment
	ProofDelta    []byte `json:"proof_delta"`     // ZK proof: new = prev - price (>= 0)
}

// SettlementHeader represents payment settlement with privacy
type SettlementHeader struct {
	BaseHeader
	
	AdvertiserID ids.ID `json:"advertiser_id"`
	PublisherID  ids.ID `json:"publisher_id"`
	CmAmount     []byte `json:"cm_amount"`      // Commitment to settlement amount
	ProofSettle  []byte `json:"proof_settle"`   // ZK proof of correct settlement
}

// Commitment represents a cryptographic commitment
type Commitment struct {
	Value []byte `json:"value"`
	Salt  []byte `json:"salt"`
}

// AuctionInput represents input to an auction
type AuctionInput struct {
	BidCommitment []byte    `json:"bid_commitment"`
	Bidder        ids.ID    `json:"bidder"`
	Timestamp     time.Time `json:"timestamp"`
}

// ViewabilityMetrics for impression measurement
type ViewabilityMetrics struct {
	PixelPercent uint8  `json:"pixel_percent"` // Percentage of pixels in view
	TimeInView   uint32 `json:"time_in_view"`  // Milliseconds in view
}

// FrequencyCounter tracks impressions per campaign
type FrequencyCounter struct {
	CampaignID ids.ID `json:"campaign_id"`
	Count      uint32 `json:"count"`
	Cap        uint32 `json:"cap"`
}