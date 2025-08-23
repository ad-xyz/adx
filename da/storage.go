// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package da

import (
	"bytes"
	"errors"
	"sync"
	"time"

	"github.com/luxfi/adx/crypto"
	"github.com/luxfi/crypto/hashing"
	"github.com/luxfi/ids"
	"github.com/luxfi/log"
)

var (
	ErrBlobNotFound    = errors.New("blob not found")
	ErrBlobTooLarge    = errors.New("blob exceeds maximum size")
	ErrInvalidCommitment = errors.New("invalid blob commitment")
)

// DALayer represents the data availability layer type
type DALayer string

const (
	DALayerEIP4844   DALayer = "eip4844"
	DALayerCelestia  DALayer = "celestia"
	DALayerIPFS      DALayer = "ipfs"
	DALayerLocal     DALayer = "local" // For testing
)

const (
	MaxBlobSize     = 128 * 1024 // 128KB (EIP-4844 size)
	BlobExpiration  = 18 * 24 * time.Hour // 18 days (EIP-4844 retention)
)

// DataAvailability manages blob storage across different DA layers
type DataAvailability struct {
	mu sync.RWMutex
	
	// Configuration
	layer    DALayer
	
	// Storage backends
	blobs    map[ids.ID]*Blob
	commits  map[ids.ID][]byte
	
	// Metrics
	stored   uint64
	retrieved uint64
	
	log      log.Logger
}

// Blob represents a data blob
type Blob struct {
	ID          ids.ID
	Data        []byte
	Commitment  []byte
	Timestamp   time.Time
	Expiry      time.Time
	Layer       DALayer
}

// NewDataAvailability creates a new DA manager
func NewDataAvailability(layer DALayer, logger log.Logger) *DataAvailability {
	return &DataAvailability{
		layer:   layer,
		blobs:   make(map[ids.ID]*Blob),
		commits: make(map[ids.ID][]byte),
		log:     logger,
	}
}

// StoreBlob stores data in the DA layer
func (da *DataAvailability) StoreBlob(data []byte) (*BlobReference, error) {
	if len(data) > MaxBlobSize {
		return nil, ErrBlobTooLarge
	}
	
	da.mu.Lock()
	defer da.mu.Unlock()
	
	// Create blob ID
	blobID := ids.GenerateTestID()
	
	// Create KZG commitment (simplified)
	commitment := da.createCommitment(data)
	
	// Store based on layer
	var ref *BlobReference
	
	switch da.layer {
	case DALayerEIP4844:
		ref = da.storeEIP4844(blobID, data, commitment)
	case DALayerCelestia:
		ref = da.storeCelestia(blobID, data, commitment)
	case DALayerIPFS:
		ref = da.storeIPFS(blobID, data, commitment)
	default:
		ref = da.storeLocal(blobID, data, commitment)
	}
	
	// Store locally for caching
	blob := &Blob{
		ID:         blobID,
		Data:       data,
		Commitment: commitment,
		Timestamp:  time.Now(),
		Expiry:     time.Now().Add(BlobExpiration),
		Layer:      da.layer,
	}
	
	da.blobs[blobID] = blob
	da.commits[blobID] = commitment
	da.stored++
	
	da.log.Debug("blob stored",
		"id", blobID,
		"size", len(data),
		"layer", da.layer,
		"commitment", commitment[:8])
	
	return ref, nil
}

// RetrieveBlob retrieves data from the DA layer
func (da *DataAvailability) RetrieveBlob(ref *BlobReference) ([]byte, error) {
	da.mu.RLock()
	defer da.mu.RUnlock()
	
	// Check local cache first
	if blob, exists := da.blobs[ref.BlobID]; exists {
		if time.Now().Before(blob.Expiry) {
			da.retrieved++
			return blob.Data, nil
		}
	}
	
	// Retrieve from DA layer
	var data []byte
	var err error
	
	switch ref.Layer {
	case DALayerEIP4844:
		data, err = da.retrieveEIP4844(ref)
	case DALayerCelestia:
		data, err = da.retrieveCelestia(ref)
	case DALayerIPFS:
		data, err = da.retrieveIPFS(ref)
	default:
		data, err = da.retrieveLocal(ref)
	}
	
	if err != nil {
		return nil, err
	}
	
	// Verify commitment
	if !da.verifyCommitment(data, ref.Commitment) {
		return nil, ErrInvalidCommitment
	}
	
	da.retrieved++
	
	return data, nil
}

// BlobReference points to data in the DA layer
type BlobReference struct {
	BlobID     ids.ID    `json:"blob_id"`
	Layer      DALayer   `json:"layer"`
	Commitment []byte    `json:"commitment"`
	
	// Layer-specific fields
	EIP4844Hash   []byte `json:"eip4844_hash,omitempty"`
	CelestiaRoot  []byte `json:"celestia_root,omitempty"`
	IPFSHash      string `json:"ipfs_hash,omitempty"`
}

// createCommitment creates a KZG commitment (simplified)
func (da *DataAvailability) createCommitment(data []byte) []byte {
	// In production, use actual KZG commitment
	return hashing.ComputeHash256(data)
}

// verifyCommitment verifies data against commitment
func (da *DataAvailability) verifyCommitment(data []byte, commitment []byte) bool {
	expected := da.createCommitment(data)
	return bytes.Equal(expected, commitment)
}

// storeEIP4844 stores in EIP-4844 blobs
func (da *DataAvailability) storeEIP4844(id ids.ID, data []byte, commitment []byte) *BlobReference {
	// In production, submit to Ethereum L1
	// For simulation, create reference
	
	hash := hashing.ComputeHash256(append(id[:], data...))
	
	return &BlobReference{
		BlobID:      id,
		Layer:       DALayerEIP4844,
		Commitment:  commitment,
		EIP4844Hash: hash,
	}
}

