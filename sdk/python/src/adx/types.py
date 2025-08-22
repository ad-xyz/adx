"""
Type definitions for ADX SDK
"""

from datetime import datetime
from typing import Dict, List, Optional, Any
from decimal import Decimal
from pydantic import BaseModel, Field


# OpenRTB Types

class Banner(BaseModel):
    w: Optional[int] = None
    h: Optional[int] = None
    wmin: Optional[int] = None
    hmin: Optional[int] = None
    wmax: Optional[int] = None
    hmax: Optional[int] = None
    pos: Optional[int] = None
    format: Optional[List[Dict[str, int]]] = None
    ext: Optional[Dict[str, Any]] = None


class Video(BaseModel):
    mimes: List[str]
    minduration: Optional[int] = None
    maxduration: Optional[int] = None
    protocols: Optional[List[int]] = None
    w: Optional[int] = None
    h: Optional[int] = None
    startdelay: Optional[int] = None
    placement: Optional[int] = None
    linearity: Optional[int] = None
    skip: Optional[int] = None
    skipmin: Optional[int] = None
    skipafter: Optional[int] = None
    sequence: Optional[int] = None
    api: Optional[List[int]] = None
    ext: Optional[Dict[str, Any]] = None


class Native(BaseModel):
    request: str
    ver: Optional[str] = None
    api: Optional[List[int]] = None
    ext: Optional[Dict[str, Any]] = None


class Impression(BaseModel):
    id: str
    banner: Optional[Banner] = None
    video: Optional[Video] = None
    native: Optional[Native] = None
    bidfloor: Optional[float] = None
    bidfloorcur: Optional[str] = None
    secure: Optional[int] = None
    ext: Optional[Dict[str, Any]] = None


class Publisher(BaseModel):
    id: Optional[str] = None
    name: Optional[str] = None
    cat: Optional[List[str]] = None
    domain: Optional[str] = None
    ext: Optional[Dict[str, Any]] = None


class Content(BaseModel):
    id: Optional[str] = None
    episode: Optional[int] = None
    title: Optional[str] = None
    series: Optional[str] = None
    season: Optional[str] = None
    artist: Optional[str] = None
    genre: Optional[str] = None
    album: Optional[str] = None
    isrc: Optional[str] = None
    url: Optional[str] = None
    cat: Optional[List[str]] = None
    prodq: Optional[int] = None
    context: Optional[int] = None
    contentrating: Optional[str] = None
    userrating: Optional[str] = None
    qagmediarating: Optional[int] = None
    keywords: Optional[str] = None
    livestream: Optional[int] = None
    sourcerelationship: Optional[int] = None
    len: Optional[int] = None
    language: Optional[str] = None
    embeddable: Optional[int] = None
    ext: Optional[Dict[str, Any]] = None


class Site(BaseModel):
    id: Optional[str] = None
    name: Optional[str] = None
    domain: Optional[str] = None
    cat: Optional[List[str]] = None
    page: Optional[str] = None
    publisher: Optional[Publisher] = None
    content: Optional[Content] = None
    ext: Optional[Dict[str, Any]] = None


class App(BaseModel):
    id: Optional[str] = None
    name: Optional[str] = None
    bundle: Optional[str] = None
    domain: Optional[str] = None
    storeurl: Optional[str] = None
    cat: Optional[List[str]] = None
    publisher: Optional[Publisher] = None
    content: Optional[Content] = None
    ext: Optional[Dict[str, Any]] = None


class Geo(BaseModel):
    lat: Optional[float] = None
    lon: Optional[float] = None
    type: Optional[int] = None
    accuracy: Optional[int] = None
    lastfix: Optional[int] = None
    ipservice: Optional[int] = None
    country: Optional[str] = None
    region: Optional[str] = None
    regionfips104: Optional[str] = None
    metro: Optional[str] = None
    city: Optional[str] = None
    zip: Optional[str] = None
    utcoffset: Optional[int] = None
    ext: Optional[Dict[str, Any]] = None


class Device(BaseModel):
    ua: Optional[str] = None
    geo: Optional[Geo] = None
    dnt: Optional[int] = None
    lmt: Optional[int] = None
    ip: Optional[str] = None
    ipv6: Optional[str] = None
    devicetype: Optional[int] = None
    make: Optional[str] = None
    model: Optional[str] = None
    os: Optional[str] = None
    osv: Optional[str] = None
    hwv: Optional[str] = None
    h: Optional[int] = None
    w: Optional[int] = None
    ppi: Optional[int] = None
    pxratio: Optional[float] = None
    js: Optional[int] = None
    flashver: Optional[str] = None
    language: Optional[str] = None
    carrier: Optional[str] = None
    connectiontype: Optional[int] = None
    ifa: Optional[str] = None
    didsha1: Optional[str] = None
    didmd5: Optional[str] = None
    dpidsha1: Optional[str] = None
    dpidmd5: Optional[str] = None
    macsha1: Optional[str] = None
    macmd5: Optional[str] = None
    ext: Optional[Dict[str, Any]] = None


