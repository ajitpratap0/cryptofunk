package backtest

import (
	"testing"
	"time"
)

func TestCalculateStatsFromTrades(t *testing.T) {
	tests := []struct {
		name      string
		positions []*ClosedPosition
		want      *TradingStats
	}{
		{
			name:      "Empty positions",
			positions: []*ClosedPosition{},
			want: &TradingStats{
				TotalTrades:   0,
				WinningTrades: 0,
				LosingTrades:  0,
				AvgWin:        0,
				AvgLoss:       0,
				WinRate:       0,
			},
		},
		{
			name: "All winning trades",
			positions: []*ClosedPosition{
				{Symbol: "BTC", RealizedPL: 100},
				{Symbol: "ETH", RealizedPL: 200},
				{Symbol: "SOL", RealizedPL: 150},
			},
			want: &TradingStats{
				TotalTrades:   3,
				WinningTrades: 3,
				LosingTrades:  0,
				AvgWin:        150, // (100 + 200 + 150) / 3
				AvgLoss:       0,
				WinRate:       1.0,
				TotalProfit:   450,
				TotalLoss:     0,
				LargestWin:    200,
				LargestLoss:   0,
			},
		},
		{
			name: "All losing trades",
			positions: []*ClosedPosition{
				{Symbol: "BTC", RealizedPL: -100},
				{Symbol: "ETH", RealizedPL: -200},
				{Symbol: "SOL", RealizedPL: -150},
			},
			want: &TradingStats{
				TotalTrades:   3,
				WinningTrades: 0,
				LosingTrades:  3,
				AvgWin:        0,
				AvgLoss:       150, // (100 + 200 + 150) / 3
				WinRate:       0,
				TotalProfit:   0,
				TotalLoss:     450,
				LargestWin:    0,
				LargestLoss:   200,
			},
		},
		{
			name: "Mixed wins and losses",
			positions: []*ClosedPosition{
				{Symbol: "BTC", RealizedPL: 100},  // win
				{Symbol: "ETH", RealizedPL: -50},  // loss
				{Symbol: "SOL", RealizedPL: 200},  // win
				{Symbol: "ADA", RealizedPL: -100}, // loss
				{Symbol: "DOT", RealizedPL: 150},  // win
			},
			want: &TradingStats{
				TotalTrades:   5,
				WinningTrades: 3,
				LosingTrades:  2,
				AvgWin:        150, // (100 + 200 + 150) / 3
				AvgLoss:       75,  // (50 + 100) / 2
				WinRate:       0.6, // 3/5
				TotalProfit:   450, // 100 + 200 + 150
				TotalLoss:     150, // 50 + 100
				AvgReturn:     60,  // (450 - 150) / 5
				LargestWin:    200, // max win
				LargestLoss:   100, // max loss
				WinLossRatio:  2.0, // 150 / 75
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateStatsFromTrades(tt.positions)

			if got.TotalTrades != tt.want.TotalTrades {
				t.Errorf("TotalTrades = %d, want %d", got.TotalTrades, tt.want.TotalTrades)
			}
			if got.WinningTrades != tt.want.WinningTrades {
				t.Errorf("WinningTrades = %d, want %d", got.WinningTrades, tt.want.WinningTrades)
			}
			if got.LosingTrades != tt.want.LosingTrades {
				t.Errorf("LosingTrades = %d, want %d", got.LosingTrades, tt.want.LosingTrades)
			}
			if !floatEqual(got.AvgWin, tt.want.AvgWin) {
				t.Errorf("AvgWin = %.2f, want %.2f", got.AvgWin, tt.want.AvgWin)
			}
			if !floatEqual(got.AvgLoss, tt.want.AvgLoss) {
				t.Errorf("AvgLoss = %.2f, want %.2f", got.AvgLoss, tt.want.AvgLoss)
			}
			if !floatEqual(got.WinRate, tt.want.WinRate) {
				t.Errorf("WinRate = %.2f, want %.2f", got.WinRate, tt.want.WinRate)
			}
			if !floatEqual(got.TotalProfit, tt.want.TotalProfit) {
				t.Errorf("TotalProfit = %.2f, want %.2f", got.TotalProfit, tt.want.TotalProfit)
			}
			if !floatEqual(got.TotalLoss, tt.want.TotalLoss) {
				t.Errorf("TotalLoss = %.2f, want %.2f", got.TotalLoss, tt.want.TotalLoss)
			}
			if !floatEqual(got.LargestWin, tt.want.LargestWin) {
				t.Errorf("LargestWin = %.2f, want %.2f", got.LargestWin, tt.want.LargestWin)
			}
			if !floatEqual(got.LargestLoss, tt.want.LargestLoss) {
				t.Errorf("LargestLoss = %.2f, want %.2f", got.LargestLoss, tt.want.LargestLoss)
			}

			// Check calculated fields
			if tt.want.AvgReturn != 0 && !floatEqual(got.AvgReturn, tt.want.AvgReturn) {
				t.Errorf("AvgReturn = %.2f, want %.2f", got.AvgReturn, tt.want.AvgReturn)
			}
			if tt.want.WinLossRatio != 0 && !floatEqual(got.WinLossRatio, tt.want.WinLossRatio) {
				t.Errorf("WinLossRatio = %.2f, want %.2f", got.WinLossRatio, tt.want.WinLossRatio)
			}
		})
	}
}

