package hashing

import (
	"crypto/sha256"
	"hash"
)

// Hash represents a cryptographic hash function
type Hash interface {
	Write([]byte) (int, error)
	Sum([]byte) []byte
	Reset()
	Size() int
	BlockSize() int
}

// NewSHA256 returns a new SHA256 hasher
func NewSHA256() Hash {
	return sha256.New()
}

// SHA256 computes the SHA256 hash of data
func SHA256(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// Hasher wraps the standard library hash interface
type Hasher struct {
	hash.Hash
}

// NewHasher creates a new hasher
func NewHasher() *Hasher {
	return &Hasher{Hash: sha256.New()}
}

// HashBytes returns the hash of the input bytes
func (h *Hasher) HashBytes(data []byte) []byte {
	h.Reset()
	h.Write(data)
	return h.Sum(nil)
}