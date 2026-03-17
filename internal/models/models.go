// Package models provides unified, platform-agnostic types for positions,
// trades, and portfolio views across Binance and Polymarket.
package models

import (
	"time"

	"github.com/google/uuid"
)

// Platform identifies the trading platform.
type Platform string

const (
	PlatformBinance    Platform = "binance"
	PlatformPolymarket Platform = "polymarket"
)

// PositionStatus represents whether a position is open or closed.
type PositionStatus string

const (
	PositionStatusOpen   PositionStatus = "OPEN"
	PositionStatusClosed PositionStatus = "CLOSED"
)

// UnifiedPosition is a platform-agnostic position representation.
type UnifiedPosition struct {
	ID            uuid.UUID              `json:"id"`
	Platform      Platform               `json:"platform"`
	SessionID     uuid.UUID              `json:"session_id"`
	Symbol        string                 `json:"symbol"`    // "BTC/USDT" for Binance, market question for Polymarket
	MarketID      string                 `json:"market_id"` // exchange symbol or Polymarket condition ID
	Side          string                 `json:"side"`      // "long"/"short" for Binance, "YES"/"NO" for Polymarket
	EntryPrice    float64                `json:"entry_price"`
	CurrentPrice  float64                `json:"current_price"`
	Quantity      float64                `json:"quantity"` // quantity for Binance, shares for Polymarket
	CostBasis     float64                `json:"cost_basis"`
	UnrealizedPnL float64                `json:"unrealized_pnl"`
	RealizedPnL   float64                `json:"realized_pnl"`
	Status        PositionStatus         `json:"status"` // OPEN, CLOSED
	OpenedAt      time.Time              `json:"opened_at"`
	ClosedAt      *time.Time             `json:"closed_at,omitempty"`
	Agent         string                 `json:"agent"`
	Confidence    float64                `json:"confidence"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// IsOpen returns true if the position is currently open.
func (p *UnifiedPosition) IsOpen() bool {
	return p.Status == PositionStatusOpen
}

// UnifiedTrade is a platform-agnostic trade representation.
type UnifiedTrade struct {
	ID         uuid.UUID `json:"id"`
	Platform   Platform  `json:"platform"`
	PositionID uuid.UUID `json:"position_id"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	Action     string    `json:"action"` // "BUY", "SELL"
	Price      float64   `json:"price"`
	Quantity   float64   `json:"quantity"`
	Amount     float64   `json:"amount"` // total value
	Agent      string    `json:"agent"`
	Confidence float64   `json:"confidence"`
	Reasoning  string    `json:"reasoning,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// UnifiedPortfolio aggregates positions across all platforms.
type UnifiedPortfolio struct {
	TotalValue    float64                      `json:"total_value"`
	CashBalance   float64                      `json:"cash_balance"`
	UnrealizedPnL float64                      `json:"unrealized_pnl"`
	RealizedPnL   float64                      `json:"realized_pnl"`
	TotalPnL      float64                      `json:"total_pnl"`
	WinRate       float64                      `json:"win_rate"`
	TotalTrades   int                          `json:"total_trades"`
	OpenPositions int                          `json:"open_positions"`
	Positions     []UnifiedPosition            `json:"positions"`
	ByPlatform    map[Platform]PlatformSummary `json:"by_platform"`
}

// PlatformSummary holds per-platform aggregated metrics.
type PlatformSummary struct {
	Platform      Platform `json:"platform"`
	TotalValue    float64  `json:"total_value"`
	PnL           float64  `json:"pnl"`
	PositionCount int      `json:"position_count"`
	TradeCount    int      `json:"trade_count"`
}
