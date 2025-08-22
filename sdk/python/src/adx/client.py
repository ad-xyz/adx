"""
ADX Client implementation for Python SDK
"""

import json
import logging
import threading
from datetime import datetime
from typing import Dict, List, Optional, Any, Callable
from urllib.parse import urljoin, urlparse

import requests
import websocket
from pydantic import BaseModel

from .types import (
    BidRequest,
    BidResponse,
    AnalyticsResponse,
    MinerConfig,
    MinerRegistration,
    MinerEarnings,
    VASTParams
)
from .exceptions import (
    ADXException,
    AuthenticationError,
    RateLimitError,
    ValidationError,
    NetworkError
)

logger = logging.getLogger(__name__)


class ADXClient:
    """
    Client for interacting with the Luxfi ADX API
    """
    
    def __init__(self, base_url: str, api_key: str, timeout: int = 10):
        """
        Initialize ADX client
        
        Args:
            base_url: Base URL of the ADX API
            api_key: API key for authentication
            timeout: Request timeout in seconds
        """
        self.base_url = base_url.rstrip('/')
        self.api_key = api_key
        self.timeout = timeout
        self.session = requests.Session()
        self.session.headers.update({
            'X-API-Key': api_key,
            'Content-Type': 'application/json',
            'User-Agent': 'luxfi-adx-python/1.0.0'
        })
        
        self.ws = None
        self.ws_thread = None
        self.event_handlers = {}
        self._ws_running = False
    
    def bid_request(self, request: Dict[str, Any]) -> Dict[str, Any]:
        """
        Send an OpenRTB bid request
        
        Args:
            request: OpenRTB bid request dictionary
            
        Returns:
            OpenRTB bid response dictionary
        """
        # Validate request
        bid_req = BidRequest.model_validate(request)
        
        url = urljoin(self.base_url, '/rtb/bid')
        
        try:
            response = self.session.post(
                url,
                json=bid_req.model_dump(exclude_none=True),
                timeout=self.timeout
            )
            response.raise_for_status()
            
            bid_resp = BidResponse.model_validate(response.json())
            return bid_resp.model_dump()
            
        except requests.exceptions.Timeout:
            raise NetworkError("Bid request timed out")
        except requests.exceptions.HTTPError as e:
            self._handle_http_error(e)
        except Exception as e:
            raise NetworkError(f"Bid request failed: {str(e)}")
    
    def get_vast(self, params: VASTParams) -> str:
        """
        Get VAST ad creative
        
        Args:
            params: VAST request parameters
            
        Returns:
            VAST XML string
        """
        url = urljoin(self.base_url, '/vast')
        
        query_params = {
            'w': params.width,
            'h': params.height,
            'dur': params.duration
        }
        
        if params.extra:
            query_params.update(params.extra)
        
        try:
            response = self.session.get(
                url,
                params=query_params,
                timeout=self.timeout
            )
            response.raise_for_status()
            return response.text
            
        except requests.exceptions.HTTPError as e:
            self._handle_http_error(e)
        except Exception as e:
            raise NetworkError(f"VAST request failed: {str(e)}")
    
    def get_analytics(
        self,
        publisher_id: str,
        start_time: datetime,
        end_time: datetime
    ) -> Dict[str, Any]:
        """
        Get analytics data
        
        Args:
            publisher_id: Publisher ID
            start_time: Start of time range
            end_time: End of time range
            
        Returns:
            Analytics response dictionary
        """
        url = urljoin(self.base_url, '/analytics')
        
        params = {
            'publisher_id': publisher_id,
            'start': start_time.isoformat(),
            'end': end_time.isoformat()
        }
        
        try:
            response = self.session.get(
                url,
                params=params,
                timeout=self.timeout
            )
            response.raise_for_status()
            
            analytics = AnalyticsResponse.model_validate(response.json())
            return analytics.model_dump()
            
        except requests.exceptions.HTTPError as e:
            self._handle_http_error(e)
        except Exception as e:
            raise NetworkError(f"Analytics request failed: {str(e)}")
    
    def register_miner(self, config: Dict[str, Any]) -> Dict[str, Any]:
        """
        Register a home miner
        
        Args:
            config: Miner configuration dictionary
            
        Returns:
            Miner registration response
        """
        miner_config = MinerConfig.model_validate(config)
        
        url = urljoin(self.base_url, '/miner/register')
        
        try:
            response = self.session.post(
                url,
                json=miner_config.model_dump(),
                timeout=self.timeout
            )
            response.raise_for_status()
            
            registration = MinerRegistration.model_validate(response.json())
            return registration.model_dump()
            
        except requests.exceptions.HTTPError as e:
            self._handle_http_error(e)
        except Exception as e:
            raise NetworkError(f"Miner registration failed: {str(e)}")
    
    def get_miner_earnings(self, miner_id: str) -> Dict[str, Any]:
        """
        Get miner earnings
        
        Args:
            miner_id: Miner ID
            
        Returns:
            Miner earnings data
        """
        url = urljoin(self.base_url, f'/miner/{miner_id}/earnings')
        
        try:
            response = self.session.get(url, timeout=self.timeout)
            response.raise_for_status()
            
            earnings = MinerEarnings.model_validate(response.json())
            return earnings.model_dump()
            
        except requests.exceptions.HTTPError as e:
            self._handle_http_error(e)
        except Exception as e:
            raise NetworkError(f"Failed to get miner earnings: {str(e)}")
    
    def update_miner_status(
        self,
        miner_id: str,
        status: str
    ) -> None:
        """
        Update miner status
        
        Args:
            miner_id: Miner ID
            status: New status (online, offline, maintenance)
        """
        if status not in ['online', 'offline', 'maintenance']:
            raise ValidationError(f"Invalid status: {status}")
        
        url = urljoin(self.base_url, f'/miner/{miner_id}/status')
        
        try:
            response = self.session.put(
                url,
                json={'status': status},
                timeout=self.timeout
            )
            response.raise_for_status()
            
        except requests.exceptions.HTTPError as e:
            self._handle_http_error(e)
        except Exception as e:
            raise NetworkError(f"Failed to update miner status: {str(e)}")
    
    def get_ad_pod(
        self,
        slot_id: str,
        duration: int,
        context: Optional[Dict[str, Any]] = None
    ) -> Dict[str, Any]:
        """
        Get ad pod for CTV
        
        Args:
            slot_id: Slot ID
            duration: Pod duration in seconds
            context: Additional context
            
        Returns:
            Ad pod data
        """
        url = urljoin(self.base_url, '/ctv/pod')
        
        payload = {
            'slot_id': slot_id,
            'duration': duration,
            'context': context or {}
        }
        
        try:
            response = self.session.post(
                url,
                json=payload,
                timeout=self.timeout
            )
            response.raise_for_status()
            return response.json()
            
        except requests.exceptions.HTTPError as e:
            self._handle_http_error(e)
        except Exception as e:
            raise NetworkError(f"Failed to get ad pod: {str(e)}")
    
    def report_impression(
        self,
        impression_id: str,
        data: Dict[str, Any]
    ) -> None:
        """
        Report impression
        
        Args:
            impression_id: Impression ID
            data: Impression data
        """
        url = urljoin(self.base_url, f'/impression/{impression_id}')
        
        try:
            response = self.session.post(
                url,
                json=data,
                timeout=self.timeout
            )
            response.raise_for_status()
            
        except requests.exceptions.HTTPError as e:
            self._handle_http_error(e)
        except Exception as e:
            raise NetworkError(f"Failed to report impression: {str(e)}")
    
    def report_viewability(
        self,
        impression_id: str,
        viewability: float,
        quartiles: List[int]
    ) -> None:
        """
        Report viewability metrics
        
        Args:
            impression_id: Impression ID
            viewability: Viewability percentage (0-100)
            quartiles: Quartile completion markers
        """
        url = urljoin(self.base_url, f'/viewability/{impression_id}')
        
        payload = {
            'viewability': viewability,
            'quartiles': quartiles
        }
        
        try:
            response = self.session.post(
                url,
                json=payload,
                timeout=self.timeout
            )
            response.raise_for_status()
            
        except requests.exceptions.HTTPError as e:
            self._handle_http_error(e)
        except Exception as e:
            raise NetworkError(f"Failed to report viewability: {str(e)}")
    
    def connect_websocket(self) -> None:
        """
        Connect to WebSocket for real-time updates
        """
        ws_url = self.base_url.replace('http', 'ws') + '/ws'
        
        def on_message(ws, message):
            try:
                data = json.loads(message)
                event_type = data.get('type')
                
                if event_type in self.event_handlers:
                    for handler in self.event_handlers[event_type]:
                        try:
                            handler(data.get('data'))
                        except Exception as e:
                            logger.error(f"Error in event handler: {e}")
            except json.JSONDecodeError:
                logger.error(f"Failed to parse WebSocket message: {message}")
        
        def on_error(ws, error):
            logger.error(f"WebSocket error: {error}")
        
        def on_close(ws, close_status_code, close_msg):
            logger.info(f"WebSocket closed: {close_status_code} - {close_msg}")
            if self._ws_running:
                self._reconnect_websocket()
        
        def on_open(ws):
            logger.info("WebSocket connected")
        
        self.ws = websocket.WebSocketApp(
            ws_url,
            header={'X-API-Key': self.api_key},
            on_message=on_message,
            on_error=on_error,
            on_close=on_close,
            on_open=on_open
        )
        
        self._ws_running = True
        self.ws_thread = threading.Thread(
            target=self.ws.run_forever,
            daemon=True
        )
        self.ws_thread.start()
    
    def subscribe(self, events: List[str]) -> None:
        """
        Subscribe to WebSocket events
        
        Args:
            events: List of event types to subscribe to
        """
        if not self.ws:
            raise ADXException("WebSocket not connected")
        
        message = {
            'type': 'subscribe',
            'events': events
        }
        
        self.ws.send(json.dumps(message))
    
    def on(self, event: str, handler: Callable[[Any], None]) -> None:
        """
        Register event handler
        
        Args:
            event: Event type
            handler: Event handler function
        """
        if event not in self.event_handlers:
            self.event_handlers[event] = []
        self.event_handlers[event].append(handler)
    
    def off(self, event: str, handler: Callable[[Any], None]) -> None:
        """
        Remove event handler
        
        Args:
            event: Event type
            handler: Event handler function
        """
        if event in self.event_handlers:
            try:
                self.event_handlers[event].remove(handler)
            except ValueError:
                pass
    
    def close(self) -> None:
        """
        Close client connections
        """
        self._ws_running = False
        
        if self.ws:
            self.ws.close()
            self.ws = None
        
        if self.ws_thread:
            self.ws_thread.join(timeout=5)
            self.ws_thread = None
        
        self.session.close()
    
    def _handle_http_error(self, error: requests.exceptions.HTTPError) -> None:
        """
        Handle HTTP errors
        
        Args:
            error: HTTP error
        """
        status_code = error.response.status_code
        
        if status_code == 401:
            raise AuthenticationError("Invalid API key")
        elif status_code == 429:
            raise RateLimitError("Rate limit exceeded")
        elif status_code == 400:
            raise ValidationError(f"Invalid request: {error.response.text}")
        else:
            raise ADXException(f"HTTP {status_code}: {error.response.text}")
    
    def _reconnect_websocket(self) -> None:
        """
        Reconnect WebSocket after disconnection
        """
        logger.info("Attempting to reconnect WebSocket...")
        
        import time
        time.sleep(5)
        
        if self._ws_running:
            self.connect_websocket()
    
    def __enter__(self):
        """Context manager entry"""
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit"""
        self.close()