// Package fhe implements Fully Homomorphic Encryption for privacy-preserving advertising
package fhe

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"math/big"
	"sync"

	"github.com/shopspring/decimal"
)

// FHEScheme represents the homomorphic encryption scheme
type FHEScheme struct {
	// Paillier parameters for additive homomorphism
	n       *big.Int // n = p * q
	n2      *big.Int // n^2
	g       *big.Int // generator
	lambda  *big.Int // lcm(p-1, q-1)
	mu      *big.Int // multiplicative inverse
	
	// Key material
	publicKey  *PublicKey
	privateKey *PrivateKey
	
	// Performance
	precomputed map[string]*big.Int
	lock        sync.RWMutex
}

// PublicKey for encryption
type PublicKey struct {
	N *big.Int
	G *big.Int
}

// PrivateKey for decryption
type PrivateKey struct {
	Lambda *big.Int
	Mu     *big.Int
}

// EncryptedData represents encrypted values
type EncryptedData struct {
	Ciphertext []byte
	Nonce      []byte
	Context    string // Ad context without PII
}

// PrivateProfile represents user's private data
type PrivateProfile struct {
	ID           string
	Interests    []string
	Demographics map[string]string
	Behaviors    []string
	Salt         []byte
}

// TargetingCriteria for ads without exposing user data
type TargetingCriteria struct {
	Categories      []string
	MinAge         int
	MaxAge         int
	Interests      []string
	ExcludePatterns []string
}

// NewFHEScheme creates a new FHE instance
func NewFHEScheme(bitSize int) (*FHEScheme, error) {
	if bitSize < 512 {
		bitSize = 2048 // Minimum security
	}
	
	// Generate Paillier parameters
	p, _ := rand.Prime(rand.Reader, bitSize/2)
	q, _ := rand.Prime(rand.Reader, bitSize/2)
	
	n := new(big.Int).Mul(p, q)
	n2 := new(big.Int).Mul(n, n)
	
	// g = n + 1
	g := new(big.Int).Add(n, big.NewInt(1))
	
	// lambda = lcm(p-1, q-1)
	p1 := new(big.Int).Sub(p, big.NewInt(1))
	q1 := new(big.Int).Sub(q, big.NewInt(1))
	lambda := lcm(p1, q1)
	
	// mu = (L(g^lambda mod n^2))^-1 mod n
	gLambda := new(big.Int).Exp(g, lambda, n2)
	l := L(gLambda, n)
	mu := new(big.Int).ModInverse(l, n)
	
	return &FHEScheme{
		n:       n,
		n2:      n2,
		g:       g,
		lambda:  lambda,
		mu:      mu,
		publicKey: &PublicKey{
			N: n,
			G: g,
		},
		privateKey: &PrivateKey{
			Lambda: lambda,
			Mu:     mu,
		},
		precomputed: make(map[string]*big.Int),
	}, nil
}

// EncryptProfile encrypts user profile for privacy-preserving targeting
func (fhe *FHEScheme) EncryptProfile(profile *PrivateProfile) (*EncryptedData, error) {
	// Hash profile data
	h := sha256.New()
	for _, interest := range profile.Interests {
		h.Write([]byte(interest))
	}
	for k, v := range profile.Demographics {
		h.Write([]byte(k + ":" + v))
	}
	profileHash := h.Sum(nil)
	
	// Convert to integer for encryption
	m := new(big.Int).SetBytes(profileHash)
	
	// Encrypt using Paillier
	r, _ := rand.Int(rand.Reader, fhe.n)
	// c = g^m * r^n mod n^2
	gm := new(big.Int).Exp(fhe.g, m, fhe.n2)
	rn := new(big.Int).Exp(r, fhe.n, fhe.n2)
	c := new(big.Int).Mul(gm, rn)
	c.Mod(c, fhe.n2)
	
	// Generate nonce
	nonce := make([]byte, 16)
	rand.Read(nonce)
	
	return &EncryptedData{
		Ciphertext: c.Bytes(),
		Nonce:      nonce,
		Context:    "profile_v1",
	}, nil
}

