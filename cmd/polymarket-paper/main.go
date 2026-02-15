// polymarket-paper is an interactive CLI for paper trading on Polymarket.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/analyzer"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/gamma"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/paper"
)

// PaperTrader abstracts the paper trading engine so both file-based and DB-backed
// engines can be used interchangeably.
type PaperTrader interface {
	Buy(marketID, question string, side paper.Side, amount, price float64) (*paper.Trade, error)
	Sell(marketID string, side paper.Side, shares, price float64) (*paper.Trade, error)
	GetPortfolio() *paper.Portfolio
	GetBalance() float64
	Reset() error
}

func main() {
	// Check for --db flag
	useDB := false
	args := os.Args[1:]
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--db" {
			useDB = true
		} else {
			filtered = append(filtered, a)
		}
	}

	if len(filtered) < 1 {
		printUsage()
		os.Exit(1)
	}

	var engine PaperTrader
	if useDB {
		database, err := db.New(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
			fmt.Fprintf(os.Stderr, "Set DATABASE_URL or configure Vault credentials\n")
			os.Exit(1)
		}
		defer database.Close()

		dbEngine, err := paper.NewDBPaperEngine(database)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create DB engine: %v\n", err)
			os.Exit(1)
		}
		engine = dbEngine
	} else {
		engine = paper.NewEngine()
	}

	cmd := filtered[0]
	// Replace os.Args for subcommands that read from os.Args
	os.Args = append([]string{os.Args[0]}, filtered...)

	switch cmd {
	case "scan":
		cmdScan()
	case "analyze":
		cmdAnalyze()
	case "buy":
		cmdBuy(engine)
	case "sell":
		cmdSell(engine)
	case "portfolio":
		cmdPortfolio(engine)
	case "history":
		cmdHistory(engine)
	case "reset":
		cmdReset(engine)
	case "auto":
		cmdAuto(engine)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `polymarket-paper — Paper trading CLI for Polymarket

Commands:
  scan                              Show interesting markets
  analyze <market_id>               LLM analysis of a market
  buy <market_id> <YES|NO> <amount> Paper buy
  sell <market_id> <YES|NO> <shares> Paper sell
  portfolio                         Show current positions and P&L
  history                           Trade history
  reset                             Reset portfolio to $100
  auto                              Auto mode: scan → analyze → trade
`)
}

func cmdScan() {
	client := gamma.NewClient()
	fmt.Fprintf(os.Stderr, "Scanning markets...\n")

	markets, err := client.FetchAllMarkets()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	filtered := gamma.FilterMarkets(markets, gamma.FilterOptions{
		MinVolume:        1000,
		BinaryOnly:       true,
		DaysToResolution: 30,
	})

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Volume > filtered[j].Volume
	})

	if len(filtered) > 20 {
		filtered = filtered[:20]
	}

	for i, m := range filtered {
		fmt.Printf("%2d. [%s] %s\n    YES: $%.2f | NO: $%.2f | Vol: $%.0f | %d days\n\n",
			i+1, m.ID, m.Question, m.OutcomeYesPrice, m.OutcomeNoPrice, m.Volume, m.DaysToResolution())
	}
}

func cmdAnalyze() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: polymarket-paper analyze <market_id>\n")
		os.Exit(1)
	}

	client := gamma.NewClient()
	market, err := client.FetchMarketByID(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	analysis, err := analyzer.Analyze(market)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(analysis)
}

func cmdBuy(engine PaperTrader) {
	if len(os.Args) < 5 {
		fmt.Fprintf(os.Stderr, "Usage: polymarket-paper buy <market_id> <YES|NO> <amount>\n")
		os.Exit(1)
	}

	marketID := os.Args[2]
	side := paper.Side(strings.ToUpper(os.Args[3]))
	amount, err := strconv.ParseFloat(os.Args[4], 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid amount: %s\n", os.Args[4])
		os.Exit(1)
	}

	if side != paper.YES && side != paper.NO {
		fmt.Fprintf(os.Stderr, "Side must be YES or NO\n")
		os.Exit(1)
	}

	// Fetch current price
	client := gamma.NewClient()
	market, err := client.FetchMarketByID(marketID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching market: %v\n", err)
		os.Exit(1)
	}

	price := market.OutcomeYesPrice
	if side == paper.NO {
		price = market.OutcomeNoPrice
	}

	trade, err := engine.Buy(marketID, market.Question, side, amount, price)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Bought %.2f %s shares of \"%s\" at $%.2f ($%.2f spent)\n",
		trade.Shares, trade.Side, market.Question, trade.Price, trade.Amount)
	fmt.Printf("Balance: $%.2f\n", engine.GetBalance())
}

