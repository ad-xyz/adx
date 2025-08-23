// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tee

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/luxfi/adx/auction"
	"github.com/luxfi/adx/core"
	"github.com/luxfi/adx/crypto"
	"github.com/luxfi/adx/pkg/ids"
	"github.com/luxfi/adx/pkg/log"
)

var (
	ErrNotAttested      = errors.New("enclave not attested")
	ErrInvalidQuote     = errors.New("invalid attestation quote")
	ErrEnclaveSealed    = errors.New("enclave is sealed")
	ErrMaxBidsExceeded  = errors.New("maximum bids exceeded")
)

// EnclaveType represents the TEE type
type EnclaveType string

const (
	EnclaveIntelSGX   EnclaveType = "intel_sgx"
	EnclaveAMDSEV     EnclaveType = "amd_sev"
	EnclaveAWSNitro   EnclaveType = "aws_nitro"
	EnclaveAzureCVM   EnclaveType = "azure_cvm"
	EnclaveSimulated  EnclaveType = "simulated" // For testing
)

// Enclave represents a Trusted Execution Environment
type Enclave struct {
	mu       sync.RWMutex
	
	// Enclave identity
	ID       ids.ID
	Type     EnclaveType
	Version  string
	
	// Attestation
	MREnclave    []byte // Measurement of enclave code
	MRSigner     []byte // Measurement of enclave signer
	Quote        []byte // Remote attestation quote
	Attested     bool
	AttestedTime time.Time
	
	// Sealing keys (never leave enclave)
	sealingKey   []byte
	
	// Auction state (encrypted at rest)
	auctions     map[ids.ID]*SealedAuction
	
	// Metrics
	processed    uint64
	errors       uint64
	
	log          log.Logger
}

// SealedAuction represents an auction sealed in the enclave
type SealedAuction struct {
	ID           ids.ID
	Bids         [][]byte // Encrypted bids
	Reserve      uint64
	PolicyRoot   []byte
	Outcome      *auction.AuctionOutcome
	Transcript   []byte // Audit log
}

// NewEnclave creates a new TEE enclave
func NewEnclave(enclaveType EnclaveType, logger log.Logger) (*Enclave, error) {
	enclave := &Enclave{
		ID:       ids.GenerateTestID(),
		Type:     enclaveType,
		Version:  "1.0.0",
		auctions: make(map[ids.ID]*SealedAuction),
		log:      logger,
	}
	
	// Generate sealing key (never exposed outside enclave)
	enclave.sealingKey = make([]byte, 32)
	if _, err := cryptorand.Read(enclave.sealingKey); err != nil {
		return nil, err
	}
	
	// Perform attestation
	if err := enclave.performAttestation(); err != nil {
		return nil, err
	}
	
	return enclave, nil
}

// performAttestation performs remote attestation
func (e *Enclave) performAttestation() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Generate measurements
	e.MREnclave = e.measureCode()
	e.MRSigner = e.measureSigner()
	
	// Generate attestation quote
	quote, err := e.generateQuote()
	if err != nil {
		return err
	}
	
	e.Quote = quote
	e.Attested = true
	e.AttestedTime = time.Now()
	
	e.log.Info("enclave attested",
		"type", e.Type,
		"id", e.ID,
		"mr_enclave", e.MREnclave[:8])
	
	return nil
}

// measureCode measures the enclave code
func (e *Enclave) measureCode() []byte {
	// In production, this would be the actual measurement
	// For simulation, hash the enclave type and version
	h := sha256.New()
	h.Write([]byte(e.Type))
	h.Write([]byte(e.Version))
	return h.Sum(nil)
}

// measureSigner measures the enclave signer
func (e *Enclave) measureSigner() []byte {
	// In production, this would be the signer's key hash
	// For simulation, use a fixed value
	return crypto.CreateCommitment([]byte("ADXYZ_SIGNER_v1"))
}

