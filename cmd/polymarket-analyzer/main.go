// polymarket-analyzer runs LLM analysis on a Polymarket market.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/analyzer"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/gamma"
)

func main() {
	// Parse --db flag manually (before positional args)
	useDB := false
	var args []string
	for _, a := range os.Args[1:] {
		if a == "--db" {
			useDB = true
		} else {
			args = append(args, a)
		}
	}

	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: polymarket-analyzer [--db] <market_id>\n")
		fmt.Fprintf(os.Stderr, "  market_id: Polymarket market ID or condition ID\n")
		fmt.Fprintf(os.Stderr, "  --db:      Write prediction to database (requires DATABASE_URL)\n")
		os.Exit(1)
	}

	marketID := args[0]

	client := gamma.NewClient()
	fmt.Fprintf(os.Stderr, "Fetching market %s...\n", marketID)

	market, err := client.FetchMarketByID(marketID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching market: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Market: %s\n", market.Question)
	fmt.Fprintf(os.Stderr, "YES: %.2f | NO: %.2f\n", market.OutcomeYesPrice, market.OutcomeNoPrice)
	fmt.Fprintf(os.Stderr, "Running LLM analysis...\n\n")

	analysis, err := analyzer.Analyze(market)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(analysis)

	// Optionally write prediction to database
	if useDB {
		if err := writePredictionToDB(market, analysis); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write prediction to DB: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Prediction saved to database\n")
		}
	}

	// Human-readable summary to stderr
	fmt.Fprintf(os.Stderr, "\n=== Analysis ===\n")
	fmt.Fprintf(os.Stderr, "Predicted Probability: %.1f%%\n", analysis.PredictedProb*100)
	fmt.Fprintf(os.Stderr, "Market Price (YES):    %.1f%%\n", analysis.CurrentYes*100)
	fmt.Fprintf(os.Stderr, "Edge:                  %+.1f%%\n", analysis.Edge*100)
	fmt.Fprintf(os.Stderr, "Confidence:            %.1f%%\n", analysis.Confidence*100)
	fmt.Fprintf(os.Stderr, "Action:                %s\n", analysis.Action)
	fmt.Fprintf(os.Stderr, "Reasoning: %s\n", analysis.Reasoning)
}

func writePredictionToDB(market *gamma.Market, analysis *analyzer.Analysis) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database, err := db.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	// Ensure market exists in DB
	endDate, _ := market.ParsedEndDate()
	var endDatePtr *time.Time
	if !endDate.IsZero() {
		endDatePtr = &endDate
	}
	cat := market.Category

	if err := database.UpsertPolymarketMarket(ctx, &db.PolymarketMarket{
		ID:       market.ConditionID,
		Question: market.Question,
		Category: &cat,
		YesPrice: market.OutcomeYesPrice,
		NoPrice:  market.OutcomeNoPrice,
		Volume:   market.Volume,
		EndDate:  endDatePtr,
		Active:   market.Active && !market.Closed,
	}); err != nil {
		return fmt.Errorf("failed to upsert market: %w", err)
	}

	reasoning := analysis.Reasoning
	prediction := &db.PolymarketPrediction{
		MarketID:      market.ConditionID,
		PredictedProb: analysis.PredictedProb,
		MarketPrice:   analysis.CurrentYes,
		Edge:          analysis.Edge,
		Confidence:    analysis.Confidence,
		Category:      &cat,
		Reasoning:     &reasoning,
	}
	if err := database.InsertPolymarketPrediction(ctx, prediction); err != nil {
		return fmt.Errorf("failed to insert prediction: %w", err)
	}

	return nil
}
