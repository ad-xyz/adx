import axios, { AxiosInstance } from 'axios';
import WebSocket from 'ws';
import Decimal from 'decimal.js';

// Export all types
export * from './types';

// Import types
import {
  BidRequest,
  BidResponse,
  VASTParams,
  AnalyticsParams,
  AnalyticsResponse,
  MinerConfig,
  MinerRegistration,
  MinerEarnings,
  WebSocketMessage,
  EventSubscription
} from './types';

/**
 * ADX Client
 */
export class ADXClient {
  private baseURL: string;
  private apiKey: string;
  private axios: AxiosInstance;
  private ws?: WebSocket;
  private eventHandlers: Map<string, Set<(data: any) => void>>;

  constructor(baseURL: string, apiKey: string) {
    this.baseURL = baseURL;
    this.apiKey = apiKey;
    this.eventHandlers = new Map();
    
    this.axios = axios.create({
      baseURL: this.baseURL,
      timeout: 10000,
      headers: {
        'X-API-Key': this.apiKey,
        'Content-Type': 'application/json'
      }
    });
  }

  /**
   * Send an OpenRTB bid request
   */
  async bidRequest(request: BidRequest): Promise<BidResponse> {
    const response = await this.axios.post<BidResponse>('/rtb/bid', request);
    return response.data;
  }

  /**
   * Get VAST ad creative
   */
  async getVAST(params: VASTParams): Promise<string> {
    const response = await this.axios.get<string>('/vast', {
      params: {
        w: params.width,
        h: params.height,
        dur: params.duration,
        ...params.extra
      },
      responseType: 'text'
    });
    return response.data;
  }

  /**
   * Connect to WebSocket for real-time updates
   */
  async connectWebSocket(): Promise<void> {
    return new Promise((resolve, reject) => {
      const wsURL = this.baseURL.replace(/^http/, 'ws') + '/ws';
      
      this.ws = new WebSocket(wsURL, {
        headers: {
          'X-API-Key': this.apiKey
        }
      });

      this.ws.on('open', () => {
        console.log('WebSocket connected');
        resolve();
      });

      this.ws.on('message', (data: Buffer) => {
        try {
          const message: WebSocketMessage = JSON.parse(data.toString());
          this.handleWebSocketMessage(message);
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error);
        }
      });

      this.ws.on('error', (error) => {
        console.error('WebSocket error:', error);
        reject(error);
      });

      this.ws.on('close', (code, reason) => {
        console.log(`WebSocket closed: ${code} - ${reason}`);
        this.reconnectWebSocket();
      });
    });
  }

  /**
   * Subscribe to WebSocket events
   */
  async subscribe(events: string[]): Promise<void> {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket not connected');
    }

    const subscription: EventSubscription = {
      type: 'subscribe',
      events
    };

    this.ws.send(JSON.stringify(subscription));
  }

  /**
   * Register event handler
   */
  on(event: string, handler: (data: any) => void): void {
    if (!this.eventHandlers.has(event)) {
      this.eventHandlers.set(event, new Set());
    }
    this.eventHandlers.get(event)!.add(handler);
  }

  /**
   * Remove event handler
   */
  off(event: string, handler: (data: any) => void): void {
    const handlers = this.eventHandlers.get(event);
    if (handlers) {
      handlers.delete(handler);
    }
  }

  /**
   * Get analytics data
   */
  async getAnalytics(params: AnalyticsParams): Promise<AnalyticsResponse> {
    const response = await this.axios.get<AnalyticsResponse>('/analytics', {
      params: {
        publisher_id: params.publisherId,
        start: params.startTime.toISOString(),
        end: params.endTime.toISOString()
      }
    });
    return response.data;
  }

  /**
   * Register a home miner
   */
  async registerMiner(config: MinerConfig): Promise<MinerRegistration> {
    const response = await this.axios.post<MinerRegistration>('/miner/register', config);
    return response.data;
  }

  /**
   * Get miner earnings
   */
  async getMinerEarnings(minerId: string): Promise<MinerEarnings> {
    const response = await this.axios.get<MinerEarnings>(`/miner/${minerId}/earnings`);
    return response.data;
  }

  /**
   * Update miner status
   */
  async updateMinerStatus(minerId: string, status: 'online' | 'offline' | 'maintenance'): Promise<void> {
    await this.axios.put(`/miner/${minerId}/status`, { status });
  }

  /**
   * Get ad pod for CTV
   */
  async getAdPod(slotId: string, duration: number, context?: any): Promise<any> {
    const response = await this.axios.post('/ctv/pod', {
      slot_id: slotId,
      duration,
      context
    });
    return response.data;
  }

  /**
   * Report impression
   */
  async reportImpression(impressionId: string, data: any): Promise<void> {
    await this.axios.post(`/impression/${impressionId}`, data);
  }

  /**
   * Report viewability
   */
  async reportViewability(impressionId: string, viewability: number, quartiles: number[]): Promise<void> {
    await this.axios.post(`/viewability/${impressionId}`, {
      viewability,
      quartiles
    });
  }

  /**
   * Close client connections
   */
  async close(): Promise<void> {
    if (this.ws) {
      this.ws.close();
      this.ws = undefined;
    }
  }

  // Private methods

  private handleWebSocketMessage(message: WebSocketMessage): void {
    const handlers = this.eventHandlers.get(message.type);
    if (handlers) {
      handlers.forEach(handler => {
        try {
          handler(message.data);
        } catch (error) {
          console.error(`Error in event handler for ${message.type}:`, error);
        }
      });
    }
  }

  private async reconnectWebSocket(): Promise<void> {
    console.log('Attempting to reconnect WebSocket...');
    setTimeout(() => {
      this.connectWebSocket().catch(error => {
        console.error('Failed to reconnect:', error);
        this.reconnectWebSocket();
      });
    }, 5000);
  }
}

// Helper functions

/**
 * Create OpenRTB bid request
 */
export function createBidRequest(params: {
  id: string;
  imp: any[];
  site?: any;
  app?: any;
  device?: any;
  user?: any;
}): BidRequest {
  return {
    id: params.id,
    imp: params.imp,
    site: params.site,
    app: params.app,
    device: params.device,
    user: params.user,
    at: 1, // First price auction
    cur: ['USD'],
    tmax: 100
  };
}

/**
 * Parse VAST response
 */
export function parseVAST(vastXML: string): any {
  // Simple VAST parser - in production use a proper XML parser
  const adMatch = vastXML.match(/<Ad\s+id="([^"]+)"/);
  const mediaMatch = vastXML.match(/<MediaFile[^>]*>([^<]+)<\/MediaFile>/);
  const durationMatch = vastXML.match(/<Duration>([^<]+)<\/Duration>/);
  
  return {
    adId: adMatch ? adMatch[1] : null,
    mediaUrl: mediaMatch ? mediaMatch[1].trim() : null,
    duration: durationMatch ? durationMatch[1] : null
  };
}

export default ADXClient;