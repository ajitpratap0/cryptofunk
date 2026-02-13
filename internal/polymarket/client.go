package polymarket

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	crand "crypto/rand"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

const (
	DefaultClobHost  = "https://clob.polymarket.com"
	DefaultGammaHost = "https://gamma-api.polymarket.com"
	DefaultChainID   = 137

	// Polymarket contract addresses on Polygon
	CTFExchangeAddress  = "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E"
	NegRiskExchangeAddr = "0xC5d563A36AE78145C45a50134d48A1215220f80a"

	// Rate limiting
	maxRequestsPerSecond = 10
)

// Header keys
const (
	headerPolyAddress    = "POLY_ADDRESS"
	headerPolySignature  = "POLY_SIGNATURE"
	headerPolyTimestamp  = "POLY_TIMESTAMP"
	headerPolyNonce      = "POLY_NONCE"
	headerPolyAPIKey     = "POLY_API_KEY"
	headerPolyPassphrase = "POLY_PASSPHRASE"
)

// Client is the Polymarket CLOB client
type Client struct {
	clobHost  string
	gammaHost string
	chainID   int
	signer    *Signer
	creds     *ApiCreds
	funder    string

	httpClient  *http.Client
	logger      zerolog.Logger
	rateLimiter *rate.Limiter
	mu          sync.Mutex
}

// ClientOption configures the client
type ClientOption func(*Client)

// WithClobHost sets a custom CLOB host
func WithClobHost(host string) ClientOption {
	return func(c *Client) { c.clobHost = strings.TrimRight(host, "/") }
}

// WithGammaHost sets a custom Gamma API host
func WithGammaHost(host string) ClientOption {
	return func(c *Client) { c.gammaHost = strings.TrimRight(host, "/") }
}

// WithChainID sets the chain ID
func WithChainID(id int) ClientOption {
	return func(c *Client) { c.chainID = id }
}

// WithCreds sets L2 API credentials
func WithCreds(creds *ApiCreds) ClientOption {
	return func(c *Client) { c.creds = creds }
}

// WithFunder sets the funder address for order signing
func WithFunder(addr string) ClientOption {
	return func(c *Client) { c.funder = addr }
}

// WithLogger sets the logger
func WithLogger(logger zerolog.Logger) ClientOption {
	return func(c *Client) { c.logger = logger }
}

// NewClient creates a new Polymarket CLOB client
func NewClient(privateKey string, opts ...ClientOption) (*Client, error) {
	c := &Client{
		clobHost:    DefaultClobHost,
		gammaHost:   DefaultGammaHost,
		chainID:     DefaultChainID,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: rate.NewLimiter(rate.Limit(maxRequestsPerSecond), maxRequestsPerSecond),
		logger:      zerolog.Nop(),
	}

	for _, opt := range opts {
		opt(c)
	}

	if privateKey != "" {
		signer, err := NewSigner(privateKey, c.chainID)
		if err != nil {
			return nil, fmt.Errorf("create signer: %w", err)
		}
		c.signer = signer
	}

	return c, nil
}

// Close cleans up resources
func (c *Client) Close() {
	if c.signer != nil {
		c.signer.Destroy()
	}
}

// GetAddress returns the signer's address
func (c *Client) GetAddress() string {
	if c.signer == nil {
		return ""
	}
	return c.signer.Address().Hex()
}

// --- L1 Auth (EIP-712 signature) ---

func (c *Client) l1Headers() (map[string]string, error) {
	if c.signer == nil {
		return nil, fmt.Errorf("signer required for L1 auth")
	}
	ts := time.Now().Unix()
	sig, err := c.signer.SignClobAuthMessage(ts, 0)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		headerPolyAddress:   c.signer.Address().Hex(),
		headerPolySignature: sig,
		headerPolyTimestamp: strconv.FormatInt(ts, 10),
		headerPolyNonce:     "0",
	}, nil
}

// --- L2 Auth (HMAC with API creds) ---

func (c *Client) l2Headers(method, path, body string) (map[string]string, error) {
	if c.signer == nil || c.creds == nil {
		return nil, fmt.Errorf("signer and API creds required for L2 auth")
	}
	ts := time.Now().Unix()
	hmacSig, err := BuildHMACSignature(c.creds.Secret, ts, method, path, body)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		headerPolyAddress:    c.signer.Address().Hex(),
		headerPolySignature:  hmacSig,
		headerPolyTimestamp:  strconv.FormatInt(ts, 10),
		headerPolyAPIKey:     c.creds.ApiKey,
		headerPolyPassphrase: c.creds.Passphrase,
	}, nil
}

