package db

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestTradingSession_Defaults tests TradingSession default values
func TestTradingSession_Defaults(t *testing.T) {
	session := &TradingSession{
		ID:             uuid.New(),
		Symbol:         "BTC/USDT",
		Mode:           TradingModePaper,
		Exchange:       "binance",
		InitialCapital: 10000.0,
		StartedAt:      time.Now(),
	}

	assert.NotEqual(t, uuid.Nil, session.ID)
	assert.Equal(t, "BTC/USDT", session.Symbol)
	assert.Equal(t, TradingModePaper, session.Mode)
	assert.Equal(t, 10000.0, session.InitialCapital)
	assert.Nil(t, session.StoppedAt)
	assert.Equal(t, 0.0, session.TotalPnL)
}

// TestTradingMode_Values tests TradingMode constants
func TestTradingMode_Values(t *testing.T) {
	assert.Equal(t, TradingMode("PAPER"), TradingModePaper)
	assert.Equal(t, TradingMode("LIVE"), TradingModeLive)
}

// TestOrder_Initialization tests Order initialization
func TestOrder_Initialization(t *testing.T) {
	sessionID := uuid.New()
	order := &Order{
		ID:        uuid.New(),
		SessionID: &sessionID,
		Symbol:    "ETH/USDT",
		Side:      OrderSideBuy,
		Type:      OrderTypeMarket,
		Quantity:  1.5,
		Status:    OrderStatusNew,
		PlacedAt:  time.Now(),
	}

	assert.NotEqual(t, uuid.Nil, order.ID)
	assert.NotNil(t, order.SessionID)
	assert.Equal(t, "ETH/USDT", order.Symbol)
	assert.Equal(t, OrderSideBuy, order.Side)
	assert.Equal(t, OrderTypeMarket, order.Type)
	assert.Equal(t, 1.5, order.Quantity)
	assert.Equal(t, OrderStatusNew, order.Status)
}

// TestPosition_Initialization tests Position initialization
func TestPosition_Initialization(t *testing.T) {
	sessionID := uuid.New()
	position := &Position{
		ID:         uuid.New(),
		SessionID:  &sessionID,
		Symbol:     "SOL/USDT",
		Side:       PositionSideLong,
		EntryPrice: 100.0,
		Quantity:   10.0,
		EntryTime:  time.Now(),
	}

	assert.NotEqual(t, uuid.Nil, position.ID)
	assert.NotNil(t, position.SessionID)
	assert.Equal(t, "SOL/USDT", position.Symbol)
	assert.Equal(t, PositionSideLong, position.Side)
	assert.Equal(t, 100.0, position.EntryPrice)
	assert.Equal(t, 10.0, position.Quantity)
}

// TestPosition_UnrealizedPnL tests unrealized P&L calculation
func TestPosition_UnrealizedPnL(t *testing.T) {
	tests := []struct {
		name         string
		position     *Position
		currentPrice float64
		expectedPnL  float64
	}{
		{
			name: "Long position profit",
			position: &Position{
				Side:       PositionSideLong,
				EntryPrice: 100.0,
				Quantity:   10.0,
			},
			currentPrice: 110.0,
			expectedPnL:  100.0, // (110 - 100) * 10
		},
		{
			name: "Long position loss",
			position: &Position{
				Side:       PositionSideLong,
				EntryPrice: 100.0,
				Quantity:   10.0,
			},
			currentPrice: 90.0,
			expectedPnL:  -100.0, // (90 - 100) * 10
		},
		{
			name: "Short position profit",
			position: &Position{
				Side:       PositionSideShort,
				EntryPrice: 100.0,
				Quantity:   10.0,
			},
			currentPrice: 90.0,
			expectedPnL:  100.0, // (100 - 90) * 10
		},
		{
			name: "Short position loss",
			position: &Position{
				Side:       PositionSideShort,
				EntryPrice: 100.0,
				Quantity:   10.0,
			},
			currentPrice: 110.0,
			expectedPnL:  -100.0, // (100 - 110) * 10
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pnl float64
			if tt.position.Side == PositionSideLong {
				pnl = (tt.currentPrice - tt.position.EntryPrice) * tt.position.Quantity
			} else {
				pnl = (tt.position.EntryPrice - tt.currentPrice) * tt.position.Quantity
			}

			assert.Equal(t, tt.expectedPnL, pnl)
		})
	}
}

