package exchange

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/polymarket"
)

// testPrivateKey is a well-known Hardhat test key — safe to use in tests.
const testPrivateKey = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// newTestPolymarketExchange creates a paper-mode PolymarketExchange pointing at a
// non-existent local server so that any accidental live network call fails fast.
func newTestPolymarketExchange(t *testing.T) *PolymarketExchange {
	t.Helper()
	ex, err := NewPolymarketExchange(
		testPrivateKey,
		true, // paper mode
		nil,  // no DB
		polymarket.WithClobHost("http://localhost:19999"),
		polymarket.WithGammaHost("http://localhost:19999"),
	)
	require.NoError(t, err)
	return ex
}

// ---------------------------------------------------------------------------
// 1. PlaceOrder — table-driven validation tests
// ---------------------------------------------------------------------------

func TestPolymarketExchange_PlaceOrder_PaperMode(t *testing.T) {
	tests := []struct {
		name        string
		req         PlaceOrderRequest
		wantErr     bool
		errContains string
		wantStatus  OrderStatus
	}{
		{
			name: "valid BUY at price 0.6 qty 10",
			req: PlaceOrderRequest{
				Symbol:   "token-abc",
				Side:     OrderSideBuy,
				Type:     OrderTypeLimit,
				Price:    0.6,
				Quantity: 10,
			},
			wantErr:    false,
			wantStatus: OrderStatusFilled,
		},
		{
			name: "valid SELL at price 0.4 qty 5",
			req: PlaceOrderRequest{
				Symbol:   "token-xyz",
				Side:     OrderSideSell,
				Type:     OrderTypeLimit,
				Price:    0.4,
				Quantity: 5,
			},
			wantErr:    false,
			wantStatus: OrderStatusFilled,
		},
		{
			name: "invalid price=0",
			req: PlaceOrderRequest{
				Symbol:   "token-abc",
				Side:     OrderSideBuy,
				Price:    0,
				Quantity: 10,
			},
			wantErr:     true,
			errContains: "between 0 and 1",
		},
		{
			name: "invalid price=1",
			req: PlaceOrderRequest{
				Symbol:   "token-abc",
				Side:     OrderSideBuy,
				Price:    1,
				Quantity: 10,
			},
			wantErr:     true,
			errContains: "between 0 and 1",
		},
		{
			name: "invalid price negative",
			req: PlaceOrderRequest{
				Symbol:   "token-abc",
				Side:     OrderSideBuy,
				Price:    -0.1,
				Quantity: 10,
			},
			wantErr:     true,
			errContains: "between 0 and 1",
		},
		{
			name: "invalid qty=0",
			req: PlaceOrderRequest{
				Symbol:   "token-abc",
				Side:     OrderSideBuy,
				Price:    0.5,
				Quantity: 0,
			},
			wantErr:     true,
			errContains: "quantity must be positive",
		},
		{
			name: "missing symbol",
			req: PlaceOrderRequest{
				Symbol:   "",
				Side:     OrderSideBuy,
				Price:    0.5,
				Quantity: 10,
			},
			wantErr:     true,
			errContains: "symbol",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ex := newTestPolymarketExchange(t)

			resp, err := ex.PlaceOrder(context.Background(), tc.req)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, resp.OrderID, "OrderID should be non-empty for valid order")
			assert.Equal(t, tc.wantStatus, resp.Status)
		})
	}
}

// ---------------------------------------------------------------------------
// 2. Safety check tests
// ---------------------------------------------------------------------------

