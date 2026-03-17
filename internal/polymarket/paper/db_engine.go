package paper

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ajitpratap0/cryptofunk/internal/db"
)

// DBPaperEngine is a database-backed paper trading engine for Polymarket.
type DBPaperEngine struct {
	db        *db.DB
	sessionID uuid.UUID
	balance   float64
}

// NewDBPaperEngine creates a new DB-backed paper engine.
// It creates or reuses a trading session with mode=PAPER, exchange=polymarket.
func NewDBPaperEngine(database *db.DB) (*DBPaperEngine, error) {
	ctx := context.Background()

	// Create a new paper trading session
	session := &db.TradingSession{
		ID:             uuid.New(),
		Mode:           db.TradingModePaper,
		Symbol:         "POLYMARKET",
		Exchange:       "polymarket",
		StartedAt:      time.Now(),
		InitialCapital: DefaultBalance,
	}
	if err := database.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create paper session: %w", err)
	}

	return &DBPaperEngine{
		db:        database,
		sessionID: session.ID,
		balance:   DefaultBalance,
	}, nil
}

// NewDBPaperEngineWithSession reuses an existing session.
func NewDBPaperEngineWithSession(database *db.DB, sessionID uuid.UUID) (*DBPaperEngine, error) {
	ctx := context.Background()
	session, err := database.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Calculate current balance from session initial capital minus open position costs
	summary, err := database.GetPolymarketPortfolioSummaryBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session portfolio summary: %w", err)
	}

	balance := session.InitialCapital - summary.TotalCostBasis

	return &DBPaperEngine{
		db:        database,
		sessionID: sessionID,
		balance:   balance,
	}, nil
}

// Buy places a virtual buy order backed by the database.
func (e *DBPaperEngine) Buy(marketID, question string, side Side, amount, price float64) (*Trade, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	if price <= 0 || price >= 1 {
		return nil, fmt.Errorf("price must be between 0 and 1")
	}
	if amount > e.balance {
		return nil, fmt.Errorf("insufficient balance: have $%.2f, need $%.2f", e.balance, amount)
	}

	ctx := context.Background()
	shares := amount / price

	// Upsert market data, seeding the trade price as a baseline so that
	// positions always have a non-zero current_price.  COALESCE in the DB
	// preserves any real price that was already seeded by the market-data
	// fetcher; we only fill in the side we traded on.
	mkt := &db.PolymarketMarket{
		ID:       marketID,
		Question: question,
		Active:   true,
	}
	if side == YES {
		mkt.YesPrice = &price
	} else {
		mkt.NoPrice = &price
	}
	if err := e.db.UpsertPolymarketMarket(ctx, mkt); err != nil {
		return nil, fmt.Errorf("failed to upsert market: %w", err)
	}

	// Find or create position
	key := marketID + ":" + string(side)
	pos, err := e.findOpenPosition(ctx, marketID, string(side))
	if err != nil {
		// Create new position
		pos = &db.PolymarketPosition{
			ID:        uuid.New(),
			SessionID: e.sessionID,
			MarketID:  marketID,
			Side:      string(side),
			Shares:    shares,
			AvgPrice:  price,
			CostBasis: amount,
			Status:    "OPEN",
		}
		if err := e.db.InsertPolymarketPosition(ctx, pos); err != nil {
			return nil, fmt.Errorf("failed to insert position: %w", err)
		}
	} else {
		// Update existing position (average in)
		totalCost := pos.CostBasis + amount
		totalShares := pos.Shares + shares
		pos.AvgPrice = totalCost / totalShares
		pos.Shares = totalShares
		pos.CostBasis = totalCost
		if err := e.db.UpdatePolymarketPosition(ctx, pos); err != nil {
			return nil, fmt.Errorf("failed to update position: %w", err)
		}
	}

	// Record trade
	trade := &db.PolymarketTrade{
		ID:         uuid.New(),
		PositionID: pos.ID,
		MarketID:   marketID,
		Action:     "BUY",
		Side:       string(side),
		Amount:     amount,
		Price:      price,
		Shares:     shares,
		Timestamp:  time.Now(),
	}
	if err := e.db.InsertPolymarketTrade(ctx, trade); err != nil {
		return nil, fmt.Errorf("failed to insert trade: %w", err)
	}

	e.balance -= amount

	_ = key // used conceptually for position lookup
	return &Trade{
		MarketID:  marketID,
		Question:  question,
		Side:      side,
		Amount:    amount,
		Price:     price,
		Shares:    shares,
		Timestamp: trade.Timestamp,
		Action:    "BUY",
	}, nil
}