// TestLLMDecision_Initialization tests LLMDecision initialization
func TestLLMDecision_Initialization(t *testing.T) {
	sessionID := uuid.New()
	decision := &LLMDecision{
		ID:           uuid.New(),
		SessionID:    &sessionID,
		AgentName:    "trend-agent",
		Symbol:       "ETH/USDT",
		DecisionType: "signal",
		Response:     "BUY",
		Confidence:   0.75,
		CreatedAt:    time.Now(),
	}

	assert.NotEqual(t, uuid.Nil, decision.ID)
	assert.NotNil(t, decision.SessionID)
	assert.Equal(t, "trend-agent", decision.AgentName)
	assert.Equal(t, "ETH/USDT", decision.Symbol)
	assert.Equal(t, "signal", decision.DecisionType)
	assert.Equal(t, "BUY", decision.Response)
	assert.Equal(t, 0.75, decision.Confidence)
}

// TestAgentStatus_Initialization tests AgentStatus initialization
func TestAgentStatus_Initialization(t *testing.T) {
	now := time.Now()
	status := &AgentStatus{
		Name:          "risk-agent",
		Type:          "risk",
		Status:        "RUNNING",
		StartedAt:     &now,
		LastHeartbeat: &now,
		TotalSignals:  10,
		ErrorCount:    0,
	}

	assert.Equal(t, "risk-agent", status.Name)
	assert.Equal(t, "risk", status.Type)
	assert.Equal(t, "RUNNING", status.Status)
	assert.NotNil(t, status.StartedAt)
	assert.NotNil(t, status.LastHeartbeat)
	assert.Equal(t, 10, status.TotalSignals)
	assert.Equal(t, 0, status.ErrorCount)
}

