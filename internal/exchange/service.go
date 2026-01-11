package exchange

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sony/gobreaker"

	"github.com/ajitpratap0/cryptofunk/internal/config"
	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/risk"
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
	circuitBreaker  *risk.CircuitBreakerManager
	safetyGuard     *risk.SafetyGuard // Live trading safety guards
}

// ServiceConfig contains configuration for the exchange service
type ServiceConfig struct {
	Mode           TradingMode
	BinanceAPIKey  string
	BinanceSecret  string
	BinanceTestnet bool
	Fees           config.FeeConfig         // Exchange fee configuration
	SafetyGuard    config.SafetyGuardConfig // Safety guard configuration
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
		// Create mock exchange for paper trading with configured fees
		exchange = NewMockExchangeWithFees(database, config.Fees)
		log.Info().Msg("Exchange service initialized (PAPER trading)")
	}

	// Create position manager with configured fee rate (average of maker/taker)
	avgFeeRate := (config.Fees.Maker + config.Fees.Taker) / 2.0
	positionManager := NewPositionManagerWithFees(database, avgFeeRate)

	// Create circuit breaker manager
	circuitBreaker := risk.NewCircuitBreakerManager()

	// Create safety guard with metrics callback
	safetyGuard := risk.NewSafetyGuard(config.SafetyGuard)
	risk.SetupMetricsCallback(safetyGuard)

	// Log safety guard status
	if config.SafetyGuard.Enabled {
		log.Info().
			Float64("max_daily_drawdown", config.SafetyGuard.MaxDailyDrawdown).
			Float64("max_position_size", config.SafetyGuard.MaxPositionSize).
			Int("max_daily_trades", config.SafetyGuard.MaxDailyTrades).
			Int("max_consecutive_losses", config.SafetyGuard.MaxConsecutiveLosses).
			Msg("Safety guards enabled")
	} else {
		log.Warn().Msg("Safety guards disabled - trading without safety limits")
	}

	return &Service{
		exchange:        exchange,
		db:              database,
		mode:            config.Mode,
		positionManager: positionManager,
		circuitBreaker:  circuitBreaker,
		safetyGuard:     safetyGuard,
	}, nil
}

// NewServicePaper creates a service in paper trading mode with default fees (for backward compatibility)
func NewServicePaper(database *db.DB) *Service {
	// Default Binance-like fees
	defaultFees := config.FeeConfig{
		Maker:        0.001,  // 0.1%
		Taker:        0.001,  // 0.1%
		BaseSlippage: 0.0005, // 0.05%
		MarketImpact: 0.0001, // 0.01%
		MaxSlippage:  0.003,  // 0.3%
	}
	service, _ := NewService(database, ServiceConfig{
		Mode: TradingModePaper,
		Fees: defaultFees,
	})
	return service
}

// SetMarketPrice sets the current market price for a symbol (delegates to exchange)
// This is primarily used for testing with mock exchange
func (s *Service) SetMarketPrice(symbol string, price float64) {
	s.exchange.SetMarketPrice(symbol, price)
}

