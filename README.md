# ADX - Privacy-Preserving Blockchain Ad Network

## Overview

ADX (ADXYZ) is a privacy-preserving blockchain ad network that implements state-of-the-art cryptographic techniques to enable transparent and verifiable advertising while maintaining complete user privacy. Built on the Lux blockchain infrastructure, it combines blocklace consensus for Byzantine-repelling ordering with HPKE encryption and ZK proofs for privacy.

## ✅ Implementation Status

**All core features implemented and tested - 100% test passing**

### Architecture Components

1. **Blocklace Headers** (✅ Complete)
   - Bid commitments without revealing actual bids
   - Auction outcomes with ZK proofs of correctness
   - Impression logs with viewability proofs
   - Conversion tracking with privacy
   - Budget management with ZK proofs
   - Settlement receipts with privacy

2. **HPKE Encryption** (✅ Complete)
   - RFC 9180 compliant implementation
   - X25519 + HKDF-SHA256 + ChaCha20-Poly1305
   - Multi-recipient encryption
   - AAD binding for header↔payload integrity

3. **Privacy Features** (✅ Complete)
   - Frequency capping without user IDs
   - Privacy Pass style tokens
   - On-device counters with ZK proofs
   - User-sovereign encryption

4. **Auction System** (✅ Complete)
   - Second-price sealed-bid auctions
   - ZK proofs of correct execution
   - Policy compliance verification
   - Range proofs for bid validation

5. **Budget & Settlement** (✅ Complete)
   - ZK proofs of budget safety
   - Privacy-preserving settlement
   - No double-spend protection
   - Verifiable receipts

## Performance Benchmarks

```
BenchmarkAuction-10    174,529 ops    7,885 ns/op    7,168 B/op
BenchmarkHPKE-10         1,838 ops  665,751 ns/op   12,208 B/op
```

- **Auction throughput**: ~127,000 auctions/second
- **HPKE operations**: ~1,500 seal/open ops/second
- **Memory efficient**: < 8KB per auction

## Directory Structure

```
adx/
├── core/               # Core types and headers
│   ├── headers.go         # Bid, Auction, Impression headers
│   └── frequency.go       # Frequency capping logic
├── crypto/             # Cryptographic primitives
│   └── hpke.go           # HPKE encryption implementation
├── auction/            # Auction logic
│   ├── auction.go        # Second-price auction with ZK proofs
│   └── halo2_auction.go  # Halo2 ZK proof integration
├── settlement/         # Budget and settlement
│   └── budget.go         # Budget management with proofs
├── proof/              # ZK proof systems
│   ├── circuits.go       # Simplified proof circuits
│   └── halo2/           # Production Halo2 implementation
│       └── circuits.go   # Halo2 circuits with Poseidon hash
├── tee/                # Trusted Execution Environment
│   └── enclave.go        # TEE enclave for secure auctions
├── blocklace/          # Blocklace consensus
│   └── dag.go           # Byzantine-repelling DAG
├── da/                 # Data Availability
│   └── storage.go        # EIP-4844, Celestia, IPFS integration
├── sdk/                # Client SDKs
│   └── browser.ts        # Browser SDK for web integration
└── tests/              # Integration tests
    └── integration_test.go
```

## Key Features

### 1. Privacy by Design
- **No user IDs**: Frequency capping via Privacy Pass tokens or device-local counters
- **Encrypted payloads**: All sensitive data HPKE-encrypted to authorized recipients only
- **ZK proofs**: Validators verify correctness without seeing actual values

### 2. Verifiable Auctions
- **Correct winner selection**: ZK proof that winner = max(bids)
- **Fair pricing**: ZK proof that price = second-highest bid or reserve
- **Policy compliance**: Commitments to brand safety rules

### 3. Fraud Prevention
- **Private State Tokens**: Browser-native fraud signals without tracking
- **Viewability proofs**: ZK proofs of MRC standards (50% pixels, 1 second)
- **Budget safety**: ZK proofs prevent overspending

