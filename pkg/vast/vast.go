package vast

import (
	"encoding/xml"
	"time"
)

// VAST 4.x Video Ad Serving Template
type VAST struct {
	XMLName xml.Name `xml:"VAST"`
	Version string   `xml:"version,attr"`
	Ads     []Ad     `xml:"Ad"`
}

// Ad represents a VAST advertisement
type Ad struct {
	ID       string   `xml:"id,attr"`
	Sequence int      `xml:"sequence,attr,omitempty"`
	InLine   *InLine  `xml:"InLine,omitempty"`
	Wrapper  *Wrapper `xml:"Wrapper,omitempty"`
}

// InLine contains all data to display the ad
type InLine struct {
	AdSystem    AdSystem     `xml:"AdSystem"`
	AdTitle     string       `xml:"AdTitle"`
	Description string       `xml:"Description,omitempty"`
	Advertiser  string       `xml:"Advertiser,omitempty"`
	Pricing     *Pricing     `xml:"Pricing,omitempty"`
	Survey      []string     `xml:"Survey,omitempty"`
	Error       []string     `xml:"Error,omitempty"`
	Impression  []Impression `xml:"Impression"`
	Creatives   Creatives    `xml:"Creatives"`
	Extensions  *Extensions  `xml:"Extensions,omitempty"`
}

// Wrapper points to another VAST response
type Wrapper struct {
	AdSystem       AdSystem     `xml:"AdSystem"`
	VASTAdTagURI   string       `xml:"VASTAdTagURI"`
	Error          []string     `xml:"Error,omitempty"`
	Impression     []Impression `xml:"Impression"`
	Creatives      Creatives    `xml:"Creatives,omitempty"`
	Extensions     *Extensions  `xml:"Extensions,omitempty"`
	FallbackOnNoAd bool         `xml:"fallbackOnNoAd,attr,omitempty"`
}

// AdSystem info
type AdSystem struct {
	Version string `xml:"version,attr,omitempty"`
	Name    string `xml:",chardata"`
}

// Pricing information
type Pricing struct {
	Model    string  `xml:"model,attr"`
	Currency string  `xml:"currency,attr"`
	Value    float64 `xml:",chardata"`
}

// Impression tracking pixel
type Impression struct {
	ID  string `xml:"id,attr,omitempty"`
	URL string `xml:",cdata"`
}

// Creatives container
type Creatives struct {
	Creative []Creative `xml:"Creative"`
}

// Creative element
type Creative struct {
	ID             string          `xml:"id,attr,omitempty"`
	AdID           string          `xml:"adId,attr,omitempty"`
	Sequence       int             `xml:"sequence,attr,omitempty"`
	Linear         *Linear         `xml:"Linear,omitempty"`
	NonLinearAds   *NonLinearAds   `xml:"NonLinearAds,omitempty"`
	CompanionAds   *CompanionAds   `xml:"CompanionAds,omitempty"`
}

// Linear video ad
type Linear struct {
	SkipOffset      string          `xml:"skipoffset,attr,omitempty"`
	Duration        string          `xml:"Duration"`
	AdParameters    *AdParameters   `xml:"AdParameters,omitempty"`
	MediaFiles      MediaFiles      `xml:"MediaFiles"`
	VideoClicks     *VideoClicks    `xml:"VideoClicks,omitempty"`
	TrackingEvents  *TrackingEvents `xml:"TrackingEvents,omitempty"`
	Icons           *Icons          `xml:"Icons,omitempty"`
}

// MediaFiles container
type MediaFiles struct {
	MediaFile []MediaFile `xml:"MediaFile"`
	Mezzanine *Mezzanine  `xml:"Mezzanine,omitempty"`
}

