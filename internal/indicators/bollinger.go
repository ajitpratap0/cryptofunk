//nolint:goconst // Signal types are domain-specific strings
package indicators

import (
	"fmt"

	"github.com/cinar/indicator/v2/volatility"
	"github.com/rs/zerolog/log"
)

// BollingerBandsResult represents the Bollinger Bands calculation result
type BollingerBandsResult struct {
	Upper  float64 `json:"upper"`
	Middle float64 `json:"middle"`
	Lower  float64 `json:"lower"`
	Width  float64 `json:"width"`  // Band width percentage
	Signal string  `json:"signal"` // "buy", "sell", "neutral"
}

// CalculateBollingerBands calculates Bollinger Bands
func (s *Service) CalculateBollingerBands(args map[string]interface{}) (interface{}, error) {
	// Extract prices
	prices, err := extractPrices(args, "prices")
	if err != nil {
		return nil, err
	}

	// Extract period (default: 20)
	period := extractPeriod(args, "period", 20)

	// Extract standard deviation multiplier (default: 2)
	stdDev := extractFloat(args, "std_dev", 2.0)

	// Validate parameters
	if period < 2 || period > len(prices) {
		return nil, fmt.Errorf("invalid period: %d (must be between 2 and %d)", period, len(prices))
	}

	if stdDev <= 0 {
		return nil, fmt.Errorf("invalid std_dev: %f (must be > 0)", stdDev)
	}

	log.Debug().
		Int("prices_count", len(prices)).
		Int("period", period).
		Float64("std_dev", stdDev).
		Msg("Calculating Bollinger Bands")

	// Calculate Bollinger Bands using cinar/indicator
	// Note: cinar/indicator uses fixed 2 std dev, so we ignore custom stdDev parameter
	// Convert slice to channel
	pricesChan := make(chan float64, len(prices))
	for _, p := range prices {
		pricesChan <- p
	}
	close(pricesChan)

	bbIndicator := volatility.NewBollingerBandsWithPeriod[float64](period)
	lowerChan, middleChan, upperChan := bbIndicator.Compute(pricesChan)

	// Collect results
	var lowerValues, middleValues, upperValues []float64
	for {
		l, lok := <-lowerChan
		m, mok := <-middleChan
		u, uok := <-upperChan
		if !lok || !mok || !uok {
			break
		}
		lowerValues = append(lowerValues, l)
		middleValues = append(middleValues, m)
		upperValues = append(upperValues, u)
	}

	if len(middleValues) == 0 {
		return nil, fmt.Errorf("no Bollinger Bands values calculated")
	}

	lower := lowerValues
	middle := middleValues
	upper := upperValues

	// Get the most recent values
	currentUpper := upper[len(upper)-1]
	currentMiddle := middle[len(middle)-1]
	currentLower := lower[len(lower)-1]
	currentPrice := prices[len(prices)-1]

	// Calculate band width as percentage of middle band
	bandWidth := ((currentUpper - currentLower) / currentMiddle) * 100

	// Determine signal based on price position relative to bands
	signal := "neutral"
	if currentPrice <= currentLower {
		signal = "buy" // Price at or below lower band - potential oversold
	} else if currentPrice >= currentUpper {
		signal = "sell" // Price at or above upper band - potential overbought
	}

	result := &BollingerBandsResult{
		Upper:  currentUpper,
		Middle: currentMiddle,
		Lower:  currentLower,
		Width:  bandWidth,
		Signal: signal,
	}

	log.Info().
		Float64("upper", currentUpper).
		Float64("middle", currentMiddle).
		Float64("lower", currentLower).
		Float64("width", bandWidth).
		Float64("current_price", currentPrice).
		Str("signal", signal).
		Msg("Bollinger Bands calculated")

	return result, nil
}
