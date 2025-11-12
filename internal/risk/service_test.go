package risk

import (
	"testing"
)

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Fatal("Expected non-nil service")
	}
}

func TestCalculatePositionSize(t *testing.T) {
	service := NewService()

	tests := []struct {
		name      string
		args      map[string]interface{}
		wantError bool
		checkFunc func(*testing.T, *PositionSizeResult)
	}{
		{
			name: "Valid Kelly calculation with positive edge",
			args: map[string]interface{}{
				"win_rate": 0.6,
				"avg_win":  2.0,
				"avg_loss": 1.0,
				"capital":  10000.0,
			},
			wantError: false,
			checkFunc: func(t *testing.T, result *PositionSizeResult) {
				if result.PositionSize <= 0 {
					t.Error("Expected positive position size")
				}
				if result.KellyPercent <= 0 {
					t.Error("Expected positive Kelly percent")
				}
			},
		},
		{
			name: "Custom Kelly fraction",
			args: map[string]interface{}{
				"win_rate":       0.55,
				"avg_win":        1.5,
				"avg_loss":       1.0,
				"capital":        10000.0,
				"kelly_fraction": 0.5,
			},
			wantError: false,
		},
		{
			name: "Negative edge (no position recommended)",
			args: map[string]interface{}{
				"win_rate": 0.3,
				"avg_win":  1.0,
				"avg_loss": 1.0,
				"capital":  10000.0,
			},
			wantError: false,
			checkFunc: func(t *testing.T, result *PositionSizeResult) {
				if result.PositionSize != 0 {
					t.Errorf("Expected 0 position size for negative edge, got %.2f", result.PositionSize)
				}
			},
		},
		{
			name: "Missing win_rate",
			args: map[string]interface{}{
				"avg_win":  2.0,
				"avg_loss": 1.0,
				"capital":  10000.0,
			},
			wantError: true,
		},
		{
			name: "Invalid win_rate (> 1)",
			args: map[string]interface{}{
				"win_rate": 1.5,
				"avg_win":  2.0,
				"avg_loss": 1.0,
				"capital":  10000.0,
			},
			wantError: true,
		},
		{
			name: "Invalid avg_win (negative)",
			args: map[string]interface{}{
				"win_rate": 0.6,
				"avg_win":  -2.0,
				"avg_loss": 1.0,
				"capital":  10000.0,
			},
			wantError: true,
		},
		{
			name: "Invalid capital (zero)",
			args: map[string]interface{}{
				"win_rate": 0.6,
				"avg_win":  2.0,
				"avg_loss": 1.0,
				"capital":  0.0,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CalculatePositionSize(tt.args)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			psResult, ok := result.(*PositionSizeResult)
			if !ok {
				t.Fatal("Expected *PositionSizeResult type")
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, psResult)
			}
		})
	}
}

func TestCalculateVaR(t *testing.T) {
	service := NewService()

	returns := []interface{}{-0.05, -0.03, -0.01, 0.01, 0.02, 0.03, 0.04, 0.05, 0.06, 0.07}

	tests := []struct {
		name      string
		args      map[string]interface{}
		wantError bool
	}{
		{
			name: "Valid VaR calculation",
			args: map[string]interface{}{
				"returns": returns,
			},
			wantError: false,
		},
		{
			name: "Custom confidence level",
			args: map[string]interface{}{
				"returns":          returns,
				"confidence_level": 0.99,
			},
			wantError: false,
		},
		{
			name: "Missing returns",
			args: map[string]interface{}{
				"confidence_level": 0.95,
			},
			wantError: true,
		},
		{
			name: "Empty returns array",
			args: map[string]interface{}{
				"returns": []interface{}{},
			},
			wantError: true,
		},
		{
			name: "Invalid confidence level (> 1)",
			args: map[string]interface{}{
				"returns":          returns,
				"confidence_level": 1.5,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CalculateVaR(tt.args)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			varResult, ok := result.(*VaRResult)
			if !ok {
				t.Fatal("Expected *VaRResult type")
			}

			if varResult.VaR < 0 {
				t.Error("VaR should be non-negative")
			}

			if varResult.CVaR < varResult.VaR {
				t.Error("CVaR should be >= VaR")
			}
		})
	}
}