func TestPolymarketExchange_PlaceOrder_SafetyCheck(t *testing.T) {
	// Safety check: reject orders whose value (price * qty) exceeds $100.
	maxValue := 100.0
	safetyFn := func(_ context.Context, _ PlaceOrderRequest, orderValue float64) error {
		if orderValue > maxValue {
			return fmt.Errorf("order value %.2f exceeds limit %.2f", orderValue, maxValue)
		}
		return nil
	}

	t.Run("order value under limit passes", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)
		ex.SetSafetyCheck(safetyFn)

		// value = 0.5 * 50 = $25 — under limit
		resp, err := ex.PlaceOrder(context.Background(), PlaceOrderRequest{
			Symbol:   "token-abc",
			Side:     OrderSideBuy,
			Price:    0.5,
			Quantity: 50,
		})
		require.NoError(t, err)
		assert.Equal(t, OrderStatusFilled, resp.Status)
	})

	t.Run("order value over limit is rejected", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)
		ex.SetSafetyCheck(safetyFn)

		// value = 0.5 * 500 = $250 — over limit
		resp, err := ex.PlaceOrder(context.Background(), PlaceOrderRequest{
			Symbol:   "token-abc",
			Side:     OrderSideBuy,
			Price:    0.5,
			Quantity: 500,
		})
		require.NoError(t, err, "safety-rejected orders should not return an error")
		assert.Equal(t, OrderStatusRejected, resp.Status)
		assert.Contains(t, resp.Message, "exceeds limit")
	})
}

// ---------------------------------------------------------------------------
// 3. CancelOrder tests
// ---------------------------------------------------------------------------

func TestPolymarketExchange_CancelOrder(t *testing.T) {
	t.Run("cancel an OPEN order succeeds", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		// Paper-mode fills immediately (status=FILLED), so to test cancellation we
		// need to inject an OPEN order directly into the internal map.
		orderID := uuid.New().String()
		ex.orders[orderID] = &Order{
			ID:     orderID,
			Symbol: "token-abc",
			Side:   OrderSideBuy,
			Status: OrderStatusOpen,
		}

		cancelled, err := ex.CancelOrder(context.Background(), orderID)
		require.NoError(t, err)
		assert.Equal(t, OrderStatusCancelled, cancelled.Status)
	})

	t.Run("cancel a FILLED order returns error", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		// Place a paper order — it lands in FILLED state.
		resp, err := ex.PlaceOrder(context.Background(), PlaceOrderRequest{
			Symbol:   "token-abc",
			Side:     OrderSideBuy,
			Price:    0.6,
			Quantity: 10,
		})
		require.NoError(t, err)
		require.Equal(t, OrderStatusFilled, resp.Status)

		_, err = ex.CancelOrder(context.Background(), resp.OrderID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot cancel order in status")
	})

	t.Run("cancel nonexistent orderID returns error", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		_, err := ex.CancelOrder(context.Background(), "does-not-exist")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "order not found")
	})
}

// ---------------------------------------------------------------------------
// 4. GetOrder tests
// ---------------------------------------------------------------------------

func TestPolymarketExchange_GetOrder(t *testing.T) {
	t.Run("get existing order returns it", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		resp, err := ex.PlaceOrder(context.Background(), PlaceOrderRequest{
			Symbol:   "token-abc",
			Side:     OrderSideBuy,
			Price:    0.7,
			Quantity: 3,
		})
		require.NoError(t, err)

		order, err := ex.GetOrder(context.Background(), resp.OrderID)
		require.NoError(t, err)
		assert.Equal(t, resp.OrderID, order.ID)
		assert.Equal(t, "token-abc", order.Symbol)
		assert.Equal(t, OrderSideBuy, order.Side)
	})

	t.Run("get nonexistent order returns error", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		_, err := ex.GetOrder(context.Background(), "ghost-order-id")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "order not found")
	})
}

// ---------------------------------------------------------------------------
// 5. GetOrderFills tests
// ---------------------------------------------------------------------------

func TestPolymarketExchange_GetOrderFills(t *testing.T) {
	t.Run("filled paper order has exactly one fill with correct qty and price", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		resp, err := ex.PlaceOrder(context.Background(), PlaceOrderRequest{
			Symbol:   "token-abc",
			Side:     OrderSideBuy,
			Price:    0.65,
			Quantity: 7,
		})
		require.NoError(t, err)

		fills, err := ex.GetOrderFills(context.Background(), resp.OrderID)
		require.NoError(t, err)
		require.Len(t, fills, 1, "paper order should produce exactly one fill")
		assert.Equal(t, 7.0, fills[0].Quantity)
		assert.Equal(t, 0.65, fills[0].Price)
		assert.Equal(t, resp.OrderID, fills[0].OrderID)
	})

	t.Run("unknown orderID returns empty slice with no error", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		fills, err := ex.GetOrderFills(context.Background(), "unknown-id")
		require.NoError(t, err)
		assert.Empty(t, fills)
	})
}