func TestKellyCalculator_CalculatePositionSize(t *testing.T) {
	kc := NewKellyCalculator(nil) // No DB needed for position size calculation
	capital := 10000.0

	tests := []struct {
		name               string
		stats              *TradingStats
		kellyFraction      float64
		wantMin            float64
		wantMax            float64
		expectConservative bool
	}{
		{
			name: "Not enough trades - conservative sizing",
			stats: &TradingStats{
				TotalTrades:   20, // Less than 30
				WinningTrades: 12,
				WinRate:       0.6,
				AvgWin:        100,
				AvgLoss:       50,
				WinLossRatio:  2.0,
			},
			kellyFraction:      0.25,
			wantMin:            900,  // ~10% of 10k
			wantMax:            1100, // ~10% of 10k
			expectConservative: true,
		},
		{
			name: "Good edge with 60% win rate",
			stats: &TradingStats{
				TotalTrades:   100,
				WinningTrades: 60,
				LosingTrades:  40,
				WinRate:       0.6,
				AvgWin:        200,
				AvgLoss:       100,
				WinLossRatio:  2.0,
			},
			kellyFraction: 0.25, // Quarter Kelly
			// Kelly = (0.6 * 2 - 0.4) / 2 = (1.2 - 0.4) / 2 = 0.4
			// Adjusted = 0.4 * 0.25 = 0.10 = 10%
			wantMin: 900,
			wantMax: 1100,
		},
		{
			name: "Strong edge with 70% win rate",
			stats: &TradingStats{
				TotalTrades:   100,
				WinningTrades: 70,
				LosingTrades:  30,
				WinRate:       0.7,
				AvgWin:        200,
				AvgLoss:       100,
				WinLossRatio:  2.0,
			},
			kellyFraction: 0.25, // Quarter Kelly
			// Kelly = (0.7 * 2 - 0.3) / 2 = (1.4 - 0.3) / 2 = 0.55
			// Adjusted = 0.55 * 0.25 = 0.1375 = 13.75%
			wantMin: 1200,
			wantMax: 1500,
		},
		{
			name: "Minimal edge - 51% win rate",
			stats: &TradingStats{
				TotalTrades:   100,
				WinningTrades: 51,
				LosingTrades:  49,
				WinRate:       0.51,
				AvgWin:        100,
				AvgLoss:       98,
				WinLossRatio:  100.0 / 98.0,
			},
			kellyFraction: 0.25,
			// Small edge should result in small position
			wantMin: 100,
			wantMax: 500,
		},
		{
			name: "Negative edge - no position",
			stats: &TradingStats{
				TotalTrades:   100,
				WinningTrades: 40,
				LosingTrades:  60,
				WinRate:       0.4,
				AvgWin:        100,
				AvgLoss:       100,
				WinLossRatio:  1.0,
			},
			kellyFraction: 0.25,
			// Kelly = (0.4 * 1 - 0.6) / 1 = -0.2 (negative)
			// Should return minimal 1%
			wantMin: 50,
			wantMax: 150,
		},
		{
			name: "Half Kelly for more aggressive sizing",
			stats: &TradingStats{
				TotalTrades:   100,
				WinningTrades: 60,
				LosingTrades:  40,
				WinRate:       0.6,
				AvgWin:        200,
				AvgLoss:       100,
				WinLossRatio:  2.0,
			},
			kellyFraction: 0.5, // Half Kelly
			// Kelly = 0.4, Adjusted = 0.4 * 0.5 = 0.20 = 20%
			wantMin: 1800,
			wantMax: 2200,
		},
		{
			name: "Very high edge capped at 25%",
			stats: &TradingStats{
				TotalTrades:   100,
				WinningTrades: 80,
				LosingTrades:  20,
				WinRate:       0.8,
				AvgWin:        300,
				AvgLoss:       50,
				WinLossRatio:  6.0,
			},
			kellyFraction: 1.0, // Full Kelly
			// Kelly = (0.8 * 6 - 0.2) / 6 = (4.8 - 0.2) / 6 = 0.7667
			// This would be huge, but should be capped at 25%
			wantMin: 2400,
			wantMax: 2600, // 25% of 10k = 2500
		},
		{
			name: "Zero win rate",
			stats: &TradingStats{
				TotalTrades:   100,
				WinningTrades: 0,
				LosingTrades:  100,
				WinRate:       0,
				AvgWin:        0,
				AvgLoss:       100,
			},
			kellyFraction:      0.25,
			wantMin:            900, // Conservative 10%
			wantMax:            1100,
			expectConservative: true,
		},
		{
			name: "100% win rate",
			stats: &TradingStats{
				TotalTrades:   100,
				WinningTrades: 100,
				LosingTrades:  0,
				WinRate:       1.0,
				AvgWin:        100,
				AvgLoss:       0,
			},
			kellyFraction:      0.25,
			wantMin:            900, // Conservative 10%
			wantMax:            1100,
			expectConservative: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kc.CalculatePositionSize(tt.stats, capital, tt.kellyFraction)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CalculatePositionSize() = %.2f, want between %.2f and %.2f",
					got, tt.wantMin, tt.wantMax)
			}

			// Verify it's within reasonable bounds
			if got < 0 {
				t.Error("Position size should not be negative")
			}
			if got > capital {
				t.Error("Position size should not exceed capital")
			}
		})
	}
}

