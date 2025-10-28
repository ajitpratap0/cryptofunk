// Package deps imports all required dependencies to ensure they are included in go.mod
// This file should be removed once actual code imports these packages
package deps

import (
	_ "github.com/gin-gonic/gin"
	_ "github.com/modelcontextprotocol/go-sdk/mcp"
	_ "github.com/nats-io/nats.go"
	_ "github.com/prometheus/client_golang/prometheus"
	_ "github.com/prometheus/client_golang/prometheus/promhttp"
	_ "github.com/stretchr/testify/assert"
	_ "github.com/stretchr/testify/mock"
	// Note: CCXT doesn't have a proper Go package
	// We'll use github.com/adshao/go-binance/v2 which is already in go.mod
)