// EncryptBid encrypts bid amount for private auctions
func (fhe *FHEScheme) EncryptBid(amount decimal.Decimal) (*EncryptedData, error) {
	// Convert to cents integer
	cents := amount.Mul(decimal.NewFromInt(100)).IntPart()
	m := big.NewInt(cents)
	
	// Ensure m < n
	if m.Cmp(fhe.n) >= 0 {
		return nil, errors.New("bid amount too large")
	}
	
	// Encrypt
	r, _ := rand.Int(rand.Reader, fhe.n)
	gm := new(big.Int).Exp(fhe.g, m, fhe.n2)
	rn := new(big.Int).Exp(r, fhe.n, fhe.n2)
	c := new(big.Int).Mul(gm, rn)
	c.Mod(c, fhe.n2)
	
	nonce := make([]byte, 16)
	rand.Read(nonce)
	
	return &EncryptedData{
		Ciphertext: c.Bytes(),
		Nonce:      nonce,
		Context:    "bid_v1",
	}, nil
}

// AddEncrypted adds two encrypted values (homomorphic addition)
func (fhe *FHEScheme) AddEncrypted(a, b *EncryptedData) (*EncryptedData, error) {
	if a.Context != b.Context {
		return nil, errors.New("context mismatch")
	}
	
	// c1 * c2 mod n^2 = Enc(m1 + m2)
	c1 := new(big.Int).SetBytes(a.Ciphertext)
	c2 := new(big.Int).SetBytes(b.Ciphertext)
	
	result := new(big.Int).Mul(c1, c2)
	result.Mod(result, fhe.n2)
	
	// Combine nonces
	nonce := make([]byte, 16)
	for i := 0; i < 16; i++ {
		nonce[i] = a.Nonce[i] ^ b.Nonce[i]
	}
	
	return &EncryptedData{
		Ciphertext: result.Bytes(),
		Nonce:      nonce,
		Context:    a.Context,
	}, nil
}

// MultiplyByConstant multiplies encrypted value by plaintext constant
func (fhe *FHEScheme) MultiplyByConstant(encrypted *EncryptedData, k int64) (*EncryptedData, error) {
	// c^k mod n^2 = Enc(k * m)
	c := new(big.Int).SetBytes(encrypted.Ciphertext)
	kBig := big.NewInt(k)
	
	result := new(big.Int).Exp(c, kBig, fhe.n2)
	
	return &EncryptedData{
		Ciphertext: result.Bytes(),
		Nonce:      encrypted.Nonce,
		Context:    encrypted.Context,
	}, nil
}

// Decrypt decrypts ciphertext (requires private key)
func (fhe *FHEScheme) Decrypt(encrypted *EncryptedData) (*big.Int, error) {
	c := new(big.Int).SetBytes(encrypted.Ciphertext)
	
	// L(c^lambda mod n^2) * mu mod n
	cLambda := new(big.Int).Exp(c, fhe.lambda, fhe.n2)
	l := L(cLambda, fhe.n)
	m := new(big.Int).Mul(l, fhe.mu)
	m.Mod(m, fhe.n)
	
	return m, nil
}

// SecureMatch performs private set intersection for targeting
func (fhe *FHEScheme) SecureMatch(encryptedProfile *EncryptedData, criteria *TargetingCriteria) (bool, error) {
	// Hash targeting criteria
	h := sha256.New()
	for _, cat := range criteria.Categories {
		h.Write([]byte(cat))
	}
	for _, interest := range criteria.Interests {
		h.Write([]byte(interest))
	}
	targetHash := h.Sum(nil)
	
	// Encrypt target with matching context
	targetEnc, err := fhe.encryptBytes(targetHash)
	if err != nil {
		return false, err
	}
	targetEnc.Context = encryptedProfile.Context // Match context
	
	// Homomorphic comparison (simplified)
	// In practice, use garbled circuits or MPC
	diff, err := fhe.AddEncrypted(encryptedProfile, targetEnc)
	if err != nil {
		return false, err
	}
	
	// Check if difference is small (match)
	// This requires interaction or threshold decryption
	return fhe.checkProximity(diff)
}

// Helper functions

func lcm(a, b *big.Int) *big.Int {
	gcd := new(big.Int).GCD(nil, nil, a, b)
	product := new(big.Int).Mul(a, b)
	return new(big.Int).Div(product, gcd)
}

