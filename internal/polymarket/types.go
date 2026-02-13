package polymarket

import "time"

// Side represents order side
type Side string

const (
	Buy  Side = "BUY"
	Sell Side = "SELL"
)

// OrderType represents order time-in-force
type OrderType string

const (
	GTC OrderType = "GTC"
	FOK OrderType = "FOK"
	GTD OrderType = "GTD"
)

// ApiCreds holds CLOB API credentials (Level 2 auth)
type ApiCreds struct {
	ApiKey     string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

// Market from Gamma API
type Market struct {
	ConditionID       string   `json:"condition_id"`
	QuestionID        string   `json:"question_id"`
	Question          string   `json:"question"`
	Description       string   `json:"description"`
	MarketSlug        string   `json:"market_slug"`
	EndDateISO        string   `json:"end_date_iso"`
	GameStartTime     string   `json:"game_start_time"`
	Active            bool     `json:"active"`
	Closed            bool     `json:"closed"`
	Funded            bool     `json:"funded"`
	Tokens            []Token  `json:"tokens"`
	MinimumOrderSize  string   `json:"minimum_order_size"`
	MinimumTickSize   string   `json:"minimum_tick_size"`
	Volume            string   `json:"volume"`
	Volume24hr        float64  `json:"volume_24hr"`
	Liquidity         string   `json:"liquidity"`
	Tags              []string `json:"tags"`
	NegRisk           bool     `json:"neg_risk"`
	NegRiskMarketID   string   `json:"neg_risk_market_id"`
	NegRiskRequestID  string   `json:"neg_risk_request_id"`
}

// Token represents a conditional token in a market
type Token struct {
	TokenID  string  `json:"token_id"`
	Outcome  string  `json:"outcome"`
	Price    float64 `json:"price"`
	Winner   bool    `json:"winner"`
}

// OrderBookLevel represents a single price level
type OrderBookLevel struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

// OrderBook represents the order book for a token
type OrderBook struct {
	Market         string           `json:"market"`
	AssetID        string           `json:"asset_id"`
	Timestamp      string           `json:"timestamp"`
	Bids           []OrderBookLevel `json:"bids"`
	Asks           []OrderBookLevel `json:"asks"`
	MinOrderSize   string           `json:"min_order_size"`
	NegRisk        bool             `json:"neg_risk"`
	TickSize       string           `json:"tick_size"`
	LastTradePrice string           `json:"last_trade_price"`
	Hash           string           `json:"hash"`
}

// MidpointResponse from /midpoint
type MidpointResponse struct {
	Mid string `json:"mid"`
}

// PriceResponse from /price
type PriceResponse struct {
	Price string `json:"price"`
}

// Order represents an order in the CLOB
type Order struct {
	ID              string    `json:"id"`
	Market          string    `json:"market"`
	AssetID         string    `json:"asset_id"`
	Side            string    `json:"side"`
	OriginalSize    string    `json:"original_size"`
	SizeMatched     string    `json:"size_matched"`
	Price           string    `json:"price"`
	Outcome         string    `json:"outcome"`
	Owner           string    `json:"owner"`
	Status          string    `json:"status"`
	OrderType       string    `json:"type"`
	Expiration      string    `json:"expiration"`
	AssociateTradeIDs []string `json:"associate_trades"`
	CreatedAt       time.Time `json:"created_at"`
}

// Trade represents a filled trade
type Trade struct {
	ID            string    `json:"id"`
	Market        string    `json:"market"`
	AssetID       string    `json:"asset_id"`
	Side          string    `json:"side"`
	Size          string    `json:"size"`
	Price         string    `json:"price"`
	Status        string    `json:"status"`
	Owner         string    `json:"owner"`
	Outcome       string    `json:"outcome"`
	FeeRateBps    string    `json:"fee_rate_bps"`
	MakerAddress  string    `json:"maker_address"`
	TradeOwner    string    `json:"trader"`
	TransactionHash string  `json:"transaction_hash"`
	MatchTime     time.Time `json:"match_time"`
}

// Position represents a user's position
type Position struct {
	AssetID  string `json:"asset_id"`
	Market   string `json:"market"`
	Size     string `json:"size"`
	AvgPrice string `json:"avg_price"`
	Side     string `json:"side"`
}

// SignedOrder is the order payload sent to the CLOB
type SignedOrder struct {
	Salt          string `json:"salt"`
	Maker         string `json:"maker"`
	Signer        string `json:"signer"`
	Taker         string `json:"taker"`
	TokenID       string `json:"tokenId"`
	MakerAmount   string `json:"makerAmount"`
	TakerAmount   string `json:"takerAmount"`
	Expiration    string `json:"expiration"`
	Nonce         string `json:"nonce"`
	FeeRateBps    string `json:"feeRateBps"`
	Side          string `json:"side"`
	SignatureType int    `json:"signatureType"`
	Signature     string `json:"signature"`
}

// PostOrderRequest is the body for POST /order
type PostOrderRequest struct {
	Order     SignedOrder `json:"order"`
	Owner     string      `json:"owner"`
	OrderType OrderType   `json:"orderType"`
}

// PostOrderResponse from POST /order
type PostOrderResponse struct {
	Success bool   `json:"success"`
	OrderID string `json:"orderID"`
	ErrorMsg string `json:"errorMsg,omitempty"`
}

// CancelOrderRequest for DELETE /order
type CancelOrderRequest struct {
	OrderID string `json:"orderID"`
}

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type    string      `json:"type"`
	Channel string      `json:"channel,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// WSOrderBookUpdate for orderbook channel
type WSOrderBookUpdate struct {
	AssetID   string           `json:"asset_id"`
	Market    string           `json:"market"`
	Timestamp string           `json:"timestamp"`
	Bids      []OrderBookLevel `json:"bids"`
	Asks      []OrderBookLevel `json:"asks"`
	Hash      string           `json:"hash"`
}

// WSTradeUpdate for trade events
type WSTradeUpdate struct {
	AssetID string `json:"asset_id"`
	Market  string `json:"market"`
	Side    string `json:"side"`
	Price   string `json:"price"`
	Size    string `json:"size"`
}
