// Package paper provides a paper trading engine for Polymarket.
package paper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const DefaultBalance = 100.0

// Side represents YES or NO.
type Side string

const (
	YES Side = "YES"
	NO  Side = "NO"
)

// Trade represents a single paper trade.
type Trade struct {
	ID        int       `json:"id"`
	MarketID  string    `json:"market_id"`
	Question  string    `json:"question"`
	Side      Side      `json:"side"`
	Amount    float64   `json:"amount"` // USD spent
	Price     float64   `json:"price"`  // price per share at time of trade
	Shares    float64   `json:"shares"` // shares acquired
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"` // BUY or SELL
}

// Position represents an open position.
type Position struct {
	MarketID  string  `json:"market_id"`
	Question  string  `json:"question"`
	Side      Side    `json:"side"`
	Shares    float64 `json:"shares"`
	AvgPrice  float64 `json:"avg_price"`
	CostBasis float64 `json:"cost_basis"`
}

// Key returns a unique key for the position.
func (p *Position) Key() string {
	return p.MarketID + ":" + string(p.Side)
}

// CurrentValue returns the current value at the given price.
func (p *Position) CurrentValue(currentPrice float64) float64 {
	return p.Shares * currentPrice
}

// PnL returns the unrealized P&L at the given price.
func (p *Position) PnL(currentPrice float64) float64 {
	return p.CurrentValue(currentPrice) - p.CostBasis
}

// Portfolio represents the paper trading portfolio.
type Portfolio struct {
	Balance   float64              `json:"balance"`
	Positions map[string]*Position `json:"positions"`
	Trades    []Trade              `json:"trades"`
	NextID    int                  `json:"next_id"`
}

// Engine is the paper trading engine.
type Engine struct {
	Portfolio *Portfolio
	FilePath  string
}

// NewEngine creates a new paper trading engine.
func NewEngine() *Engine {
	home, _ := os.UserHomeDir()
	fp := filepath.Join(home, ".cryptofunk", "paper_portfolio.json")
	e := &Engine{FilePath: fp}
	if err := e.Load(); err != nil {
		e.Portfolio = &Portfolio{
			Balance:   DefaultBalance,
			Positions: make(map[string]*Position),
			NextID:    1,
		}
	}
	return e
}

// Buy places a virtual buy order.
func (e *Engine) Buy(marketID, question string, side Side, amount, price float64) (*Trade, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	if price <= 0 || price >= 1 {
		return nil, fmt.Errorf("price must be between 0 and 1")
	}
	if amount > e.Portfolio.Balance {
		return nil, fmt.Errorf("insufficient balance: have $%.2f, need $%.2f", e.Portfolio.Balance, amount)
	}

	shares := amount / price
	trade := Trade{
		ID:        e.Portfolio.NextID,
		MarketID:  marketID,
		Question:  question,
		Side:      side,
		Amount:    amount,
		Price:     price,
		Shares:    shares,
		Timestamp: time.Now(),
		Action:    "BUY",
	}

	e.Portfolio.Balance -= amount
	e.Portfolio.NextID++

	key := marketID + ":" + string(side)
	pos, exists := e.Portfolio.Positions[key]
	if !exists {
		e.Portfolio.Positions[key] = &Position{
			MarketID:  marketID,
			Question:  question,
			Side:      side,
			Shares:    shares,
			AvgPrice:  price,
			CostBasis: amount,
		}
	} else {
		totalCost := pos.CostBasis + amount
		totalShares := pos.Shares + shares
		pos.AvgPrice = totalCost / totalShares
		pos.Shares = totalShares
		pos.CostBasis = totalCost
	}

	e.Portfolio.Trades = append(e.Portfolio.Trades, trade)
	return &trade, e.Save()
}

// Sell sells shares from a position.
func (e *Engine) Sell(marketID string, side Side, shares, price float64) (*Trade, error) {
	key := marketID + ":" + string(side)
	pos, exists := e.Portfolio.Positions[key]
	if !exists {
		return nil, fmt.Errorf("no position for %s %s", marketID, side)
	}
	if shares > pos.Shares {
		return nil, fmt.Errorf("insufficient shares: have %.2f, selling %.2f", pos.Shares, shares)
	}

	amount := shares * price
	trade := Trade{
		ID:        e.Portfolio.NextID,
		MarketID:  marketID,
		Question:  pos.Question,
		Side:      side,
		Amount:    amount,
		Price:     price,
		Shares:    shares,
		Timestamp: time.Now(),
		Action:    "SELL",
	}

	e.Portfolio.Balance += amount
	e.Portfolio.NextID++

	costReduced := (shares / pos.Shares) * pos.CostBasis
	pos.Shares -= shares
	pos.CostBasis -= costReduced
	if pos.Shares <= 0.0001 {
		delete(e.Portfolio.Positions, key)
	}

	e.Portfolio.Trades = append(e.Portfolio.Trades, trade)
	return &trade, e.Save()
}

// GetPortfolio returns a summary of the portfolio.
func (e *Engine) GetPortfolio() *Portfolio {
	return e.Portfolio
}

// Reset resets the portfolio to defaults.
func (e *Engine) Reset() error {
	e.Portfolio = &Portfolio{
		Balance:   DefaultBalance,
		Positions: make(map[string]*Position),
		NextID:    1,
	}
	return e.Save()
}

// Save persists the portfolio to disk.
func (e *Engine) Save() error {
	dir := filepath.Dir(e.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(e.Portfolio, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(e.FilePath, data, 0o644)
}

// Load loads the portfolio from disk.
func (e *Engine) Load() error {
	data, err := os.ReadFile(e.FilePath)
	if err != nil {
		return err
	}
	var p Portfolio
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	if p.Positions == nil {
		p.Positions = make(map[string]*Position)
	}
	e.Portfolio = &p
	return nil
}