// generateQuote generates an attestation quote
func (e *Enclave) generateQuote() ([]byte, error) {
	// In production, this would call the TEE's quote generation
	// For simulation, create a signed statement
	
	statement := AttestationStatement{
		EnclaveID:    e.ID,
		Type:         e.Type,
		MREnclave:    e.MREnclave,
		MRSigner:     e.MRSigner,
		Timestamp:    time.Now(),
		Nonce:        make([]byte, 16),
	}
	
	// Add random nonce
	cryptorand.Read(statement.Nonce)
	
	// Serialize and sign (simplified)
	data, err := json.Marshal(statement)
	if err != nil {
		return nil, err
	}
	
	// In production, use TEE's signing key
	signature := crypto.CreateCommitment(data)
	
	quote := append(data, signature...)
	return quote, nil
}

// AttestationStatement represents the attestation data
type AttestationStatement struct {
	EnclaveID ids.ID      `json:"enclave_id"`
	Type      EnclaveType `json:"type"`
	MREnclave []byte      `json:"mr_enclave"`
	MRSigner  []byte      `json:"mr_signer"`
	Timestamp time.Time   `json:"timestamp"`
	Nonce     []byte      `json:"nonce"`
}

// RunAuction runs an auction inside the enclave
func (e *Enclave) RunAuction(auctionID ids.ID, reserve uint64, encryptedBids [][]byte) (*EnclaveAuctionResult, error) {
	if !e.Attested {
		return nil, ErrNotAttested
	}
	
	if len(encryptedBids) > 1000 {
		return nil, ErrMaxBidsExceeded
	}
	
	e.mu.Lock()
	defer e.mu.Unlock()
	
	startTime := time.Now()
	
	// Create sealed auction
	sealed := &SealedAuction{
		ID:         auctionID,
		Bids:       encryptedBids,
		Reserve:    reserve,
		PolicyRoot: crypto.CreateCommitment([]byte("policy_v1")),
	}
	
	// Decrypt bids inside enclave
	decryptedBids := make([]*BidData, 0, len(encryptedBids))
	for i, encBid := range encryptedBids {
		bid, err := e.decryptBid(encBid)
		if err != nil {
			e.log.Debug("failed to decrypt bid", "index", i, "error", err)
			continue
		}
		decryptedBids = append(decryptedBids, bid)
	}
	
	// Run second-price auction
	outcome := e.runSecondPriceAuction(decryptedBids, reserve)
	sealed.Outcome = outcome
	
	// Generate audit transcript
	transcript := e.generateTranscript(sealed, decryptedBids, outcome)
	sealed.Transcript = transcript
	
	// Store sealed auction
	e.auctions[auctionID] = sealed
	
	// Create result with attestation
	result := &EnclaveAuctionResult{
		AuctionID:     auctionID,
		WinnerCommit:  crypto.CreateCommitment([]byte(outcome.WinnerID.String())),
		PriceCommit:   e.commitToPrice(outcome.ClearingPrice),
		NumBids:       len(decryptedBids),
		ExecutionTime: time.Since(startTime),
		EnclaveQuote:  e.Quote,
		Transcript:    e.sealTranscript(transcript),
	}
	
	e.processed++
	
	e.log.Info("auction executed in enclave",
		"auction", auctionID,
		"bids", len(decryptedBids),
		"winner", outcome.WinnerID,
		"price", outcome.ClearingPrice,
		"time_ms", result.ExecutionTime.Milliseconds())
	
	return result, nil
}

// BidData represents decrypted bid data
type BidData struct {
	BidderID   ids.ID
	Value      uint64
	CreativeID ids.ID
	Targeting  map[string]string
}

// decryptBid decrypts a bid inside the enclave
func (e *Enclave) decryptBid(encryptedBid []byte) (*BidData, error) {
	// In production, use proper decryption with enclave keys
	// For simulation, extract bid from encrypted data
	
	if len(encryptedBid) < 16 {
		return nil, errors.New("invalid encrypted bid")
	}
	
	// Simulated decryption
	bid := &BidData{
		BidderID:   ids.GenerateTestID(),
		Value:      uint64(rand.Intn(1000)),
		CreativeID: ids.GenerateTestID(),
		Targeting:  make(map[string]string),
	}
	
	return bid, nil
}