// ---------------------------------------------------------------------------
// 6. SetMarketPrice / GetMarketPrice
// ---------------------------------------------------------------------------

func TestPolymarketExchange_SetGetMarketPrice(t *testing.T) {
	t.Run("set then get returns cached price", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		ex.SetMarketPrice("token-A", 0.72)
		price := ex.GetMarketPrice("token-A")
		assert.Equal(t, 0.72, price)
	})

	t.Run("unknown token falls back to CLOB which fails and returns 0", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		// The CLOB host is pointed at localhost:19999 which doesn't exist, so the
		// GetMidpoint call fails and GetMarketPrice should return 0.
		price := ex.GetMarketPrice("token-unknown")
		assert.Equal(t, 0.0, price)
	})
}

// ---------------------------------------------------------------------------
// 7. SetSession / GetSession
// ---------------------------------------------------------------------------

func TestPolymarketExchange_SetGetSession(t *testing.T) {
	t.Run("SetSession then GetSession returns same ID", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		id := uuid.New()
		ex.SetSession(&id)

		got := ex.GetSession()
		require.NotNil(t, got)
		assert.Equal(t, id, *got)
	})

	t.Run("GetSession with no session set returns nil", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		got := ex.GetSession()
		assert.Nil(t, got)
	})

	t.Run("SetSession with nil clears the session", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		id := uuid.New()
		ex.SetSession(&id)
		require.NotNil(t, ex.GetSession())

		ex.SetSession(nil)
		assert.Nil(t, ex.GetSession())
	})
}

// ---------------------------------------------------------------------------
// 8. GetOpenOrders (paper mode)
// ---------------------------------------------------------------------------

func TestPolymarketExchange_GetOpenOrders_PaperMode(t *testing.T) {
	t.Run("no orders returns empty slice", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		orders, err := ex.GetOpenOrders(context.Background())
		require.NoError(t, err)
		assert.Empty(t, orders)
	})

	t.Run("paper FILLED orders are not returned as open", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		// Place 2 paper orders — both will be FILLED immediately.
		for _, sym := range []string{"token-A", "token-B"} {
			_, err := ex.PlaceOrder(context.Background(), PlaceOrderRequest{
				Symbol:   sym,
				Side:     OrderSideBuy,
				Price:    0.5,
				Quantity: 1,
			})
			require.NoError(t, err)
		}

		openOrders, err := ex.GetOpenOrders(context.Background())
		require.NoError(t, err)
		assert.Empty(t, openOrders, "FILLED paper orders should not appear as OPEN")
	})

	t.Run("manually injected OPEN order appears in GetOpenOrders", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		// Inject an OPEN order directly (simulates a resting order).
		openID := uuid.New().String()
		ex.orders[openID] = &Order{
			ID:              openID,
			ExchangeOrderID: "paper-" + openID[:8],
			Symbol:          "token-open",
			Side:            OrderSideSell,
			Status:          OrderStatusOpen,
			Price:           0.45,
		}

		openOrders, err := ex.GetOpenOrders(context.Background())
		require.NoError(t, err)
		require.Len(t, openOrders, 1)
		assert.Equal(t, "token-open", openOrders[0].AssetID)
		assert.Equal(t, "sell", openOrders[0].Side)
	})
}

// ---------------------------------------------------------------------------
// 9. convertToDBOrder + nil-DB safety
// ---------------------------------------------------------------------------

