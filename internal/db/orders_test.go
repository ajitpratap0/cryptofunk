package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConvertOrderSide tests the ConvertOrderSide function
func TestConvertOrderSide(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected OrderSide
	}{
		{
			name:     "Uppercase BUY",
			input:    "BUY",
			expected: OrderSideBuy,
		},
		{
			name:     "Uppercase SELL",
			input:    "SELL",
			expected: OrderSideSell,
		},
		{
			name:     "Lowercase buy",
			input:    "buy",
			expected: OrderSideBuy,
		},
		{
			name:     "Lowercase sell",
			input:    "sell",
			expected: OrderSideSell,
		},
		{
			name:     "Mixed case Buy",
			input:    "Buy",
			expected: OrderSideBuy,
		},
		{
			name:     "Unknown value defaults to BUY",
			input:    "UNKNOWN",
			expected: OrderSideBuy,
		},
		{
			name:     "Empty string defaults to BUY",
			input:    "",
			expected: OrderSideBuy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertOrderSide(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertOrderType tests the ConvertOrderType function
func TestConvertOrderType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected OrderType
	}{
		{
			name:     "Uppercase MARKET",
			input:    "MARKET",
			expected: OrderTypeMarket,
		},
		{
			name:     "Uppercase LIMIT",
			input:    "LIMIT",
			expected: OrderTypeLimit,
		},
		{
			name:     "Lowercase market",
			input:    "market",
			expected: OrderTypeMarket,
		},
		{
			name:     "Lowercase limit",
			input:    "limit",
			expected: OrderTypeLimit,
		},
		{
			name:     "Mixed case Market",
			input:    "Market",
			expected: OrderTypeMarket,
		},
		{
			name:     "Unknown value defaults to MARKET",
			input:    "UNKNOWN",
			expected: OrderTypeMarket,
		},
		{
			name:     "Empty string defaults to MARKET",
			input:    "",
			expected: OrderTypeMarket,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertOrderType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertOrderStatus tests the ConvertOrderStatus function
func TestConvertOrderStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected OrderStatus
	}{
		{
			name:     "Uppercase NEW",
			input:    "NEW",
			expected: OrderStatusNew,
		},
		{
			name:     "Uppercase PENDING maps to NEW",
			input:    "PENDING",
			expected: OrderStatusNew,
		},
		{
			name:     "Uppercase OPEN maps to PARTIALLY_FILLED",
			input:    "OPEN",
			expected: OrderStatusPartiallyFilled,
		},
		{
			name:     "Uppercase PARTIALLY_FILLED",
			input:    "PARTIALLY_FILLED",
			expected: OrderStatusPartiallyFilled,
		},
		{
			name:     "Uppercase FILLED",
			input:    "FILLED",
			expected: OrderStatusFilled,
		},
		{
			name:     "Uppercase CANCELLED maps to CANCELED",
			input:    "CANCELLED",
			expected: OrderStatusCanceled,
		},
		{
			name:     "Uppercase CANCELED",
			input:    "CANCELED",
			expected: OrderStatusCanceled,
		},
		{
			name:     "Uppercase REJECTED",
			input:    "REJECTED",
			expected: OrderStatusRejected,
		},
		{
			name:     "Lowercase new",
			input:    "new",
			expected: OrderStatusNew,
		},
		{
			name:     "Lowercase filled",
			input:    "filled",
			expected: OrderStatusFilled,
		},
		{
			name:     "Mixed case Filled",
			input:    "Filled",
			expected: OrderStatusFilled,
		},
		{
			name:     "Unknown value defaults to NEW",
			input:    "UNKNOWN",
			expected: OrderStatusNew,
		},
		{
			name:     "Empty string defaults to NEW",
			input:    "",
			expected: OrderStatusNew,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertOrderStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestOrderSideConstants tests that order side constants are defined correctly
func TestOrderSideConstants(t *testing.T) {
	assert.Equal(t, OrderSide("BUY"), OrderSideBuy)
	assert.Equal(t, OrderSide("SELL"), OrderSideSell)
}

// TestOrderTypeConstants tests that order type constants are defined correctly
func TestOrderTypeConstants(t *testing.T) {
	assert.Equal(t, OrderType("MARKET"), OrderTypeMarket)
	assert.Equal(t, OrderType("LIMIT"), OrderTypeLimit)
}

// TestOrderStatusConstants tests that order status constants are defined correctly
func TestOrderStatusConstants(t *testing.T) {
	assert.Equal(t, OrderStatus("NEW"), OrderStatusNew)
	assert.Equal(t, OrderStatus("PARTIALLY_FILLED"), OrderStatusPartiallyFilled)
	assert.Equal(t, OrderStatus("FILLED"), OrderStatusFilled)
	assert.Equal(t, OrderStatus("CANCELED"), OrderStatusCanceled)
	assert.Equal(t, OrderStatus("REJECTED"), OrderStatusRejected)
}
