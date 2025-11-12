package indicators

import (
	"testing"
)

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Fatal("Expected non-nil service")
	}
}

func TestExtractPrices(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]interface{}
		key       string
		want      []float64
		wantError bool
	}{
		{
			name: "Valid float64 prices",
			args: map[string]interface{}{
				"prices": []interface{}{10.0, 20.0, 30.0, 40.0, 50.0},
			},
			key:       "prices",
			want:      []float64{10.0, 20.0, 30.0, 40.0, 50.0},
			wantError: false,
		},
		{
			name: "Valid int prices",
			args: map[string]interface{}{
				"prices": []interface{}{10, 20, 30, 40, 50},
			},
			key:       "prices",
			want:      []float64{10.0, 20.0, 30.0, 40.0, 50.0},
			wantError: false,
		},
		{
			name: "Mixed int and float64 prices",
			args: map[string]interface{}{
				"prices": []interface{}{10, 20.5, 30, 40.5, 50},
			},
			key:       "prices",
			want:      []float64{10.0, 20.5, 30.0, 40.5, 50.0},
			wantError: false,
		},
		{
			name: "Missing prices parameter",
			args: map[string]interface{}{
				"other": "value",
			},
			key:       "prices",
			wantError: true,
		},
		{
			name: "Invalid prices format (not array)",
			args: map[string]interface{}{
				"prices": "not an array",
			},
			key:       "prices",
			wantError: true,
		},
		{
			name: "Invalid price type (string in array)",
			args: map[string]interface{}{
				"prices": []interface{}{10.0, "invalid", 30.0},
			},
			key:       "prices",
			wantError: true,
		},
		{
			name: "Empty prices array",
			args: map[string]interface{}{
				"prices": []interface{}{},
			},
			key:       "prices",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractPrices(tt.args, tt.key)

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

			if len(got) != len(tt.want) {
				t.Errorf("Expected %d prices, got %d", len(tt.want), len(got))
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Price at index %d: expected %.2f, got %.2f", i, tt.want[i], got[i])
				}
			}
		})
	}
}

func TestExtractPeriod(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]interface{}
		key          string
		defaultValue int
		want         int
	}{
		{
			name: "Valid float64 period",
			args: map[string]interface{}{
				"period": 14.0,
			},
			key:          "period",
			defaultValue: 20,
			want:         14,
		},
		{
			name: "Valid int period",
			args: map[string]interface{}{
				"period": 14,
			},
			key:          "period",
			defaultValue: 20,
			want:         14,
		},
		{
			name:         "Missing period (use default)",
			args:         map[string]interface{}{},
			key:          "period",
			defaultValue: 20,
			want:         20,
		},
		{
			name: "Invalid period type (use default)",
			args: map[string]interface{}{
				"period": "invalid",
			},
			key:          "period",
			defaultValue: 20,
			want:         20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPeriod(tt.args, tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("Expected period %d, got %d", tt.want, got)
			}
		})
	}
}

func TestExtractFloat(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]interface{}
		key          string
		defaultValue float64
		want         float64
	}{
		{
			name: "Valid float64 value",
			args: map[string]interface{}{
				"value": 2.5,
			},
			key:          "value",
			defaultValue: 1.0,
			want:         2.5,
		},
		{
			name: "Valid int value",
			args: map[string]interface{}{
				"value": 3,
			},
			key:          "value",
			defaultValue: 1.0,
			want:         3.0,
		},
		{
			name:         "Missing value (use default)",
			args:         map[string]interface{}{},
			key:          "value",
			defaultValue: 1.0,
			want:         1.0,
		},
		{
			name: "Invalid value type (use default)",
			args: map[string]interface{}{
				"value": "invalid",
			},
			key:          "value",
			defaultValue: 1.0,
			want:         1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFloat(tt.args, tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("Expected value %.2f, got %.2f", tt.want, got)
			}
		})
	}
}
