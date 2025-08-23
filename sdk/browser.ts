// Copyright (C) 2025, ADXYZ Inc. All rights reserved.
// See the file LICENSE for licensing terms.

/**
 * ADX Browser SDK for privacy-preserving ad operations
 */

// Types
interface ADXConfig {
  network: 'mainnet' | 'testnet' | 'local';
  endpoint: string;
  publicKey?: Uint8Array;
}

interface BidRequest {
  auctionId: string;
  bidValue: number;
  creativeId: string;
  targeting?: Record<string, string>;
}

interface ImpressionData {
  auctionOutcome: string;
  viewabilityMetrics: ViewabilityMetrics;
  timestamp: number;
}

interface ViewabilityMetrics {
  pixelPercent: number;
  timeInView: number;
}

interface PrivacyToken {
  token: Uint8Array;
  proof: Uint8Array;
  campaignId: string;
}

// Main SDK class
export class ADXSDK {
  private config: ADXConfig;
  private keypair?: CryptoKeyPair;
  private frequencyCounters: Map<string, number>;
  private privacyTokens: Map<string, PrivacyToken[]>;

  constructor(config: ADXConfig) {
    this.config = config;
    this.frequencyCounters = new Map();
    this.privacyTokens = new Map();
  }

  /**
   * Initialize the SDK and generate keypair
   */
  async initialize(): Promise<void> {
    // Generate ECDH keypair for HPKE
    this.keypair = await crypto.subtle.generateKey(
      {
        name: 'ECDH',
        namedCurve: 'P-256',
      },
      true,
      ['deriveKey', 'deriveBits']
    );

    // Initialize Protected Audience API if available
    if ('navigator' in globalThis && 'joinAdInterestGroup' in navigator) {
      await this.initializeProtectedAudience();
    }

    // Initialize Private State Tokens if available
    if ('document' in globalThis && 'hasPrivateToken' in document) {
      await this.initializePrivateStateTokens();
    }
  }