// MediaFile represents a video file
type MediaFile struct {
	ID                  string `xml:"id,attr,omitempty"`
	Delivery            string `xml:"delivery,attr"`
	Type                string `xml:"type,attr"`
	Bitrate             int    `xml:"bitrate,attr,omitempty"`
	MinBitrate          int    `xml:"minBitrate,attr,omitempty"`
	MaxBitrate          int    `xml:"maxBitrate,attr,omitempty"`
	Width               int    `xml:"width,attr"`
	Height              int    `xml:"height,attr"`
	Scalable            bool   `xml:"scalable,attr,omitempty"`
	MaintainAspectRatio bool   `xml:"maintainAspectRatio,attr,omitempty"`
	Codec               string `xml:"codec,attr,omitempty"`
	APIFramework        string `xml:"apiFramework,attr,omitempty"`
	URL                 string `xml:",cdata"`
}

// Mezzanine file for high-quality source
type Mezzanine struct {
	ID       string `xml:"id,attr,omitempty"`
	Delivery string `xml:"delivery,attr"`
	Type     string `xml:"type,attr"`
	Width    int    `xml:"width,attr"`
	Height   int    `xml:"height,attr"`
	Codec    string `xml:"codec,attr,omitempty"`
	URL      string `xml:",cdata"`
}

// VideoClicks for clickthrough and tracking
type VideoClicks struct {
	ClickThrough  *ClickThrough   `xml:"ClickThrough,omitempty"`
	ClickTracking []ClickTracking `xml:"ClickTracking,omitempty"`
	CustomClick   []CustomClick   `xml:"CustomClick,omitempty"`
}

// ClickThrough URL
type ClickThrough struct {
	ID  string `xml:"id,attr,omitempty"`
	URL string `xml:",cdata"`
}

// ClickTracking URL
type ClickTracking struct {
	ID  string `xml:"id,attr,omitempty"`
	URL string `xml:",cdata"`
}

// CustomClick URL
type CustomClick struct {
	ID  string `xml:"id,attr,omitempty"`
	URL string `xml:",cdata"`
}

// TrackingEvents container
type TrackingEvents struct {
	Tracking []Tracking `xml:"Tracking"`
}

// Tracking event
type Tracking struct {
	Event  string `xml:"event,attr"`
	Offset string `xml:"offset,attr,omitempty"`
	URL    string `xml:",cdata"`
}

// NonLinearAds container
type NonLinearAds struct {
	TrackingEvents *TrackingEvents `xml:"TrackingEvents,omitempty"`
	NonLinear      []NonLinear     `xml:"NonLinear"`
}

// NonLinear ad (overlay, banner)
type NonLinear struct {
	ID                  string          `xml:"id,attr,omitempty"`
	Width               int             `xml:"width,attr"`
	Height              int             `xml:"height,attr"`
	ExpandedWidth       int             `xml:"expandedWidth,attr,omitempty"`
	ExpandedHeight      int             `xml:"expandedHeight,attr,omitempty"`
	Scalable            bool            `xml:"scalable,attr,omitempty"`
	MaintainAspectRatio bool            `xml:"maintainAspectRatio,attr,omitempty"`
	MinSuggestedDuration string         `xml:"minSuggestedDuration,attr,omitempty"`
	APIFramework        string          `xml:"apiFramework,attr,omitempty"`
	StaticResource      *StaticResource `xml:"StaticResource,omitempty"`
	IFrameResource      string          `xml:"IFrameResource,omitempty"`
	HTMLResource        string          `xml:"HTMLResource,omitempty"`
	AdParameters        *AdParameters   `xml:"AdParameters,omitempty"`
	NonLinearClickThrough string        `xml:"NonLinearClickThrough,omitempty"`
	NonLinearClickTracking []string     `xml:"NonLinearClickTracking,omitempty"`
}

// StaticResource for images
type StaticResource struct {
	CreativeType string `xml:"creativeType,attr"`
	URL          string `xml:",cdata"`
}

// CompanionAds container
type CompanionAds struct {
	Required  string       `xml:"required,attr,omitempty"`
	Companion []Companion  `xml:"Companion"`
}

