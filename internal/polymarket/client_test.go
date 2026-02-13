package polymarket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test private key (DO NOT use in production)
const testPrivateKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

func TestNewSigner(t *testing.T) {
	signer, err := NewSigner(testPrivateKey, 137)
	require.NoError(t, err)
	assert.Equal(t, "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", signer.Address().Hex())
}

func TestSignClobAuthMessage(t *testing.T) {
	signer, err := NewSigner(testPrivateKey, 137)
	require.NoError(t, err)

	sig, err := signer.SignClobAuthMessage(1700000000, 0)
	require.NoError(t, err)
	assert.True(t, len(sig) > 0)
	assert.True(t, sig[:2] == "0x")
	// EIP-712 signature is 65 bytes = 130 hex chars + "0x"
	assert.Equal(t, 132, len(sig))
}

func TestBuildHMACSignature(t *testing.T) {
	// Base64 encoded secret
	secret := "dGVzdHNlY3JldA==" // "testsecret" in base64
	sig, err := BuildHMACSignature(secret, 1700000000, "GET", "/data/orders", "")
	require.NoError(t, err)
	assert.NotEmpty(t, sig)

	// Same inputs should produce same signature
	sig2, err := BuildHMACSignature(secret, 1700000000, "GET", "/data/orders", "")
	require.NoError(t, err)
	assert.Equal(t, sig, sig2)

	// Different timestamp should produce different signature
	sig3, err := BuildHMACSignature(secret, 1700000001, "GET", "/data/orders", "")
	require.NoError(t, err)
	assert.NotEqual(t, sig, sig3)
}

func setupMockServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	server := httptest.NewServer(handler)
	client, err := NewClient("", WithClobHost(server.URL), WithGammaHost(server.URL))
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return server, client
}

func TestGetMarkets(t *testing.T) {
	markets := []Market{
		{
			ConditionID: "0x123",
			Question:    "Will BTC reach 100k?",
			Active:      true,
			Tokens: []Token{
				{TokenID: "tok1", Outcome: "Yes", Price: 0.65},
				{TokenID: "tok2", Outcome: "No", Price: 0.35},
			},
		},
	}

	_, client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/markets")
		json.NewEncoder(w).Encode(markets)
	})

	result, err := client.GetMarkets(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Will BTC reach 100k?", result[0].Question)
	assert.Len(t, result[0].Tokens, 2)
}

func TestGetMarket(t *testing.T) {
	markets := []Market{
		{ConditionID: "0xabc", Question: "Test market"},
	}

	_, client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "condition_id=0xabc")
		json.NewEncoder(w).Encode(markets)
	})

	result, err := client.GetMarket(context.Background(), "0xabc")
	require.NoError(t, err)
	assert.Equal(t, "Test market", result.Question)
}

func TestGetMarketNotFound(t *testing.T) {
	_, client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Market{})
	})

	_, err := client.GetMarket(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetOrderBook(t *testing.T) {
	book := OrderBook{
		AssetID: "tok1",
		Bids:    []OrderBookLevel{{Price: "0.60", Size: "100"}, {Price: "0.59", Size: "200"}},
		Asks:    []OrderBookLevel{{Price: "0.61", Size: "150"}},
	}

	_, client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/book", r.URL.Path)
		assert.Equal(t, "tok1", r.URL.Query().Get("token_id"))
		json.NewEncoder(w).Encode(book)
	})

	result, err := client.GetOrderBook(context.Background(), "tok1")
	require.NoError(t, err)
	assert.Len(t, result.Bids, 2)
	assert.Len(t, result.Asks, 1)
	assert.Equal(t, "0.60", result.Bids[0].Price)
}

func TestGetMidpoint(t *testing.T) {
	_, client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/midpoint", r.URL.Path)
		json.NewEncoder(w).Encode(MidpointResponse{Mid: "0.605"})
	})

	mid, err := client.GetMidpoint(context.Background(), "tok1")
	require.NoError(t, err)
	assert.Equal(t, "0.605", mid)
}

func TestGetPrice(t *testing.T) {
	_, client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/price", r.URL.Path)
		assert.Equal(t, "BUY", r.URL.Query().Get("side"))
		json.NewEncoder(w).Encode(PriceResponse{Price: "0.61"})
	})

	price, err := client.GetPrice(context.Background(), "tok1", Buy)
	require.NoError(t, err)
	assert.Equal(t, "0.61", price)
}

func TestGetOpenOrdersRequiresAuth(t *testing.T) {
	_, client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]Order{})
	})

	_, err := client.GetOpenOrders(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "L2 auth required")
}

func TestCreateOrderRequiresAuth(t *testing.T) {
	_, client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	_, err := client.CreateOrder(context.Background(), "tok1", Buy, 0.65, 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "L2 auth required")
}

func TestGetOpenOrdersWithAuth(t *testing.T) {
	orders := []Order{
		{ID: "order1", Side: "BUY", Price: "0.65", Status: "live"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify L2 auth headers are present
		assert.NotEmpty(t, r.Header.Get(headerPolyAddress))
		assert.NotEmpty(t, r.Header.Get(headerPolySignature))
		assert.NotEmpty(t, r.Header.Get(headerPolyAPIKey))
		json.NewEncoder(w).Encode(orders)
	}))
	defer server.Close()

	client, err := NewClient(testPrivateKey,
		WithClobHost(server.URL),
		WithCreds(&APICreds{
			APIKey:     "testkey",
			Secret:     "dGVzdHNlY3JldA==",
			Passphrase: "testpass",
		}),
	)
	require.NoError(t, err)
	defer client.Close()

	result, err := client.GetOpenOrders(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "order1", result[0].ID)
}

func TestCancelOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/order", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, err := NewClient(testPrivateKey,
		WithClobHost(server.URL),
		WithCreds(&APICreds{
			APIKey:     "testkey",
			Secret:     "dGVzdHNlY3JldA==",
			Passphrase: "testpass",
		}),
	)
	require.NoError(t, err)
	defer client.Close()

	err = client.CancelOrder(context.Background(), "order123")
	require.NoError(t, err)
}

func TestSignOrder(t *testing.T) {
	signer, err := NewSigner(testPrivateKey, 137)
	require.NoError(t, err)

	order := &SignedOrder{
		Salt:          "12345",
		Maker:         signer.Address().Hex(),
		Signer:        signer.Address().Hex(),
		Taker:         "0x0000000000000000000000000000000000000000",
		TokenID:       "123456789",
		MakerAmount:   "650000",
		TakerAmount:   "1000000",
		Expiration:    "0",
		Nonce:         "0",
		FeeRateBps:    "0",
		Side:          "BUY",
		SignatureType: 0,
	}

	sig, err := signer.SignOrder(order, CTFExchangeAddress, false)
	require.NoError(t, err)
	assert.True(t, len(sig) > 0)
	assert.Equal(t, "0x", sig[:2])
	assert.Equal(t, 132, len(sig))
}

func TestNewClientNoKey(t *testing.T) {
	client, err := NewClient("")
	require.NoError(t, err)
	assert.Empty(t, client.GetAddress())
	client.Close()
}

func TestHTTPError(t *testing.T) {
	_, client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	})

	_, err := client.GetOrderBook(context.Background(), "tok1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}