func TestCheckPortfolioLimits(t *testing.T) {
	service := NewService()

	currentPositions := []interface{}{
		map[string]interface{}{"symbol": "BTC", "size": 5000.0},
		map[string]interface{}{"symbol": "ETH", "size": 3000.0},
	}

	limits := map[string]interface{}{
		"max_position_size":  10000.0,
		"max_total_exposure": 50000.0,
		"max_concentration":  0.3,
		"max_open_positions": float64(5),
	}

	tests := []struct {
		name      string
		args      map[string]interface{}
		wantError bool
		checkFunc func(*testing.T, *PortfolioLimitsResult)
	}{
		{
			name: "Valid trade within limits",
			args: map[string]interface{}{
				"current_positions": currentPositions,
				"new_trade":         map[string]interface{}{"symbol": "BTC", "size": 2000.0},
				"limits":            limits,
			},
			wantError: false,
			checkFunc: func(t *testing.T, result *PortfolioLimitsResult) {
				if !result.Approved {
					t.Errorf("Expected trade to be approved, got: %s", result.Reason)
				}
			},
		},
		{
			name: "Trade exceeds position size limit",
			args: map[string]interface{}{
				"current_positions": currentPositions,
				"new_trade":         map[string]interface{}{"symbol": "BTC", "size": 15000.0},
				"limits":            limits,
			},
			wantError: false,
			checkFunc: func(t *testing.T, result *PortfolioLimitsResult) {
				if result.Approved {
					t.Error("Expected trade to be rejected for exceeding position size")
				}
				if len(result.Violations) == 0 {
					t.Error("Expected violations list")
				}
			},
		},
		{
			name: "Trade exceeds total exposure",
			args: map[string]interface{}{
				"current_positions": currentPositions,
				"new_trade":         map[string]interface{}{"symbol": "SOL", "size": 45000.0},
				"limits":            limits,
			},
			wantError: false,
			checkFunc: func(t *testing.T, result *PortfolioLimitsResult) {
				if result.Approved {
					t.Error("Expected trade to be rejected for exceeding total exposure")
				}
			},
		},
		{
			name: "Missing new_trade",
			args: map[string]interface{}{
				"current_positions": currentPositions,
				"limits":            limits,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CheckPortfolioLimits(tt.args)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			limitsResult, ok := result.(*PortfolioLimitsResult)
			if !ok {
				t.Fatal("Expected *PortfolioLimitsResult type")
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, limitsResult)
			}
		})
	}
}

func TestCalculateSharpe(t *testing.T) {
	service := NewService()

	returns := []interface{}{0.01, 0.02, -0.01, 0.03, 0.02, 0.01, -0.02, 0.04, 0.02, 0.01}

	tests := []struct {
		name      string
		args      map[string]interface{}
		wantError bool
	}{
		{
			name: "Valid Sharpe calculation",
			args: map[string]interface{}{
				"returns": returns,
			},
			wantError: false,
		},
		{
			name: "Custom risk-free rate",
			args: map[string]interface{}{
				"returns":        returns,
				"risk_free_rate": 0.05,
			},
			wantError: false,
		},
		{
			name: "Missing returns",
			args: map[string]interface{}{
				"risk_free_rate": 0.03,
			},
			wantError: true,
		},
		{
			name: "Empty returns array",
			args: map[string]interface{}{
				"returns": []interface{}{},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CalculateSharpe(tt.args)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			sharpeResult, ok := result.(*SharpeResult)
			if !ok {
				t.Fatal("Expected *SharpeResult type")
			}

			if sharpeResult.StdDev <= 0 {
				t.Error("Expected positive standard deviation")
			}
		})
	}
}

func TestCalculateDrawdown(t *testing.T) {
	service := NewService()

	// Equity curve with a clear drawdown
	equityCurve := []interface{}{
		100.0, 110.0, 120.0, 115.0, 110.0, 105.0, 115.0, 125.0, 130.0, 125.0,
	}

	tests := []struct {
		name      string
		args      map[string]interface{}
		wantError bool
		checkFunc func(*testing.T, *DrawdownResult)
	}{
		{
			name: "Valid drawdown calculation",
			args: map[string]interface{}{
				"equity_curve": equityCurve,
			},
			wantError: false,
			checkFunc: func(t *testing.T, result *DrawdownResult) {
				if result.MaxDrawdown < 0 {
					t.Error("MaxDrawdown should be non-negative")
				}
				if result.Peak <= 0 {
					t.Error("Peak should be positive")
				}
			},
		},
		{
			name:      "Missing equity_curve",
			args:      map[string]interface{}{},
			wantError: true,
		},
		{
			name: "Empty equity_curve",
			args: map[string]interface{}{
				"equity_curve": []interface{}{},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CalculateDrawdown(tt.args)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			ddResult, ok := result.(*DrawdownResult)
			if !ok {
				t.Fatal("Expected *DrawdownResult type")
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, ddResult)
			}
		})
	}
}

func TestInterpretationFunctions(t *testing.T) {
	tests := []struct {
		name         string
		input        float64
		funcCall     func(float64) string
		wantNonEmpty bool
	}{
		{
			name:         "Position recommendation - negative",
			input:        -0.01,
			funcCall:     getPositionRecommendation,
			wantNonEmpty: true,
		},
		{
			name:         "Position recommendation - positive",
			input:        0.05,
			funcCall:     getPositionRecommendation,
			wantNonEmpty: true,
		},
		{
			name:         "VaR interpretation - low",
			input:        0.02,
			funcCall:     getVaRInterpretation,
			wantNonEmpty: true,
		},
		{
			name:         "VaR interpretation - high",
			input:        0.15,
			funcCall:     getVaRInterpretation,
			wantNonEmpty: true,
		},
		{
			name:         "Sharpe interpretation - negative",
			input:        -0.5,
			funcCall:     getSharpeInterpretation,
			wantNonEmpty: true,
		},
		{
			name:         "Sharpe interpretation - excellent",
			input:        3.5,
			funcCall:     getSharpeInterpretation,
			wantNonEmpty: true,
		},
		{
			name:         "Drawdown interpretation - low",
			input:        0.05,
			funcCall:     getDrawdownInterpretation,
			wantNonEmpty: true,
		},
		{
			name:         "Drawdown interpretation - extreme",
			input:        0.60,
			funcCall:     getDrawdownInterpretation,
			wantNonEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.funcCall(tt.input)
			if tt.wantNonEmpty && result == "" {
				t.Error("Expected non-empty interpretation")
			}
		})
	}
}

