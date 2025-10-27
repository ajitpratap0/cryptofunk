package indicators

import (
	"fmt"

	"github.com/cinar/indicator/v2/trend"
	"github.com/rs/zerolog/log"
)

// EMAResult represents the EMA calculation result
type EMAResult struct {
	Value float64 `json:"value"`
	Trend string  `json:"trend"` // "bullish", "bearish", "neutral"
}

// CalculateEMA calculates the Exponential Moving Average
func (s *Service) CalculateEMA(args map[string]interface{}) (interface{}, error) {
	// Extract prices
	prices, err := extractPrices(args, "prices")
	if err != nil {
		return nil, err
	}

	// Extract period (required for EMA)
	period := extractPeriod(args, "period", 0)
	if period == 0 {
		return nil, fmt.Errorf("period is required for EMA calculation")
	}

	// Validate period
	if period < 1 || period > len(prices) {
		return nil, fmt.Errorf("invalid period: %d (must be between 1 and %d)", period, len(prices))
	}

	log.Debug().
		Int("prices_count", len(prices)).
		Int("period", period).
		Msg("Calculating EMA")

	// Calculate EMA using cinar/indicator
	// Convert slice to channel
	pricesChan := make(chan float64, len(prices))
	for _, p := range prices {
		pricesChan <- p
	}
	close(pricesChan)

	emaIndicator := trend.NewEmaWithPeriod[float64](period)
	emaChan := emaIndicator.Compute(pricesChan)

	// Collect results
	var emaValues []float64
	for val := range emaChan {
		emaValues = append(emaValues, val)
	}

	if len(emaValues) == 0 {
		return nil, fmt.Errorf("no EMA values calculated")
	}

	ema := emaValues

	// Get the most recent EMA value
	currentEMA := ema[len(ema)-1]
	currentPrice := prices[len(prices)-1]

	// Determine trend based on price vs EMA
	trendSignal := "neutral"
	if currentPrice > currentEMA {
		trendSignal = "bullish" // Price above EMA
	} else if currentPrice < currentEMA {
		trendSignal = "bearish" // Price below EMA
	}

	result := &EMAResult{
		Value: currentEMA,
		Trend: trendSignal,
	}

	log.Info().
		Float64("ema", currentEMA).
		Float64("current_price", currentPrice).
		Str("trend", trendSignal).
		Msg("EMA calculated")

	return result, nil
}
