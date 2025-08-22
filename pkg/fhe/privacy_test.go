package fhe

import (
	"math/big"
	"testing"

	"github.com/shopspring/decimal"
)

func TestNewFHEScheme(t *testing.T) {
	fhe, err := NewFHEScheme(512)
	if err != nil {
		t.Fatalf("Failed to create FHE scheme: %v", err)
	}
	
	if fhe.n == nil || fhe.g == nil {
		t.Error("FHE parameters not initialized")
	}
	
	if fhe.publicKey == nil || fhe.privateKey == nil {
		t.Error("Keys not generated")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	fhe, err := NewFHEScheme(512)
	if err != nil {
		t.Fatalf("Failed to create FHE scheme: %v", err)
	}
	
	// Test bid encryption
	amount := decimal.NewFromFloat(25.50)
	encrypted, err := fhe.EncryptBid(amount)
	if err != nil {
		t.Fatalf("Failed to encrypt bid: %v", err)
	}
	
	// Decrypt
	decrypted, err := fhe.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}
	
	// Check value (in cents)
	expectedCents := int64(2550)
	if decrypted.Int64() != expectedCents {
		t.Errorf("Expected %d cents, got %d", expectedCents, decrypted.Int64())
	}
}

func TestHomomorphicAddition(t *testing.T) {
	fhe, err := NewFHEScheme(512)
	if err != nil {
		t.Fatalf("Failed to create FHE scheme: %v", err)
	}
	
	// Encrypt two bids
	bid1 := decimal.NewFromFloat(10.00)
	bid2 := decimal.NewFromFloat(15.00)
	
	enc1, err := fhe.EncryptBid(bid1)
	if err != nil {
		t.Fatalf("Failed to encrypt bid1: %v", err)
	}
	
	enc2, err := fhe.EncryptBid(bid2)
	if err != nil {
		t.Fatalf("Failed to encrypt bid2: %v", err)
	}
	
	// Add encrypted values
	sum, err := fhe.AddEncrypted(enc1, enc2)
	if err != nil {
		t.Fatalf("Failed to add encrypted values: %v", err)
	}
	
	// Decrypt sum
	decrypted, err := fhe.Decrypt(sum)
	if err != nil {
		t.Fatalf("Failed to decrypt sum: %v", err)
	}
	
	// Check sum (in cents)
	expectedSum := int64(2500)
	if decrypted.Int64() != expectedSum {
		t.Errorf("Expected sum %d cents, got %d", expectedSum, decrypted.Int64())
	}
}

func TestHomomorphicMultiplication(t *testing.T) {
	fhe, err := NewFHEScheme(512)
	if err != nil {
		t.Fatalf("Failed to create FHE scheme: %v", err)
	}
	
	// Encrypt bid
	bid := decimal.NewFromFloat(5.00)
	encrypted, err := fhe.EncryptBid(bid)
	if err != nil {
		t.Fatalf("Failed to encrypt bid: %v", err)
	}
	
	// Multiply by constant
	multiplied, err := fhe.MultiplyByConstant(encrypted, 3)
	if err != nil {
		t.Fatalf("Failed to multiply: %v", err)
	}
	
	// Decrypt
	decrypted, err := fhe.Decrypt(multiplied)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}
	
	// Check result (5.00 * 3 = 15.00 = 1500 cents)
	expectedCents := int64(1500)
	if decrypted.Int64() != expectedCents {
		t.Errorf("Expected %d cents, got %d", expectedCents, decrypted.Int64())
	}
}

func TestPrivateProfile(t *testing.T) {
	fhe, err := NewFHEScheme(512)
	if err != nil {
		t.Fatalf("Failed to create FHE scheme: %v", err)
	}
	
	profile := &PrivateProfile{
		ID:        "user-123",
		Interests: []string{"sports", "technology", "gaming"},
		Demographics: map[string]string{
			"age_group": "25-34",
			"gender":    "unknown",
			"location":  "US",
		},
		Behaviors: []string{"frequent_viewer", "prime_time"},
	}
	
	encrypted, err := fhe.EncryptProfile(profile)
	if err != nil {
		t.Fatalf("Failed to encrypt profile: %v", err)
	}
	
	if len(encrypted.Ciphertext) == 0 {
		t.Error("Empty ciphertext")
	}
	
	if len(encrypted.Nonce) != 16 {
		t.Errorf("Expected 16 byte nonce, got %d", len(encrypted.Nonce))
	}
	
	if encrypted.Context != "profile_v1" {
		t.Errorf("Wrong context: %s", encrypted.Context)
	}
}

