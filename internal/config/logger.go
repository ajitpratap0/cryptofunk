package config

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level      string
	Format     string // "json" or "console"
	TimeFormat string
	Output     io.Writer
}

// InitLogger initializes the global logger
func InitLogger(level, format string) {
	// Parse log level
	logLevel, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	// Set time format
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// Configure output format
	var output io.Writer = os.Stdout
	if format == "console" {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	}

	// Set global logger
	log.Logger = zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()

	log.Info().
		Str("level", logLevel.String()).
		Str("format", format).
		Msg("Logger initialized")
}

// NewLogger creates a new logger with a component name
func NewLogger(component string) zerolog.Logger {
	return log.With().Str("component", component).Logger()
}

// NewAgentLogger creates a logger for an agent
func NewAgentLogger(agentName, agentType string) zerolog.Logger {
	return log.With().
		Str("component", "agent").
		Str("agent_name", agentName).
		Str("agent_type", agentType).
		Logger()
}

// NewMCPLogger creates a logger for an MCP server
func NewMCPLogger(serverName string) zerolog.Logger {
	return log.With().
		Str("component", "mcp_server").
		Str("server_name", serverName).
		Logger()
}