// Companion ad
type Companion struct {
	ID                  string          `xml:"id,attr,omitempty"`
	Width               int             `xml:"width,attr"`
	Height              int             `xml:"height,attr"`
	AssetWidth          int             `xml:"assetWidth,attr,omitempty"`
	AssetHeight         int             `xml:"assetHeight,attr,omitempty"`
	ExpandedWidth       int             `xml:"expandedWidth,attr,omitempty"`
	ExpandedHeight      int             `xml:"expandedHeight,attr,omitempty"`
	APIFramework        string          `xml:"apiFramework,attr,omitempty"`
	AdSlotID            string          `xml:"adSlotId,attr,omitempty"`
	StaticResource      *StaticResource `xml:"StaticResource,omitempty"`
	IFrameResource      string          `xml:"IFrameResource,omitempty"`
	HTMLResource        string          `xml:"HTMLResource,omitempty"`
	AdParameters        *AdParameters   `xml:"AdParameters,omitempty"`
	AltText             string          `xml:"AltText,omitempty"`
	CompanionClickThrough string        `xml:"CompanionClickThrough,omitempty"`
	CompanionClickTracking []string     `xml:"CompanionClickTracking,omitempty"`
	TrackingEvents      *TrackingEvents `xml:"TrackingEvents,omitempty"`
}

// AdParameters for VPAID
type AdParameters struct {
	XMLEncoded bool   `xml:"xmlEncoded,attr,omitempty"`
	Parameters string `xml:",cdata"`
}

// Icons container
type Icons struct {
	Icon []Icon `xml:"Icon"`
}

// Icon for ad choices
type Icon struct {
	Program      string        `xml:"program,attr,omitempty"`
	Width        int           `xml:"width,attr"`
	Height       int           `xml:"height,attr"`
	XPosition    string        `xml:"xPosition,attr"`
	YPosition    string        `xml:"yPosition,attr"`
	Duration     string        `xml:"duration,attr,omitempty"`
	Offset       string        `xml:"offset,attr,omitempty"`
	APIFramework string        `xml:"apiFramework,attr,omitempty"`
	StaticResource *StaticResource `xml:"StaticResource,omitempty"`
	IFrameResource string        `xml:"IFrameResource,omitempty"`
	HTMLResource  string        `xml:"HTMLResource,omitempty"`
	IconClicks    *IconClicks   `xml:"IconClicks,omitempty"`
	IconViewTracking []string   `xml:"IconViewTracking,omitempty"`
}

// IconClicks container
type IconClicks struct {
	IconClickThrough  string   `xml:"IconClickThrough,omitempty"`
	IconClickTracking []string `xml:"IconClickTracking,omitempty"`
}

// Extensions container
type Extensions struct {
	Extension []Extension `xml:"Extension"`
}

// Extension for custom data
type Extension struct {
	Type string `xml:"type,attr,omitempty"`
	Data string `xml:",innerxml"`
}

// AdPod for CTV ad breaks
type AdPod struct {
	ID            string
	MaxDuration   time.Duration
	MinDuration   time.Duration
	MaxAds        int
	MinAds        int
	AdBreakType   string // pre-roll, mid-roll, post-roll
	Ads           []Ad
	TotalDuration time.Duration
}

// CTVAdRequest for Connected TV specific requests
type CTVAdRequest struct {
	DeviceID      string
	AppID         string
	ContentID     string
	ContentGenre  string
	ContentRating string
	AdPodSlots    []AdPod
	UserAgent     string
	IP            string
	Lat           float64
	Lon           float64
	DNT           bool
	LMT           bool // Limit Ad Tracking
	COPPA         bool
}

// CTVAdResponse for Connected TV
type CTVAdResponse struct {
	RequestID string
	AdPods    []AdPod
	VAST      *VAST
	Errors    []string
}