// polymarket-scanner fetches and filters active Polymarket markets.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