// retrieveEIP4844 retrieves from EIP-4844
func (da *DataAvailability) retrieveEIP4844(ref *BlobReference) ([]byte, error) {
	// In production, query Ethereum L1
	// For simulation, return from cache
	
	if blob, exists := da.blobs[ref.BlobID]; exists {
		return blob.Data, nil
	}
	
	return nil, ErrBlobNotFound
}

// storeCelestia stores in Celestia DA
func (da *DataAvailability) storeCelestia(id ids.ID, data []byte, commitment []byte) *BlobReference {
	// In production, submit to Celestia
	// For simulation, create reference
	
	// Celestia uses Namespaced Merkle Trees
	root := da.createNMTRoot(data)
	
	return &BlobReference{
		BlobID:       id,
		Layer:        DALayerCelestia,
		Commitment:   commitment,
		CelestiaRoot: root,
	}
}

// retrieveCelestia retrieves from Celestia
func (da *DataAvailability) retrieveCelestia(ref *BlobReference) ([]byte, error) {
	// In production, use Data Availability Sampling
	// For simulation, return from cache
	
	if blob, exists := da.blobs[ref.BlobID]; exists {
		return blob.Data, nil
	}
	
	return nil, ErrBlobNotFound
}

// storeIPFS stores in IPFS
func (da *DataAvailability) storeIPFS(id ids.ID, data []byte, commitment []byte) *BlobReference {
	// In production, pin to IPFS
	// For simulation, create fake CID
	
	cid := "Qm" + id.String()[:44] // Fake IPFS CID
	
	return &BlobReference{
		BlobID:     id,
		Layer:      DALayerIPFS,
		Commitment: commitment,
		IPFSHash:   cid,
	}
}

// retrieveIPFS retrieves from IPFS
func (da *DataAvailability) retrieveIPFS(ref *BlobReference) ([]byte, error) {
	// In production, fetch from IPFS
	// For simulation, return from cache
	
	if blob, exists := da.blobs[ref.BlobID]; exists {
		return blob.Data, nil
	}
	
	return nil, ErrBlobNotFound
}

// storeLocal stores locally for testing
func (da *DataAvailability) storeLocal(id ids.ID, data []byte, commitment []byte) *BlobReference {
	return &BlobReference{
		BlobID:     id,
		Layer:      DALayerLocal,
		Commitment: commitment,
	}
}

// retrieveLocal retrieves from local storage
func (da *DataAvailability) retrieveLocal(ref *BlobReference) ([]byte, error) {
	if blob, exists := da.blobs[ref.BlobID]; exists {
		return blob.Data, nil
	}
	
	return nil, ErrBlobNotFound
}

// createNMTRoot creates a Namespaced Merkle Tree root (simplified)
func (da *DataAvailability) createNMTRoot(data []byte) []byte {
	// In production, use actual NMT construction
	// For simulation, use simple hash
	
	namespace := []byte("adx_v1")
	return hashing.ComputeHash256(append(namespace, data...))
}

// GetMetrics returns DA metrics
func (da *DataAvailability) GetMetrics() DAMetrics {
	da.mu.RLock()
	defer da.mu.RUnlock()
	
	active := 0
	expired := 0
	now := time.Now()
	
	for _, blob := range da.blobs {
		if now.Before(blob.Expiry) {
			active++
		} else {
			expired++
		}
	}
	
	return DAMetrics{
		Layer:     string(da.layer),
		Stored:    da.stored,
		Retrieved: da.retrieved,
		Active:    active,
		Expired:   expired,
	}
}

// DAMetrics represents DA statistics
type DAMetrics struct {
	Layer     string
	Stored    uint64
	Retrieved uint64
	Active    int
	Expired   int
}

// EncryptAndStore encrypts data with HPKE before storing
func (da *DataAvailability) EncryptAndStore(
	plaintext []byte,
	recipients [][]byte,
	aad []byte,
) (*EncryptedBlobReference, error) {
	// Encrypt with HPKE
	hpke := crypto.NewHPKE()
	envelope, err := hpke.Seal(plaintext, recipients, aad)
	if err != nil {
		return nil, err
	}
	
	// Serialize envelope
	data := da.serializeEnvelope(envelope)
	
	// Store encrypted data
	ref, err := da.StoreBlob(data)
	if err != nil {
		return nil, err
	}
	
	return &EncryptedBlobReference{
		BlobReference:  *ref,
		NumRecipients:  len(recipients),
		EncryptedSize:  len(data),
		PlaintextHash:  hashing.ComputeHash256(plaintext),
	}, nil
}

// EncryptedBlobReference points to encrypted data
type EncryptedBlobReference struct {
	BlobReference
	NumRecipients int    `json:"num_recipients"`
	EncryptedSize int    `json:"encrypted_size"`
	PlaintextHash []byte `json:"plaintext_hash"`
}

// serializeEnvelope converts HPKE envelope to bytes
func (da *DataAvailability) serializeEnvelope(envelope *crypto.SealedEnvelope) []byte {
	// Simplified serialization
	// In production, use proper encoding
	
	data := make([]byte, 0)
	
	// Number of encapsulations
	data = append(data, byte(len(envelope.Encapsulations)))
	
	// Each encapsulation
	for _, encap := range envelope.Encapsulations {
		data = append(data, encap.EncapsulatedKey...)
		data = append(data, encap.SharedSecret...)
	}
	
	// Ciphertext
	data = append(data, envelope.Ciphertext...)
	
	// AAD
	data = append(data, envelope.AAD...)
	
	return data
}