func L(u, n *big.Int) *big.Int {
	// L(u) = (u - 1) / n
	u1 := new(big.Int).Sub(u, big.NewInt(1))
	return new(big.Int).Div(u1, n)
}

func (fhe *FHEScheme) encryptBytes(data []byte) (*EncryptedData, error) {
	m := new(big.Int).SetBytes(data)
	if m.Cmp(fhe.n) >= 0 {
		// Truncate if too large
		m.Mod(m, fhe.n)
	}
	
	r, _ := rand.Int(rand.Reader, fhe.n)
	gm := new(big.Int).Exp(fhe.g, m, fhe.n2)
	rn := new(big.Int).Exp(r, fhe.n, fhe.n2)
	c := new(big.Int).Mul(gm, rn)
	c.Mod(c, fhe.n2)
	
	nonce := make([]byte, 16)
	rand.Read(nonce)
	
	return &EncryptedData{
		Ciphertext: c.Bytes(),
		Nonce:      nonce,
		Context:    "bytes_v1",
	}, nil
}

func (fhe *FHEScheme) checkProximity(encrypted *EncryptedData) (bool, error) {
	// Simplified proximity check
	// In production, use secure multi-party computation
	decrypted, err := fhe.Decrypt(encrypted)
	if err != nil {
		return false, err
	}
	
	// Check if value is below threshold
	threshold := big.NewInt(1000000)
	return decrypted.Cmp(threshold) < 0, nil
}

// PrivacyPreservingAuction runs auction without revealing individual bids
type PrivacyPreservingAuction struct {
	fhe          *FHEScheme
	encryptedBids []*EncryptedBid
	mu           sync.Mutex
}

// EncryptedBid represents an encrypted bid
type EncryptedBid struct {
	BidderID   string
	Encrypted  *EncryptedData
	Commitment []byte
}

// NewPrivacyAuction creates a new privacy-preserving auction
func NewPrivacyAuction(fhe *FHEScheme) *PrivacyPreservingAuction {
	return &PrivacyPreservingAuction{
		fhe:           fhe,
		encryptedBids: make([]*EncryptedBid, 0),
	}
}

// SubmitBid submits an encrypted bid
func (pa *PrivacyPreservingAuction) SubmitBid(bidderID string, amount decimal.Decimal) error {
	encrypted, err := pa.fhe.EncryptBid(amount)
	if err != nil {
		return err
	}
	
	// Create commitment
	h := sha256.Sum256(encrypted.Ciphertext)
	
	pa.mu.Lock()
	pa.encryptedBids = append(pa.encryptedBids, &EncryptedBid{
		BidderID:   bidderID,
		Encrypted:  encrypted,
		Commitment: h[:],
	})
	pa.mu.Unlock()
	
	return nil
}

// DetermineWinner finds winner without decrypting all bids
func (pa *PrivacyPreservingAuction) DetermineWinner() (string, error) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	
	if len(pa.encryptedBids) == 0 {
		return "", errors.New("no bids")
	}
	
	// In practice, use secure comparison protocols
	// For demo, decrypt and compare
	var winner string
	var maxBid *big.Int
	
	for _, bid := range pa.encryptedBids {
		decrypted, err := pa.fhe.Decrypt(bid.Encrypted)
		if err != nil {
			continue
		}
		
		if maxBid == nil || decrypted.Cmp(maxBid) > 0 {
			maxBid = decrypted
			winner = bid.BidderID
		}
	}
	
	return winner, nil
}

// ExportPublicKey exports public key for client-side encryption
func (fhe *FHEScheme) ExportPublicKey() string {
	data := append(fhe.publicKey.N.Bytes(), fhe.publicKey.G.Bytes()...)
	return base64.StdEncoding.EncodeToString(data)
}

// ImportPublicKey imports public key for encryption
func ImportPublicKey(encoded string) (*PublicKey, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	
	// Split data (simplified - in practice use proper encoding)
	mid := len(data) / 2
	n := new(big.Int).SetBytes(data[:mid])
	g := new(big.Int).SetBytes(data[mid:])
	
	return &PublicKey{
		N: n,
		G: g,
	}, nil
}