// Package resolver checks Polymarket markets for resolution and updates predictions.
package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/gamma"
)

// Resolver resolves Polymarket predictions by checking market outcomes.
type Resolver struct {
	db    *db.DB
	gamma *gamma.Client
}

// NewResolver creates a new Resolver.
func NewResolver(database *db.DB, gammaClient *gamma.Client) *Resolver {
	return &Resolver{
		db:    database,
		gamma: gammaClient,
	}
}

// ResolveMarkets checks all unresolved predictions and resolves those whose markets have settled.
func (r *Resolver) ResolveMarkets(ctx context.Context) (int, error) {
	predictions, err := r.db.GetUnresolvedPolymarketPredictions(ctx)
	if err != nil {
		return 0, fmt.Errorf("get unresolved predictions: %w", err)
	}

	resolved := 0
	for _, pred := range predictions {
		isResolved, outcomePrice, err := r.CheckMarketResolution(ctx, pred.MarketID)
		if err != nil {
			log.Warn().Err(err).Str("market_id", pred.MarketID).Msg("Failed to check market resolution")
			continue
		}
		if !isResolved {
			continue
		}

		// Calculate P&L: if we predicted YES (prob > market_price), outcome=1.0 means win
		// Edge was calculated as predicted_prob - market_price
		// PnL = edge * direction_correctness
		pnl := calculatePredictionPnl(pred.PredictedProb, pred.MarketPrice, outcomePrice)

		if err := r.db.ResolvePrediction(ctx, pred.ID, outcomePrice, pnl); err != nil {
			log.Error().Err(err).Str("prediction_id", pred.ID.String()).Msg("Failed to resolve prediction")
			continue
		}

		resolved++
		log.Info().
			Str("prediction_id", pred.ID.String()).
			Str("market_id", pred.MarketID).
			Float64("outcome", outcomePrice).
			Float64("pnl", pnl).
			Msg("Resolved prediction")
	}

	return resolved, nil
}

// CheckMarketResolution checks if a market has resolved on Polymarket.
func (r *Resolver) CheckMarketResolution(ctx context.Context, marketID string) (resolved bool, outcome float64, err error) {
	market, err := r.gamma.FetchMarketByID(marketID)
	if err != nil {
		return false, 0, fmt.Errorf("fetch market %s: %w", marketID, err)
	}

	// Check if market is resolved via the Resolution field
	if market.Resolution == "" {
		return false, 0, nil
	}

	// Resolution can be "Yes", "No", or a price
	resolution := strings.ToLower(strings.TrimSpace(market.Resolution))
	switch resolution {
	case "yes":
		return true, 1.0, nil
	case "no":
		return true, 0.0, nil
	default:
		// Market not yet resolved or ambiguous
		if market.Closed && market.Resolution != "" {
			// Try parsing as a numeric outcome
			return true, market.OutcomeYesPrice, nil
		}
		return false, 0, nil
	}
}

// calculatePredictionPnl calculates P&L for a prediction.
// predictedProb: our predicted probability (0-1)
// marketPrice: the market price when we made the prediction (0-1)
// outcome: the actual outcome (0 or 1)
func calculatePredictionPnl(predictedProb, marketPrice, outcome float64) float64 {
	// If we thought YES was underpriced (predicted > market), we'd "buy YES"
	// If we thought YES was overpriced (predicted < market), we'd "buy NO"
	if predictedProb > marketPrice {
		// We would have bought YES at marketPrice
		// Payout: outcome * 1.0, cost: marketPrice
		return outcome - marketPrice
	}
	// We would have bought NO at (1 - marketPrice)
	// Payout: (1 - outcome) * 1.0, cost: (1 - marketPrice)
	return (1 - outcome) - (1 - marketPrice)
}

// ResolveDecisionsFromPositions resolves Binance decisions linked to closed positions.
func (r *Resolver) ResolveDecisionsFromPositions(ctx context.Context) (int, error) {
	return r.db.ResolveDecisionsFromPositions(ctx)
}

// ResolveAll runs all resolution: Polymarket predictions + Binance decision-positions.
func (r *Resolver) ResolveAll(ctx context.Context) (polyResolved int, binanceResolved int, err error) {
	polyResolved, err = r.ResolveMarkets(ctx)
	if err != nil {
		return polyResolved, 0, fmt.Errorf("resolve polymarket: %w", err)
	}

	binanceResolved, err = r.ResolveDecisionsFromPositions(ctx)
	if err != nil {
		return polyResolved, binanceResolved, fmt.Errorf("resolve binance: %w", err)
	}

	return polyResolved, binanceResolved, nil
}