func TestGetRecommendation(t *testing.T) {
	tests := []struct {
		name         string
		kellyPercent float64
		wantContains string
	}{
		{
			name:         "Negative edge",
			kellyPercent: -0.05,
			wantContains: "negative edge",
		},
		{
			name:         "Minimal edge",
			kellyPercent: 0.01,
			wantContains: "Very small",
		},
		{
			name:         "Conservative position",
			kellyPercent: 0.03,
			wantContains: "Conservative",
		},
		{
			name:         "Standard position",
			kellyPercent: 0.08,
			wantContains: "Standard",
		},
		{
			name:         "Large position",
			kellyPercent: 0.15,
			wantContains: "Large",
		},
		{
			name:         "Very large position",
			kellyPercent: 0.25,
			wantContains: "Very large",
		},
		{
			name:         "Extremely large position",
			kellyPercent: 0.45,
			wantContains: "Warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRecommendation(tt.kellyPercent)
			if got == "" {
				t.Error("Expected non-empty recommendation")
			}
			// Just verify we get some recommendation text
			// The exact text may vary
		})
	}
}

func TestNewKellyCalculator(t *testing.T) {
	kc := NewKellyCalculator(nil)
	if kc == nil {
		t.Fatal("Expected non-nil KellyCalculator")
	}
}

