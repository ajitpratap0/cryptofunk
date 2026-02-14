// polymarket-scanner fetches and filters active Polymarket markets.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/gamma"
)

type marketOutput struct {
	ID          string  `json:"id"`
	ConditionID string  `json:"condition_id"`
	Question    string  `json:"question"`
	Category    string  `json:"category"`
	YesPrice    float64 `json:"yes_price"`
	NoPrice     float64 `json:"no_price"`
	Volume      float64 `json:"volume"`
	Liquidity   float64 `json:"liquidity"`
	EndDate     string  `json:"end_date"`
	DaysLeft    int     `json:"days_left"`
}

func main() {
	category := flag.String("category", "", "Filter by category")
	minVolume := flag.Float64("min-volume", 0, "Minimum volume in USD")
	daysToRes := flag.Int("days-to-resolution", 0, "Max days to resolution (0=no filter)")
	jsonOutput := flag.Bool("json", true, "Output as JSON")
	limit := flag.Int("limit", 50, "Max markets to display")
	useDB := flag.Bool("db", false, "Write discovered markets to the database (requires DATABASE_URL)")
	flag.Parse()

	client := gamma.NewClient()
	fmt.Fprintf(os.Stderr, "Fetching markets from Polymarket...\n")

	markets, err := client.FetchAllMarkets()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Fetched %d markets, filtering...\n", len(markets))

	filtered := gamma.FilterMarkets(markets, gamma.FilterOptions{
		Category:         *category,
		MinVolume:        *minVolume,
		DaysToResolution: *daysToRes,
		BinaryOnly:       true,
	})

	// Sort by volume descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Volume > filtered[j].Volume
	})

	if len(filtered) > *limit {
		filtered = filtered[:*limit]
	}

	fmt.Fprintf(os.Stderr, "Found %d tradeable markets\n\n", len(filtered))

	// Optionally write to database
	if *useDB {
		if err := writeMarketsToDB(filtered); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write to DB: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Wrote %d markets to database\n", len(filtered))
		}
	}

	var output []marketOutput
	for _, m := range filtered {
		output = append(output, marketOutput{
			ID:          m.ID,
			ConditionID: m.ConditionID,
			Question:    m.Question,
			Category:    m.Category,
			YesPrice:    m.OutcomeYesPrice,
			NoPrice:     m.OutcomeNoPrice,
			Volume:      m.Volume,
			Liquidity:   m.Liquidity,
			EndDate:     m.EndDate,
			DaysLeft:    m.DaysToResolution(),
		})
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(output)
	} else {
		for _, m := range output {
			fmt.Printf("%-60s YES:%.2f NO:%.2f Vol:$%.0f Days:%d\n",
				truncate(m.Question, 60), m.YesPrice, m.NoPrice, m.Volume, m.DaysLeft)
		}
	}
}

func writeMarketsToDB(markets []gamma.Market) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database, err := db.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	for _, m := range markets {
		endDate, _ := m.ParsedEndDate()
		var endDatePtr *time.Time
		if !endDate.IsZero() {
			endDatePtr = &endDate
		}
		cat := m.Category

		dbMarket := &db.PolymarketMarket{
			ID:       m.ConditionID,
			Question: m.Question,
			Category: &cat,
			YesPrice: &m.OutcomeYesPrice,
			NoPrice:  &m.OutcomeNoPrice,
			Volume:   &m.Volume,
			EndDate:  endDatePtr,
			Active:   m.Active && !m.Closed,
		}
		if err := database.UpsertPolymarketMarket(ctx, dbMarket); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to upsert market %s: %v\n", m.ID, err)
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