// PlaceMarketOrder places a market order
func (s *Service) PlaceMarketOrder(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("PlaceMarketOrder called")

	// Create context with 30-second timeout for exchange API calls
	exchangeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
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

	// Extract optional price for order value calculation
	// For market orders, use current market price if not provided
	price, _ := extractFloat(args, "price")
	if price <= 0 {
		// Get current market price from exchange
		price = s.exchange.GetMarketPrice(symbol)
	}
	orderValue := quantity * price

	// Extract confirmation flag for large trades
	isConfirmed, _ := args["confirmed"].(bool)

	// Safety guard check (especially important for live trading)
	if s.safetyGuard != nil {
		safetyReq := risk.OrderRequest{
			Symbol:      symbol,
			Side:        sideStr,
			OrderType:   "market",
			Quantity:    quantity,
			OrderValue:  orderValue,
			IsConfirmed: isConfirmed,
		}
		result := s.safetyGuard.ValidateOrder(ctx, safetyReq)
		if !result.Allowed {
			log.Warn().
				Str("symbol", symbol).
				Str("violation", string(result.Violation)).
				Str("message", result.Message).
				Msg("Order rejected by safety guard")
			return nil, fmt.Errorf("order rejected by safety guard: %s", result.Message)
		}
	}

	// Create request
	req := PlaceOrderRequest{
		Symbol:   symbol,
		Side:     side,
		Type:     OrderTypeMarket,
		Quantity: quantity,
	}

	// Place order through circuit breaker
	var resp *PlaceOrderResponse
	cbResult, err := s.circuitBreaker.Exchange().Execute(func() (interface{}, error) {
		return s.exchange.PlaceOrder(exchangeCtx, req)
	})

	if err != nil {
		// Check if circuit breaker is open
		if err == gobreaker.ErrOpenState {
			s.circuitBreaker.Metrics().RecordRequest("exchange", false)
			return nil, fmt.Errorf("exchange circuit breaker is open, system unavailable")
		}
		s.circuitBreaker.Metrics().RecordRequest("exchange", false)
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	s.circuitBreaker.Metrics().RecordRequest("exchange", true)
	resp = cbResult.(*PlaceOrderResponse)

	// Get order details through circuit breaker
	var order *Order
	orderResult, err := s.circuitBreaker.Exchange().Execute(func() (interface{}, error) {
		return s.exchange.GetOrder(exchangeCtx, resp.OrderID)
	})

	if err != nil {
		// Check if circuit breaker is open
		if err == gobreaker.ErrOpenState {
			s.circuitBreaker.Metrics().RecordRequest("exchange", false)
			log.Error().Str("order_id", resp.OrderID).Msg("Circuit breaker open: failed to retrieve order after placement")
			return resp, nil // Still return the response even if we can't get details
		}
		s.circuitBreaker.Metrics().RecordRequest("exchange", false)
		log.Error().Err(err).Str("order_id", resp.OrderID).Msg("Failed to retrieve order after placement")
		return resp, nil // Still return the response even if we can't get details
	}

	s.circuitBreaker.Metrics().RecordRequest("exchange", true)
	order = orderResult.(*Order)

	// Record order placement in safety guard for rate limiting
	if s.safetyGuard != nil {
		s.safetyGuard.RecordOrderPlaced()
	}

	// Update positions if order was filled
	if order.Status == OrderStatusFilled {
		// Get order fills through circuit breaker
		fillsResult, err := s.circuitBreaker.Exchange().Execute(func() (interface{}, error) {
			return s.exchange.GetOrderFills(exchangeCtx, order.ID)
		})

		if err != nil {
			// Check if circuit breaker is open
			if err == gobreaker.ErrOpenState {
				s.circuitBreaker.Metrics().RecordRequest("exchange", false)
				log.Error().Str("order_id", order.ID).Msg("Circuit breaker open: failed to get order fills for position update")
			} else {
				s.circuitBreaker.Metrics().RecordRequest("exchange", false)
				log.Error().Err(err).Str("order_id", order.ID).Msg("Failed to get order fills for position update")
			}
		} else {
			s.circuitBreaker.Metrics().RecordRequest("exchange", true)
			fills := fillsResult.([]Fill)
			if len(fills) > 0 {
				if err := s.positionManager.OnOrderFilled(exchangeCtx, order, fills); err != nil {
					log.Error().Err(err).Msg("Failed to update positions after order fill")
				}

				// Update safety guard position tracking
				if s.safetyGuard != nil {
					positionValue := order.FilledQty * order.AvgFillPrice
					s.safetyGuard.UpdatePosition(order.Symbol, positionValue)
				}
			}
		}
	}

	return order, nil
}

// PlaceLimitOrder places a limit order
func (s *Service) PlaceLimitOrder(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("PlaceLimitOrder called")

	// Create context with 30-second timeout for exchange API calls
	exchangeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
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

	// Calculate order value
	orderValue := quantity * price

	// Extract confirmation flag for large trades
	isConfirmed, _ := args["confirmed"].(bool)

	// Safety guard check (especially important for live trading)
	if s.safetyGuard != nil {
		safetyReq := risk.OrderRequest{
			Symbol:      symbol,
			Side:        sideStr,
			OrderType:   "limit",
			Quantity:    quantity,
			Price:       price,
			OrderValue:  orderValue,
			IsConfirmed: isConfirmed,
		}
		result := s.safetyGuard.ValidateOrder(ctx, safetyReq)
		if !result.Allowed {
			log.Warn().
				Str("symbol", symbol).
				Str("violation", string(result.Violation)).
				Str("message", result.Message).
				Msg("Order rejected by safety guard")
			return nil, fmt.Errorf("order rejected by safety guard: %s", result.Message)
		}
	}

	// Create request
	req := PlaceOrderRequest{
		Symbol:   symbol,
		Side:     side,
		Type:     OrderTypeLimit,
		Quantity: quantity,
		Price:    price,
	}

	// Place order through circuit breaker
	var resp *PlaceOrderResponse
	cbResult, err := s.circuitBreaker.Exchange().Execute(func() (interface{}, error) {
		return s.exchange.PlaceOrder(exchangeCtx, req)
	})

	if err != nil {
		// Check if circuit breaker is open
		if err == gobreaker.ErrOpenState {
			s.circuitBreaker.Metrics().RecordRequest("exchange", false)
			return nil, fmt.Errorf("exchange circuit breaker is open, system unavailable")
		}
		s.circuitBreaker.Metrics().RecordRequest("exchange", false)
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	s.circuitBreaker.Metrics().RecordRequest("exchange", true)
	resp = cbResult.(*PlaceOrderResponse)

	// Get order details through circuit breaker
	var order *Order
	orderResult, err := s.circuitBreaker.Exchange().Execute(func() (interface{}, error) {
		return s.exchange.GetOrder(exchangeCtx, resp.OrderID)
	})

	if err != nil {
		// Check if circuit breaker is open
		if err == gobreaker.ErrOpenState {
			s.circuitBreaker.Metrics().RecordRequest("exchange", false)
			log.Error().Str("order_id", resp.OrderID).Msg("Circuit breaker open: failed to retrieve order after placement")
			return resp, nil // Still return the response even if we can't get details
		}
		s.circuitBreaker.Metrics().RecordRequest("exchange", false)
		log.Error().Err(err).Str("order_id", resp.OrderID).Msg("Failed to retrieve order after placement")
		return resp, nil // Still return the response even if we can't get details
	}

	s.circuitBreaker.Metrics().RecordRequest("exchange", true)
	order = orderResult.(*Order)

	// Record order placement in safety guard for rate limiting
	if s.safetyGuard != nil {
		s.safetyGuard.RecordOrderPlaced()
	}

	// Update positions if order was filled (limit orders may fill immediately in some cases)
	if order.Status == OrderStatusFilled {
		// Get order fills through circuit breaker
		fillsResult, err := s.circuitBreaker.Exchange().Execute(func() (interface{}, error) {
			return s.exchange.GetOrderFills(exchangeCtx, order.ID)
		})

		if err != nil {
			// Check if circuit breaker is open
			if err == gobreaker.ErrOpenState {
				s.circuitBreaker.Metrics().RecordRequest("exchange", false)
				log.Error().Str("order_id", order.ID).Msg("Circuit breaker open: failed to get order fills for position update")
			} else {
				s.circuitBreaker.Metrics().RecordRequest("exchange", false)
				log.Error().Err(err).Str("order_id", order.ID).Msg("Failed to get order fills for position update")
			}
		} else {
			s.circuitBreaker.Metrics().RecordRequest("exchange", true)
			fills := fillsResult.([]Fill)
			if len(fills) > 0 {
				if err := s.positionManager.OnOrderFilled(exchangeCtx, order, fills); err != nil {
					log.Error().Err(err).Msg("Failed to update positions after order fill")
				}

				// Update safety guard position tracking
				if s.safetyGuard != nil {
					positionValue := order.FilledQty * order.AvgFillPrice
					s.safetyGuard.UpdatePosition(order.Symbol, positionValue)
				}
			}
		}
	}

	return order, nil
}

// CancelOrder cancels an existing order
func (s *Service) CancelOrder(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("CancelOrder called")

	exchangeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Extract order_id
	orderID, ok := args["order_id"].(string)
	if !ok || orderID == "" {
		return nil, fmt.Errorf("order_id is required and must be a string")
	}

	// Cancel order through circuit breaker
	var order *Order
	cbResult, err := s.circuitBreaker.Exchange().Execute(func() (interface{}, error) {
		return s.exchange.CancelOrder(exchangeCtx, orderID)
	})

	if err != nil {
		// Check if circuit breaker is open
		if err == gobreaker.ErrOpenState {
			s.circuitBreaker.Metrics().RecordRequest("exchange", false)
			return nil, fmt.Errorf("exchange circuit breaker is open, system unavailable")
		}
		s.circuitBreaker.Metrics().RecordRequest("exchange", false)
		return nil, fmt.Errorf("failed to cancel order: %w", err)
	}

	s.circuitBreaker.Metrics().RecordRequest("exchange", true)
	order = cbResult.(*Order)

	return order, nil
}

// GetOrderStatus retrieves order status
func (s *Service) GetOrderStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("GetOrderStatus called")

	exchangeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Extract order_id
	orderID, ok := args["order_id"].(string)
	if !ok || orderID == "" {
		return nil, fmt.Errorf("order_id is required and must be a string")
	}

	// Get order through circuit breaker
	var order *Order
	cbResult, err := s.circuitBreaker.Exchange().Execute(func() (interface{}, error) {
		return s.exchange.GetOrder(exchangeCtx, orderID)
	})

	if err != nil {
		// Check if circuit breaker is open
		if err == gobreaker.ErrOpenState {
			s.circuitBreaker.Metrics().RecordRequest("exchange", false)
			return nil, fmt.Errorf("exchange circuit breaker is open, system unavailable")
		}
		s.circuitBreaker.Metrics().RecordRequest("exchange", false)
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	s.circuitBreaker.Metrics().RecordRequest("exchange", true)
	order = cbResult.(*Order)

	// Get fills through circuit breaker
	var fills []Fill
	fillsResult, err := s.circuitBreaker.Exchange().Execute(func() (interface{}, error) {
		return s.exchange.GetOrderFills(exchangeCtx, orderID)
	})

	if err != nil {
		// Check if circuit breaker is open
		if err == gobreaker.ErrOpenState {
			s.circuitBreaker.Metrics().RecordRequest("exchange", false)
			log.Error().Str("order_id", orderID).Msg("Circuit breaker open: failed to get order fills")
		} else {
			s.circuitBreaker.Metrics().RecordRequest("exchange", false)
			log.Error().Err(err).Str("order_id", orderID).Msg("Failed to get order fills")
		}
		// Continue even if we can't get fills
	} else {
		s.circuitBreaker.Metrics().RecordRequest("exchange", true)
		fills = fillsResult.([]Fill)
	}

	return map[string]interface{}{
		"order": order,
		"fills": fills,
	}, nil
}

// StartSession starts a new trading session
func (s *Service) StartSession(ctx context.Context, args map[string]interface{}) (interface{}, error) {
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

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.db.CreateSession(dbCtx, session); err != nil {
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
func (s *Service) StopSession(ctx context.Context, args map[string]interface{}) (interface{}, error) {
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
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.db.StopSession(dbCtx, *sessionID, finalCapital); err != nil {
		return nil, fmt.Errorf("failed to stop session: %w", err)
	}

	// Clear session from exchange
	s.exchange.SetSession(nil)

	// Clear session from position manager
	s.positionManager.SetSession(nil)

	// Get final session data
	session, err := s.db.GetSession(dbCtx, *sessionID)
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
func (s *Service) GetSessionStats(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("GetSessionStats called")

	// Get current session from exchange
	sessionID := s.exchange.GetSession()
	if sessionID == nil {
		return nil, fmt.Errorf("no active trading session")
	}

	// Get session from database
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	session, err := s.db.GetSession(dbCtx, *sessionID)
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
func (s *Service) GetPositions(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	log.Debug().Interface("args", args).Msg("GetPositions called")

	positions := s.positionManager.GetOpenPositions()

	return map[string]interface{}{
		"positions": positions,
		"count":     len(positions),
	}, nil
}

// GetPositionBySymbol retrieves a specific position by symbol
func (s *Service) GetPositionBySymbol(ctx context.Context, args map[string]interface{}) (interface{}, error) {
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
func (s *Service) UpdatePositionPnL(ctx context.Context, args map[string]interface{}) (interface{}, error) {
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

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := s.positionManager.UpdateUnrealizedPnL(dbCtx, prices)
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
func (s *Service) ClosePositionBySymbol(ctx context.Context, args map[string]interface{}) (interface{}, error) {
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

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = s.db.ClosePosition(dbCtx, position.ID, exitPrice, exitReason, 0.0)
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

// === Safety Guard Methods ===

// GetSafetyGuardStats returns current safety guard statistics
func (s *Service) GetSafetyGuardStats(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.safetyGuard == nil {
		return map[string]interface{}{
			"enabled": false,
			"message": "Safety guards not configured",
		}, nil
	}

	stats := s.safetyGuard.GetStats()
	isTripped := s.safetyGuard.IsCircuitBreakerTripped()

	return map[string]interface{}{
		"enabled":                 true,
		"daily_pnl":               stats.DailyPnL,
		"daily_pnl_percent":       stats.DailyPnLPercent * 100,
		"daily_trade_count":       stats.DailyTradeCount,
		"consecutive_losses":      stats.ConsecutiveLosses,
		"total_exposure":          stats.TotalExposure,
		"total_exposure_percent":  stats.TotalExposurePercent * 100,
		"capital":                 stats.Capital,
		"circuit_breaker_tripped": isTripped,
	}, nil
}

// SetSafetyGuardCapital sets the capital for safety guard calculations
func (s *Service) SetSafetyGuardCapital(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.safetyGuard == nil {
		return nil, fmt.Errorf("safety guards not configured")
	}

	capital, err := extractFloat(args, "capital")
	if err != nil {
		return nil, fmt.Errorf("capital error: %w", err)
	}
	if capital <= 0 {
		return nil, fmt.Errorf("capital must be positive")
	}

	s.safetyGuard.SetCapital(capital)

	return map[string]interface{}{
		"success": true,
		"capital": capital,
	}, nil
}

// RecordTradePnL records a trade's P&L for safety guard tracking
func (s *Service) RecordTradePnL(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.safetyGuard == nil {
		return nil, fmt.Errorf("safety guards not configured")
	}

	pnl, err := extractFloat(args, "pnl")
	if err != nil {
		return nil, fmt.Errorf("pnl error: %w", err)
	}

	s.safetyGuard.RecordTrade(pnl)

	stats := s.safetyGuard.GetStats()
	isTripped := s.safetyGuard.IsCircuitBreakerTripped()

	return map[string]interface{}{
		"success":                 true,
		"pnl_recorded":            pnl,
		"daily_pnl":               stats.DailyPnL,
		"consecutive_losses":      stats.ConsecutiveLosses,
		"circuit_breaker_tripped": isTripped,
	}, nil
}

// ResetSafetyGuardCircuitBreaker manually resets the safety guard circuit breaker
func (s *Service) ResetSafetyGuardCircuitBreaker(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.safetyGuard == nil {
		return nil, fmt.Errorf("safety guards not configured")
	}

	s.safetyGuard.ResetCircuitBreaker()

	return map[string]interface{}{
		"success": true,
		"message": "Circuit breaker reset successfully",
	}, nil
}

// ResetDailyCounters resets the daily counters (usually called at market open)
func (s *Service) ResetDailyCounters(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.safetyGuard == nil {
		return nil, fmt.Errorf("safety guards not configured")
	}

	capital, err := extractFloat(args, "capital")
	if err != nil {
		return nil, fmt.Errorf("capital error: %w", err)
	}
	if capital <= 0 {
		return nil, fmt.Errorf("capital must be positive")
	}

	s.safetyGuard.ResetDailyCounters(capital)

	return map[string]interface{}{
		"success":     true,
		"new_capital": capital,
		"message":     "Daily counters reset successfully",
	}, nil
}

// === Emergency Stop Methods ===

// EmergencyStop immediately halts all trading with a given reason
func (s *Service) EmergencyStop(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.safetyGuard == nil {
		return nil, fmt.Errorf("safety guards not configured")
	}

	reason, ok := args["reason"].(string)
	if !ok || reason == "" {
		reason = "Manual emergency stop"
	}

	s.safetyGuard.EmergencyStop(reason)

	return map[string]interface{}{
		"success":   true,
		"message":   "Emergency stop activated",
		"reason":    reason,
		"timestamp": time.Now(),
	}, nil
}

// ClearEmergencyStop clears the emergency stop state
func (s *Service) ClearEmergencyStop(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.safetyGuard == nil {
		return nil, fmt.Errorf("safety guards not configured")
	}

	wasActive := s.safetyGuard.IsEmergencyStopActive()
	s.safetyGuard.ClearEmergencyStop()

	return map[string]interface{}{
		"success":    true,
		"was_active": wasActive,
		"message":    "Emergency stop cleared",
	}, nil
}

// GetEmergencyStopStatus returns the current emergency stop status
func (s *Service) GetEmergencyStopStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.safetyGuard == nil {
		return map[string]interface{}{
			"active":  false,
			"message": "Safety guards not configured",
		}, nil
	}

	active, reason, since := s.safetyGuard.GetEmergencyStopInfo()

	result := map[string]interface{}{
		"active": active,
	}

	if active {
		result["reason"] = reason
		result["since"] = since
		result["duration"] = time.Since(since).String()
	}

	return result, nil
}

// === Trading Hours Methods ===

// GetTradingHoursStatus returns the current trading hours status
func (s *Service) GetTradingHoursStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.safetyGuard == nil {
		return map[string]interface{}{
			"enabled":      false,
			"within_hours": true,
			"message":      "Safety guards not configured",
		}, nil
	}

	enabled, start, end, timezone, withinHours := s.safetyGuard.GetTradingHoursInfo()

	return map[string]interface{}{
		"enabled":      enabled,
		"within_hours": withinHours,
		"start":        start,
		"end":          end,
		"timezone":     timezone,
		"current_time": time.Now().Format(time.RFC3339),
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
