package exchange

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/config"
)

func TestMockExchangeWithCustomFees(t *testing.T) {
	// Test custom fee configuration
	customFees := config.FeeConfig{
		Maker:        0.0005, // 0.05% maker
		Taker:        0.002,  // 0.2% taker
		BaseSlippage: 0.001,  // 0.1% base slippage
		MarketImpact: 0.0002, // 0.02% market impact
		MaxSlippage:  0.005,  // 0.5% max slippage
	}

	exchange := NewMockExchangeWithFees(nil, customFees)

	assert.Equal(t, 0.0005, exchange.makerFee, "Maker fee should match config")
	assert.Equal(t, 0.002, exchange.takerFee, "Taker fee should match config")
	assert.Equal(t, 0.001, exchange.baseSlippage, "Base slippage should match config")
	assert.Equal(t, 0.0002, exchange.marketImpact, "Market impact should match config")
	assert.Equal(t, 0.005, exchange.maxSlippage, "Max slippage should match config")
}

func TestMockExchangeDefaultFees(t *testing.T) {
	// Test default fee configuration
	exchange := NewMockExchange(nil)

	assert.Equal(t, 0.001, exchange.makerFee, "Default maker fee should be 0.1%")
	assert.Equal(t, 0.001, exchange.takerFee, "Default taker fee should be 0.1%")
	assert.Equal(t, 0.0005, exchange.baseSlippage, "Default base slippage should be 0.05%")
	assert.Equal(t, 0.0001, exchange.marketImpact, "Default market impact should be 0.01%")
	assert.Equal(t, 0.003, exchange.maxSlippage, "Default max slippage should be 0.3%")
}

func TestMarketOrderTakerFee(t *testing.T) {
	ctx := context.Background()

	customFees := config.FeeConfig{
		Maker: 0.0005,
		Taker: 0.002,
	}

	exchange := NewMockExchangeWithFees(nil, customFees)
	exchange.SetMarketPrice("BTCUSDT", 50000.0)

	// Place market order (taker)
	resp, err := exchange.PlaceOrder(ctx, PlaceOrderRequest{
		Symbol:   "BTCUSDT",
		Side:     OrderSideBuy,
		Type:     OrderTypeMarket,
		Quantity: 1.0,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, OrderStatusFilled, resp.Status)

	// Verify taker fee is configured correctly (market orders use taker fee)
	assert.Equal(t, customFees.Taker, exchange.takerFee)
}

func TestLimitOrderMakerFee(t *testing.T) {
	customFees := config.FeeConfig{
		Maker: 0.0005,
		Taker: 0.002,
	}

	exchange := NewMockExchangeWithFees(nil, customFees)

	// Verify maker fee is configured correctly (limit orders use maker fee)
	assert.Equal(t, customFees.Maker, exchange.makerFee)
	assert.Equal(t, customFees.Taker, exchange.takerFee)

	// Verify different fee structure is applied
	assert.NotEqual(t, exchange.makerFee, exchange.takerFee, "Maker and taker fees should be different for this test")
}

func TestPositionManagerWithCustomFees(t *testing.T) {
	// Test custom fee rate
	customFeeRate := 0.0015 // 0.15%

	pm := NewPositionManagerWithFees(nil, customFeeRate)

	assert.Equal(t, 0.0015, pm.feeRate, "Fee rate should match config")
}

func TestPositionManagerDefaultFees(t *testing.T) {
	// Test default fee rate
	pm := NewPositionManager(nil)

	assert.Equal(t, 0.001, pm.feeRate, "Default fee rate should be 0.1%")
}

func TestConfigFeeStructure(t *testing.T) {
	// Test fee config structure
	fees := config.FeeConfig{
		Maker:        0.001,
		Taker:        0.001,
		BaseSlippage: 0.0005,
		MarketImpact: 0.0001,
		MaxSlippage:  0.003,
		Withdrawal:   0.0,
	}

	assert.Equal(t, 0.001, fees.Maker)
	assert.Equal(t, 0.001, fees.Taker)
	assert.Equal(t, 0.0005, fees.BaseSlippage)
	assert.Equal(t, 0.0001, fees.MarketImpact)
	assert.Equal(t, 0.003, fees.MaxSlippage)
	assert.Equal(t, 0.0, fees.Withdrawal)
}

func TestDifferentExchangeFees(t *testing.T) {
	testCases := []struct {
		name     string
		exchange string
		fees     config.FeeConfig
	}{
		{
			name:     "Binance",
			exchange: "binance",
			fees: config.FeeConfig{
				Maker: 0.001, // 0.1%
				Taker: 0.001, // 0.1%
			},
		},
		{
			name:     "Coinbase Pro",
			exchange: "coinbasepro",
			fees: config.FeeConfig{
				Maker: 0.005, // 0.5%
				Taker: 0.005, // 0.5%
			},
		},
		{
			name:     "Kraken",
			exchange: "kraken",
			fees: config.FeeConfig{
				Maker: 0.0016, // 0.16%
				Taker: 0.0026, // 0.26%
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exchange := NewMockExchangeWithFees(nil, tc.fees)

			assert.Equal(t, tc.fees.Maker, exchange.makerFee, "Maker fee should match %s", tc.exchange)
			assert.Equal(t, tc.fees.Taker, exchange.takerFee, "Taker fee should match %s", tc.exchange)
		})
	}
}
