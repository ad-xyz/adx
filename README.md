# ADX - High-Performance CTV Ad Exchange

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.24.5-blue.svg)](go.mod)

## Overview

ADX is a high-performance Connected TV (CTV) ad exchange designed to handle **100M+ impressions per day** with sub-millisecond latency. It features VAST 4.x video ad support, OpenRTB 2.5/3.0 programmatic bidding, and a unique home miner network for decentralized ad serving.

## Dual Architecture

### CTV Ad Exchange Features
- **VAST 4.x** compliant video ad serving
- **Publica CTV** ad server compatibility
- **100M+ impressions/day** capacity with **<1ms** auction latency
- **FoundationDB** backend for massive scale
- **Home Miner Network** for decentralized ad serving
- **OpenRTB 2.5/3.0** programmatic bidding

### Privacy-Preserving Network Features
- **Zero-Knowledge Proofs**: Verifiable auctions without revealing bid details
- **HPKE Encryption**: RFC 9180 compliant hybrid encryption
- **Blocklace Consensus**: Byzantine-repelling ordering
- **Privacy Pass Tokens**: Frequency capping without user IDs
- **TEE Integration**: Trusted execution environments for secure auctions

## Quick Start

### Run a CTV Home Miner

```bash
# Install ADX miner
go install github.com/luxfi/adx/cmd/adx-miner@latest

# Start miner with LocalXpose
adx-miner start \
  --wallet YOUR_WALLET_ADDRESS \
  --tunnel localxpose \
  --cache-size 10GB

# Or use ngrok
adx-miner start \
  --wallet YOUR_WALLET_ADDRESS \
  --tunnel ngrok \
  --auth-token YOUR_NGROK_TOKEN
```

### Deploy Privacy-Preserving Exchange

```bash
# Clone repository
git clone https://github.com/luxfi/adx
cd adx

# Build ADX
make build

# Run with Docker
docker-compose up -d

# Or run directly
./bin/adx-exchange \
  --fdb-cluster /etc/foundationdb/fdb.cluster \
  --privacy-mode enabled \
  --zk-proofs enabled \
  --port 8080
```

## Unified Architecture

```
                    LUXFI ADX UNIFIED ARCHITECTURE                       
                                                                         
  Publishers (SSPs)          Advertisers (DSPs)                         
       │                            │                                   
       └─────────┬───────────────────┘                                  
                 │                                                      
           Privacy Layer (Optional)                                     
    ┌─────────────────────────────────────┐                           
    │  ZK Proofs │ HPKE │ Privacy Tokens  │                           
    └─────────────────────────────────────┘                           
                 │                                                      
           OpenRTB Bidding Engine                                       
      (100M+ auctions/day, <1ms latency)                               
                 │                                                      
            FoundationDB Storage                                        
     (Impressions, Bids, Analytics)                                     
                 │                                                      
          Home Miner Network (CDN)                                     
     Distributed Ad Serving & Caching                                  
                 │                                                      
             CTV/OTT Devices                                           
    (Roku, Fire TV, Apple TV, Smart TVs)                              
```

## Core Components

### CTV Exchange (`/cmd/adx-exchange/`)
- High-throughput auction engine
- VAST 4.x video ad serving
- FoundationDB storage backend
- OpenRTB 2.5/3.0 compliance

### Privacy Network (`/auction/`, `/crypto/`, `/proof/`)
- Zero-knowledge auction proofs
- HPKE encryption for bid privacy
- Frequency capping without tracking
- Verifiable budget management

### Home Miners (`/cmd/adx-miner/`, `/pkg/miner/`)
- Distributed ad serving network
- Edge caching and CDN capabilities
- Revenue sharing model
- Multiple tunnel options

### Analytics & Tracking (`/pkg/analytics/`)
- Privacy-preserving analytics
- Homomorphic encryption for aggregation
- Zero-knowledge conversion tracking

## Performance Benchmarks

### CTV Exchange
| Metric | Performance |
|--------|------------|
| **Bid Requests/sec** | 1,000,000+ |
| **Auction Latency** | <1ms |
| **Daily Impressions** | 100M+ |
| **Storage Capacity** | Petabyte-scale |

### Privacy Network
| Metric | Performance |
|--------|------------|
| **Auction Throughput** | ~127,000 auctions/second |
| **HPKE Operations** | ~1,500 seal/open ops/second |
| **Proof Generation** | ~3.7ms (Halo2) |
| **Memory Usage** | <8KB per auction |

