package models

import (
	"strings"

	"github.com/google/uuid"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// BinancePositionToUnified converts a Binance DB position to the unified model.
func BinancePositionToUnified(p *db.Position) UnifiedPosition {
	u := UnifiedPosition{
		ID:         p.ID,
		Platform:   PlatformBinance,
		Symbol:     p.Symbol,
		MarketID:   p.Symbol,
		Side:       strings.ToLower(string(p.Side)),
		EntryPrice: p.EntryPrice,
		Quantity:   p.Quantity,
		CostBasis:  p.EntryPrice * p.Quantity,
		OpenedAt:   p.EntryTime,
		ClosedAt:   p.ExitTime,
	}

	if p.SessionID != nil {
		u.SessionID = *p.SessionID
	}

	if p.UnrealizedPnL != nil {
		u.UnrealizedPnL = *p.UnrealizedPnL
	}
	if p.RealizedPnL != nil {
		u.RealizedPnL = *p.RealizedPnL
	}

	if p.ExitTime != nil {
		u.Status = PositionStatusClosed
		if p.ExitPrice != nil {
			u.CurrentPrice = *p.ExitPrice
		}
	} else {
		u.Status = PositionStatusOpen
		u.CurrentPrice = p.EntryPrice // best we have without live price
	}

	if p.EntryReason != nil {
		u.Agent = *p.EntryReason // closest field we have
	}

	// Copy metadata if it's a map
	if m, ok := p.Metadata.(map[string]interface{}); ok {
		u.Metadata = m
	}

	return u
}

// PolymarketPositionToUnified converts a Polymarket DB position to the unified model.
// If market is non-nil, it enriches the position with market question and current price.
func PolymarketPositionToUnified(p *db.PolymarketPosition, market *db.PolymarketMarket) UnifiedPosition {
	u := UnifiedPosition{
		ID:         p.ID,
		Platform:   PlatformPolymarket,
		SessionID:  p.SessionID,
		MarketID:   p.MarketID,
		Side:       p.Side,
		EntryPrice: p.AvgPrice,
		Quantity:   p.Shares,
		CostBasis:  p.CostBasis,
		Status:     PositionStatus(p.Status),
		OpenedAt:   p.OpenedAt,
		ClosedAt:   p.ClosedAt,
	}

	if p.RealizedPnl != nil {
		u.RealizedPnL = *p.RealizedPnl
	}

	// Enrich from market data
	if market != nil {
		u.Symbol = market.Question
		if strings.EqualFold(p.Side, "YES") && market.YesPrice != nil {
			u.CurrentPrice = *market.YesPrice
		} else if market.NoPrice != nil {
			u.CurrentPrice = *market.NoPrice
		}
		u.UnrealizedPnL = (u.CurrentPrice - u.EntryPrice) * u.Quantity
	} else {
		u.Symbol = p.MarketID
	}

	// Copy metadata if it's a map
	if m, ok := p.Metadata.(map[string]interface{}); ok {
		u.Metadata = m
	}

	return u
}

// BinanceTradeToUnified converts a Binance DB trade + order to the unified model.
func BinanceTradeToUnified(t *db.Trade, order *db.Order) UnifiedTrade {
	u := UnifiedTrade{
		ID:        t.ID,
		Platform:  PlatformBinance,
		Symbol:    t.Symbol,
		Side:      t.Side.ToDBSide(),
		Price:     t.Price,
		Quantity:  t.Quantity,
		Amount:    t.QuoteQuantity,
		Timestamp: t.ExecutedAt,
	}

	// Determine action from side
	if strings.EqualFold(string(t.Side), "buy") {
		u.Action = "BUY"
	} else {
		u.Action = "SELL"
	}

	if order != nil {
		if order.PositionID != nil {
			u.PositionID = *order.PositionID
		}
	}

	return u
}

// PolymarketTradeToUnified converts a Polymarket DB trade to the unified model.
func PolymarketTradeToUnified(t *db.PolymarketTrade) UnifiedTrade {
	return UnifiedTrade{
		ID:         t.ID,
		Platform:   PlatformPolymarket,
		PositionID: t.PositionID,
		Symbol:     t.MarketID,
		Side:       t.Side,
		Action:     t.Action,
		Price:      t.Price,
		Quantity:   t.Shares,
		Amount:     t.Amount,
		Timestamp:  t.Timestamp,
	}
}

// NewEmptyPortfolio creates an empty UnifiedPortfolio with initialized maps.
func NewEmptyPortfolio() *UnifiedPortfolio {
	return &UnifiedPortfolio{
		Positions: []UnifiedPosition{},
		ByPlatform: map[Platform]PlatformSummary{
			PlatformBinance:    {Platform: PlatformBinance},
			PlatformPolymarket: {Platform: PlatformPolymarket},
		},
	}
}

// AddPosition adds a position to the portfolio and updates aggregates.
func (p *UnifiedPortfolio) AddPosition(pos UnifiedPosition) {
	p.Positions = append(p.Positions, pos)

	if pos.Status == PositionStatusOpen {
		p.OpenPositions++
		p.UnrealizedPnL += pos.UnrealizedPnL
	}
	p.RealizedPnL += pos.RealizedPnL
	p.TotalPnL = p.UnrealizedPnL + p.RealizedPnL

	summary := p.ByPlatform[pos.Platform]
	summary.PositionCount++
	summary.PnL += pos.UnrealizedPnL + pos.RealizedPnL
	summary.TotalValue += pos.CurrentPrice * pos.Quantity
	p.ByPlatform[pos.Platform] = summary

	p.TotalValue += pos.CurrentPrice * pos.Quantity
}

// SetPlatformTradeCount sets the trade count for a platform.
func (p *UnifiedPortfolio) SetPlatformTradeCount(platform Platform, count int) {
	summary := p.ByPlatform[platform]
	summary.TradeCount = count
	p.ByPlatform[platform] = summary
	// Recalculate total trades
	p.TotalTrades = 0
	for _, s := range p.ByPlatform {
		p.TotalTrades += s.TradeCount
	}
}

// Ignore returns a dummy reference for unused imports.
func init() {
	// Ensure uuid import is used
	_ = uuid.Nil
}
