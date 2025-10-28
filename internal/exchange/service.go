package exchange

import (
	"context"
	"fmt"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/rs/zerolog/log"
)

// Service provides order execution functionality
type Service struct {
	exchange *MockExchange
	db       *db.DB
}

// NewService creates a new exchange service
func NewService(database *db.DB) *Service {
	log.Info().Msg("Exchange service initialized")

	return &Service{
		exchange: NewMockExchange(database),
		db:       database,
	}
}

// PlaceMarketOrder places a market order
func (s *Service) PlaceMarketOrder(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("PlaceMarketOrder called")

	// Extract symbol
	symbol, ok := args["symbol"].(string)
	if !ok || symbol == "" {
		return nil, fmt.Errorf("symbol is required and must be a string")
	}

	// Extract side
	sideStr, ok := args["side"].(string)
	if !ok || sideStr == "" {
		return nil, fmt.Errorf("side is required and must be a string")
	}
	side := OrderSide(sideStr)
	if side != OrderSideBuy && side != OrderSideSell {
		return nil, fmt.Errorf("side must be 'buy' or 'sell'")
	}

	// Extract quantity
	quantity, err := extractFloat(args, "quantity")
	if err != nil {
		return nil, fmt.Errorf("quantity error: %w", err)
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}

	// Create request
	req := PlaceOrderRequest{
		Symbol:   symbol,
		Side:     side,
		Type:     OrderTypeMarket,
		Quantity: quantity,
	}

	// Place order
	resp, err := s.exchange.PlaceOrder(req)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	// Get order details
	order, err := s.exchange.GetOrder(resp.OrderID)
	if err != nil {
		log.Error().Err(err).Str("order_id", resp.OrderID).Msg("Failed to retrieve order after placement")
		return resp, nil // Still return the response even if we can't get details
	}

	return order, nil
}

// PlaceLimitOrder places a limit order
func (s *Service) PlaceLimitOrder(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("PlaceLimitOrder called")

	// Extract symbol
	symbol, ok := args["symbol"].(string)
	if !ok || symbol == "" {
		return nil, fmt.Errorf("symbol is required and must be a string")
	}

	// Extract side
	sideStr, ok := args["side"].(string)
	if !ok || sideStr == "" {
		return nil, fmt.Errorf("side is required and must be a string")
	}
	side := OrderSide(sideStr)
	if side != OrderSideBuy && side != OrderSideSell {
		return nil, fmt.Errorf("side must be 'buy' or 'sell'")
	}

	// Extract quantity
	quantity, err := extractFloat(args, "quantity")
	if err != nil {
		return nil, fmt.Errorf("quantity error: %w", err)
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}

	// Extract price
	price, err := extractFloat(args, "price")
	if err != nil {
		return nil, fmt.Errorf("price error: %w", err)
	}
	if price <= 0 {
		return nil, fmt.Errorf("price must be positive")
	}

	// Create request
	req := PlaceOrderRequest{
		Symbol:   symbol,
		Side:     side,
		Type:     OrderTypeLimit,
		Quantity: quantity,
		Price:    price,
	}

	// Place order
	resp, err := s.exchange.PlaceOrder(req)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	// Get order details
	order, err := s.exchange.GetOrder(resp.OrderID)
	if err != nil {
		log.Error().Err(err).Str("order_id", resp.OrderID).Msg("Failed to retrieve order after placement")
		return resp, nil
	}

	return order, nil
}

// CancelOrder cancels an existing order
func (s *Service) CancelOrder(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("CancelOrder called")

	// Extract order_id
	orderID, ok := args["order_id"].(string)
	if !ok || orderID == "" {
		return nil, fmt.Errorf("order_id is required and must be a string")
	}

	// Cancel order
	order, err := s.exchange.CancelOrder(orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel order: %w", err)
	}

	return order, nil
}