  /**
   * Submit a sealed bid to an auction
   */
  async submitBid(request: BidRequest): Promise<string> {
    if (!this.keypair) {
      throw new Error('SDK not initialized');
    }

    // Create bid commitment
    const commitment = await this.createBidCommitment(request);

    // Create range proof (simplified)
    const rangeProof = await this.createRangeProof(request.bidValue, 0, 10000);

    // Encrypt bid details
    const encryptedBid = await this.encryptBid(request);

    // Submit to network
    const response = await fetch(`${this.config.endpoint}/auction/bid`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        auctionId: request.auctionId,
        commitment: Array.from(commitment),
        encryptedBid: Array.from(encryptedBid),
        rangeProof: Array.from(rangeProof),
      }),
    });

    const result = await response.json();
    return result.bidId;
  }

  /**
   * Log an impression with viewability proof
   */
  async logImpression(data: ImpressionData): Promise<void> {
    // Check frequency cap
    const frequencyProof = await this.checkFrequencyCap(data.auctionOutcome);

    // Create viewability proof
    const viewProof = await this.createViewabilityProof(data.viewabilityMetrics);

    // Get Privacy State Token if available
    const pstProof = await this.getPrivacyStateToken();

    // Submit impression
    await fetch(`${this.config.endpoint}/impression`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        outcomeRef: data.auctionOutcome,
        viewProof: Array.from(viewProof),
        frequencyProof: frequencyProof ? Array.from(frequencyProof) : null,
        pstProof: pstProof ? Array.from(pstProof) : null,
        timestamp: data.timestamp,
      }),
    });
  }

  /**
   * Report a conversion event
   */
  async reportConversion(conversionValue: number, metadata?: Record<string, any>): Promise<void> {
    // Use Privacy Sandbox Attribution Reporting API if available
    if ('navigator' in globalThis && 'attribution' in navigator) {
      await this.reportAttributionConversion(conversionValue, metadata);
      return;
    }

    // Fallback to encrypted reporting
    const encryptedData = await this.encryptConversionData({
      value: conversionValue,
      metadata,
      timestamp: Date.now(),
    });

    await fetch(`${this.config.endpoint}/conversion`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        encryptedData: Array.from(encryptedData),
        timestamp: Date.now(),
      }),
    });
  }

  /**
   * Check frequency cap using local counter
   */
  private async checkFrequencyCap(campaignId: string): Promise<Uint8Array | null> {
    const key = `freq_${campaignId}`;
    const current = this.frequencyCounters.get(key) || 0;
    
    // Check against cap (simplified - in production, get cap from server)
    const cap = 3;
    if (current >= cap) {
      return null; // Cap exceeded
    }

    // Increment counter
    this.frequencyCounters.set(key, current + 1);

    // Create ZK proof of compliance
    return this.createFrequencyProof(current, current + 1, cap);
  }

  /**
   * Create a bid commitment
   */
  private async createBidCommitment(request: BidRequest): Promise<Uint8Array> {
    const encoder = new TextEncoder();
    const data = encoder.encode(JSON.stringify({
      bid: request.bidValue,
      creative: request.creativeId,
      targeting: request.targeting,
    }));

    const hash = await crypto.subtle.digest('SHA-256', data);
    return new Uint8Array(hash);
  }

  /**
   * Encrypt bid using HPKE (simplified)
   */
  private async encryptBid(request: BidRequest): Promise<Uint8Array> {
    if (!this.keypair) {
      throw new Error('Keypair not initialized');
    }

    const encoder = new TextEncoder();
    const plaintext = encoder.encode(JSON.stringify(request));

    // In production, use proper HPKE with server's public key
    // For now, use Web Crypto API encryption
    const key = await crypto.subtle.generateKey(
      {
        name: 'AES-GCM',
        length: 256,
      },
      true,
      ['encrypt']
    );

    const iv = crypto.getRandomValues(new Uint8Array(12));
    const ciphertext = await crypto.subtle.encrypt(
      {
        name: 'AES-GCM',
        iv: iv,
      },
      key,
      plaintext
    );

    // Combine IV and ciphertext
    const result = new Uint8Array(iv.length + ciphertext.byteLength);
    result.set(iv);
    result.set(new Uint8Array(ciphertext), iv.length);

    return result;
  }

  /**
   * Create a range proof (simplified)
   */
  private async createRangeProof(value: number, min: number, max: number): Promise<Uint8Array> {
    // In production, use actual ZK range proof
    // For now, create a commitment
    const encoder = new TextEncoder();
    const data = encoder.encode(`range_${value}_${min}_${max}`);
    const hash = await crypto.subtle.digest('SHA-256', data);
    return new Uint8Array(hash);
  }

  /**
   * Create viewability proof
   */
  private async createViewabilityProof(metrics: ViewabilityMetrics): Promise<Uint8Array> {
    // Check MRC standards (50% pixels, 1 second)
    const meetsStandard = metrics.pixelPercent >= 50 && metrics.timeInView >= 1000;

    const encoder = new TextEncoder();
    const data = encoder.encode(JSON.stringify({
      meets: meetsStandard,
      pixels: metrics.pixelPercent,
      time: metrics.timeInView,
    }));

    const hash = await crypto.subtle.digest('SHA-256', data);
    return new Uint8Array(hash);
  }

  /**
   * Create frequency proof
   */
  private async createFrequencyProof(before: number, after: number, cap: number): Promise<Uint8Array> {
    const encoder = new TextEncoder();
    const data = encoder.encode(`freq_${before}_${after}_${cap}`);
    const hash = await crypto.subtle.digest('SHA-256', data);
    return new Uint8Array(hash);
  }

  /**
   * Initialize Protected Audience API (Chrome)
   */
  private async initializeProtectedAudience(): Promise<void> {
    // Join interest groups
    try {
      await (navigator as any).joinAdInterestGroup({
        owner: 'https://adx.example',
        name: 'automotive-enthusiasts',
        biddingLogicUrl: `${this.config.endpoint}/bidding.js`,
        dailyUpdateUrl: `${this.config.endpoint}/update`,
        trustedBiddingSignalsUrl: `${this.config.endpoint}/signals`,
        userBiddingSignals: { segments: ['auto', 'luxury'] },
        ads: [],
      }, 30 * 24 * 60 * 60); // 30 days
    } catch (e) {
      console.log('Protected Audience not available');
    }
  }

  /**
   * Initialize Private State Tokens (Privacy Pass)
   */
  private async initializePrivateStateTokens(): Promise<void> {
    try {
      // Check if tokens are available
      const hasToken = await (document as any).hasPrivateToken('https://adx.example');
      
      if (!hasToken) {
        // Request token issuance
        await (document as any).requestPrivateToken('https://adx.example');
      }
    } catch (e) {
      console.log('Private State Tokens not available');
    }
  }

  /**
   * Get Privacy State Token for fraud prevention
   */
  private async getPrivacyStateToken(): Promise<Uint8Array | null> {
    try {
      const token = await (document as any).redeemPrivateToken('https://adx.example');
      return new Uint8Array(token);
    } catch (e) {
      return null;
    }
  }

  /**
   * Report conversion using Attribution Reporting API
   */
  private async reportAttributionConversion(value: number, metadata?: Record<string, any>): Promise<void> {
    try {
      await (navigator as any).attribution.registerConversion({
        conversionValue: value,
        metadata: metadata,
      });
    } catch (e) {
      console.log('Attribution Reporting API not available');
    }
  }

  /**
   * Encrypt conversion data
   */
  private async encryptConversionData(data: any): Promise<Uint8Array> {
    const encoder = new TextEncoder();
    const plaintext = encoder.encode(JSON.stringify(data));

    // Simplified encryption
    const key = await crypto.subtle.generateKey(
      {
        name: 'AES-GCM',
        length: 256,
      },
      true,
      ['encrypt']
    );

    const iv = crypto.getRandomValues(new Uint8Array(12));
    const ciphertext = await crypto.subtle.encrypt(
      {
        name: 'AES-GCM',
        iv: iv,
      },
      key,
      plaintext
    );

    const result = new Uint8Array(iv.length + ciphertext.byteLength);
    result.set(iv);
    result.set(new Uint8Array(ciphertext), iv.length);

    return result;
  }

  /**
   * Measure viewability metrics
   */
  static measureViewability(element: HTMLElement): ViewabilityMetrics {
    const rect = element.getBoundingClientRect();
    const windowHeight = window.innerHeight;
    const windowWidth = window.innerWidth;

    // Calculate visible pixels
    const visibleHeight = Math.min(rect.bottom, windowHeight) - Math.max(rect.top, 0);
    const visibleWidth = Math.min(rect.right, windowWidth) - Math.max(rect.left, 0);
    const visibleArea = Math.max(0, visibleHeight) * Math.max(0, visibleWidth);
    const totalArea = rect.height * rect.width;

    const pixelPercent = totalArea > 0 ? (visibleArea / totalArea) * 100 : 0;

    // In production, track time in view
    const timeInView = 0; // Placeholder

    return {
      pixelPercent: Math.round(pixelPercent),
      timeInView,
    };
  }
}

// Export for use in browsers
if (typeof window !== 'undefined') {
  (window as any).ADXSDK = ADXSDK;
}