func cmdSell(engine PaperTrader) {
	if len(os.Args) < 5 {
		fmt.Fprintf(os.Stderr, "Usage: polymarket-paper sell <market_id> <YES|NO> <shares>\n")
		os.Exit(1)
	}

	marketID := os.Args[2]
	side := paper.Side(strings.ToUpper(os.Args[3]))
	shares, err := strconv.ParseFloat(os.Args[4], 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid shares: %s\n", os.Args[4])
		os.Exit(1)
	}

	client := gamma.NewClient()
	market, err := client.FetchMarketByID(marketID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	price := market.OutcomeYesPrice
	if side == paper.NO {
		price = market.OutcomeNoPrice
	}

	trade, err := engine.Sell(marketID, side, shares, price)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Sold %.2f %s shares at $%.2f ($%.2f received)\n",
		trade.Shares, trade.Side, trade.Price, trade.Amount)
	fmt.Printf("Balance: $%.2f\n", engine.GetBalance())
}

func cmdPortfolio(engine PaperTrader) {
	p := engine.GetPortfolio()
	fmt.Printf("💰 Paper Portfolio\n")
	fmt.Printf("==================\n")
	fmt.Printf("Cash Balance: $%.2f\n\n", p.Balance)

	if len(p.Positions) == 0 {
		fmt.Println("No open positions.")
		return
	}

	fmt.Println("Open Positions:")
	for _, pos := range p.Positions {
		fmt.Printf("  %s %s\n", pos.Side, pos.Question)
		fmt.Printf("    Shares: %.2f | Avg Price: $%.2f | Cost: $%.2f\n",
			pos.Shares, pos.AvgPrice, pos.CostBasis)
	}
	fmt.Printf("\nTotal Trades: %d\n", len(p.Trades))
}

func cmdHistory(engine PaperTrader) {
	p := engine.GetPortfolio()
	if len(p.Trades) == 0 {
		fmt.Println("No trades yet.")
		return
	}

	fmt.Println("Trade History:")
	for _, t := range p.Trades {
		fmt.Printf("  #%d %s %s %.2f shares @ $%.2f ($%.2f) — %s [%s]\n",
			t.ID, t.Action, t.Side, t.Shares, t.Price, t.Amount,
			t.Question, t.Timestamp.Format("2006-01-02 15:04"))
	}
}

func cmdReset(engine PaperTrader) {
	if err := engine.Reset(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("🔄 Portfolio reset to $100.00")
}

func cmdAuto(engine PaperTrader) {
	fmt.Println("🤖 Auto Mode: Scanning → Analyzing → Trading")
	fmt.Println()

	client := gamma.NewClient()
	markets, err := client.FetchAllMarkets()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	filtered := gamma.FilterMarkets(markets, gamma.FilterOptions{
		MinVolume:        5000,
		BinaryOnly:       true,
		DaysToResolution: 14,
	})

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Volume > filtered[j].Volume
	})

	if len(filtered) > 10 {
		filtered = filtered[:10]
	}

	fmt.Printf("Found %d candidate markets\n\n", len(filtered))

	for i, m := range filtered {
		fmt.Printf("--- Market %d/%d ---\n", i+1, len(filtered))
		fmt.Printf("Q: %s\n", m.Question)
		fmt.Printf("YES: $%.2f | NO: $%.2f\n", m.OutcomeYesPrice, m.OutcomeNoPrice)

		analysis, err := analyzer.Analyze(&m)
		if err != nil {
			fmt.Printf("⚠️  Analysis failed: %v\n\n", err)
			continue
		}

		fmt.Printf("Predicted: %.1f%% | Edge: %+.1f%% | Action: %s\n",
			analysis.PredictedProb*100, analysis.Edge*100, analysis.Action)

		if analysis.Action != "SKIP" && analysis.Confidence >= 0.6 {
			edge := analysis.Edge
			if edge < 0 {
				edge = -edge
			}
			if edge >= 0.10 {
				// Size based on edge: 5-15% of balance
				sizePct := 0.05 + (edge * 0.5)
				if sizePct > 0.15 {
					sizePct = 0.15
				}
				amount := engine.GetBalance() * sizePct

				side := paper.YES
				price := m.OutcomeYesPrice
				if analysis.Action == "BUY NO" {
					side = paper.NO
					price = m.OutcomeNoPrice
				}

				trade, err := engine.Buy(m.ID, m.Question, side, amount, price)
				if err != nil {
					fmt.Printf("⚠️  Trade failed: %v\n", err)
				} else {
					fmt.Printf("✅ AUTO: Bought %.2f %s shares ($%.2f)\n",
						trade.Shares, trade.Side, trade.Amount)
				}
			}
		}
		fmt.Println()
	}

	fmt.Printf("\n💰 Balance: $%.2f | Positions: %d\n",
		engine.GetBalance(), len(engine.GetPortfolio().Positions))
}