// runSecondPriceAuction executes the auction logic
func (e *Enclave) runSecondPriceAuction(bids []*BidData, reserve uint64) *auction.AuctionOutcome {
	if len(bids) == 0 {
		return &auction.AuctionOutcome{
			WinnerID:      ids.Empty,
			WinningBid:    0,
			ClearingPrice: 0,
		}
	}
	
	// Find highest and second highest
	var highest, secondHighest *BidData
	
	for _, bid := range bids {
		if bid.Value < reserve {
			continue
		}
		
		if highest == nil || bid.Value > highest.Value {
			secondHighest = highest
			highest = bid
		} else if secondHighest == nil || bid.Value > secondHighest.Value {
			secondHighest = bid
		}
	}
	
	if highest == nil {
		return &auction.AuctionOutcome{
			WinnerID:      ids.Empty,
			WinningBid:    0,
			ClearingPrice: 0,
		}
	}
	
	// Determine clearing price
	clearingPrice := reserve
	if secondHighest != nil {
		clearingPrice = secondHighest.Value
	}
	
	return &auction.AuctionOutcome{
		WinnerID:      highest.BidderID,
		WinningBid:    highest.Value,
		ClearingPrice: clearingPrice,
	}
}

// generateTranscript creates an audit log
func (e *Enclave) generateTranscript(sealed *SealedAuction, bids []*BidData, outcome *auction.AuctionOutcome) []byte {
	transcript := map[string]interface{}{
		"auction_id":     sealed.ID.String(),
		"num_bids":       len(bids),
		"reserve":        sealed.Reserve,
		"winner_id":      outcome.WinnerID.String(),
		"winning_bid":    outcome.WinningBid,
		"clearing_price": outcome.ClearingPrice,
		"timestamp":      time.Now().Unix(),
		"enclave_id":     e.ID.String(),
	}
	
	data, _ := json.Marshal(transcript)
	return data
}

// sealTranscript encrypts the transcript with the sealing key
func (e *Enclave) sealTranscript(transcript []byte) []byte {
	// In production, use SGX sealing or equivalent
	// For simulation, XOR with sealing key
	sealed := make([]byte, len(transcript))
	for i := range transcript {
		sealed[i] = transcript[i] ^ e.sealingKey[i%len(e.sealingKey)]
	}
	return sealed
}

// commitToPrice creates a commitment to the price
func (e *Enclave) commitToPrice(price uint64) []byte {
	priceBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		priceBytes[i] = byte(price >> (8 * (7 - i)))
	}
	return crypto.CreateCommitment(priceBytes)
}

// EnclaveAuctionResult represents the output from enclave auction
type EnclaveAuctionResult struct {
	AuctionID     ids.ID
	WinnerCommit  []byte
	PriceCommit   []byte
	NumBids       int
	ExecutionTime time.Duration
	EnclaveQuote  []byte
	Transcript    []byte // Sealed audit log
}

// VerifyAttestation verifies an enclave's attestation quote
func VerifyAttestation(quote []byte, expectedMREnclave []byte) bool {
	// In production, verify with Intel/AMD attestation service
	// For simulation, check structure
	
	if len(quote) < 64 {
		return false
	}
	
	// Extract statement from quote
	statementData := quote[:len(quote)-32]
	
	var statement AttestationStatement
	if err := json.Unmarshal(statementData, &statement); err != nil {
		return false
	}
	
	// Verify MREnclave matches expected
	if expectedMREnclave != nil {
		if string(statement.MREnclave) != string(expectedMREnclave) {
			return false
		}
	}
	
	// Verify signature (simplified)
	signature := quote[len(quote)-32:]
	expectedSig := crypto.CreateCommitment(statementData)
	
	return string(signature) == string(expectedSig)
}

// GetAttestation returns the enclave's attestation data
func (e *Enclave) GetAttestation() *core.BaseHeader {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	return &core.BaseHeader{
		Type:      "enclave_attestation",
		ID:        e.ID,
		Timestamp: e.AttestedTime,
		Signature: e.Quote,
	}
}