func TestPrivacyAuction(t *testing.T) {
	fhe, err := NewFHEScheme(512)
	if err != nil {
		t.Fatalf("Failed to create FHE scheme: %v", err)
	}
	
	auction := NewPrivacyAuction(fhe)
	
	// Submit encrypted bids
	bids := map[string]decimal.Decimal{
		"bidder1": decimal.NewFromFloat(10.00),
		"bidder2": decimal.NewFromFloat(15.00),
		"bidder3": decimal.NewFromFloat(12.50),
		"bidder4": decimal.NewFromFloat(18.00),
	}
	
	for bidderID, amount := range bids {
		err := auction.SubmitBid(bidderID, amount)
		if err != nil {
			t.Errorf("Failed to submit bid for %s: %v", bidderID, err)
		}
	}
	
	// Determine winner
	winner, err := auction.DetermineWinner()
	if err != nil {
		t.Fatalf("Failed to determine winner: %v", err)
	}
	
	// Should be bidder4 with highest bid
	if winner != "bidder4" {
		t.Errorf("Expected bidder4 to win, got %s", winner)
	}
}

func TestSecureMatch(t *testing.T) {
	fhe, err := NewFHEScheme(512)
	if err != nil {
		t.Fatalf("Failed to create FHE scheme: %v", err)
	}
	
	// Create and encrypt profile
	profile := &PrivateProfile{
		ID:        "user-456",
		Interests: []string{"sports", "fitness", "health"},
		Demographics: map[string]string{
			"age_group": "25-34",
		},
	}
	
	encProfile, err := fhe.EncryptProfile(profile)
	if err != nil {
		t.Fatalf("Failed to encrypt profile: %v", err)
	}
	
	// Create targeting criteria
	criteria := &TargetingCriteria{
		Categories: []string{"sports", "fitness"},
		MinAge:     25,
		MaxAge:     34,
		Interests:  []string{"sports", "fitness"},
	}
	
	// Test secure matching
	match, err := fhe.SecureMatch(encProfile, criteria)
	if err != nil {
		t.Fatalf("Failed to perform secure match: %v", err)
	}
	
	// Should match based on overlapping interests
	if !match {
		t.Error("Expected profile to match criteria")
	}
}

func TestPublicKeyExportImport(t *testing.T) {
	fhe, err := NewFHEScheme(512)
	if err != nil {
		t.Fatalf("Failed to create FHE scheme: %v", err)
	}
	
	// Export public key
	exported := fhe.ExportPublicKey()
	if exported == "" {
		t.Error("Failed to export public key")
	}
	
	// Import public key
	imported, err := ImportPublicKey(exported)
	if err != nil {
		t.Fatalf("Failed to import public key: %v", err)
	}
	
	// Verify keys match
	if imported.N.Cmp(fhe.publicKey.N) != 0 {
		t.Error("Imported N doesn't match")
	}
	
	if imported.G.Cmp(fhe.publicKey.G) != 0 {
		t.Error("Imported G doesn't match")
	}
}

func BenchmarkEncryption(b *testing.B) {
	fhe, _ := NewFHEScheme(1024)
	amount := decimal.NewFromFloat(10.50)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fhe.EncryptBid(amount)
	}
}

func BenchmarkHomomorphicAddition(b *testing.B) {
	fhe, _ := NewFHEScheme(1024)
	bid1 := decimal.NewFromFloat(10.00)
	bid2 := decimal.NewFromFloat(15.00)
	
	enc1, _ := fhe.EncryptBid(bid1)
	enc2, _ := fhe.EncryptBid(bid2)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fhe.AddEncrypted(enc1, enc2)
	}
}

func BenchmarkDecryption(b *testing.B) {
	fhe, _ := NewFHEScheme(1024)
	amount := decimal.NewFromFloat(10.50)
	encrypted, _ := fhe.EncryptBid(amount)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fhe.Decrypt(encrypted)
	}
}

// Helper function for testing
func compareBigInt(a, b *big.Int) bool {
	return a.Cmp(b) == 0
}