// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package core

import (
	"crypto/rand"
	"errors"
	"sync"
	"time"

	"github.com/luxfi/adx/crypto"
	"github.com/luxfi/adx/pkg/ids"
	"github.com/luxfi/adx/pkg/log"
)

var (
	ErrFrequencyCapExceeded = errors.New("frequency cap exceeded")
	ErrInvalidToken         = errors.New("invalid privacy token")
)

// FrequencyManager manages frequency capping without user IDs
type FrequencyManager struct {
	mu        sync.RWMutex
	counters  map[string]*CampaignCounter // Device-local counters
	tokens    map[string]*TokenBucket     // Privacy-preserving tokens
	epochRoot []byte                      // Commitment to current epoch state
	log       log.Logger
}

// CampaignCounter tracks impressions per campaign (device-local)
type CampaignCounter struct {
	CampaignID ids.ID
	Count      uint32
	Cap        uint32
	EpochID    uint32
}

// TokenBucket implements Privacy Pass style tokens
type TokenBucket struct {
	CampaignID ids.ID
	Tokens     []PrivacyToken
	MaxTokens  uint32
	EpochID    uint32
}

// PrivacyToken represents an unlinkable token (Privacy Pass / Private State Token)
type PrivacyToken struct {
	Token     []byte // Blinded token
	Proof     []byte // Redemption proof
	Redeemed  bool
}

// NewFrequencyManager creates a new frequency manager
func NewFrequencyManager(logger log.Logger) *FrequencyManager {
	return &FrequencyManager{
		counters: make(map[string]*CampaignCounter),
		tokens:   make(map[string]*TokenBucket),
		log:      logger,
	}
}

// CheckAndIncrementCounter checks frequency cap and increments if allowed
func (fm *FrequencyManager) CheckAndIncrementCounter(
	deviceID string,
	campaignID ids.ID,
	cap uint32,
) (*FrequencyProof, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	
	key := deviceID + ":" + campaignID.String()
	counter, exists := fm.counters[key]
	
	if !exists {
		counter = &CampaignCounter{
			CampaignID: campaignID,
			Count:      0,
			Cap:        cap,
			EpochID:    fm.getCurrentEpoch(),
		}
		fm.counters[key] = counter
	}
	
	// Check if cap exceeded
	if counter.Count >= counter.Cap {
		return nil, ErrFrequencyCapExceeded
	}
	
	// Generate ZK proof that counter < cap
	proof := fm.generateFrequencyProof(counter, false)
	
	// Increment counter
	counter.Count++
	
	// Update epoch root
	fm.updateEpochRoot()
	
	fm.log.Debug("frequency check passed",
		"campaign", campaignID,
		"count", counter.Count,
		"cap", counter.Cap)
	
	return proof, nil
}

// RedeemPrivacyToken redeems a Privacy Pass style token
func (fm *FrequencyManager) RedeemPrivacyToken(
	campaignID ids.ID,
	token []byte,
	proof []byte,
) (*FrequencyProof, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	
	bucket, exists := fm.tokens[campaignID.String()]
	if !exists {
		return nil, ErrInvalidToken
	}
	
	// Find and validate token
	var foundToken *PrivacyToken
	for i := range bucket.Tokens {
		if string(bucket.Tokens[i].Token) == string(token) {
			foundToken = &bucket.Tokens[i]
			break
		}
	}
	
	if foundToken == nil {
		return nil, ErrInvalidToken
	}
	
	if foundToken.Redeemed {
		return nil, errors.New("token already redeemed")
	}
	
	// Verify redemption proof
	if !fm.verifyTokenProof(token, proof) {
		return nil, ErrInvalidToken
	}
	
	// Mark as redeemed
	foundToken.Redeemed = true
	
	// Generate frequency proof
	freqProof := fm.generateTokenProof(bucket, token)
	
	fm.log.Debug("privacy token redeemed",
		"campaign", campaignID)
	
	return freqProof, nil
}

// FrequencyProof proves frequency cap compliance without revealing counts
type FrequencyProof struct {
	Type       string `json:"type"` // "counter" or "token"
	FreqRoot   []byte `json:"freq_root"`
	Proof      []byte `json:"proof"`
	EpochID    uint32 `json:"epoch_id"`
}

// generateFrequencyProof creates a ZK proof of frequency compliance
func (fm *FrequencyManager) generateFrequencyProof(counter *CampaignCounter, exceeded bool) *FrequencyProof {
	// Simplified proof generation
	// In production, use actual ZK proving system (Halo2/Plonky3)
	// Proves: counter.Count < counter.Cap
	
	proofData := []byte{
		byte(counter.Count >> 8),
		byte(counter.Count),
		byte(counter.Cap >> 8),
		byte(counter.Cap),
	}
	
	if exceeded {
		proofData = append(proofData, 0xFF)
	} else {
		proofData = append(proofData, 0x00)
	}
	
	return &FrequencyProof{
		Type:     "counter",
		FreqRoot: fm.epochRoot,
		Proof:    crypto.CreateCommitment(proofData),
		EpochID:  counter.EpochID,
	}
}

// generateTokenProof creates a proof of valid token redemption
func (fm *FrequencyManager) generateTokenProof(bucket *TokenBucket, token []byte) *FrequencyProof {
	// Simplified proof generation
	// Proves: token is valid and not previously redeemed
	
	proofData := append(token[:16], byte(bucket.EpochID))
	
	return &FrequencyProof{
		Type:     "token",
		FreqRoot: fm.epochRoot,
		Proof:    crypto.CreateCommitment(proofData),
		EpochID:  bucket.EpochID,
	}
}

// verifyTokenProof verifies a Privacy Pass redemption proof
func (fm *FrequencyManager) verifyTokenProof(token, proof []byte) bool {
	// Simplified verification
	// In production, use actual Privacy Pass verification
	return len(token) > 0 && len(proof) > 0
}

// updateEpochRoot updates the commitment to epoch state
func (fm *FrequencyManager) updateEpochRoot() {
	// Create merkle root of all counters
	// Simplified to hash of counter count
	rootData := make([]byte, 0)
	
	for _, counter := range fm.counters {
		rootData = append(rootData, byte(counter.Count))
	}
	
	fm.epochRoot = crypto.CreateCommitment(rootData)
}

// getCurrentEpoch returns the current epoch ID
func (fm *FrequencyManager) getCurrentEpoch() uint32 {
	// Simplified: use hour-based epochs
	return uint32(time.Now().Unix() / 3600)
}

// IssueTokens issues Privacy Pass tokens for a campaign
func (fm *FrequencyManager) IssueTokens(
	deviceID string,
	campaignID ids.ID,
	count uint32,
) ([]PrivacyToken, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	
	bucket := &TokenBucket{
		CampaignID: campaignID,
		Tokens:     make([]PrivacyToken, count),
		MaxTokens:  count,
		EpochID:    fm.getCurrentEpoch(),
	}
	
	// Generate unlinkable tokens
	for i := uint32(0); i < count; i++ {
		tokenData := make([]byte, 32)
		rand.Read(tokenData)
		
		bucket.Tokens[i] = PrivacyToken{
			Token:    tokenData,
			Proof:    crypto.CreateCommitment(tokenData),
			Redeemed: false,
		}
	}
	
	fm.tokens[campaignID.String()] = bucket
	
	fm.log.Debug("tokens issued",
		"campaign", campaignID,
		"count", count)
	
	return bucket.Tokens, nil
}