// GetOrderStatus retrieves order status
func (s *Service) GetOrderStatus(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("GetOrderStatus called")

	// Extract order_id
	orderID, ok := args["order_id"].(string)
	if !ok || orderID == "" {
		return nil, fmt.Errorf("order_id is required and must be a string")
	}

	// Get order
	order, err := s.exchange.GetOrder(orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Get fills
	fills, err := s.exchange.GetOrderFills(orderID)
	if err != nil {
		log.Error().Err(err).Str("order_id", orderID).Msg("Failed to get order fills")
		// Continue even if we can't get fills
	}

	return map[string]interface{}{
		"order": order,
		"fills": fills,
	}, nil
}

// StartSession starts a new trading session
func (s *Service) StartSession(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("StartSession called")

	// Extract symbol
	symbol, ok := args["symbol"].(string)
	if !ok || symbol == "" {
		return nil, fmt.Errorf("symbol is required and must be a string")
	}

	// Extract initial capital
	initialCapital, err := extractFloat(args, "initial_capital")
	if err != nil {
		return nil, fmt.Errorf("initial_capital error: %w", err)
	}
	if initialCapital <= 0 {
		return nil, fmt.Errorf("initial_capital must be positive")
	}

	// Extract optional config
	var config map[string]interface{}
	if configArg, ok := args["config"].(map[string]interface{}); ok {
		config = configArg
	}

	// Create session in database
	session := &db.TradingSession{
		Mode:           db.TradingModePaper,
		Symbol:         symbol,
		Exchange:       "PAPER",
		StartedAt:      time.Now(),
		InitialCapital: initialCapital,
		Config:         config,
	}

	ctx := context.Background()
	if err := s.db.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Set session in exchange
	s.exchange.SetSession(&session.ID)

	log.Info().
		Str("session_id", session.ID.String()).
		Str("symbol", symbol).
		Float64("initial_capital", initialCapital).
		Msg("Trading session started")

	return map[string]interface{}{
		"session_id":      session.ID.String(),
		"symbol":          session.Symbol,
		"exchange":        session.Exchange,
		"mode":            string(session.Mode),
		"initial_capital": session.InitialCapital,
		"started_at":      session.StartedAt,
	}, nil
}

// StopSession stops the current trading session
func (s *Service) StopSession(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("StopSession called")

	// Get current session from exchange
	sessionID := s.exchange.GetSession()
	if sessionID == nil {
		return nil, fmt.Errorf("no active trading session")
	}

	// Extract final capital
	finalCapital, err := extractFloat(args, "final_capital")
	if err != nil {
		return nil, fmt.Errorf("final_capital error: %w", err)
	}
	if finalCapital < 0 {
		return nil, fmt.Errorf("final_capital cannot be negative")
	}

	// Stop session in database
	ctx := context.Background()
	if err := s.db.StopSession(ctx, *sessionID, finalCapital); err != nil {
		return nil, fmt.Errorf("failed to stop session: %w", err)
	}

	// Clear session from exchange
	s.exchange.SetSession(nil)

	// Get final session data
	session, err := s.db.GetSession(ctx, *sessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID.String()).Msg("Failed to retrieve session after stopping")
		return map[string]interface{}{
			"session_id":    sessionID.String(),
			"final_capital": finalCapital,
			"stopped":       true,
		}, nil
	}

	log.Info().
		Str("session_id", sessionID.String()).
		Float64("final_capital", finalCapital).
		Float64("total_pnl", session.TotalPnL).
		Int("total_trades", session.TotalTrades).
		Msg("Trading session stopped")

	return map[string]interface{}{
		"session_id":      session.ID.String(),
		"symbol":          session.Symbol,
		"mode":            string(session.Mode),
		"initial_capital": session.InitialCapital,
		"final_capital":   finalCapital,
		"started_at":      session.StartedAt,
		"stopped_at":      session.StoppedAt,
		"total_trades":    session.TotalTrades,
		"winning_trades":  session.WinningTrades,
		"losing_trades":   session.LosingTrades,
		"total_pnl":       session.TotalPnL,
		"max_drawdown":    session.MaxDrawdown,
		"sharpe_ratio":    session.SharpeRatio,
	}, nil
}

// GetSessionStats retrieves current session statistics
func (s *Service) GetSessionStats(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("GetSessionStats called")

	// Get current session from exchange
	sessionID := s.exchange.GetSession()
	if sessionID == nil {
		return nil, fmt.Errorf("no active trading session")
	}

	// Get session from database
	ctx := context.Background()
	session, err := s.db.GetSession(ctx, *sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return map[string]interface{}{
		"session_id":      session.ID.String(),
		"symbol":          session.Symbol,
		"exchange":        session.Exchange,
		"mode":            string(session.Mode),
		"initial_capital": session.InitialCapital,
		"started_at":      session.StartedAt,
		"total_trades":    session.TotalTrades,
		"winning_trades":  session.WinningTrades,
		"losing_trades":   session.LosingTrades,
		"total_pnl":       session.TotalPnL,
		"max_drawdown":    session.MaxDrawdown,
		"sharpe_ratio":    session.SharpeRatio,
	}, nil
}

// extractFloat extracts a float64 from the args map
func extractFloat(args map[string]interface{}, key string) (float64, error) {
	value, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}

	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("%s must be a number", key)
	}
}
