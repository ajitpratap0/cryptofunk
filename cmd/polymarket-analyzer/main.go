// polymarket-analyzer runs LLM analysis on a Polymarket market.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ajitpratap0/cryptofunk/internal/polymarket/analyzer"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/gamma"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: polymarket-analyzer <market_id>\n")
		fmt.Fprintf(os.Stderr, "  market_id: Polymarket market ID or condition ID\n")
		os.Exit(1)
	}

	marketID := os.Args[1]

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

	// Human-readable summary to stderr
	fmt.Fprintf(os.Stderr, "\n=== Analysis ===\n")
	fmt.Fprintf(os.Stderr, "Predicted Probability: %.1f%%\n", analysis.PredictedProb*100)
	fmt.Fprintf(os.Stderr, "Market Price (YES):    %.1f%%\n", analysis.CurrentYes*100)
	fmt.Fprintf(os.Stderr, "Edge:                  %+.1f%%\n", analysis.Edge*100)
	fmt.Fprintf(os.Stderr, "Confidence:            %.1f%%\n", analysis.Confidence*100)
	fmt.Fprintf(os.Stderr, "Action:                %s\n", analysis.Action)
	fmt.Fprintf(os.Stderr, "Reasoning: %s\n", analysis.Reasoning)
}
