package exchange

import (
	"context"
	"fmt"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/rs/zerolog/log"
)

// TradingMode represents the trading mode (paper or live)
type TradingMode string

const (
	TradingModePaper TradingMode = "paper"
	TradingModeLive  TradingMode = "live"
)

// Service provides order execution functionality
type Service struct {
	exchange        Exchange // Interface - can be MockExchange or BinanceExchange
	db              *db.DB
	mode            TradingMode
	positionManager *PositionManager
}

// ServiceConfig contains configuration for the exchange service
type ServiceConfig struct {
	Mode           TradingMode
	BinanceAPIKey  string
	BinanceSecret  string
	BinanceTestnet bool
}

// NewService creates a new exchange service with specified trading mode
func NewService(database *db.DB, config ServiceConfig) (*Service, error) {
	var exchange Exchange
	var err error

	switch config.Mode {
	case TradingModeLive:
		// Create Binance exchange for live trading
		binanceConfig := BinanceConfig{
			APIKey:    config.BinanceAPIKey,
			SecretKey: config.BinanceSecret,
			Testnet:   config.BinanceTestnet,
		}
		exchange, err = NewBinanceExchange(binanceConfig, database)
		if err != nil {
			return nil, fmt.Errorf("failed to create Binance exchange: %w", err)
		}
		log.Info().Bool("testnet", config.BinanceTestnet).Msg("Exchange service initialized (LIVE trading)")

	case TradingModePaper:
		fallthrough
	default:
		// Create mock exchange for paper trading
		exchange = NewMockExchange(database)
		log.Info().Msg("Exchange service initialized (PAPER trading)")
	}

	// Create position manager
	positionManager := NewPositionManager(database)

	return &Service{
		exchange:        exchange,
		db:              database,
		mode:            config.Mode,
		positionManager: positionManager,
	}, nil
}

// NewServicePaper creates a service in paper trading mode (for backward compatibility)
func NewServicePaper(database *db.DB) *Service {
	service, _ := NewService(database, ServiceConfig{Mode: TradingModePaper})
	return service
}

