package main

import (
	"time"

	"github.com/google/uuid"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// WebSocket broadcast helpers

// BroadcastPositionUpdate broadcasts a position update to all WebSocket clients
func (s *APIServer) BroadcastPositionUpdate(position *db.Position) error {
	data := map[string]interface{}{
		"position_id":    position.ID.String(),
		"session_id":     position.SessionID,
		"symbol":         position.Symbol,
		"exchange":       position.Exchange,
		"side":           position.Side,
		"entry_price":    position.EntryPrice,
		"exit_price":     position.ExitPrice,
		"quantity":       position.Quantity,
		"entry_time":     position.EntryTime,
		"exit_time":      position.ExitTime,
		"stop_loss":      position.StopLoss,
		"take_profit":    position.TakeProfit,
		"realized_pnl":   position.RealizedPnL,
		"unrealized_pnl": position.UnrealizedPnL,
		"fees":           position.Fees,
		"entry_reason":   position.EntryReason,
		"exit_reason":    position.ExitReason,
	}

	return s.hub.Broadcast(MessageTypePositionUpdate, data)
}

// BroadcastPnLUpdate broadcasts a P&L update to all WebSocket clients
func (s *APIServer) BroadcastPnLUpdate(sessionID uuid.UUID, totalPnL, realizedPnL, unrealizedPnL float64, positions []*db.Position) error {
	data := map[string]interface{}{
		"session_id":     sessionID.String(),
		"total_pnl":      totalPnL,
		"realized_pnl":   realizedPnL,
		"unrealized_pnl": unrealizedPnL,
		"position_count": len(positions),
		"timestamp":      time.Now(),
	}

	return s.hub.Broadcast(MessageTypePositionUpdate, data)
}

// BroadcastTradeNotification broadcasts a trade (fill) notification
func (s *APIServer) BroadcastTradeNotification(trade *db.Trade) error {
	data := map[string]interface{}{
		"trade_id":          trade.ID.String(),
		"order_id":          trade.OrderID.String(),
		"exchange_trade_id": trade.ExchangeTradeID,
		"symbol":            trade.Symbol,
		"exchange":          trade.Exchange,
		"side":              trade.Side,
		"price":             trade.Price,
		"quantity":          trade.Quantity,
		"quote_quantity":    trade.QuoteQuantity,
		"commission":        trade.Commission,
		"commission_asset":  trade.CommissionAsset,
		"executed_at":       trade.ExecutedAt,
		"is_maker":          trade.IsMaker,
	}

	return s.hub.Broadcast(MessageTypeTrade, data)
}

// BroadcastOrderUpdate broadcasts an order status update.
// Only client-safe fields are included — session_id, position_id, and
// exchange_order_id are intentionally omitted to avoid leaking internal IDs.
func (s *APIServer) BroadcastOrderUpdate(order *db.Order) error {
	data := map[string]interface{}{
		"order_id":                order.ID.String(),
		"symbol":                  order.Symbol,
		"exchange":                order.Exchange,
		"side":                    order.Side,
		"type":                    order.Type,
		"status":                  order.Status,
		"price":                   order.Price,
		"stop_price":              order.StopPrice,
		"quantity":                order.Quantity,
		"executed_quantity":       order.ExecutedQuantity,
		"executed_quote_quantity": order.ExecutedQuoteQuantity,
		"time_in_force":           order.TimeInForce,
		"placed_at":               order.PlacedAt,
		"filled_at":               order.FilledAt,
		"canceled_at":             order.CanceledAt,
	}

	return s.hub.Broadcast(MessageTypeOrderUpdate, data)
}

// BroadcastAgentStatus broadcasts agent status change
func (s *APIServer) BroadcastAgentStatus(agent *db.AgentStatus) error {
	data := map[string]interface{}{
		"name":           agent.Name,
		"type":           agent.Type,
		"status":         agent.Status,
		"last_heartbeat": agent.LastHeartbeat,
		"started_at":     agent.StartedAt,
		"total_signals":  agent.TotalSignals,
		"error_count":    agent.ErrorCount,
		"metadata":       agent.Metadata,
	}

	return s.hub.Broadcast(MessageTypeAgentStatus, data)
}

// BroadcastSystemStatus broadcasts system status update
func (s *APIServer) BroadcastSystemStatus(status string, message string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"status":    status,
		"message":   message,
		"metadata":  metadata,
		"timestamp": time.Now(),
	}

	return s.hub.Broadcast(MessageTypeSystemStatus, data)
}

// BroadcastDecision broadcasts a new LLM decision to all WebSocket clients
func (s *APIServer) BroadcastDecision(decision *db.LLMDecision) error {
	data := map[string]interface{}{
		"id":            decision.ID.String(),
		"session_id":    decision.SessionID,
		"decision_type": decision.DecisionType,
		"symbol":        decision.Symbol,
		"agent_name":    decision.AgentName,
		"model":         decision.Model,
		"confidence":    decision.Confidence,
		"outcome":       decision.Outcome,
		"pnl":           decision.PnL,
		"tokens_used":   decision.TokensUsed,
		"latency_ms":    decision.LatencyMs,
		"created_at":    decision.CreatedAt,
		// Truncate prompt/response for real-time updates (full details via API)
		"prompt_preview":   truncateString(decision.Prompt, 200),
		"response_preview": truncateString(decision.Response, 200),
	}

	return s.hub.Broadcast(MessageTypeDecision, data)
}

// BroadcastDecisionStats broadcasts aggregated decision statistics.
// TODO: Integrate with periodic stats updates or decision outcome events.
func (s *APIServer) BroadcastDecisionStats(stats map[string]interface{}) error {
	data := map[string]interface{}{
		"stats":     stats,
		"timestamp": time.Now(),
	}

	return s.hub.Broadcast(MessageTypeDecisionStats, data)
}

// BroadcastPolymarketPositionUpdate broadcasts a Polymarket paper position update
func (s *APIServer) BroadcastPolymarketPositionUpdate(position *db.PolymarketPosition) error {
	data := map[string]interface{}{
		"id":           position.ID.String(),
		"session_id":   position.SessionID.String(),
		"market_id":    position.MarketID,
		"side":         position.Side,
		"shares":       position.Shares,
		"avg_price":    position.AvgPrice,
		"cost_basis":   position.CostBasis,
		"status":       position.Status,
		"opened_at":    position.OpenedAt,
		"closed_at":    position.ClosedAt,
		"realized_pnl": position.RealizedPnl,
		"timestamp":    time.Now(),
	}
	return s.hub.Broadcast(MessageTypePolymarketPosition, data)
}

// BroadcastPolymarketMarketUpdate broadcasts a Polymarket market price update
func (s *APIServer) BroadcastPolymarketMarketUpdate(market *db.PolymarketMarket) error {
	data := map[string]interface{}{
		"id":         market.ID,
		"question":   market.Question,
		"category":   market.Category,
		"yes_price":  market.YesPrice,
		"no_price":   market.NoPrice,
		"volume":     market.Volume,
		"active":     market.Active,
		"updated_at": market.UpdatedAt,
		"timestamp":  time.Now(),
	}
	return s.hub.Broadcast(MessageTypePolymarketMarket, data)
}