// --- HTTP helpers ---

func (c *Client) rateLimit(ctx context.Context) error {
	return c.rateLimiter.Wait(ctx)
}

func (c *Client) doRequest(ctx context.Context, method, url string, body io.Reader, headers map[string]string) ([]byte, error) {
	if err := c.rateLimit(ctx); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

func (c *Client) get(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, url, nil, headers)
}

func (c *Client) post(ctx context.Context, url string, payload interface{}, headers map[string]string) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	return c.doRequest(ctx, http.MethodPost, url, body, headers)
}

func (c *Client) delete(ctx context.Context, url string, payload interface{}, headers map[string]string) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	return c.doRequest(ctx, http.MethodDelete, url, body, headers)
}

// --- API Credentials ---

// CreateApiKey creates a new CLOB API key (L1 auth)
func (c *Client) CreateApiKey(ctx context.Context) (*ApiCreds, error) {
	headers, err := c.l1Headers()
	if err != nil {
		return nil, err
	}
	data, err := c.post(ctx, c.clobHost+"/auth/api-key", nil, headers)
	if err != nil {
		return nil, err
	}
	var creds ApiCreds
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

// DeriveApiKey derives existing API credentials (L1 auth)
func (c *Client) DeriveApiKey(ctx context.Context) (*ApiCreds, error) {
	headers, err := c.l1Headers()
	if err != nil {
		return nil, err
	}
	data, err := c.get(ctx, c.clobHost+"/auth/derive-api-key", headers)
	if err != nil {
		return nil, err
	}
	var creds ApiCreds
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

// CreateOrDeriveApiCreds creates or derives API credentials
func (c *Client) CreateOrDeriveApiCreds(ctx context.Context) (*ApiCreds, error) {
	creds, err := c.CreateApiKey(ctx)
	if err != nil {
		return c.DeriveApiKey(ctx)
	}
	return creds, nil
}

// SetApiCreds sets the L2 credentials
func (c *Client) SetApiCreds(creds *ApiCreds) {
	c.creds = creds
}

// --- Market Data (unauthenticated) ---

// GetMarkets fetches markets from the Gamma API
func (c *Client) GetMarkets(ctx context.Context) ([]Market, error) {
	data, err := c.get(ctx, c.gammaHost+"/markets?active=true&closed=false&limit=100", nil)
	if err != nil {
		return nil, err
	}
	var markets []Market
	if err := json.Unmarshal(data, &markets); err != nil {
		return nil, fmt.Errorf("decode markets: %w", err)
	}
	return markets, nil
}

// GetMarket fetches a single market by condition ID
func (c *Client) GetMarket(ctx context.Context, conditionID string) (*Market, error) {
	data, err := c.get(ctx, c.gammaHost+"/markets?condition_id="+conditionID, nil)
	if err != nil {
		return nil, err
	}
	var markets []Market
	if err := json.Unmarshal(data, &markets); err != nil {
		return nil, err
	}
	if len(markets) == 0 {
		return nil, fmt.Errorf("market not found: %s", conditionID)
	}
	return &markets[0], nil
}

// GetOrderBook fetches the order book for a token
func (c *Client) GetOrderBook(ctx context.Context, tokenID string) (*OrderBook, error) {
	data, err := c.get(ctx, c.clobHost+"/book?token_id="+tokenID, nil)
	if err != nil {
		return nil, err
	}
	var book OrderBook
	if err := json.Unmarshal(data, &book); err != nil {
		return nil, err
	}
	return &book, nil
}

// GetMidpoint returns the midpoint price for a token
func (c *Client) GetMidpoint(ctx context.Context, tokenID string) (string, error) {
	data, err := c.get(ctx, c.clobHost+"/midpoint?token_id="+tokenID, nil)
	if err != nil {
		return "", err
	}
	var resp MidpointResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	return resp.Mid, nil
}

// GetPrice returns the best price for a token and side
func (c *Client) GetPrice(ctx context.Context, tokenID string, side Side) (string, error) {
	data, err := c.get(ctx, c.clobHost+"/price?token_id="+tokenID+"&side="+string(side), nil)
	if err != nil {
		return "", err
	}
	var resp PriceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	return resp.Price, nil
}

// --- Trading (L2 auth required) ---

// CreateOrder creates and signs a limit order, then submits it
func (c *Client) CreateOrder(ctx context.Context, tokenID string, side Side, price, size float64) (*PostOrderResponse, error) {
	if c.signer == nil || c.creds == nil {
		return nil, fmt.Errorf("L2 auth required for trading")
	}

	// Build the signed order
	order, err := c.buildOrder(tokenID, side, price, size)
	if err != nil {
		return nil, fmt.Errorf("build order: %w", err)
	}

	reqBody := PostOrderRequest{
		Order:     *order,
		Owner:     c.signer.Address().Hex(),
		OrderType: GTC,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	headers, err := c.l2Headers(http.MethodPost, "/order", string(bodyBytes))
	if err != nil {
		return nil, err
	}

	data, err := c.post(ctx, c.clobHost+"/order", reqBody, headers)
	if err != nil {
		return nil, err
	}

	var resp PostOrderResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) buildOrder(tokenID string, side Side, price, size float64) (*SignedOrder, error) {
	// USDC has 6 decimals
	usdcDecimals := new(big.Int).Exp(big.NewInt(10), big.NewInt(6), nil)

	priceBig := new(big.Float).SetFloat64(price)
	sizeBig := new(big.Float).SetFloat64(size)

	var makerAmount, takerAmount *big.Int

	if side == Buy {
		// Maker pays USDC, taker delivers tokens
		cost := new(big.Float).Mul(priceBig, sizeBig)
		costScaled := new(big.Float).Mul(cost, new(big.Float).SetInt(usdcDecimals))
		makerAmount, _ = costScaled.Int(nil)

		sizeScaled := new(big.Float).Mul(sizeBig, new(big.Float).SetInt(usdcDecimals))
		takerAmount, _ = sizeScaled.Int(nil)
	} else {
		// Maker delivers tokens, taker pays USDC
		sizeScaled := new(big.Float).Mul(sizeBig, new(big.Float).SetInt(usdcDecimals))
		makerAmount, _ = sizeScaled.Int(nil)

		cost := new(big.Float).Mul(priceBig, sizeBig)
		costScaled := new(big.Float).Mul(cost, new(big.Float).SetInt(usdcDecimals))
		takerAmount, _ = costScaled.Int(nil)
	}

	maker := c.signer.Address().Hex()
	signer := maker
	if c.funder != "" {
		maker = c.funder
	}

	var saltBytes [32]byte
	if _, err := crand.Read(saltBytes[:]); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	salt := hex.EncodeToString(saltBytes[:])

	order := &SignedOrder{
		Salt:          salt,
		Maker:         maker,
		Signer:        signer,
		Taker:         common.Address{}.Hex(), // zero address = public
		TokenID:       tokenID,
		MakerAmount:   makerAmount.String(),
		TakerAmount:   takerAmount.String(),
		Expiration:    "0", // no expiration
		Nonce:         "0",
		FeeRateBps:    "0",
		Side:          string(side),
		SignatureType: 0, // EOA
	}

	// Determine exchange address (would need neg_risk info; default to standard)
	sig, err := c.signer.SignOrder(order, CTFExchangeAddress, false)
	if err != nil {
		return nil, err
	}
	order.Signature = sig

	return order, nil
}

// CancelOrder cancels an order by ID
func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
	if c.creds == nil {
		return fmt.Errorf("L2 auth required")
	}

	reqBody := CancelOrderRequest{OrderID: orderID}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	headers, err := c.l2Headers(http.MethodDelete, "/order", string(bodyBytes))
	if err != nil {
		return err
	}

	_, err = c.delete(ctx, c.clobHost+"/order", reqBody, headers)
	return err
}

// GetOpenOrders returns open orders (L2 auth)
func (c *Client) GetOpenOrders(ctx context.Context) ([]Order, error) {
	if c.creds == nil {
		return nil, fmt.Errorf("L2 auth required")
	}

	headers, err := c.l2Headers(http.MethodGet, "/data/orders", "")
	if err != nil {
		return nil, err
	}

	data, err := c.get(ctx, c.clobHost+"/data/orders", headers)
	if err != nil {
		return nil, err
	}

	var orders []Order
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

// GetTrades returns trade history (L2 auth)
func (c *Client) GetTrades(ctx context.Context) ([]Trade, error) {
	if c.creds == nil {
		return nil, fmt.Errorf("L2 auth required")
	}

	headers, err := c.l2Headers(http.MethodGet, "/data/trades", "")
	if err != nil {
		return nil, err
	}

	data, err := c.get(ctx, c.clobHost+"/data/trades", headers)
	if err != nil {
		return nil, err
	}

	var trades []Trade
	if err := json.Unmarshal(data, &trades); err != nil {
		return nil, err
	}
	return trades, nil
}