func TestSqrt(t *testing.T) {
	tests := []struct {
		name      string
		input     float64
		expected  float64
		tolerance float64
	}{
		{
			name:      "Perfect square",
			input:     4.0,
			expected:  2.0,
			tolerance: 0.0001,
		},
		{
			name:      "Non-perfect square",
			input:     2.0,
			expected:  1.414,
			tolerance: 0.001,
		},
		{
			name:      "Zero",
			input:     0.0,
			expected:  0.0,
			tolerance: 0.0001,
		},
		{
			name:      "Negative (returns 0)",
			input:     -4.0,
			expected:  0.0,
			tolerance: 0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sqrt(tt.input)
			if abs(result-tt.expected) > tt.tolerance {
				t.Errorf("Expected %.4f, got %.4f", tt.expected, result)
			}
		})
	}
}

func TestSortFloat64s(t *testing.T) {
	tests := []struct {
		name     string
		input    []float64
		expected []float64
	}{
		{
			name:     "Unsorted array",
			input:    []float64{5.0, 2.0, 8.0, 1.0, 9.0},
			expected: []float64{1.0, 2.0, 5.0, 8.0, 9.0},
		},
		{
			name:     "Already sorted",
			input:    []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			expected: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
		},
		{
			name:     "Reverse sorted",
			input:    []float64{5.0, 4.0, 3.0, 2.0, 1.0},
			expected: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
		},
		{
			name:     "Single element",
			input:    []float64{42.0},
			expected: []float64{42.0},
		},
		{
			name:     "Empty array",
			input:    []float64{},
			expected: []float64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortFloat64s(tt.input)
			if len(tt.input) != len(tt.expected) {
				t.Fatalf("Length mismatch: expected %d, got %d", len(tt.expected), len(tt.input))
			}
			for i := range tt.input {
				if tt.input[i] != tt.expected[i] {
					t.Errorf("At index %d: expected %.2f, got %.2f", i, tt.expected[i], tt.input[i])
				}
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("CalculateTotalExposure", func(t *testing.T) {
		positions := []Position{
			{Symbol: "BTC", Size: 1000.0},
			{Symbol: "ETH", Size: 500.0},
			{Symbol: "BTC", Size: 300.0},
		}
		total := calculateTotalExposure(positions)
		expected := 1800.0
		if total != expected {
			t.Errorf("Expected total %.2f, got %.2f", expected, total)
		}
	})

	t.Run("CalculateSymbolExposure", func(t *testing.T) {
		positions := []Position{
			{Symbol: "BTC", Size: 1000.0},
			{Symbol: "ETH", Size: 500.0},
			{Symbol: "BTC", Size: 300.0},
		}
		btcExposure := calculateSymbolExposure(positions, "BTC")
		expected := 1300.0
		if btcExposure != expected {
			t.Errorf("Expected BTC exposure %.2f, got %.2f", expected, btcExposure)
		}
	})

	t.Run("ParseLimits", func(t *testing.T) {
		raw := map[string]interface{}{
			"max_position_size":  5000.0,
			"max_total_exposure": 50000.0,
			"max_concentration":  0.25,
			"max_open_positions": float64(8),
		}
		limits := parseLimits(raw)
		if limits.MaxPositionSize != 5000.0 {
			t.Errorf("Expected MaxPositionSize 5000.0, got %.2f", limits.MaxPositionSize)
		}
		if limits.MaxOpenPositions != 8 {
			t.Errorf("Expected MaxOpenPositions 8, got %d", limits.MaxOpenPositions)
		}
	})

	t.Run("ParsePositions", func(t *testing.T) {
		raw := []interface{}{
			map[string]interface{}{"symbol": "BTC", "size": 1000.0},
			map[string]interface{}{"symbol": "ETH", "size": 500.0},
		}
		positions := parsePositions(raw)
		if len(positions) != 2 {
			t.Errorf("Expected 2 positions, got %d", len(positions))
		}
		if positions[0].Symbol != "BTC" {
			t.Errorf("Expected first symbol BTC, got %s", positions[0].Symbol)
		}
	})

	t.Run("ParseTrade", func(t *testing.T) {
		raw := map[string]interface{}{
			"symbol": "BTC",
			"size":   2000.0,
		}
		trade := parseTrade(raw)
		if trade.Symbol != "BTC" {
			t.Errorf("Expected symbol BTC, got %s", trade.Symbol)
		}
		if trade.Size != 2000.0 {
			t.Errorf("Expected size 2000.0, got %.2f", trade.Size)
		}
	})
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
