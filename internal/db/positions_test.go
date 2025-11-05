package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConvertPositionSide tests the ConvertPositionSide function
func TestConvertPositionSide(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected PositionSide
	}{
		{
			name:     "Uppercase LONG",
			input:    "LONG",
			expected: PositionSideLong,
		},
		{
			name:     "Lowercase long",
			input:    "long",
			expected: PositionSideLong,
		},
		{
			name:     "Uppercase BUY maps to LONG",
			input:    "BUY",
			expected: PositionSideLong,
		},
		{
			name:     "Lowercase buy maps to LONG",
			input:    "buy",
			expected: PositionSideLong,
		},
		{
			name:     "Uppercase SHORT",
			input:    "SHORT",
			expected: PositionSideShort,
		},
		{
			name:     "Lowercase short",
			input:    "short",
			expected: PositionSideShort,
		},
		{
			name:     "Uppercase SELL maps to SHORT",
			input:    "SELL",
			expected: PositionSideShort,
		},
		{
			name:     "Lowercase sell maps to SHORT",
			input:    "sell",
			expected: PositionSideShort,
		},
		{
			name:     "Unknown value defaults to FLAT",
			input:    "UNKNOWN",
			expected: PositionSideFlat,
		},
		{
			name:     "Empty string defaults to FLAT",
			input:    "",
			expected: PositionSideFlat,
		},
		{
			name:     "Mixed case Long",
			input:    "Long",
			expected: PositionSideFlat, // Only exact matches work, mixed case defaults to FLAT
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertPositionSide(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPositionSideConstants tests that position side constants are defined correctly
func TestPositionSideConstants(t *testing.T) {
	assert.Equal(t, PositionSide("LONG"), PositionSideLong)
	assert.Equal(t, PositionSide("SHORT"), PositionSideShort)
	assert.Equal(t, PositionSide("FLAT"), PositionSideFlat)
}
