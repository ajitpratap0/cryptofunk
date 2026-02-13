// Package gamma provides a client for the Polymarket Gamma API.
package gamma

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const BaseURL = "https://gamma-api.polymarket.com"

// Market represents a Polymarket market from the Gamma API.
type Market struct {
	ID              string  `json:"id"`
	Question        string  `json:"question"`
	ConditionID     string  `json:"conditionId"`
	Slug            string  `json:"slug"`
	Active          bool    `json:"active"`
	Closed          bool    `json:"closed"`
	AcceptingOrders bool    `json:"acceptingOrders"`
	Category        string  `json:"category"`
	EndDate         string  `json:"endDate"`
	Volume          float64 `json:"volume"`
	Liquidity       float64 `json:"liquidity"`
	OutcomeYesPrice float64 `json:"-"`
	OutcomeNoPrice  float64 `json:"-"`
	OutcomePrices   string  `json:"outcomePrices"`
	Outcomes        string  `json:"outcomes"`
	Description     string  `json:"description"`
	Resolution      string  `json:"resolution"`
	ResolvedAt      string  `json:"resolvedAt"`
}

// ParsedEndDate returns the parsed end date.
func (m *Market) ParsedEndDate() (time.Time, error) {
	if m.EndDate == "" {
		return time.Time{}, fmt.Errorf("no end date")
	}
	// Try multiple formats
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, m.EndDate); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse end date: %s", m.EndDate)
}

// DaysToResolution returns days until the market resolves.
func (m *Market) DaysToResolution() int {
	t, err := m.ParsedEndDate()
	if err != nil {
		return -1
	}
	d := time.Until(t).Hours() / 24
	if d < 0 {
		return 0
	}
	return int(d)
}

// ParseOutcomePrices parses the outcome prices JSON string.
func (m *Market) ParseOutcomePrices() {
	if m.OutcomePrices == "" {
		return
	}
	var prices []float64
	// Try as JSON array of strings first, then floats
	var strPrices []string
	if err := json.Unmarshal([]byte(m.OutcomePrices), &strPrices); err == nil {
		for _, s := range strPrices {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				prices = append(prices, f)
			}
		}
	} else {
		_ = json.Unmarshal([]byte(m.OutcomePrices), &prices)
	}
	if len(prices) >= 1 {
		m.OutcomeYesPrice = prices[0]
	}
	if len(prices) >= 2 {
		m.OutcomeNoPrice = prices[1]
	}
}

// IsBinary returns true if the market has exactly 2 outcomes.
func (m *Market) IsBinary() bool {
	var outcomes []string
	if err := json.Unmarshal([]byte(m.Outcomes), &outcomes); err != nil {
		return false
	}
	return len(outcomes) == 2
}

// Client is a Gamma API client.
type Client struct {
	HTTPClient *http.Client
	BaseURL    string
}

// NewClient creates a new Gamma API client.
func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		BaseURL:    BaseURL,
	}
}

// FetchMarkets fetches markets from the Gamma API with pagination.
func (c *Client) FetchMarkets(limit, offset int) ([]Market, error) {
	u, _ := url.Parse(c.BaseURL + "/markets")
	q := u.Query()
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	q.Set("active", "true")
	q.Set("closed", "false")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch markets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gamma API returned %d: %s", resp.StatusCode, string(body))
	}

	var markets []Market
	if err := json.NewDecoder(resp.Body).Decode(&markets); err != nil {
		return nil, fmt.Errorf("decode markets: %w", err)
	}

	for i := range markets {
		markets[i].ParseOutcomePrices()
	}
	return markets, nil
}

// FetchAllMarkets fetches all active markets with pagination.
func (c *Client) FetchAllMarkets() ([]Market, error) {
	var all []Market
	limit := 100
	offset := 0
	for {
		markets, err := c.FetchMarkets(limit, offset)
		if err != nil {
			return nil, err
		}
		if len(markets) == 0 {
			break
		}
		all = append(all, markets...)
		if len(markets) < limit {
			break
		}
		offset += limit
	}
	return all, nil
}

// FetchMarketByID fetches a single market by condition ID or slug.
func (c *Client) FetchMarketByID(id string) (*Market, error) {
	u := c.BaseURL + "/markets/" + id
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch market: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try by condition ID via query
		return c.fetchMarketByConditionID(id)
	}

	var m Market
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("decode market: %w", err)
	}
	m.ParseOutcomePrices()
	return &m, nil
}

func (c *Client) fetchMarketByConditionID(conditionID string) (*Market, error) {
	u, _ := url.Parse(c.BaseURL + "/markets")
	q := u.Query()
	q.Set("condition_id", conditionID)
	q.Set("limit", "1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch by condition: %w", err)
	}
	defer resp.Body.Close()

	var markets []Market
	if err := json.NewDecoder(resp.Body).Decode(&markets); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(markets) == 0 {
		return nil, fmt.Errorf("market not found: %s", conditionID)
	}
	markets[0].ParseOutcomePrices()
	return &markets[0], nil
}

// FilterOptions for filtering markets.
type FilterOptions struct {
	Category         string
	MinVolume        float64
	DaysToResolution int // max days, 0 = no filter
	BinaryOnly       bool
}

// FilterMarkets filters markets by the given options.
func FilterMarkets(markets []Market, opts FilterOptions) []Market {
	var result []Market
	for _, m := range markets {
		if !m.Active || m.Closed {
			continue
		}
		if opts.BinaryOnly && !m.IsBinary() {
			continue
		}
		if opts.MinVolume > 0 && m.Volume < opts.MinVolume {
			continue
		}
		if opts.Category != "" && m.Category != opts.Category {
			continue
		}
		if opts.DaysToResolution > 0 {
			d := m.DaysToResolution()
			if d < 0 || d > opts.DaysToResolution {
				continue
			}
		}
		result = append(result, m)
	}
	return result
}
