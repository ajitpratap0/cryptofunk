package indicators

import (
	"fmt"

	"github.com/cinar/indicator/v2/trend"
	"github.com/rs/zerolog/log"
)

// MACDResult represents the MACD calculation result
type MACDResult struct {
	MACD      float64 `json:"macd"`
	Signal    float64 `json:"signal"`
	Histogram float64 `json:"histogram"`
	Crossover string  `json:"crossover"` // "bullish", "bearish", "none"
}

// CalculateMACD calculates the Moving Average Convergence Divergence
func (s *Service) CalculateMACD(args map[string]interface{}) (interface{}, error) {
	// Extract prices
	prices, err := extractPrices(args, "prices")
	if err != nil {
		return nil, err
	}

	// Extract periods (defaults: fast=12, slow=26, signal=9)
	fastPeriod := extractPeriod(args, "fast_period", 12)
	slowPeriod := extractPeriod(args, "slow_period", 26)
	signalPeriod := extractPeriod(args, "signal_period", 9)

	// Validate periods
	if fastPeriod < 1 || slowPeriod < 1 || signalPeriod < 1 {
		return nil, fmt.Errorf("invalid periods: fast=%d, slow=%d, signal=%d", fastPeriod, slowPeriod, signalPeriod)
	}

	if fastPeriod >= slowPeriod {
		return nil, fmt.Errorf("fast period (%d) must be less than slow period (%d)", fastPeriod, slowPeriod)
	}

	minRequired := slowPeriod + signalPeriod
	if len(prices) < minRequired {
		return nil, fmt.Errorf("insufficient data: need at least %d prices, got %d", minRequired, len(prices))
	}

	log.Debug().
		Int("prices_count", len(prices)).
		Int("fast", fastPeriod).
		Int("slow", slowPeriod).
		Int("signal", signalPeriod).
		Msg("Calculating MACD")

	// Calculate MACD using cinar/indicator
	// Convert slice to channel
	pricesChan := make(chan float64, len(prices))
	for _, p := range prices {
		pricesChan <- p
	}
	close(pricesChan)

	macdIndicator := trend.NewMacdWithPeriod[float64](fastPeriod, slowPeriod, signalPeriod)
	macdChan, signalChan := macdIndicator.Compute(pricesChan)

	// Collect results
	var macdValues, signalValues []float64
	for {
		m, mok := <-macdChan
		s, sok := <-signalChan
		if !mok || !sok {
			break
		}
		macdValues = append(macdValues, m)
		signalValues = append(signalValues, s)
	}

	if len(macdValues) == 0 {
		return nil, fmt.Errorf("no MACD values calculated")
	}

	// Get the most recent values
	currentMACD := macdValues[len(macdValues)-1]
	currentSignal := signalValues[len(signalValues)-1]
	currentHistogram := currentMACD - currentSignal

	// Detect crossover
	crossover := "none"
	if len(macdValues) >= 2 {
		prevMACD := macdValues[len(macdValues)-2]
		prevSignal := signalValues[len(signalValues)-2]
		prevHistogram := prevMACD - prevSignal

		// Bullish crossover: MACD crosses above signal line
		if prevHistogram <= 0 && currentHistogram > 0 {
			crossover = "bullish"
		}
		// Bearish crossover: MACD crosses below signal line
		if prevHistogram >= 0 && currentHistogram < 0 {
			crossover = "bearish"
		}
	}

	result := &MACDResult{
		MACD:      currentMACD,
		Signal:    currentSignal,
		Histogram: currentHistogram,
		Crossover: crossover,
	}

	log.Info().
		Float64("macd", currentMACD).
		Float64("signal", currentSignal).
		Float64("histogram", currentHistogram).
		Str("crossover", crossover).
		Msg("MACD calculated")

	return result, nil
}