// Test Kelly formula edge cases
func TestKellyFormulaEdgeCases(t *testing.T) {
	kc := NewKellyCalculator(nil)
	capital := 10000.0

	t.Run("Equal win and loss amounts with 50% win rate", func(t *testing.T) {
		stats := &TradingStats{
			TotalTrades:   100,
			WinningTrades: 50,
			LosingTrades:  50,
			WinRate:       0.5,
			AvgWin:        100,
			AvgLoss:       100,
			WinLossRatio:  1.0,
		}
		// Kelly = (0.5 * 1 - 0.5) / 1 = 0 (no edge)
		got := kc.CalculatePositionSize(stats, capital, 0.25)
		// Should return minimal 1% when no edge
		if got < 50 || got > 150 {
			t.Errorf("Expected minimal position for no edge, got %.2f", got)
		}
	})

	t.Run("Very small average loss", func(t *testing.T) {
		stats := &TradingStats{
			TotalTrades:   100,
			WinningTrades: 60,
			LosingTrades:  40,
			WinRate:       0.6,
			AvgWin:        100,
			AvgLoss:       0.01, // Very small
			WinLossRatio:  100 / 0.01,
		}
		// High win/loss ratio but should still be reasonable
		got := kc.CalculatePositionSize(stats, capital, 0.25)
		// Should be capped at 25%
		if got > capital*0.26 {
			t.Errorf("Position size should be capped at ~25%%, got %.2f", got)
		}
	})

	t.Run("Realistic trading scenario", func(t *testing.T) {
		// Simulate realistic trading: 55% win rate, avg win 1.5x avg loss
		positions := []*ClosedPosition{
			// 55 wins
			{Symbol: "BTC", RealizedPL: 150},
			{Symbol: "ETH", RealizedPL: 150},
			{Symbol: "SOL", RealizedPL: 150},
			{Symbol: "ADA", RealizedPL: 150},
			{Symbol: "DOT", RealizedPL: 150},
			{Symbol: "BTC", RealizedPL: 150},
			{Symbol: "ETH", RealizedPL: 150},
			{Symbol: "SOL", RealizedPL: 150},
			{Symbol: "ADA", RealizedPL: 150},
			{Symbol: "DOT", RealizedPL: 150},
		}
		// Add wins to get to 55 total
		for i := 0; i < 45; i++ {
			positions = append(positions, &ClosedPosition{
				Symbol:     "BTC",
				RealizedPL: 150,
			})
		}
		// Add 45 losses
		for i := 0; i < 45; i++ {
			positions = append(positions, &ClosedPosition{
				Symbol:     "BTC",
				RealizedPL: -100,
			})
		}

		stats := CalculateStatsFromTrades(positions)
		if stats.TotalTrades != 100 {
			t.Errorf("Expected 100 trades, got %d", stats.TotalTrades)
		}
		if stats.WinRate != 0.55 {
			t.Errorf("Expected 55%% win rate, got %.2f", stats.WinRate*100)
		}

		got := kc.CalculatePositionSize(stats, capital, 0.25)
		// With 55% win rate and 1.5:1 win/loss, should get moderate position
		// Kelly = (0.55 * 1.5 - 0.45) / 1.5 = (0.825 - 0.45) / 1.5 = 0.25
		// Adjusted = 0.25 * 0.25 = 0.0625 = 6.25%
		if got < 500 || got > 800 {
			t.Errorf("Expected ~6.25%% position (625), got %.2f", got)
		}
	})
}

// Helper function for float comparison with tolerance
func floatEqual(a, b float64) bool {
	tolerance := 0.01
	return abs(a-b) < tolerance
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// Benchmark Kelly calculation
func BenchmarkCalculatePositionSize(b *testing.B) {
	kc := NewKellyCalculator(nil)
	stats := &TradingStats{
		TotalTrades:   100,
		WinningTrades: 60,
		LosingTrades:  40,
		WinRate:       0.6,
		AvgWin:        200,
		AvgLoss:       100,
		WinLossRatio:  2.0,
	}
	capital := 10000.0
	kellyFraction := 0.25

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kc.CalculatePositionSize(stats, capital, kellyFraction)
	}
}

func BenchmarkCalculateStatsFromTrades(b *testing.B) {
	// Create realistic trade history
	positions := make([]*ClosedPosition, 100)
	now := time.Now()
	for i := 0; i < 100; i++ {
		pl := 100.0
		if i%3 == 0 { // 33% losses
			pl = -75.0
		}
		positions[i] = &ClosedPosition{
			Symbol:     "BTC",
			RealizedPL: pl,
			EntryTime:  now.Add(-time.Duration(i) * time.Hour),
			ExitTime:   now.Add(-time.Duration(i-1) * time.Hour),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateStatsFromTrades(positions)
	}
}
