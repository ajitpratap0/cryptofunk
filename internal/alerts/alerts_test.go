package alerts

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockAlerter is a test implementation of Alerter
type MockAlerter struct {
	alerts []Alert
	err    error
}

func NewMockAlerter(err error) *MockAlerter {
	return &MockAlerter{
		alerts: make([]Alert, 0),
		err:    err,
	}
}

func (m *MockAlerter) Send(ctx context.Context, alert Alert) error {
	m.alerts = append(m.alerts, alert)
	return m.err
}

func TestNewManager(t *testing.T) {
	alerter1 := NewMockAlerter(nil)
	alerter2 := NewMockAlerter(nil)

	manager := NewManager(alerter1, alerter2)

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if len(manager.alerters) != 2 {
		t.Errorf("Expected 2 alerters, got %d", len(manager.alerters))
	}
}

func TestManager_Send(t *testing.T) {
	tests := []struct {
		name           string
		alert          Alert
		mockErr        error
		expectErr      bool
		checkTimestamp bool
	}{
		{
			name: "Successful send",
			alert: Alert{
				Title:    "Test Alert",
				Message:  "Test Message",
				Severity: SeverityInfo,
			},
			mockErr:        nil,
			expectErr:      false,
			checkTimestamp: true,
		},
		{
			name: "Send with error",
			alert: Alert{
				Title:    "Test Alert",
				Message:  "Test Message",
				Severity: SeverityWarning,
			},
			mockErr:   errors.New("send error"),
			expectErr: true,
		},
		{
			name: "Send with explicit timestamp",
			alert: Alert{
				Title:     "Test Alert",
				Message:   "Test Message",
				Severity:  SeverityCritical,
				Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			mockErr:        nil,
			expectErr:      false,
			checkTimestamp: false,
		},
		{
			name: "Send with metadata",
			alert: Alert{
				Title:    "Test Alert",
				Message:  "Test Message",
				Severity: SeverityInfo,
				Metadata: map[string]interface{}{
					"key1": "value1",
					"key2": 123,
				},
			},
			mockErr:   nil,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAlerter := NewMockAlerter(tt.mockErr)
			manager := NewManager(mockAlerter)

			err := manager.Send(context.Background(), tt.alert)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if len(mockAlerter.alerts) != 1 {
				t.Fatalf("Expected 1 alert to be sent, got %d", len(mockAlerter.alerts))
			}

			sentAlert := mockAlerter.alerts[0]

			if sentAlert.Title != tt.alert.Title {
				t.Errorf("Expected title %q, got %q", tt.alert.Title, sentAlert.Title)
			}

			if sentAlert.Message != tt.alert.Message {
				t.Errorf("Expected message %q, got %q", tt.alert.Message, sentAlert.Message)
			}

			if sentAlert.Severity != tt.alert.Severity {
				t.Errorf("Expected severity %q, got %q", tt.alert.Severity, sentAlert.Severity)
			}

			if tt.checkTimestamp {
				if sentAlert.Timestamp.IsZero() {
					t.Error("Expected timestamp to be set, got zero value")
				}
			}
		})
	}
}

func TestManager_SendToMultipleAlerters(t *testing.T) {
	alerter1 := NewMockAlerter(nil)
	alerter2 := NewMockAlerter(errors.New("alerter2 error"))
	alerter3 := NewMockAlerter(nil)

	manager := NewManager(alerter1, alerter2, alerter3)

	alert := Alert{
		Title:    "Multi-send Test",
		Message:  "Testing multiple alerters",
		Severity: SeverityWarning,
	}

	err := manager.Send(context.Background(), alert)

	// Should return the last error (from alerter2)
	if err == nil {
		t.Error("Expected error from alerter2, got nil")
	}

	// All alerters should have received the alert
	if len(alerter1.alerts) != 1 {
		t.Errorf("Expected alerter1 to receive 1 alert, got %d", len(alerter1.alerts))
	}
	if len(alerter2.alerts) != 1 {
		t.Errorf("Expected alerter2 to receive 1 alert, got %d", len(alerter2.alerts))
	}
	if len(alerter3.alerts) != 1 {
		t.Errorf("Expected alerter3 to receive 1 alert, got %d", len(alerter3.alerts))
	}
}

