package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/polymarket/gamma"
)

func TestParseResponse(t *testing.T) {
	t.Run("valid JSON response", func(t *testing.T) {
		resp := `{"predicted_probability":0.75,"confidence":0.8,"reasoning":"strong evidence","action":"BUY YES"}`
		market := &gamma.Market{
			ConditionID:     "123",
			Question:        "Test?",
			OutcomeYesPrice: 0.5,
			OutcomeNoPrice:  0.5,
		}

		analysis := parseResponse(resp, market)

		require.NotNil(t, analysis)
		assert.InDelta(t, 0.75, analysis.PredictedProb, 1e-9)
		assert.InDelta(t, 0.8, analysis.Confidence, 1e-9)
		assert.Equal(t, "BUY YES", analysis.Action)
		assert.InDelta(t, 0.25, analysis.Edge, 1e-9) // 0.75 - 0.5
		assert.Equal(t, "123", analysis.MarketID)
	})

	t.Run("JSON embedded in surrounding text", func(t *testing.T) {
		resp := `Some preamble text {"predicted_probability":0.3,"confidence":0.6,"reasoning":"weak","action":"BUY NO"} trailing text`
		market := &gamma.Market{
			ConditionID:     "456",
			Question:        "Will X happen?",
			OutcomeYesPrice: 0.5,
			OutcomeNoPrice:  0.5,
		}

		analysis := parseResponse(resp, market)

		require.NotNil(t, analysis)
		assert.Equal(t, "BUY NO", analysis.Action)
		assert.InDelta(t, 0.3, analysis.PredictedProb, 1e-9)
	})

	t.Run("invalid response with no valid JSON", func(t *testing.T) {
		resp := "I cannot analyze this market"
		market := &gamma.Market{
			ConditionID:     "789",
			Question:        "Another question?",
			OutcomeYesPrice: 0.6,
			OutcomeNoPrice:  0.4,
		}

		analysis := parseResponse(resp, market)

		require.NotNil(t, analysis)
		assert.Equal(t, "SKIP", analysis.Action)
		assert.InDelta(t, 0.0, analysis.PredictedProb, 1e-9)
	})

	t.Run("empty string response", func(t *testing.T) {
		resp := ""
		market := &gamma.Market{
			ConditionID:     "000",
			Question:        "Empty test?",
			OutcomeYesPrice: 0.5,
			OutcomeNoPrice:  0.5,
		}

		analysis := parseResponse(resp, market)

		require.NotNil(t, analysis)
		assert.Equal(t, "SKIP", analysis.Action)
	})
}