class Segment(BaseModel):
    id: Optional[str] = None
    name: Optional[str] = None
    value: Optional[str] = None
    ext: Optional[Dict[str, Any]] = None


class Data(BaseModel):
    id: Optional[str] = None
    name: Optional[str] = None
    segment: Optional[List[Segment]] = None
    ext: Optional[Dict[str, Any]] = None


class User(BaseModel):
    id: Optional[str] = None
    buyeruid: Optional[str] = None
    yob: Optional[int] = None
    gender: Optional[str] = None
    keywords: Optional[str] = None
    data: Optional[List[Data]] = None
    ext: Optional[Dict[str, Any]] = None


class Regulations(BaseModel):
    coppa: Optional[int] = None
    gdpr: Optional[int] = None
    us_privacy: Optional[str] = None
    ext: Optional[Dict[str, Any]] = None


class BidRequest(BaseModel):
    id: str
    imp: List[Impression]
    site: Optional[Site] = None
    app: Optional[App] = None
    device: Optional[Device] = None
    user: Optional[User] = None
    at: Optional[int] = None
    tmax: Optional[int] = None
    cur: Optional[List[str]] = None
    bcat: Optional[List[str]] = None
    badv: Optional[List[str]] = None
    regs: Optional[Regulations] = None
    ext: Optional[Dict[str, Any]] = None


class Bid(BaseModel):
    id: str
    impid: str
    price: float
    nurl: Optional[str] = None
    burl: Optional[str] = None
    lurl: Optional[str] = None
    adm: Optional[str] = None
    adid: Optional[str] = None
    adomain: Optional[List[str]] = None
    bundle: Optional[str] = None
    iurl: Optional[str] = None
    cid: Optional[str] = None
    crid: Optional[str] = None
    cat: Optional[List[str]] = None
    attr: Optional[List[int]] = None
    api: Optional[int] = None
    protocol: Optional[int] = None
    qagmediarating: Optional[int] = None
    language: Optional[str] = None
    dealid: Optional[str] = None
    w: Optional[int] = None
    h: Optional[int] = None
    wratio: Optional[int] = None
    hratio: Optional[int] = None
    exp: Optional[int] = None
    ext: Optional[Dict[str, Any]] = None


class SeatBid(BaseModel):
    bid: List[Bid]
    seat: Optional[str] = None
    group: Optional[int] = None
    ext: Optional[Dict[str, Any]] = None


class BidResponse(BaseModel):
    id: str
    seatbid: Optional[List[SeatBid]] = None
    bidid: Optional[str] = None
    cur: Optional[str] = None
    customdata: Optional[str] = None
    nbr: Optional[int] = None
    ext: Optional[Dict[str, Any]] = None


# SDK Specific Types

class VASTParams(BaseModel):
    width: int
    height: int
    duration: int
    extra: Optional[Dict[str, Any]] = None


class DailyStat(BaseModel):
    date: str
    impressions: int
    revenue: str
    fill_rate: float = Field(alias="fillRate")


class AnalyticsResponse(BaseModel):
    publisher_id: str = Field(alias="publisherId")
    total_impressions: int = Field(alias="totalImpressions")
    total_revenue: str = Field(alias="totalRevenue")
    fill_rate: float = Field(alias="fillRate")
    ecpm: str
    time_range: Dict[str, str] = Field(alias="timeRange")
    daily_stats: List[DailyStat] = Field(alias="dailyStats")


class Location(BaseModel):
    country: str
    region: str
    city: str
    lat: float
    lon: float


class Hardware(BaseModel):
    cpu_cores: int = Field(alias="cpuCores")
    memory_gb: int = Field(alias="memoryGb")
    disk_gb: int = Field(alias="diskGb")
    network_mbps: int = Field(alias="networkMbps")


class MinerConfig(BaseModel):
    wallet_address: str = Field(alias="walletAddress")
    public_url: str = Field(alias="publicUrl")
    cache_size: str = Field(alias="cacheSize")
    location: Location
    hardware: Hardware


class MinerRegistration(BaseModel):
    miner_id: str = Field(alias="minerId")
    status: str
    registered_at: str = Field(alias="registeredAt")
    websocket_url: str = Field(alias="websocketUrl")


class MinerEarnings(BaseModel):
    miner_id: str = Field(alias="minerId")
    total_earnings: str = Field(alias="totalEarnings")
    pending_payout: str = Field(alias="pendingPayout")
    last_payout: str = Field(alias="lastPayout")
    total_impressions: int = Field(alias="totalImpressions")
    total_bandwidth: int = Field(alias="totalBandwidth")
    period: str