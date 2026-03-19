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
)

func TestPaperTrade_PersistsAllRows(t *testing.T) {
	srv, _ := setupTestAPIServer(t)

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
}
