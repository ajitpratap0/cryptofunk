package strategy

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// NewsFetcher fetches relevant news for market analysis.
type NewsFetcher struct {
	httpClient *http.Client
	cache      map[string]*newsCache
	mu         sync.RWMutex
	cacheTTL   time.Duration
}

type newsCache struct {
	items     []NewsItem
	fetchedAt time.Time
}

// NewNewsFetcher creates a new news fetcher.
func NewNewsFetcher(cacheTTL time.Duration) *NewsFetcher {
	if cacheTTL == 0 {
		cacheTTL = 15 * time.Minute
	}
	return &NewsFetcher{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache:      make(map[string]*newsCache),
		cacheTTL:   cacheTTL,
	}
}

// FetchNews fetches recent news relevant to a market question.
func (nf *NewsFetcher) FetchNews(ctx context.Context, query string, category string) ([]NewsItem, error) {
	cacheKey := normalizeQuery(query)

	// Check cache
	nf.mu.RLock()
	if cached, ok := nf.cache[cacheKey]; ok && time.Since(cached.fetchedAt) < nf.cacheTTL {
		nf.mu.RUnlock()
		return cached.items, nil
	}
	nf.mu.RUnlock()

	// Fetch from Google News RSS
	items, err := nf.fetchGoogleNews(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("google news fetch failed: %w", err)
	}

	// Score relevance
	for i := range items {
		items[i].RelevanceScore = scoreRelevance(items[i], query)
	}

	// Sort by relevance (simple bubble — small list)
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].RelevanceScore > items[i].RelevanceScore {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	// Cap at 10
	if len(items) > 10 {
		items = items[:10]
	}

	// Cache
	nf.mu.Lock()
	nf.cache[cacheKey] = &newsCache{items: items, fetchedAt: time.Now()}
	nf.mu.Unlock()

	return items, nil
}

// ClearCache clears the news cache.
func (nf *NewsFetcher) ClearCache() {
	nf.mu.Lock()
	nf.cache = make(map[string]*newsCache)
	nf.mu.Unlock()
}

// googleRSS types
type rssResponse struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
	Source  string `xml:"source"`
}

func (nf *NewsFetcher) fetchGoogleNews(ctx context.Context, query string) ([]NewsItem, error) {
	// Extract key terms for search (first 8 words)
	searchQuery := extractSearchTerms(query)
	u := fmt.Sprintf("https://news.google.com/rss/search?q=%s&hl=en-US&gl=US&ceid=US:en",
		url.QueryEscape(searchQuery))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CryptoFunk/1.0")

	resp, err := nf.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google news returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, err
	}

	var rss rssResponse
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("failed to parse RSS: %w", err)
	}

	var items []NewsItem
	for _, item := range rss.Channel.Items {
		source := item.Source
		if source == "" {
			source = "Google News"
		}
		items = append(items, NewsItem{
			Title:   item.Title,
			Source:  source,
			Date:    item.PubDate,
			URL:     item.Link,
			Summary: item.Title, // RSS titles often are the summary
		})
	}

	return items, nil
}

func extractSearchTerms(question string) string {
	// Remove common question words and limit to key terms
	words := strings.Fields(question)
	stopWords := map[string]bool{
		"will": true, "the": true, "be": true, "a": true, "an": true,
		"is": true, "are": true, "was": true, "were": true, "do": true,
		"does": true, "did": true, "has": true, "have": true, "had": true,
		"by": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "before": true, "after": true,
		"?": true, "this": true, "that": true, "it": true,
	}

	var terms []string
	for _, w := range words {
		cleaned := strings.ToLower(strings.Trim(w, "?.,!"))
		if !stopWords[cleaned] && len(cleaned) > 1 {
			terms = append(terms, cleaned)
		}
		if len(terms) >= 8 {
			break
		}
	}
	return strings.Join(terms, " ")
}

func normalizeQuery(q string) string {
	return strings.ToLower(strings.TrimSpace(q))
}

func scoreRelevance(item NewsItem, query string) float64 {
	title := strings.ToLower(item.Title)
	queryWords := strings.Fields(strings.ToLower(query))

	matches := 0
	for _, w := range queryWords {
		w = strings.Trim(w, "?.,!")
		if len(w) > 2 && strings.Contains(title, w) {
			matches++
		}
	}

	if len(queryWords) == 0 {
		return 0.5
	}
	return float64(matches) / float64(len(queryWords))
}