// PlaceMarketOrder places a market order
func (s *Service) PlaceMarketOrder(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("PlaceMarketOrder called")

	// Create context with 30-second timeout for exchange API calls
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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
	resp, err := s.exchange.PlaceOrder(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	// Get order details
	order, err := s.exchange.GetOrder(ctx, resp.OrderID)
	if err != nil {
		log.Error().Err(err).Str("order_id", resp.OrderID).Msg("Failed to retrieve order after placement")
		return resp, nil // Still return the response even if we can't get details
	}

	// Update positions if order was filled
	if order.Status == OrderStatusFilled {
		fills, err := s.exchange.GetOrderFills(ctx, order.ID)
		if err == nil && len(fills) > 0 {
			if err := s.positionManager.OnOrderFilled(ctx, order, fills); err != nil {
				log.Error().Err(err).Msg("Failed to update positions after order fill")
			}
		}
	}

	return order, nil
}

// PlaceLimitOrder places a limit order
func (s *Service) PlaceLimitOrder(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("PlaceLimitOrder called")

	// Create context with 30-second timeout for exchange API calls
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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
	resp, err := s.exchange.PlaceOrder(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	// Get order details
	order, err := s.exchange.GetOrder(ctx, resp.OrderID)
	if err != nil {
		log.Error().Err(err).Str("order_id", resp.OrderID).Msg("Failed to retrieve order after placement")
		return resp, nil
	}

	// Update positions if order was filled (limit orders may fill immediately in some cases)
	if order.Status == OrderStatusFilled {
		fills, err := s.exchange.GetOrderFills(ctx, order.ID)
		if err == nil && len(fills) > 0 {
			if err := s.positionManager.OnOrderFilled(ctx, order, fills); err != nil {
				log.Error().Err(err).Msg("Failed to update positions after order fill")
			}
		}
	}

	return order, nil
}

// CancelOrder cancels an existing order
func (s *Service) CancelOrder(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("CancelOrder called")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Extract order_id
	orderID, ok := args["order_id"].(string)
	if !ok || orderID == "" {
		return nil, fmt.Errorf("order_id is required and must be a string")
	}

	// Cancel order
	order, err := s.exchange.CancelOrder(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel order: %w", err)
	}

	return order, nil
}

// GetOrderStatus retrieves order status
func (s *Service) GetOrderStatus(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("GetOrderStatus called")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Extract order_id
	orderID, ok := args["order_id"].(string)
	if !ok || orderID == "" {
		return nil, fmt.Errorf("order_id is required and must be a string")
	}

	// Get order
	order, err := s.exchange.GetOrder(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Get fills
	fills, err := s.exchange.GetOrderFills(ctx, orderID)
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.db.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Set session in exchange
	s.exchange.SetSession(&session.ID)

	// Set session in position manager
	s.positionManager.SetSession(&session.ID)

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.db.StopSession(ctx, *sessionID, finalCapital); err != nil {
		return nil, fmt.Errorf("failed to stop session: %w", err)
	}

	// Clear session from exchange
	s.exchange.SetSession(nil)

	// Clear session from position manager
	s.positionManager.SetSession(nil)

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
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

// GetPositions retrieves current open positions
func (s *Service) GetPositions(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("GetPositions called")

	positions := s.positionManager.GetOpenPositions()

	return map[string]interface{}{
		"positions": positions,
		"count":     len(positions),
	}, nil
}

// GetPositionBySymbol retrieves a specific position by symbol
func (s *Service) GetPositionBySymbol(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("GetPositionBySymbol called")

	symbol, ok := args["symbol"].(string)
	if !ok || symbol == "" {
		return nil, fmt.Errorf("symbol is required and must be a string")
	}

	position, exists := s.positionManager.GetPosition(symbol)
	if !exists {
		return map[string]interface{}{
			"position": nil,
			"exists":   false,
		}, nil
	}

	return map[string]interface{}{
		"position": position,
		"exists":   true,
	}, nil
}

// UpdatePositionPnL updates unrealized P&L for positions based on current prices
func (s *Service) UpdatePositionPnL(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("UpdatePositionPnL called")

	// Extract prices map
	pricesArg, ok := args["prices"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("prices must be a map of symbol -> price")
	}

	// Convert to map[string]float64
	prices := make(map[string]float64)
	for symbol, priceVal := range pricesArg {
		switch v := priceVal.(type) {
		case float64:
			prices[symbol] = v
		case int:
			prices[symbol] = float64(v)
		case int64:
			prices[symbol] = float64(v)
		default:
			return nil, fmt.Errorf("invalid price for symbol %s", symbol)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := s.positionManager.UpdateUnrealizedPnL(ctx, prices)
	if err != nil {
		return nil, fmt.Errorf("failed to update P&L: %w", err)
	}

	totalUnrealizedPnL := s.positionManager.GetTotalUnrealizedPnL()

	return map[string]interface{}{
		"total_unrealized_pnl": totalUnrealizedPnL,
		"success":              true,
	}, nil
}

// ClosePositionBySymbol closes a position for a specific symbol
func (s *Service) ClosePositionBySymbol(args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("ClosePositionBySymbol called")

	symbol, ok := args["symbol"].(string)
	if !ok || symbol == "" {
		return nil, fmt.Errorf("symbol is required and must be a string")
	}

	exitPrice, err := extractFloat(args, "exit_price")
	if err != nil {
		return nil, fmt.Errorf("exit_price error: %w", err)
	}

	exitReason, ok := args["exit_reason"].(string)
	if !ok {
		exitReason = "Manual close"
	}

	position, exists := s.positionManager.GetPosition(symbol)
	if !exists {
		return nil, fmt.Errorf("no open position for symbol: %s", symbol)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = s.db.ClosePosition(ctx, position.ID, exitPrice, exitReason, 0.0)
	if err != nil {
		return nil, fmt.Errorf("failed to close position: %w", err)
	}

	log.Info().
		Str("symbol", symbol).
		Float64("exit_price", exitPrice).
		Msg("Position closed via ClosePositionBySymbol")

	return map[string]interface{}{
		"position_id": position.ID.String(),
		"symbol":      symbol,
		"closed":      true,
	}, nil
}

// StartWebSocketUpdates starts real-time WebSocket updates (Binance only)
// This enables real-time order and position updates via WebSocket
func (s *Service) StartWebSocketUpdates(ctx context.Context) error {
	if s.mode != TradingModeLive {
		log.Debug().Msg("WebSocket updates only available in LIVE mode")
		return nil // Not an error, just not applicable
	}

	// Type assert to BinanceExchange
	binanceExchange, ok := s.exchange.(*BinanceExchange)
	if !ok {
		return fmt.Errorf("WebSocket updates only supported for Binance exchange")
	}

	if err := binanceExchange.StartUserDataStream(ctx); err != nil {
		return fmt.Errorf("failed to start WebSocket updates: %w", err)
	}

	log.Info().Msg("WebSocket position updates started")
	return nil
}

// StopWebSocketUpdates stops WebSocket updates (Binance only)
func (s *Service) StopWebSocketUpdates(ctx context.Context) error {
	if s.mode != TradingModeLive {
		return nil // Not applicable in paper mode
	}

	// Type assert to BinanceExchange
	binanceExchange, ok := s.exchange.(*BinanceExchange)
	if !ok {
		return fmt.Errorf("WebSocket updates only supported for Binance exchange")
	}

	if err := binanceExchange.StopUserDataStream(ctx); err != nil {
		return fmt.Errorf("failed to stop WebSocket updates: %w", err)
	}

	log.Info().Msg("WebSocket position updates stopped")
	return nil
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