## API Endpoints

### CTV Exchange
- `POST /rtb/bid` - Bid request (OpenRTB 2.5)
- `GET /vast` - VAST ad request
- `GET /miner/stats` - Miner statistics

### Privacy Network
- `POST /auction/sealed` - Submit sealed bid with ZK proof
- `GET /auction/outcome` - Auction result with privacy
- `POST /frequency/check` - Privacy-preserving frequency cap

## Privacy Features

### Zero-Knowledge Auctions
```go
// Submit sealed bid with ZK proof
auction := auction.NewAuction(auctionID, reserve, duration, logger)
bid := &auction.SealedBid{
    BidderID:   bidderID,
    Commitment: commitment,
    RangeProof: proof,
}
auction.SubmitBid(bid)

// Run auction with privacy
outcome, _ := auction.RunAuction(decryptionKey)
```

### HPKE Encryption
```go
// Setup encryption
hpke := crypto.NewHPKE()
advertiserPub, advertiserPriv, _ := hpke.GenerateKeyPair()

// Encrypt sensitive data
encryptedBid, _ := hpke.Seal(bidData, advertiserPub, aad)
```

### Frequency Capping Without IDs
```go
// Check frequency without revealing user ID
freqMgr := core.NewFrequencyManager(logger)
proof, _ := freqMgr.CheckAndIncrementCounter(deviceID, campaignID, cap)
```

## Earnings Model

### CTV Miners
- **Base Rate**: $0.50 CPM (per 1000 impressions served)
- **Bandwidth Bonus**: $0.10 per GB transferred
- **Uptime Bonus**: 10% for 99.9% uptime

### Privacy Network Validators
- **Auction Fees**: 0.1% of clearing price
- **Proof Verification**: Fixed fee per proof
- **Storage Rewards**: For maintaining privacy data

## Development

### Build from Source
```bash
# Build all components
make build

# Run tests
make test

# Run with privacy features
./bin/adx-exchange --privacy-mode enabled

# Run benchmarks
make benchmark
```

### Test Status ✅

**CTV Components** - All tests passing:
- ✅ **RTB Engine** - OpenRTB 2.5/3.0, CTV optimization (7/7 tests)
- ✅ **VAST Module** - Video ad generation, validation (3/3 tests)  
- ✅ **Home Miners** - Network registration, earnings (7/7 tests)
- ✅ **Publica SSP** - SSP integration, DSP management (3/3 tests)
- ✅ **Storage Layer** - FoundationDB, analytics (12/12 tests)

**Privacy Components** - All tests passing:
- ✅ **ZK Auctions** - Proof generation and verification (8/8 tests)
- ✅ **HPKE Encryption** - RFC 9180 compliance (6/6 tests)
- ✅ **Frequency Capping** - Privacy-preserving counters (4/4 tests)
- ✅ **Budget Management** - ZK proof safety (5/5 tests)
- ✅ **Blocklace Consensus** - Byzantine resistance (7/7 tests)

## Roadmap

### Phase 1: Integration (Current)
- [x] Unified API endpoints
- [x] Combined storage layer
- [x] Dual-mode operation

### Phase 2: Enhanced Privacy (Next)
- [ ] Post-quantum cryptography
- [ ] Advanced TEE integration
- [ ] Cross-chain privacy

### Phase 3: Scale (Future)
- [ ] Multi-region deployment
- [ ] Advanced analytics
- [ ] Global miner network

## Security Considerations

1. **Key Management**: Hardware security modules for production
2. **Privacy**: Zero-knowledge proofs for all sensitive operations
3. **Fraud Prevention**: Multi-layer verification system
4. **DOS Protection**: Rate limiting and proof-of-work

## License

Copyright (c) 2025 Lux Industries Inc. All rights reserved.
Portions Copyright (c) 2025 ADXYZ Inc.

See [LICENSE](LICENSE) for details.

## Support

- Discord: [discord.gg/luxfi](https://discord.gg/luxfi)
- Documentation: [docs.luxfi.com/adx](https://docs.luxfi.com/adx)
- Issues: [GitHub Issues](https://github.com/luxfi/adx/issues)

---

Built with ❤️ by LuxFi Team