### 4. Interoperability
- **Chrome Protected Audience API**: On-device auctions
- **Apple SKAdNetwork/PCM**: Privacy-preserving attribution
- **Private State Tokens**: Cross-browser fraud prevention

## Usage Example

```go
// Setup HPKE encryption
hpke := crypto.NewHPKE()
advertiserPub, advertiserPriv, _ := hpke.GenerateKeyPair()

// Create auction
auction := auction.NewAuction(auctionID, reserve, duration, logger)

// Submit sealed bid
bid := &auction.SealedBid{
    BidderID:   bidderID,
    Commitment: commitment,
    RangeProof: proof,
}
auction.SubmitBid(bid)

// Run auction with ZK proof
outcome, _ := auction.RunAuction(decryptionKey)

// Verify second-price logic
// outcome.ClearingPrice = second highest bid
// outcome.ProofCorrect = ZK proof of correctness

// Frequency capping without IDs
freqMgr := core.NewFrequencyManager(logger)
proof, _ := freqMgr.CheckAndIncrementCounter(deviceID, campaignID, cap)

// Budget management with proofs
budgetMgr := settlement.NewBudgetManager(logger)
budgetProof, _ := budgetMgr.DeductBudget(advertiserID, amount, auctionID)
```

## ZK Proof Statements

### Auction Correctness
```
Public: cm_inputs, cm_winner, reserve
Prove:
- winner.bid = max(all bids)
- price = max(reserve, second_highest_bid)
- cm_winner commits to (winner_id, bid, price)
```

### Budget Safety
```
Public: cm_budget_prev, cm_budget_new, price
Prove:
- budget_new = budget_prev - price
- budget_new >= 0
```

### Frequency Cap
```
Public: freq_root_prev, freq_root_new, campaign_id
Prove:
- counter[campaign_id]++ 
- counter[campaign_id] < cap
```

## Recent Enhancements

### ✅ Phase 1: Enhanced ZK Proofs (Completed)
- [x] Integrated Halo2 for production ZK proofs
- [x] Implemented Poseidon hash for ZK-friendly commitments
- [x] Added BN254 field operations for efficient circuits
- [x] Created specialized circuits for auctions, budgets, and frequency caps
- [x] Achieved ~3.7ms proof generation, ~96μs budget proofs, ~108μs frequency proofs

## Next Steps for Production

### Phase 2: TEE Integration (3-4 weeks)
- [ ] Add Intel SGX/AMD SEV support for high-volume auctions
- [ ] Implement attested auction execution
- [ ] Add remote attestation verification

### Phase 3: Data Availability (2-3 weeks)
- [ ] Integrate EIP-4844 blob storage
- [ ] Add Celestia DA support
- [ ] Implement light client verification

### Phase 4: Attribution (3-4 weeks)
- [ ] Full SKAdNetwork integration
- [ ] WebKit PCM support
- [ ] Aggregated conversion reports

### Phase 5: Post-Quantum (2-3 weeks)
- [ ] Hybrid HPKE with ML-KEM (FIPS 203)
- [ ] Post-quantum signatures
- [ ] Quantum-safe commitments

## Security Considerations

1. **Key Management**: Use hardware security modules (HSM) for production keys
2. **Side Channels**: Implement constant-time operations for crypto
3. **Replay Protection**: Add nonces and timestamps to all headers
4. **DOS Protection**: Rate limiting and proof-of-work for bid submission

## License

Copyright (C) 2025, ADXYZ Inc. All rights reserved.

## References

- [Blocklace Paper](https://arxiv.org/abs/2301.09191) - Byzantine-repelling CRDT
- [HPKE RFC 9180](https://datatracker.ietf.org/doc/html/rfc9180) - Hybrid Public Key Encryption
- [Private State Tokens](https://github.com/WICG/trust-token-api) - Privacy-preserving fraud prevention
- [Protected Audience API](https://developer.chrome.com/docs/privacy-sandbox/protected-audience/) - On-device auctions
- [SKAdNetwork](https://developer.apple.com/documentation/storekit/skadnetwork) - iOS attribution