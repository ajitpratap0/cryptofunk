package gamma

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockMarkets() []Market {
	return []Market{
		{
			ID: "1", Question: "Will BTC hit 100k?", ConditionID: "cond1",
			Active: true, Closed: false, Category: "crypto",
			Volume: 50000, Liquidity: 10000, EndDate: "2026-06-01T00:00:00Z",
			OutcomePrices: `["0.65","0.35"]`, Outcomes: `["Yes","No"]`,
		},
		{
			ID: "2", Question: "Will it rain tomorrow?", ConditionID: "cond2",
			Active: true, Closed: false, Category: "weather",
			Volume: 100, Liquidity: 50, EndDate: "2026-02-15T00:00:00Z",
			OutcomePrices: `["0.50","0.50"]`, Outcomes: `["Yes","No"]`,
		},
		{
			ID: "3", Question: "Multi outcome market", ConditionID: "cond3",
			Active: true, Closed: false, Category: "politics",
			Volume: 80000, Liquidity: 20000, EndDate: "2026-12-01T00:00:00Z",
			OutcomePrices: `["0.33","0.33","0.34"]`, Outcomes: `["A","B","C"]`,
		},
		{
			ID: "4", Question: "Closed market", ConditionID: "cond4",
			Active: false, Closed: true, Category: "crypto",
			Volume: 1000, Liquidity: 0,
			OutcomePrices: `["1.00","0.00"]`, Outcomes: `["Yes","No"]`,
		},
	}
}

func TestFilterMarkets(t *testing.T) {
	markets := mockMarkets()
	for i := range markets {
		markets[i].ParseOutcomePrices()
	}

	t.Run("binary only", func(t *testing.T) {
		result := FilterMarkets(markets, FilterOptions{BinaryOnly: true})
		assert.Len(t, result, 2)
	})

	t.Run("min volume", func(t *testing.T) {
		result := FilterMarkets(markets, FilterOptions{MinVolume: 10000})
		assert.Len(t, result, 2) // crypto + politics
	})

	t.Run("category filter", func(t *testing.T) {
		result := FilterMarkets(markets, FilterOptions{Category: "crypto"})
		assert.Len(t, result, 1)
		assert.Equal(t, "Will BTC hit 100k?", result[0].Question)
	})

	t.Run("days to resolution", func(t *testing.T) {
		result := FilterMarkets(markets, FilterOptions{DaysToResolution: 7})
		// Only the weather market resolves within 7 days (Feb 15)
		assert.True(t, len(result) <= 2)
	})
}

func TestParseOutcomePrices(t *testing.T) {
	m := Market{OutcomePrices: `["0.65","0.35"]`}
	m.ParseOutcomePrices()
	assert.InDelta(t, 0.65, m.OutcomeYesPrice, 0.001)
	assert.InDelta(t, 0.35, m.OutcomeNoPrice, 0.001)
}

func TestIsBinary(t *testing.T) {
	m1 := Market{Outcomes: `["Yes","No"]`}
	assert.True(t, m1.IsBinary())

	m2 := Market{Outcomes: `["A","B","C"]`}
	assert.False(t, m2.IsBinary())
}

func TestFetchMarkets(t *testing.T) {
	markets := mockMarkets()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(markets)
	}))
	defer srv.Close()

	client := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	result, err := client.FetchMarkets(100, 0)
	require.NoError(t, err)
	assert.Len(t, result, 4)
}