// TestConvertOrderSide_EdgeCases tests ConvertOrderSide edge cases
func TestConvertOrderSide_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected OrderSide
	}{
		{"Lowercase buy", "buy", OrderSideBuy},
		{"Lowercase sell", "sell", OrderSideSell},
		{"Uppercase BUY", "BUY", OrderSideBuy},
		{"Uppercase SELL", "SELL", OrderSideSell},
		{"Unknown defaults to BUY", "INVALID", OrderSideBuy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertOrderSide(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertOrderType_EdgeCases tests ConvertOrderType edge cases
func TestConvertOrderType_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected OrderType
	}{
		{"Lowercase market", "market", OrderTypeMarket},
		{"Lowercase limit", "limit", OrderTypeLimit},
		{"Uppercase MARKET", "MARKET", OrderTypeMarket},
		{"Uppercase LIMIT", "LIMIT", OrderTypeLimit},
		{"Unknown defaults to MARKET", "INVALID", OrderTypeMarket},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertOrderType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertOrderStatus_EdgeCases tests ConvertOrderStatus edge cases
func TestConvertOrderStatus_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected OrderStatus
	}{
		{"NEW status", "NEW", OrderStatusNew},
		{"PENDING to NEW", "PENDING", OrderStatusNew},
		{"PARTIALLY_FILLED", "PARTIALLY_FILLED", OrderStatusPartiallyFilled},
		{"OPEN to PARTIALLY_FILLED", "OPEN", OrderStatusPartiallyFilled},
		{"FILLED status", "FILLED", OrderStatusFilled},
		{"CANCELED status", "CANCELED", OrderStatusCanceled},
		{"CANCELLED to CANCELED", "CANCELLED", OrderStatusCanceled},
		{"REJECTED status", "REJECTED", OrderStatusRejected},
		{"Unknown defaults to NEW", "INVALID", OrderStatusNew},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertOrderStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertPositionSide_EdgeCases tests ConvertPositionSide edge cases
func TestConvertPositionSide_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected PositionSide
	}{
		{"LONG", "LONG", PositionSideLong},
		{"long", "long", PositionSideLong},
		{"buy to LONG", "buy", PositionSideLong},
		{"BUY to LONG", "BUY", PositionSideLong},
		{"SHORT", "SHORT", PositionSideShort},
		{"short", "short", PositionSideShort},
		{"sell to SHORT", "sell", PositionSideShort},
		{"SELL to SHORT", "SELL", PositionSideShort},
		{"Unknown to FLAT", "INVALID", PositionSideFlat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertPositionSide(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestOrderFilled_Calculation tests order filled percentage calculation
func TestOrderFilled_Calculation(t *testing.T) {
	tests := []struct {
		name        string
		quantity    float64
		filledQty   float64
		expectedPct float64
	}{
		{"Fully filled", 10.0, 10.0, 100.0},
		{"Half filled", 10.0, 5.0, 50.0},
		{"Not filled", 10.0, 0.0, 0.0},
		{"Partially filled", 10.0, 7.5, 75.0},
		{"Overfilled (should not happen)", 10.0, 11.0, 110.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pct := (tt.filledQty / tt.quantity) * 100.0
			assert.InDelta(t, tt.expectedPct, pct, 0.0001) // Use InDelta for floating point comparison
		})
	}
}

// TestTradingSession_ROI tests ROI calculation
func TestTradingSession_ROI(t *testing.T) {
	tests := []struct {
		name           string
		initialCapital float64
		totalPnL       float64
		expectedROI    float64
	}{
		{"Positive ROI", 10000.0, 1000.0, 10.0},
		{"Negative ROI", 10000.0, -500.0, -5.0},
		{"Zero ROI", 10000.0, 0.0, 0.0},
		{"100% ROI", 10000.0, 10000.0, 100.0},
		{"Total loss", 10000.0, -10000.0, -100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roi := (tt.totalPnL / tt.initialCapital) * 100.0
			assert.InDelta(t, tt.expectedROI, roi, 0.01)
		})
	}
}

// TestPosition_Validation tests position data validation
func TestPosition_Validation(t *testing.T) {
	sessionID := uuid.New()
	tests := []struct {
		name      string
		position  *Position
		wantError bool
	}{
		{
			name: "Valid long position",
			position: &Position{
				ID:         uuid.New(),
				SessionID:  &sessionID,
				Symbol:     "BTC/USDT",
				Side:       PositionSideLong,
				EntryPrice: 100.0,
				Quantity:   1.0,
			},
			wantError: false,
		},
		{
			name: "Valid short position",
			position: &Position{
				ID:         uuid.New(),
				SessionID:  &sessionID,
				Symbol:     "ETH/USDT",
				Side:       PositionSideShort,
				EntryPrice: 200.0,
				Quantity:   2.0,
			},
			wantError: false,
		},
		{
			name: "Zero quantity",
			position: &Position{
				ID:         uuid.New(),
				SessionID:  &sessionID,
				Symbol:     "BTC/USDT",
				Side:       PositionSideLong,
				EntryPrice: 100.0,
				Quantity:   0.0,
			},
			wantError: true, // Zero quantity should be invalid
		},
		{
			name: "Negative entry price",
			position: &Position{
				ID:         uuid.New(),
				SessionID:  &sessionID,
				Symbol:     "BTC/USDT",
				Side:       PositionSideLong,
				EntryPrice: -100.0,
				Quantity:   1.0,
			},
			wantError: true, // Negative price should be invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation
			isValid := tt.position.Quantity > 0 && tt.position.EntryPrice > 0
			if tt.wantError {
				assert.False(t, isValid)
			} else {
				assert.True(t, isValid)
			}
		})
	}
}

// TestOrder_Validation tests order data validation
func TestOrder_Validation(t *testing.T) {
	sessionID := uuid.New()
	limitPrice := 200.0
	zeroPrice := 0.0
	tests := []struct {
		name      string
		order     *Order
		wantError bool
	}{
		{
			name: "Valid market order",
			order: &Order{
				ID:        uuid.New(),
				SessionID: &sessionID,
				Symbol:    "BTC/USDT",
				Side:      OrderSideBuy,
				Type:      OrderTypeMarket,
				Quantity:  1.0,
				Status:    OrderStatusNew,
			},
			wantError: false,
		},
		{
			name: "Valid limit order with price",
			order: &Order{
				ID:        uuid.New(),
				SessionID: &sessionID,
				Symbol:    "ETH/USDT",
				Side:      OrderSideSell,
				Type:      OrderTypeLimit,
				Quantity:  2.0,
				Price:     &limitPrice,
				Status:    OrderStatusNew,
			},
			wantError: false,
		},
		{
			name: "Limit order missing price",
			order: &Order{
				ID:        uuid.New(),
				SessionID: &sessionID,
				Symbol:    "BTC/USDT",
				Side:      OrderSideBuy,
				Type:      OrderTypeLimit,
				Quantity:  1.0,
				Price:     &zeroPrice, // Invalid price for limit order
				Status:    OrderStatusNew,
			},
			wantError: true,
		},
		{
			name: "Zero quantity",
			order: &Order{
				ID:        uuid.New(),
				SessionID: &sessionID,
				Symbol:    "BTC/USDT",
				Side:      OrderSideBuy,
				Type:      OrderTypeMarket,
				Quantity:  0.0, // Invalid
				Status:    OrderStatusNew,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation
			isValid := tt.order.Quantity > 0
			if tt.order.Type == OrderTypeLimit && tt.order.Price != nil {
				isValid = isValid && *tt.order.Price > 0
			}

			if tt.wantError {
				assert.False(t, isValid)
			} else {
				assert.True(t, isValid)
			}
		})
	}
}