// Sell sells shares from a position.
func (e *DBPaperEngine) Sell(marketID string, side Side, shares, price float64) (*Trade, error) {
	ctx := context.Background()

	pos, err := e.findOpenPosition(ctx, marketID, string(side))
	if err != nil {
		return nil, fmt.Errorf("no position for %s %s", marketID, side)
	}
	if shares > pos.Shares {
		return nil, fmt.Errorf("insufficient shares: have %.2f, selling %.2f", pos.Shares, shares)
	}

	amount := shares * price

	// Record trade
	trade := &db.PolymarketTrade{
		ID:         uuid.New(),
		PositionID: pos.ID,
		MarketID:   marketID,
		Action:     "SELL",
		Side:       string(side),
		Amount:     amount,
		Price:      price,
		Shares:     shares,
		Timestamp:  time.Now(),
	}
	if err := e.db.InsertPolymarketTrade(ctx, trade); err != nil {
		return nil, fmt.Errorf("failed to insert trade: %w", err)
	}

	e.balance += amount

	// Update position
	costReduced := (shares / pos.Shares) * pos.CostBasis
	pos.Shares -= shares
	pos.CostBasis -= costReduced

	if pos.Shares <= 0.0001 {
		realizedPnl := amount - costReduced
		if err := e.db.ClosePolymarketPosition(ctx, pos.ID, realizedPnl); err != nil {
			return nil, fmt.Errorf("failed to close position: %w", err)
		}
	} else {
		if err := e.db.UpdatePolymarketPosition(ctx, pos); err != nil {
			return nil, fmt.Errorf("failed to update position: %w", err)
		}
	}

	return &Trade{
		MarketID:  marketID,
		Question:  "",
		Side:      side,
		Amount:    amount,
		Price:     price,
		Shares:    shares,
		Timestamp: trade.Timestamp,
		Action:    "SELL",
	}, nil
}

// GetPortfolio returns a Portfolio compatible with the file-based engine.
// Only positions and trades belonging to this engine's session are returned,
// ensuring isolation between concurrent paper trading sessions.
func (e *DBPaperEngine) GetPortfolio() *Portfolio {
	ctx := context.Background()
	positions, _ := e.db.GetOpenPolymarketPositionsBySession(ctx, e.sessionID)
	trades, _ := e.db.GetPolymarketTradesBySession(ctx, e.sessionID, 1000)

	posMap := make(map[string]*Position)
	for _, p := range positions {
		key := p.MarketID + ":" + p.Side
		posMap[key] = &Position{
			MarketID:  p.MarketID,
			Side:      Side(p.Side),
			Shares:    p.Shares,
			AvgPrice:  p.AvgPrice,
			CostBasis: p.CostBasis,
		}
	}

	var tradeList []Trade
	for i, t := range trades {
		tradeList = append(tradeList, Trade{
			ID:        i + 1,
			MarketID:  t.MarketID,
			Side:      Side(t.Side),
			Amount:    t.Amount,
			Price:     t.Price,
			Shares:    t.Shares,
			Timestamp: t.Timestamp,
			Action:    t.Action,
		})
	}

	return &Portfolio{
		Balance:   e.balance,
		Positions: posMap,
		Trades:    tradeList,
	}
}

// GetBalance returns the current cash balance.
func (e *DBPaperEngine) GetBalance() float64 {
	return e.balance
}

// Reset resets the portfolio by closing all positions and creating a new session.
func (e *DBPaperEngine) Reset() error {
	ctx := context.Background()

	// Close all open positions for this session with zero PnL
	positions, err := e.db.GetOpenPolymarketPositionsBySession(ctx, e.sessionID)
	if err != nil {
		return fmt.Errorf("failed to get open positions: %w", err)
	}
	for _, p := range positions {
		if err := e.db.ClosePolymarketPosition(ctx, p.ID, 0); err != nil {
			return fmt.Errorf("failed to close position %s: %w", p.ID, err)
		}
	}

	// Stop old session
	_ = e.db.StopSession(ctx, e.sessionID, e.balance)

	// Create new session
	session := &db.TradingSession{
		ID:             uuid.New(),
		Mode:           db.TradingModePaper,
		Symbol:         "POLYMARKET",
		Exchange:       "polymarket",
		StartedAt:      time.Now(),
		InitialCapital: DefaultBalance,
	}
	if err := e.db.CreateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to create new session: %w", err)
	}

	e.sessionID = session.ID
	e.balance = DefaultBalance
	return nil
}

// SessionID returns the session UUID for this engine instance.
func (e *DBPaperEngine) SessionID() uuid.UUID {
	return e.sessionID
}

// findOpenPosition finds an open position by market ID and side within this session.
func (e *DBPaperEngine) findOpenPosition(ctx context.Context, marketID, side string) (*db.PolymarketPosition, error) {
	positions, err := e.db.GetOpenPolymarketPositionsBySession(ctx, e.sessionID)
	if err != nil {
		return nil, err
	}
	for _, p := range positions {
		if p.MarketID == marketID && p.Side == side {
			return p, nil
		}
	}
	return nil, fmt.Errorf("position not found")
}
