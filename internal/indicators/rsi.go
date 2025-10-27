package indicators

import (
	"fmt"

	"github.com/cinar/indicator/v2/momentum"
	"github.com/rs/zerolog/log"
)

// RSIResult represents the RSI calculation result
type RSIResult struct {
	Value  float64 `json:"value"`
	Signal string  `json:"signal"` // "oversold", "overbought", "neutral"
}

// CalculateRSI calculates the Relative Strength Index
func (s *Service) CalculateRSI(args map[string]interface{}) (interface{}, error) {
	// Extract prices
	prices, err := extractPrices(args, "prices")
	if err != nil {
		return nil, err
	}

	// Extract period (default: 14)
	period := extractPeriod(args, "period", 14)

	// Validate period
	if period < 1 || period > len(prices) {
		return nil, fmt.Errorf("invalid period: %d (must be between 1 and %d)", period, len(prices))
	}

	log.Debug().
		Int("prices_count", len(prices)).
		Int("period", period).
		Msg("Calculating RSI")

	// Calculate RSI using cinar/indicator
	// Convert slice to channel
	pricesChan := make(chan float64, len(prices))
	for _, p := range prices {
		pricesChan <- p
	}
	close(pricesChan)

	rsiIndicator := momentum.NewRsiWithPeriod[float64](period)
	rsiChan := rsiIndicator.Compute(pricesChan)

	// Collect results
	var rsiValues []float64
	for val := range rsiChan {
		rsiValues = append(rsiValues, val)
	}

	if len(rsiValues) == 0 {
		return nil, fmt.Errorf("no RSI values calculated")
	}

	// Get the most recent RSI value
	currentRSI := rsiValues[len(rsiValues)-1]

	// Determine signal
	signal := "neutral"
	if currentRSI < 30 {
		signal = "oversold" // Potential buy signal
	} else if currentRSI > 70 {
		signal = "overbought" // Potential sell signal
	}

	result := &RSIResult{
		Value:  currentRSI,
		Signal: signal,
	}

	log.Info().
		Float64("rsi", currentRSI).
		Str("signal", signal).
		Msg("RSI calculated")

	return result, nil
}