func TestManager_SendCritical(t *testing.T) {
	mockAlerter := NewMockAlerter(nil)
	manager := NewManager(mockAlerter)

	err := manager.SendCritical(context.Background(), "Critical Test", "Critical message", map[string]interface{}{
		"test": "value",
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(mockAlerter.alerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(mockAlerter.alerts))
	}

	alert := mockAlerter.alerts[0]
	if alert.Title != "Critical Test" {
		t.Errorf("Expected title 'Critical Test', got %q", alert.Title)
	}
	if alert.Severity != SeverityCritical {
		t.Errorf("Expected severity CRITICAL, got %q", alert.Severity)
	}
	if alert.Metadata["test"] != "value" {
		t.Errorf("Expected metadata test='value', got %v", alert.Metadata["test"])
	}
}

func TestManager_SendWarning(t *testing.T) {
	mockAlerter := NewMockAlerter(nil)
	manager := NewManager(mockAlerter)

	err := manager.SendWarning(context.Background(), "Warning Test", "Warning message", nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(mockAlerter.alerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(mockAlerter.alerts))
	}

	alert := mockAlerter.alerts[0]
	if alert.Severity != SeverityWarning {
		t.Errorf("Expected severity WARNING, got %q", alert.Severity)
	}
}

func TestManager_SendInfo(t *testing.T) {
	mockAlerter := NewMockAlerter(nil)
	manager := NewManager(mockAlerter)

	err := manager.SendInfo(context.Background(), "Info Test", "Info message", nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(mockAlerter.alerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(mockAlerter.alerts))
	}

	alert := mockAlerter.alerts[0]
	if alert.Severity != SeverityInfo {
		t.Errorf("Expected severity INFO, got %q", alert.Severity)
	}
}

func TestLogAlerter_Send(t *testing.T) {
	alerter := NewLogAlerter()

	tests := []struct {
		name     string
		severity Severity
	}{
		{"Critical alert", SeverityCritical},
		{"Warning alert", SeverityWarning},
		{"Info alert", SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert := Alert{
				Title:     "Log Test",
				Message:   "Log test message",
				Severity:  tt.severity,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"test_key": "test_value",
				},
			}

			err := alerter.Send(context.Background(), alert)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestConsoleAlerter_Send(t *testing.T) {
	alerter := NewConsoleAlerter()

	tests := []struct {
		name     string
		severity Severity
	}{
		{"Critical alert to console", SeverityCritical},
		{"Warning alert to console", SeverityWarning},
		{"Info alert to console", SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert := Alert{
				Title:     "Console Test",
				Message:   "Console test message",
				Severity:  tt.severity,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"symbol": "BTCUSDT",
					"price":  50000.0,
				},
			}

			err := alerter.Send(context.Background(), alert)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestConsoleAlerter_SendWithoutMetadata(t *testing.T) {
	alerter := NewConsoleAlerter()

	alert := Alert{
		Title:     "No Metadata Test",
		Message:   "Testing without metadata",
		Severity:  SeverityInfo,
		Timestamp: time.Now(),
		Metadata:  nil,
	}

	err := alerter.Send(context.Background(), alert)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestDefaultManager(t *testing.T) {
	manager := GetDefaultManager()

	if manager == nil {
		t.Fatal("Expected non-nil default manager")
	}

	// Test setting a custom default manager
	mockAlerter := NewMockAlerter(nil)
	customManager := NewManager(mockAlerter)
	SetDefaultManager(customManager)

	retrievedManager := GetDefaultManager()
	if retrievedManager != customManager {
		t.Error("Expected to retrieve the custom manager")
	}

	// Reset to original for other tests
	SetDefaultManager(manager)
}

func TestAlertOrderFailed(t *testing.T) {
	mockAlerter := NewMockAlerter(nil)
	originalManager := GetDefaultManager()
	SetDefaultManager(NewManager(mockAlerter))
	defer SetDefaultManager(originalManager)

	AlertOrderFailed(context.Background(), "BTCUSDT", "BUY", 0.1, errors.New("insufficient funds"))

	if len(mockAlerter.alerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(mockAlerter.alerts))
	}

	alert := mockAlerter.alerts[0]
	if alert.Severity != SeverityCritical {
		t.Errorf("Expected CRITICAL severity, got %q", alert.Severity)
	}
	if alert.Metadata["symbol"] != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %v", alert.Metadata["symbol"])
	}
	if alert.Metadata["side"] != "BUY" {
		t.Errorf("Expected side BUY, got %v", alert.Metadata["side"])
	}
	if alert.Metadata["quantity"] != 0.1 {
		t.Errorf("Expected quantity 0.1, got %v", alert.Metadata["quantity"])
	}
}

func TestAlertOrderCancelFailed(t *testing.T) {
	mockAlerter := NewMockAlerter(nil)
	originalManager := GetDefaultManager()
	SetDefaultManager(NewManager(mockAlerter))
	defer SetDefaultManager(originalManager)

	AlertOrderCancelFailed(context.Background(), "order123", "ETHUSDT", errors.New("order not found"))

	if len(mockAlerter.alerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(mockAlerter.alerts))
	}

	alert := mockAlerter.alerts[0]
	if alert.Severity != SeverityWarning {
		t.Errorf("Expected WARNING severity, got %q", alert.Severity)
	}
	if alert.Metadata["order_id"] != "order123" {
		t.Errorf("Expected order_id order123, got %v", alert.Metadata["order_id"])
	}
}

func TestAlertConnectionError(t *testing.T) {
	mockAlerter := NewMockAlerter(nil)
	originalManager := GetDefaultManager()
	SetDefaultManager(NewManager(mockAlerter))
	defer SetDefaultManager(originalManager)

	AlertConnectionError(context.Background(), "Binance", errors.New("connection timeout"))

	if len(mockAlerter.alerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(mockAlerter.alerts))
	}

	alert := mockAlerter.alerts[0]
	if alert.Severity != SeverityCritical {
		t.Errorf("Expected CRITICAL severity, got %q", alert.Severity)
	}
	if alert.Metadata["exchange"] != "Binance" {
		t.Errorf("Expected exchange Binance, got %v", alert.Metadata["exchange"])
	}
}

func TestAlertPositionRisk(t *testing.T) {
	mockAlerter := NewMockAlerter(nil)
	originalManager := GetDefaultManager()
	SetDefaultManager(NewManager(mockAlerter))
	defer SetDefaultManager(originalManager)

	AlertPositionRisk(context.Background(), "BTCUSDT", 0.95, "Exceeds max risk threshold")

	if len(mockAlerter.alerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(mockAlerter.alerts))
	}

	alert := mockAlerter.alerts[0]
	if alert.Severity != SeverityCritical {
		t.Errorf("Expected CRITICAL severity, got %q", alert.Severity)
	}
	if alert.Metadata["symbol"] != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %v", alert.Metadata["symbol"])
	}
	if alert.Metadata["risk_level"] != 0.95 {
		t.Errorf("Expected risk_level 0.95, got %v", alert.Metadata["risk_level"])
	}
}

func TestAlertSystemError(t *testing.T) {
	mockAlerter := NewMockAlerter(nil)
	originalManager := GetDefaultManager()
	SetDefaultManager(NewManager(mockAlerter))
	defer SetDefaultManager(originalManager)

	AlertSystemError(context.Background(), "OrderExecutor", errors.New("database connection lost"))

	if len(mockAlerter.alerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(mockAlerter.alerts))
	}

	alert := mockAlerter.alerts[0]
	if alert.Severity != SeverityCritical {
		t.Errorf("Expected CRITICAL severity, got %q", alert.Severity)
	}
	if alert.Metadata["component"] != "OrderExecutor" {
		t.Errorf("Expected component OrderExecutor, got %v", alert.Metadata["component"])
	}
}

func TestSeverityConstants(t *testing.T) {
	if SeverityInfo != "INFO" {
		t.Errorf("Expected SeverityInfo to be 'INFO', got %q", SeverityInfo)
	}
	if SeverityWarning != "WARNING" {
		t.Errorf("Expected SeverityWarning to be 'WARNING', got %q", SeverityWarning)
	}
	if SeverityCritical != "CRITICAL" {
		t.Errorf("Expected SeverityCritical to be 'CRITICAL', got %q", SeverityCritical)
	}
}
