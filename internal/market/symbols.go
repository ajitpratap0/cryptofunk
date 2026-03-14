package market

import "strings"

// CoinGeckoIDToBinanceSymbol converts a CoinGecko coin ID to a Binance trading pair.
func CoinGeckoIDToBinanceSymbol(coinGeckoID string) string {
	mapping := map[string]string{
		"bitcoin":   "BTCUSDT",
		"ethereum":  "ETHUSDT",
		"solana":    "SOLUSDT",
		"cardano":   "ADAUSDT",
		"ripple":    "XRPUSDT",
		"dogecoin":  "DOGEUSDT",
		"polkadot":  "DOTUSDT",
		"avalanche": "AVAXUSDT",
		"chainlink": "LINKUSDT",
		"polygon":   "MATICUSDT",
	}
	if sym, ok := mapping[coinGeckoID]; ok {
		return sym
	}
	// Fallback: uppercase + USDT
	return strings.ToUpper(coinGeckoID) + "USDT"
}
