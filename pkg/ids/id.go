package ids

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// ID represents a unique identifier
type ID [32]byte

// GenerateTestID creates a random ID for testing
func GenerateTestID() ID {
	var id ID
	rand.Read(id[:])
	return id
}

// String returns the hex representation of the ID
func (id ID) String() string {
	return hex.EncodeToString(id[:])
}

// Bytes returns the byte representation of the ID
func (id ID) Bytes() []byte {
	return id[:]
}

// FromString creates an ID from a hex string
func FromString(s string) (ID, error) {
	var id ID
	bytes, err := hex.DecodeString(s)
	if err != nil {
		return id, err
	}
	if len(bytes) != 32 {
		return id, fmt.Errorf("invalid ID length: expected 32, got %d", len(bytes))
	}
	copy(id[:], bytes)
	return id, nil
}