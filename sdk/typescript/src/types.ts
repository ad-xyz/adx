import Decimal from 'decimal.js';

// OpenRTB types
export interface BidRequest {
  id: string;
  imp: Impression[];
  site?: Site;
  app?: App;
  device?: Device;
  user?: User;
  at?: number;
  tmax?: number;
  cur?: string[];
  bcat?: string[];
  badv?: string[];
  regs?: Regulations;
  ext?: any;
}

export interface BidResponse {
  id: string;
  seatbid?: SeatBid[];
  bidid?: string;
  cur?: string;
  customdata?: string;
  nbr?: number;
  ext?: any;
}

export interface Impression {
  id: string;
  banner?: Banner;
  video?: Video;
  native?: Native;
  bidfloor?: number;
  bidfloorcur?: string;
  secure?: number;
  ext?: any;
}

export interface Banner {
  w?: number;
  h?: number;
  wmin?: number;
  hmin?: number;
  wmax?: number;
  hmax?: number;
  format?: Format[];
  pos?: number;
  ext?: any;
}

export interface Video {
  mimes: string[];
  minduration?: number;
  maxduration?: number;
  protocols?: number[];
  w?: number;
  h?: number;
  startdelay?: number;
  placement?: number;
  linearity?: number;
  skip?: number;
  skipmin?: number;
  skipafter?: number;
  sequence?: number;
  api?: number[];
  ext?: any;
}

export interface Native {
  request: string;
  ver?: string;
  api?: number[];
  ext?: any;
}

export interface Format {
  w: number;
  h: number;
}

export interface Site {
  id?: string;
  name?: string;
  domain?: string;
  cat?: string[];
  page?: string;
  publisher?: Publisher;
  content?: Content;
  ext?: any;
}

export interface App {
  id?: string;
  name?: string;
  bundle?: string;
  domain?: string;
  storeurl?: string;
  cat?: string[];
  publisher?: Publisher;
  content?: Content;
  ext?: any;
}

export interface Publisher {
  id?: string;
  name?: string;
  cat?: string[];
  domain?: string;
  ext?: any;
}

export interface Content {
  id?: string;
  episode?: number;
  title?: string;
  series?: string;
  season?: string;
  artist?: string;
  genre?: string;
  album?: string;
  isrc?: string;
  url?: string;
  cat?: string[];
  prodq?: number;
  context?: number;
  contentrating?: string;
  userrating?: string;
  qagmediarating?: number;
  keywords?: string;
  livestream?: number;
  sourcerelationship?: number;
  len?: number;
  language?: string;
  embeddable?: number;
  ext?: any;
}

export interface Device {
  ua?: string;
  geo?: Geo;
  dnt?: number;
  lmt?: number;
  ip?: string;
  ipv6?: string;
  devicetype?: number;
  make?: string;
  model?: string;
  os?: string;
  osv?: string;
  hwv?: string;
  h?: number;
  w?: number;
  ppi?: number;
  pxratio?: number;
  js?: number;
  flashver?: string;
  language?: string;
  carrier?: string;
  connectiontype?: number;
  ifa?: string;
  didsha1?: string;
  didmd5?: string;
  dpidsha1?: string;
  dpidmd5?: string;
  macsha1?: string;
  macmd5?: string;
  ext?: any;
}

export interface Geo {
  lat?: number;
  lon?: number;
  type?: number;
  accuracy?: number;
  lastfix?: number;
  ipservice?: number;
  country?: string;
  region?: string;
  regionfips104?: string;
  metro?: string;
  city?: string;
  zip?: string;
  utcoffset?: number;
  ext?: any;
}

export interface User {
  id?: string;
  buyeruid?: string;
  yob?: number;
  gender?: string;
  keywords?: string;
  data?: Data[];
  ext?: any;
}

export interface Data {
  id?: string;
  name?: string;
  segment?: Segment[];
  ext?: any;
}

export interface Segment {
  id?: string;
  name?: string;
  value?: string;
  ext?: any;
}

export interface Regulations {
  coppa?: number;
  gdpr?: number;
  us_privacy?: string;
  ext?: any;
}

export interface SeatBid {
  bid: Bid[];
  seat?: string;
  group?: number;
  ext?: any;
}

export interface Bid {
  id: string;
  impid: string;
  price: number;
  nurl?: string;
  burl?: string;
  lurl?: string;
  adm?: string;
  adid?: string;
  adomain?: string[];
  bundle?: string;
  iurl?: string;
  cid?: string;
  crid?: string;
  cat?: string[];
  attr?: number[];
  api?: number;
  protocol?: number;
  qagmediarating?: number;
  language?: string;
  dealid?: string;
  w?: number;
  h?: number;
  wratio?: number;
  hratio?: number;
  exp?: number;
  ext?: any;
}

// SDK specific types
export interface VASTParams {
  width: number;
  height: number;
  duration: number;
  extra?: Record<string, any>;
}

export interface AnalyticsParams {
  publisherId: string;
  startTime: Date;
  endTime: Date;
}

export interface AnalyticsResponse {
  publisherId: string;
  totalImpressions: number;
  totalRevenue: string;
  fillRate: number;
  ecpm: string;
  timeRange: {
    start: string;
    end: string;
  };
  dailyStats: DailyStat[];
}

export interface DailyStat {
  date: string;
  impressions: number;
  revenue: string;
  fillRate: number;
}

export interface MinerConfig {
  walletAddress: string;
  publicUrl: string;
  cacheSize: string;
  location: {
    country: string;
    region: string;
    city: string;
    lat: number;
    lon: number;
  };
  hardware: {
    cpuCores: number;
    memoryGb: number;
    diskGb: number;
    networkMbps: number;
  };
}

export interface MinerRegistration {
  minerId: string;
  status: string;
  registeredAt: string;
  websocketUrl: string;
}

export interface MinerEarnings {
  minerId: string;
  totalEarnings: string;
  pendingPayout: string;
  lastPayout: string;
  totalImpressions: number;
  totalBandwidth: number;
  period: string;
}

export interface WebSocketMessage {
  type: string;
  data: any;
  timestamp?: string;
}

export interface EventSubscription {
  type: 'subscribe' | 'unsubscribe';
  events: string[];
}