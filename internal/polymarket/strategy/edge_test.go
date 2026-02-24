package strategy

import (
	"math"
	"testing"
)

func TestNewEdgeCalculator_Defaults(t *testing.T) {
	ec := NewEdgeCalculator(0, 0)
	r := ec.Calculate(0.70, 0.50, 0.8)
	if !r.MeetsThreshold {
		t.Error("20% edge should meet 10% default threshold")
	}
}

func TestEdgeCalculator_ExactNoTrade(t *testing.T) {
	ec := NewEdgeCalculator(0.10, 0.25)
	r := ec.Calculate(0.50, 0.50, 1.0)
	if r.Side != SideNoTrade {
		t.Errorf("expected NO_TRADE, got %s", r.Side)
	}
	if r.MeetsThreshold {
		t.Error("zero edge should not meet threshold")
	}
	if r.KellyFraction != 0 {
		t.Errorf("expected kelly 0, got %f", r.KellyFraction)
	}
	if r.ExpectedValue != 0 {
		t.Errorf("expected EV 0, got %f", r.ExpectedValue)
	}
}

func TestEdgeCalculator_ExpectedValue(t *testing.T) {
	ec := NewEdgeCalculator(0.10, 0.50)
	r := ec.Calculate(0.80, 0.50, 1.0)
	// EV = (0.8 * (1/0.5)) - 1 = 0.6
	if math.Abs(r.ExpectedValue-0.6) > 0.001 {
		t.Errorf("expected EV 0.6, got %.4f", r.ExpectedValue)
	}
}

func TestEdgeCalculator_NOSide_ExpectedValue(t *testing.T) {
	ec := NewEdgeCalculator(0.10, 0.50)
	r := ec.Calculate(0.20, 0.60, 1.0)
	// noProb = 0.8, noPrice = 0.4
	// EV = (0.8 * (1/0.4)) - 1 = 1.0
	if math.Abs(r.ExpectedValue-1.0) > 0.001 {
		t.Errorf("expected EV 1.0, got %.4f", r.ExpectedValue)
	}
}

func TestKellyFraction_BoundaryValues(t *testing.T) {
	tests := []struct {
		name  string
		p     float64
		price float64
		want  float64
	}{
		{"zero price", 0.5, 0, 0},
		{"one price", 0.5, 1, 0},
		{"zero prob", 0, 0.5, 0},
		{"one prob", 1, 0.5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kellyFraction(tt.p, tt.price)
			if got != tt.want {
				t.Errorf("kellyFraction(%v, %v) = %v, want %v", tt.p, tt.price, got, tt.want)
			}
		})
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		v, min, max, want float64
	}{
		{0.5, 0, 1, 0.5},
		{-1, 0, 1, 0},
		{2, 0, 1, 1},
		{0, 0, 0, 0},
	}
	for _, tt := range tests {
		got := clamp(tt.v, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("clamp(%v, %v, %v) = %v, want %v", tt.v, tt.min, tt.max, got, tt.want)
		}
	}
}

func TestCategoryConfidenceWeight(t *testing.T) {
	tests := []struct {
		name       string
		brierScore float64
		wantMin    float64
		wantMax    float64
	}{
		{"zero brier", 0, 1.0, 1.0},
		{"good brier", 0.15, 0.9, 1.1},
		{"bad brier", 0.5, 0.1, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &CategoryStats{BrierScore: tt.brierScore}
			w := categoryConfidenceWeight(stats)
			if w < tt.wantMin || w > tt.wantMax {
				t.Errorf("weight %v not in [%v, %v]", w, tt.wantMin, tt.wantMax)
			}
		})
	}
}
