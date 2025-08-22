# ADX Python SDK

Python SDK for Ad Exchange.

## Installation

```bash
pip install adx
```

## Quick Start

```python
from adx import ADXClient
from datetime import datetime, timedelta

# Initialize client
client = ADXClient(
    base_url="https://api.ad.xyz",
    api_key="your-api-key"
)

# Send bid request
bid_request = {
    "id": "bid-123",
    "imp": [{
        "id": "imp-1",
        "video": {
            "mimes": ["video/mp4"],
            "minduration": 5,
            "maxduration": 30,
            "w": 1920,
            "h": 1080
        },
        "bidfloor": 2.50
    }],
    "device": {
        "devicetype": 3,  # Connected TV
        "ua": "Mozilla/5.0 (Web0S; Linux/SmartTV)"
    }
}

response = client.bid_request(bid_request)
print(f"Bid response: {response}")

# Get analytics
analytics = client.get_analytics(
    publisher_id="pub-123",
    start_time=datetime.now() - timedelta(days=7),
    end_time=datetime.now()
)
print(f"Weekly revenue: ${analytics['total_revenue']}")

# Register miner
miner_config = {
    "wallet_address": "0x1234567890abcdef",
    "public_url": "https://myminer.example.com",
    "cache_size": "50GB",
    "location": {
        "country": "US",
        "region": "CA",
        "city": "San Francisco",
        "lat": 37.7749,
        "lon": -122.4194
    },
    "hardware": {
        "cpu_cores": 8,
        "memory_gb": 32,
        "disk_gb": 500,
        "network_mbps": 1000
    }
}

registration = client.register_miner(miner_config)
print(f"Miner ID: {registration['miner_id']}")

# Connect WebSocket for real-time updates
async def handle_impression(data):
    print(f"New impression: {data}")

client.on("impression", handle_impression)
await client.connect_websocket()
await client.subscribe(["impression", "fill", "bid"])
```

## Features

- OpenRTB 2.5/3.0 bid requests
- VAST 4.x ad serving
- Real-time WebSocket updates
- Analytics and reporting
- Home miner management
- CTV ad pod assembly
- Viewability tracking

## Documentation

Full documentation available at [https://docs.ad.xyz](https://docs.ad.xyz)