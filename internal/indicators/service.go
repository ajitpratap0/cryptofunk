package indicators

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

// Service provides technical indicator calculations
type Service struct {
	// Can add configuration, caching, etc. here in the future
}

// NewService creates a new indicator service
func NewService() *Service {
	log.Info().Msg("Indicator service initialized")
	return &Service{}
}

// extractPrices extracts price array from arguments
func extractPrices(args map[string]interface{}, key string) ([]float64, error) {
	pricesInterface, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: %s", key)
	}

	pricesArray, ok := pricesInterface.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid %s format: expected array", key)
	}

	prices := make([]float64, len(pricesArray))
	for i, p := range pricesArray {
		switch v := p.(type) {
		case float64:
			prices[i] = v
		case int:
			prices[i] = float64(v)
		default:
			return nil, fmt.Errorf("invalid price at index %d: expected number", i)
		}
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("%s array is empty", key)
	}

	return prices, nil
}

// extractPeriod extracts period from arguments with default value
func extractPeriod(args map[string]interface{}, key string, defaultValue int) int {
	periodInterface, ok := args[key]
	if !ok {
		return defaultValue
	}

	switch v := periodInterface.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		log.Warn().
			Str("key", key).
			Interface("value", periodInterface).
			Msg("Invalid period type, using default")
		return defaultValue
	}
}

// extractFloat extracts float value from arguments with default
func extractFloat(args map[string]interface{}, key string, defaultValue float64) float64 {
	valueInterface, ok := args[key]
	if !ok {
		return defaultValue
	}

	switch v := valueInterface.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		log.Warn().
			Str("key", key).
			Interface("value", valueInterface).
			Msg("Invalid float type, using default")
		return defaultValue
	}
}
