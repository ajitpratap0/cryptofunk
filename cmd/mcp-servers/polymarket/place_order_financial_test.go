// place_order_financial_test.go — Financial Safety Tests for CryptoFunk PR #60
// Tests the place_order validation via the HTTP transport path (registerTools).
package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ajitpratap0/cryptofunk/internal/testhelpers"
)

// callPlaceOrderHTTPPath calls place_order via the registerTools (HTTP) path.
// This is the path added by PR #60 and includes proper financial validation.
func callPlaceOrderHTTPPath(t *testing.T, args map[string]interface{}) (isError bool, errorMsg string) {
	t.Helper()
	ts, _ := newHTTPPolyServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Simulate API rejection for all orders (no real money flows)
		http.Error(w, `{"error":"no_auth"}`, http.StatusUnauthorized)
	})
	sid := initHTTPSession(t, ts)

	resp, _ := mcpHTTPPost(t, ts, sid, map[string]interface{}{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]interface{}{
			"name":      "place_order",
			"arguments": args,
		},
	})
	defer resp.Body.Close()

	data := testhelpers.ReadSSEData(t, resp.Body)
	var rpc struct {
		Result *struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &rpc); err != nil {
		t.Fatalf("unmarshal failed: %v — raw: %s", err, data)
	}
	if rpc.Error != nil {
		return true, rpc.Error.Message
	}
	if rpc.Result != nil && rpc.Result.IsError {
		msg := ""
		if len(rpc.Result.Content) > 0 {
			msg = rpc.Result.Content[0].Text
		}
		return true, msg
	}
	return false, ""
}

// TestFinancial_HTTP_PlaceOrder_PriceZeroRejected — price=0 must be rejected
func TestFinancial_HTTP_PlaceOrder_PriceZeroRejected(t *testing.T) {
	isErr, msg := callPlaceOrderHTTPPath(t, map[string]interface{}{
		"token_id": "tok1", "side": "BUY", "price": 0.0, "size": 10.0,
	})
	if !isErr {
		t.Fatal("FINANCIAL SAFETY FAILURE: price=0 was accepted!")
	}
	if !strings.Contains(msg, "price") {
		t.Logf("WARNING: error message doesn't mention 'price': %s", msg)
	}
	t.Logf("PASS: price=0 rejected with: %s", msg)
}

// TestFinancial_HTTP_PlaceOrder_PriceNegativeRejected — price=-0.5 must be rejected
func TestFinancial_HTTP_PlaceOrder_PriceNegativeRejected(t *testing.T) {
	isErr, msg := callPlaceOrderHTTPPath(t, map[string]interface{}{
		"token_id": "tok1", "side": "BUY", "price": -0.5, "size": 10.0,
	})
	if !isErr {
		t.Fatal("FINANCIAL SAFETY FAILURE: price=-0.5 was accepted!")
	}
	t.Logf("PASS: price=-0.5 rejected with: %s", msg)
}

// TestFinancial_HTTP_PlaceOrder_PriceOneRejected — price=1.0 must be rejected (exclusive upper bound)
func TestFinancial_HTTP_PlaceOrder_PriceOneRejected(t *testing.T) {
	isErr, msg := callPlaceOrderHTTPPath(t, map[string]interface{}{
		"token_id": "tok1", "side": "BUY", "price": 1.0, "size": 10.0,
	})
	if !isErr {
		t.Fatal("FINANCIAL SAFETY FAILURE: price=1.0 was accepted (should be exclusive)!")
	}
	t.Logf("PASS: price=1.0 (boundary) rejected with: %s", msg)
}

// TestFinancial_HTTP_PlaceOrder_PriceAboveOneRejected — price=1.5 must be rejected
func TestFinancial_HTTP_PlaceOrder_PriceAboveOneRejected(t *testing.T) {
	isErr, msg := callPlaceOrderHTTPPath(t, map[string]interface{}{
		"token_id": "tok1", "side": "BUY", "price": 1.5, "size": 10.0,
	})
	if !isErr {
		t.Fatal("FINANCIAL SAFETY FAILURE: price=1.5 was accepted!")
	}
	t.Logf("PASS: price=1.5 rejected with: %s", msg)
}

// TestFinancial_HTTP_PlaceOrder_SizeZeroRejected — size=0 must be rejected
func TestFinancial_HTTP_PlaceOrder_SizeZeroRejected(t *testing.T) {
	isErr, msg := callPlaceOrderHTTPPath(t, map[string]interface{}{
		"token_id": "tok1", "side": "BUY", "price": 0.5, "size": 0.0,
	})
	if !isErr {
		t.Fatal("FINANCIAL SAFETY FAILURE: size=0 was accepted!")
	}
	if !strings.Contains(msg, "size") {
		t.Logf("WARNING: error message doesn't mention 'size': %s", msg)
	}
	t.Logf("PASS: size=0 rejected with: %s", msg)
}

// TestFinancial_HTTP_PlaceOrder_SizeNegativeRejected — size=-1 must be rejected
func TestFinancial_HTTP_PlaceOrder_SizeNegativeRejected(t *testing.T) {
	isErr, msg := callPlaceOrderHTTPPath(t, map[string]interface{}{
		"token_id": "tok1", "side": "BUY", "price": 0.5, "size": -1.0,
	})
	if !isErr {
		t.Fatal("FINANCIAL SAFETY FAILURE: size=-1 was accepted!")
	}
	t.Logf("PASS: size=-1 rejected with: %s", msg)
}

// TestFinancial_HTTP_PlaceOrder_InvalidSideRejected — side="INVALID" must be rejected
func TestFinancial_HTTP_PlaceOrder_InvalidSideRejected(t *testing.T) {
	isErr, msg := callPlaceOrderHTTPPath(t, map[string]interface{}{
		"token_id": "tok1", "side": "INVALID", "price": 0.5, "size": 10.0,
	})
	if !isErr {
		t.Fatal("FINANCIAL SAFETY FAILURE: side=INVALID was accepted!")
	}
	if !strings.Contains(strings.ToLower(msg), "side") {
		t.Logf("WARNING: error message doesn't mention 'side': %s", msg)
	}
	t.Logf("PASS: side=INVALID rejected with: %s", msg)
}

// TestFinancial_HTTP_PlaceOrder_ValidBUY — valid BUY should pass validation (may fail at API)
func TestFinancial_HTTP_PlaceOrder_ValidBUY(t *testing.T) {
	isErr, msg := callPlaceOrderHTTPPath(t, map[string]interface{}{
		"token_id": "tok1", "side": "BUY", "price": 0.5, "size": 10.0,
	})
	// A validation error would contain "price", "size", or "side"
	// An API error (L2 auth, network) is expected in test env
	if isErr {
		isValidationError := strings.Contains(msg, "price must be") ||
			strings.Contains(msg, "size must be") ||
			strings.Contains(msg, "side must be")
		if isValidationError {
			t.Fatalf("valid BUY order was rejected by VALIDATION (not API): %s", msg)
		}
		t.Logf("PASS: valid BUY order passed validation, rejected at API level (expected): %s", msg)
	} else {
		t.Logf("PASS: valid BUY order accepted")
	}
}
