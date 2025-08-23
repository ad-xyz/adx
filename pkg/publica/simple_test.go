package publica

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestNewPublicaSSP(t *testing.T) {
	ssp := NewPublicaSSP("test-publisher", "test-key")
	
	assert.NotNil(t, ssp)
	assert.Equal(t, "test-publisher", ssp.PublisherID)
	assert.Equal(t, "test-key", ssp.APIKey)
	assert.NotNil(t, ssp.DSPConnections)
	assert.NotNil(t, ssp.Analytics)
	assert.NotNil(t, ssp.PodCache)
}

func TestDSPConfig_Basic(t *testing.T) {
	config := &DSPConfig{
		ID:         "test-dsp",
		Name:       "Test DSP",
		Endpoint:   "https://test.example.com",
		BidderCode: "test",
		Priority:   1,
		MaxBid:     decimal.NewFromFloat(10.0),
		Categories: []string{"IAB1", "IAB2"},
	}
	
	assert.Equal(t, "test-dsp", config.ID)
	assert.Equal(t, "Test DSP", config.Name)
	assert.True(t, config.MaxBid.Equal(decimal.NewFromFloat(10.0)))
	assert.Len(t, config.Categories, 2)
}

func TestPublicaSSP_AddDSP(t *testing.T) {
	ssp := NewPublicaSSP("test-publisher", "test-key")
	
	config := &DSPConfig{
		ID:         "test-dsp",
		Name:       "Test DSP",
		Endpoint:   "https://test.example.com",
		BidderCode: "test",
		Priority:   1,
		MaxBid:     decimal.NewFromFloat(5.0),
	}
	
	ssp.AddDSP(config)
	
	assert.Len(t, ssp.DSPConnections, 1)
	assert.Equal(t, config, ssp.DSPConnections["test-dsp"])
}