func TestPolymarketExchange_ConvertToDBOrder_PaperMode(t *testing.T) {
	t.Run("PlaceOrder with nil DB still succeeds (no panic)", func(t *testing.T) {
		ex := newTestPolymarketExchange(t) // db is nil by default

		resp, err := ex.PlaceOrder(context.Background(), PlaceOrderRequest{
			Symbol:   "token-abc",
			Side:     OrderSideBuy,
			Price:    0.55,
			Quantity: 4,
		})
		require.NoError(t, err)
		assert.Equal(t, OrderStatusFilled, resp.Status)
	})

	t.Run("convertToDBOrder sets correct exchange name and fields", func(t *testing.T) {
		ex := newTestPolymarketExchange(t)

		// Place a paper order so the internal order map is populated.
		resp, err := ex.PlaceOrder(context.Background(), PlaceOrderRequest{
			Symbol:   "token-abc",
			Side:     OrderSideSell,
			Price:    0.3,
			Quantity: 8,
		})
		require.NoError(t, err)

		// Retrieve the internal order directly.
		internalOrder := ex.orders[resp.OrderID]
		require.NotNil(t, internalOrder)

		// Convert to DB order and verify fields.
		dbOrder := ex.convertToDBOrder(internalOrder)

		assert.Equal(t, "POLYMARKET_PAPER", dbOrder.Exchange)
		assert.Equal(t, "token-abc", dbOrder.Symbol)
		assert.Equal(t, 0.3, *dbOrder.Price)
		assert.Equal(t, 8.0, dbOrder.Quantity)
		assert.Equal(t, 8.0, dbOrder.ExecutedQuantity)
		assert.InDelta(t, 0.3*8, dbOrder.ExecutedQuoteQuantity, 0.0001)
	})

	t.Run("convertToDBOrder uses POLYMARKET (not POLYMARKET_PAPER) in live mode", func(t *testing.T) {
		// Create a live-mode exchange just to verify the exchange name.
		// We won't actually call PlaceOrder in live mode.
		ex, err := NewPolymarketExchange(
			testPrivateKey,
			false, // live mode
			nil,
			polymarket.WithClobHost("http://localhost:19999"),
			polymarket.WithGammaHost("http://localhost:19999"),
		)
		require.NoError(t, err)

		order := &Order{
			ID:     uuid.New().String(),
			Symbol: "token-abc",
			Side:   OrderSideBuy,
			Price:  0.5,
		}
		dbOrder := ex.convertToDBOrder(order)
		assert.Equal(t, "POLYMARKET", dbOrder.Exchange)
	})
}

// ---------------------------------------------------------------------------
// 10. Concurrent access (race detector)
// ---------------------------------------------------------------------------

func TestPolymarketExchange_ConcurrentPaperOrders(t *testing.T) {
	ex := newTestPolymarketExchange(t)

	const n = 20
	done := make(chan error, n)

	for i := 0; i < n; i++ {
		go func(idx int) {
			_, err := ex.PlaceOrder(context.Background(), PlaceOrderRequest{
				Symbol:   fmt.Sprintf("token-%d", idx%5),
				Side:     OrderSideBuy,
				Price:    0.1 + float64(idx%8)*0.1, // 0.1 – 0.8
				Quantity: float64(1 + idx),
			})
			done <- err
		}(i)
	}

	for i := 0; i < n; i++ {
		assert.NoError(t, <-done)
	}

	ex.mu.RLock()
	count := len(ex.orders)
	ex.mu.RUnlock()
	assert.Equal(t, n, count)
}

// ---------------------------------------------------------------------------
// 11. Constructor edge cases
// ---------------------------------------------------------------------------

func TestPolymarketExchange_Constructor(t *testing.T) {
	t.Run("empty private key is allowed (no signer)", func(t *testing.T) {
		ex, err := NewPolymarketExchange(
			"", // no private key
			true,
			nil,
			polymarket.WithClobHost("http://localhost:19999"),
		)
		require.NoError(t, err)
		assert.NotNil(t, ex)
		assert.Equal(t, "", ex.client.GetAddress())
	})

	t.Run("invalid private key returns error", func(t *testing.T) {
		_, err := NewPolymarketExchange(
			"not-a-valid-hex-key",
			true,
			nil,
		)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// 12. SetDB
// ---------------------------------------------------------------------------

func TestPolymarketExchange_SetDB(t *testing.T) {
	ex := newTestPolymarketExchange(t)
	assert.Nil(t, ex.db)

	// SetDB to nil is a no-op but must not panic.
	assert.NotPanics(t, func() { ex.SetDB(nil) })
}

// ---------------------------------------------------------------------------
// 13. Close
// ---------------------------------------------------------------------------

func TestPolymarketExchange_Close(t *testing.T) {
	ex := newTestPolymarketExchange(t)
	assert.NotPanics(t, func() { ex.Close() })
}
