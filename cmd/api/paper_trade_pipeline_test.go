//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/exchange"
)

func TestPaperTrade_PersistsAllRows(t *testing.T) {
	t.Run("with_explicit_price", func(t *testing.T) {
		srv, _ := setupTestAPIServer(t)
		// Provide a mock exchange so s.exchange is never nil.
		srv.exchange = exchange.NewMockExchange(srv.db)

		body := `{"symbol":"BTCUSDT","side":"BUY","type":"market","quantity":0.1,"price":45000}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trade", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		orderID := resp["order"].(map[string]interface{})["id"].(string)

		ctx := context.Background()

		// Verify order row is filled with non-zero executed_quote_quantity
		order, err := srv.db.GetOrder(ctx, uuid.MustParse(orderID))
		require.NoError(t, err)
		assert.Equal(t, "FILLED", string(order.Status))
		assert.Greater(t, order.ExecutedQuantity, 0.0)
		assert.Greater(t, order.ExecutedQuoteQuantity, 0.0, "executed_quote_quantity must be non-zero (price bug)")

		// Verify trade fill row via GetTradesByOrderID (already exists in orders.go)
		fills, err := srv.db.GetTradesByOrderID(ctx, uuid.MustParse(orderID))
		require.NoError(t, err)
		assert.NotEmpty(t, fills, "expected at least one trade fill row in trades table")
		assert.Greater(t, fills[0].Price, 0.0, "fill price must be > 0")

		// Verify open position exists
		positions, err := srv.db.GetAllOpenPositions(ctx)
		require.NoError(t, err)
		found := false
		for _, p := range positions {
			if p.Symbol == "BTCUSDT" {
				found = true
				assert.Greater(t, p.EntryPrice, 0.0)
				assert.Greater(t, p.Quantity, 0.0)
			}
		}
		assert.True(t, found, "expected BTCUSDT open position after paper trade")
	})

	t.Run("uses_get_market_price_when_no_price_in_request", func(t *testing.T) {
		srv, _ := setupTestAPIServer(t)
		// Seed a market price so GetMarketPrice returns a non-zero value.
		mockEx := exchange.NewMockExchange(srv.db)
		mockEx.SetMarketPrice("ETHUSDT", 3000.0)
		srv.exchange = mockEx

		body := `{"symbol":"ETHUSDT","side":"BUY","type":"market","quantity":0.05}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trade", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		orderID := resp["order"].(map[string]interface{})["id"].(string)

		ctx := context.Background()

		// Verify trade fill row exists and has a positive price derived from GetMarketPrice.
		fills, err := srv.db.GetTradesByOrderID(ctx, uuid.MustParse(orderID))
		require.NoError(t, err)
		assert.NotEmpty(t, fills, "expected at least one trade fill row")
		assert.Greater(t, fills[0].Price, 0.0, "fill price must be > 0 when price comes from GetMarketPrice")
	})

	t.Run("returns_400_when_no_price_and_no_market_price_seeded", func(t *testing.T) {
		srv, _ := setupTestAPIServer(t)
		// Use a fresh mock exchange with no price seeded for SOLUSDT.
		srv.exchange = exchange.NewMockExchange(srv.db)

		body := `{"symbol":"SOLUSDT","side":"BUY","type":"market","quantity":0.05}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trade", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Contains(t, resp["error"], "no market price configured for symbol")
	})
}
