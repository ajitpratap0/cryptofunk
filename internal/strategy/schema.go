package strategy

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ValidationError contains details about validation failures
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("validation failed: %s", strings.Join(msgs, "; "))
}

// ErrInvalidSchema is returned when the schema version is not supported
var ErrInvalidSchema = errors.New("invalid or unsupported schema version")

// ErrMissingRequiredField is returned when a required field is missing
var ErrMissingRequiredField = errors.New("missing required field")

// SupportedSchemaVersions lists all supported schema versions
var SupportedSchemaVersions = []string{"1.0"}

// Validate performs comprehensive validation on a strategy configuration.
// Returns nil if valid, or ValidationErrors with all issues found.
func (s *StrategyConfig) Validate() error {
	var errs ValidationErrors

	// Validate metadata
	if err := s.validateMetadata(); err != nil {
		errs = append(errs, err...)
	}

	// Validate agents
	if err := s.validateAgents(); err != nil {
		errs = append(errs, err...)
	}

	// Validate risk settings
	if err := s.validateRisk(); err != nil {
		errs = append(errs, err...)
	}

	// Validate orchestration
	if err := s.validateOrchestration(); err != nil {
		errs = append(errs, err...)
	}

	// Validate indicators
	if err := s.validateIndicators(); err != nil {
		errs = append(errs, err...)
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (s *StrategyConfig) validateMetadata() ValidationErrors {
	var errs ValidationErrors

	// Schema version is required
	if s.Metadata.SchemaVersion == "" {
		errs = append(errs, ValidationError{
			Field:   "metadata.schema_version",
			Message: "schema version is required",
		})
	} else if !isVersionSupported(s.Metadata.SchemaVersion) {
		errs = append(errs, ValidationError{
			Field:   "metadata.schema_version",
			Message: fmt.Sprintf("unsupported schema version %s, supported: %v", s.Metadata.SchemaVersion, SupportedSchemaVersions),
		})
	}

	// Name is required
	if s.Metadata.Name == "" {
		errs = append(errs, ValidationError{
			Field:   "metadata.name",
			Message: "strategy name is required",
		})
	} else if len(s.Metadata.Name) > 100 {
		errs = append(errs, ValidationError{
			Field:   "metadata.name",
			Message: "strategy name must be 100 characters or less",
		})
	}

	// Description length limit
	if len(s.Metadata.Description) > 2000 {
		errs = append(errs, ValidationError{
			Field:   "metadata.description",
			Message: "description must be 2000 characters or less",
		})
	}

	// Tags validation
	if len(s.Metadata.Tags) > 20 {
		errs = append(errs, ValidationError{
			Field:   "metadata.tags",
			Message: "maximum 20 tags allowed",
		})
	}
	for i, tag := range s.Metadata.Tags {
		if len(tag) > 50 {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("metadata.tags[%d]", i),
				Message: "tag must be 50 characters or less",
			})
		}
	}

	return errs
}

func (s *StrategyConfig) validateAgents() ValidationErrors {
	var errs ValidationErrors

	// Validate weights are between 0 and 1
	weights := map[string]float64{
		"agents.weights.technical": s.Agents.Weights.Technical,
		"agents.weights.orderbook": s.Agents.Weights.OrderBook,
		"agents.weights.sentiment": s.Agents.Weights.Sentiment,
		"agents.weights.trend":     s.Agents.Weights.Trend,
		"agents.weights.reversion": s.Agents.Weights.Reversion,
		"agents.weights.arbitrage": s.Agents.Weights.Arbitrage,
	}

	for field, weight := range weights {
		if weight < 0 || weight > 1 {
			errs = append(errs, ValidationError{
				Field:   field,
				Message: "weight must be between 0 and 1",
			})
		}
	}

	// At least one agent must be enabled (besides risk)
	if !s.Agents.Enabled.Technical &&
		!s.Agents.Enabled.OrderBook &&
		!s.Agents.Enabled.Sentiment &&
		!s.Agents.Enabled.Trend &&
		!s.Agents.Enabled.Reversion &&
		!s.Agents.Enabled.Arbitrage {
		errs = append(errs, ValidationError{
			Field:   "agents.enabled",
			Message: "at least one trading agent must be enabled",
		})
	}

	// Cross-validate: enabled agents should have non-zero weights
	type agentCheck struct {
		enabled bool
		weight  float64
		name    string
	}
	agentChecks := []agentCheck{
		{s.Agents.Enabled.Technical, s.Agents.Weights.Technical, "technical"},
		{s.Agents.Enabled.OrderBook, s.Agents.Weights.OrderBook, "orderbook"},
		{s.Agents.Enabled.Sentiment, s.Agents.Weights.Sentiment, "sentiment"},
		{s.Agents.Enabled.Trend, s.Agents.Weights.Trend, "trend"},
		{s.Agents.Enabled.Reversion, s.Agents.Weights.Reversion, "reversion"},
		{s.Agents.Enabled.Arbitrage, s.Agents.Weights.Arbitrage, "arbitrage"},
	}

	for _, check := range agentChecks {
		if check.enabled && check.weight == 0 {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("agents.weights.%s", check.name),
				Message: fmt.Sprintf("%s agent is enabled but has zero weight - it will have no influence on decisions", check.name),
			})
		}
	}

	// Validate agent-specific configs if enabled
	if s.Agents.Enabled.Technical && s.Agents.Technical != nil {
		if err := validateTechnicalConfig(s.Agents.Technical); err != nil {
			errs = append(errs, err...)
		}
	}

	if s.Agents.Enabled.Trend && s.Agents.Trend != nil {
		if err := validateTrendConfig(s.Agents.Trend); err != nil {
			errs = append(errs, err...)
		}
	}

	if s.Agents.Enabled.Reversion && s.Agents.Reversion != nil {
		if err := validateReversionConfig(s.Agents.Reversion); err != nil {
			errs = append(errs, err...)
		}
	}

	return errs
}

func validateTechnicalConfig(cfg *TechnicalAgentConfig) ValidationErrors {
	var errs ValidationErrors

	// Validate confidence threshold
	if cfg.ConfidenceThreshold < 0 || cfg.ConfidenceThreshold > 1 {
		errs = append(errs, ValidationError{
			Field:   "agents.technical.confidence_threshold",
			Message: "confidence threshold must be between 0 and 1",
		})
	}

	// Validate confidence weights sum to approximately 1
	weightSum := cfg.ConfidenceWeights.RSI +
		cfg.ConfidenceWeights.MACD +
		cfg.ConfidenceWeights.Bollinger +
		cfg.ConfidenceWeights.Trend +
		cfg.ConfidenceWeights.Volume

	if weightSum > 0 && (weightSum < 0.99 || weightSum > 1.01) {
		errs = append(errs, ValidationError{
			Field:   "agents.technical.confidence_weights",
			Message: fmt.Sprintf("confidence weights should sum to 1.0, got %.2f", weightSum),
		})
	}

	return errs
}

func validateTrendConfig(cfg *TrendAgentConfig) ValidationErrors {
	var errs ValidationErrors

	// Validate EMA periods
	if cfg.FastEMAPeriod > 0 && cfg.SlowEMAPeriod > 0 {
		if cfg.FastEMAPeriod >= cfg.SlowEMAPeriod {
			errs = append(errs, ValidationError{
				Field:   "agents.trend.fast_ema_period",
				Message: "fast EMA period must be less than slow EMA period",
			})
		}
	}

	// Validate ADX threshold
	if cfg.ADXThreshold < 0 || cfg.ADXThreshold > 100 {
		errs = append(errs, ValidationError{
			Field:   "agents.trend.adx_threshold",
			Message: "ADX threshold must be between 0 and 100",
		})
	}

	// Validate confidence threshold
	if cfg.ConfidenceThreshold < 0 || cfg.ConfidenceThreshold > 1 {
		errs = append(errs, ValidationError{
			Field:   "agents.trend.confidence_threshold",
			Message: "confidence threshold must be between 0 and 1",
		})
	}

	// Validate risk management if set
	if cfg.RiskManagement.StopLossPct > 0 || cfg.RiskManagement.TakeProfitPct > 0 {
		if err := validateRiskManagement(&cfg.RiskManagement, "agents.trend.risk_management"); err != nil {
			errs = append(errs, err...)
		}
	}

	return errs
}

func validateReversionConfig(cfg *ReversionAgentConfig) ValidationErrors {
	var errs ValidationErrors

	// Validate RSI thresholds
	if cfg.EntryConditions.RSIOversold < 0 || cfg.EntryConditions.RSIOversold > 100 {
		errs = append(errs, ValidationError{
			Field:   "agents.reversion.entry_conditions.rsi_oversold",
			Message: "RSI oversold must be between 0 and 100",
		})
	}

	if cfg.ExitConditions.RSINeutral < 0 || cfg.ExitConditions.RSINeutral > 100 {
		errs = append(errs, ValidationError{
			Field:   "agents.reversion.exit_conditions.rsi_neutral",
			Message: "RSI neutral must be between 0 and 100",
		})
	}

	// Validate stop loss and take profit
	if cfg.ExitConditions.StopLossPct < 0 || cfg.ExitConditions.StopLossPct > 1 {
		errs = append(errs, ValidationError{
			Field:   "agents.reversion.exit_conditions.stop_loss_pct",
			Message: "stop loss must be between 0 and 1 (0-100%)",
		})
	}

	if cfg.ExitConditions.TakeProfitPct < 0 || cfg.ExitConditions.TakeProfitPct > 1 {
		errs = append(errs, ValidationError{
			Field:   "agents.reversion.exit_conditions.take_profit_pct",
			Message: "take profit must be between 0 and 1 (0-100%)",
		})
	}

	return errs
}

func validateRiskManagement(rm *RiskManagement, prefix string) ValidationErrors {
	var errs ValidationErrors

	// Stop loss validation
	if rm.StopLossPct < 0 || rm.StopLossPct > 0.5 {
		errs = append(errs, ValidationError{
			Field:   prefix + ".stop_loss_pct",
			Message: "stop loss must be between 0 and 0.5 (0-50%)",
		})
	}

	// Take profit validation
	if rm.TakeProfitPct < 0 || rm.TakeProfitPct > 1 {
		errs = append(errs, ValidationError{
			Field:   prefix + ".take_profit_pct",
			Message: "take profit must be between 0 and 1 (0-100%)",
		})
	}

	// Trailing stop validation
	if rm.TrailingStopPct < 0 || rm.TrailingStopPct > 0.5 {
		errs = append(errs, ValidationError{
			Field:   prefix + ".trailing_stop_pct",
			Message: "trailing stop must be between 0 and 0.5 (0-50%)",
		})
	}

	// Risk/reward validation
	if rm.MinRiskReward < 0 {
		errs = append(errs, ValidationError{
			Field:   prefix + ".min_risk_reward",
			Message: "minimum risk/reward ratio cannot be negative",
		})
	}

	return errs
}

func (s *StrategyConfig) validateRisk() ValidationErrors {
	var errs ValidationErrors

	// Portfolio exposure
	if s.Risk.MaxPortfolioExposure < 0 || s.Risk.MaxPortfolioExposure > 1 {
		errs = append(errs, ValidationError{
			Field:   "risk.max_portfolio_exposure",
			Message: "max portfolio exposure must be between 0 and 1",
		})
	}

	// Position size
	if s.Risk.MaxPositionSize < 0 || s.Risk.MaxPositionSize > 1 {
		errs = append(errs, ValidationError{
			Field:   "risk.max_position_size",
			Message: "max position size must be between 0 and 1",
		})
	}

	// Max positions
	if s.Risk.MaxPositions < 1 {
		errs = append(errs, ValidationError{
			Field:   "risk.max_positions",
			Message: "max positions must be at least 1",
		})
	}

	// Daily loss
	if s.Risk.MaxDailyLoss < 0 || s.Risk.MaxDailyLoss > 1 {
		errs = append(errs, ValidationError{
			Field:   "risk.max_daily_loss",
			Message: "max daily loss must be between 0 and 1",
		})
	}

	// Drawdown
	if s.Risk.MaxDrawdown < 0 || s.Risk.MaxDrawdown > 1 {
		errs = append(errs, ValidationError{
			Field:   "risk.max_drawdown",
			Message: "max drawdown must be between 0 and 1",
		})
	}

	// Confidence
	if s.Risk.MinStrategyConfidence < 0 || s.Risk.MinStrategyConfidence > 1 {
		errs = append(errs, ValidationError{
			Field:   "risk.min_strategy_confidence",
			Message: "min strategy confidence must be between 0 and 1",
		})
	}

	// Consensus votes
	if s.Risk.MinConsensusVotes < 1 {
		errs = append(errs, ValidationError{
			Field:   "risk.min_consensus_votes",
			Message: "min consensus votes must be at least 1",
		})
	}

	// Default stop loss
	if s.Risk.DefaultStopLoss < 0 || s.Risk.DefaultStopLoss > 0.5 {
		errs = append(errs, ValidationError{
			Field:   "risk.default_stop_loss",
			Message: "default stop loss must be between 0 and 0.5 (0-50%)",
		})
	}

	// Default take profit
	if s.Risk.DefaultTakeProfit < 0 || s.Risk.DefaultTakeProfit > 1 {
		errs = append(errs, ValidationError{
			Field:   "risk.default_take_profit",
			Message: "default take profit must be between 0 and 1 (0-100%)",
		})
	}

	// Circuit breakers - validate even if not enabled, as values may be enabled later
	if s.Risk.CircuitBreakers.MaxTradesPerHour < 0 {
		errs = append(errs, ValidationError{
			Field:   "risk.circuit_breakers.max_trades_per_hour",
			Message: "max trades per hour cannot be negative",
		})
	}
	if s.Risk.CircuitBreakers.MaxLossesPerDay < 0 {
		errs = append(errs, ValidationError{
			Field:   "risk.circuit_breakers.max_losses_per_day",
			Message: "max losses per day cannot be negative",
		})
	}
	if s.Risk.CircuitBreakers.VolatilityThreshold < 0 {
		errs = append(errs, ValidationError{
			Field:   "risk.circuit_breakers.volatility_threshold",
			Message: "volatility threshold cannot be negative",
		})
	}
	if s.Risk.CircuitBreakers.DrawdownHalt < 0 || s.Risk.CircuitBreakers.DrawdownHalt > 1 {
		errs = append(errs, ValidationError{
			Field:   "risk.circuit_breakers.drawdown_halt",
			Message: "drawdown halt must be between 0 and 1",
		})
	}

	// Position sizing
	if s.Risk.KellyFraction < 0 || s.Risk.KellyFraction > 1 {
		errs = append(errs, ValidationError{
			Field:   "risk.kelly_fraction",
			Message: "Kelly fraction must be between 0 and 1",
		})
	}

	if s.Risk.MinPositionUSD > 0 && s.Risk.MaxPositionUSD > 0 {
		if s.Risk.MinPositionUSD > s.Risk.MaxPositionUSD {
			errs = append(errs, ValidationError{
				Field:   "risk.min_position_usd",
				Message: "min position USD must be less than max position USD",
			})
		}
	}

	// Cross-field validations
	// MaxPositionSize cannot exceed MaxPortfolioExposure
	if s.Risk.MaxPositionSize > s.Risk.MaxPortfolioExposure {
		errs = append(errs, ValidationError{
			Field:   "risk.max_position_size",
			Message: fmt.Sprintf("max position size (%.2f) cannot exceed max portfolio exposure (%.2f)", s.Risk.MaxPositionSize, s.Risk.MaxPortfolioExposure),
		})
	}

	// MaxDailyLoss should not exceed MaxDrawdown (daily limit should be stricter than total limit)
	if s.Risk.MaxDailyLoss > s.Risk.MaxDrawdown {
		errs = append(errs, ValidationError{
			Field:   "risk.max_daily_loss",
			Message: fmt.Sprintf("max daily loss (%.2f) should not exceed max drawdown (%.2f) - daily limit should be stricter than total limit", s.Risk.MaxDailyLoss, s.Risk.MaxDrawdown),
		})
	}

	// DefaultStopLoss should be less than DefaultTakeProfit for a positive risk/reward ratio
	if s.Risk.DefaultStopLoss > 0 && s.Risk.DefaultTakeProfit > 0 {
		if s.Risk.DefaultStopLoss >= s.Risk.DefaultTakeProfit {
			errs = append(errs, ValidationError{
				Field:   "risk.default_stop_loss",
				Message: fmt.Sprintf("default stop loss (%.2f) should be less than default take profit (%.2f) for positive risk/reward", s.Risk.DefaultStopLoss, s.Risk.DefaultTakeProfit),
			})
		}
	}

	// CircuitBreaker drawdown_halt should not exceed MaxDrawdown
	if s.Risk.CircuitBreakers.Enabled && s.Risk.CircuitBreakers.DrawdownHalt > 0 {
		if s.Risk.CircuitBreakers.DrawdownHalt > s.Risk.MaxDrawdown {
			errs = append(errs, ValidationError{
				Field:   "risk.circuit_breakers.drawdown_halt",
				Message: fmt.Sprintf("circuit breaker drawdown halt (%.2f) should not exceed max drawdown (%.2f)", s.Risk.CircuitBreakers.DrawdownHalt, s.Risk.MaxDrawdown),
			})
		}
	}

	return errs
}

func (s *StrategyConfig) validateOrchestration() ValidationErrors {
	var errs ValidationErrors

	// Voting method
	validMethods := []string{"weighted_consensus", "majority"}
	if s.Orchestration.VotingMethod != "" {
		valid := false
		for _, m := range validMethods {
			if s.Orchestration.VotingMethod == m {
				valid = true
				break
			}
		}
		if !valid {
			errs = append(errs, ValidationError{
				Field:   "orchestration.voting_method",
				Message: fmt.Sprintf("voting method must be one of: %v", validMethods),
			})
		}
	}

	// Min votes
	if s.Orchestration.MinVotes < 1 {
		errs = append(errs, ValidationError{
			Field:   "orchestration.min_votes",
			Message: "min votes must be at least 1",
		})
	}

	// Quorum
	if s.Orchestration.Quorum < 0 || s.Orchestration.Quorum > 1 {
		errs = append(errs, ValidationError{
			Field:   "orchestration.quorum",
			Message: "quorum must be between 0 and 1",
		})
	}

	// Validate duration strings
	if s.Orchestration.StepInterval != "" {
		if _, err := time.ParseDuration(s.Orchestration.StepInterval); err != nil {
			errs = append(errs, ValidationError{
				Field:   "orchestration.step_interval",
				Message: fmt.Sprintf("invalid duration format: %s (use formats like '30s', '5m', '1h')", s.Orchestration.StepInterval),
			})
		}
	}

	if s.Orchestration.MaxSignalAge != "" {
		if _, err := time.ParseDuration(s.Orchestration.MaxSignalAge); err != nil {
			errs = append(errs, ValidationError{
				Field:   "orchestration.max_signal_age",
				Message: fmt.Sprintf("invalid duration format: %s (use formats like '30s', '5m', '1h')", s.Orchestration.MaxSignalAge),
			})
		}
	}

	// Consensus
	if s.Orchestration.MinConsensus < 0 || s.Orchestration.MinConsensus > 1 {
		errs = append(errs, ValidationError{
			Field:   "orchestration.min_consensus",
			Message: "min consensus must be between 0 and 1",
		})
	}

	// Confidence
	if s.Orchestration.MinConfidence < 0 || s.Orchestration.MinConfidence > 1 {
		errs = append(errs, ValidationError{
			Field:   "orchestration.min_confidence",
			Message: "min confidence must be between 0 and 1",
		})
	}

	// LLM temperature
	if s.Orchestration.LLMReasoningEnabled {
		if s.Orchestration.LLMTemperature < 0 || s.Orchestration.LLMTemperature > 2 {
			errs = append(errs, ValidationError{
				Field:   "orchestration.llm_temperature",
				Message: "LLM temperature must be between 0 and 2",
			})
		}
	}

	return errs
}

func (s *StrategyConfig) validateIndicators() ValidationErrors {
	var errs ValidationErrors

	// RSI validation
	if s.Indicators.RSI.Period < 1 {
		errs = append(errs, ValidationError{
			Field:   "indicators.rsi.period",
			Message: "RSI period must be at least 1",
		})
	}
	if s.Indicators.RSI.Overbought < 50 || s.Indicators.RSI.Overbought > 100 {
		errs = append(errs, ValidationError{
			Field:   "indicators.rsi.overbought",
			Message: "RSI overbought must be between 50 and 100",
		})
	}
	if s.Indicators.RSI.Oversold < 0 || s.Indicators.RSI.Oversold > 50 {
		errs = append(errs, ValidationError{
			Field:   "indicators.rsi.oversold",
			Message: "RSI oversold must be between 0 and 50",
		})
	}
	if s.Indicators.RSI.Oversold >= s.Indicators.RSI.Overbought {
		errs = append(errs, ValidationError{
			Field:   "indicators.rsi.oversold",
			Message: "RSI oversold must be less than overbought",
		})
	}

	// MACD validation
	if s.Indicators.MACD.FastPeriod < 1 {
		errs = append(errs, ValidationError{
			Field:   "indicators.macd.fast_period",
			Message: "MACD fast period must be at least 1",
		})
	}
	if s.Indicators.MACD.SlowPeriod < 1 {
		errs = append(errs, ValidationError{
			Field:   "indicators.macd.slow_period",
			Message: "MACD slow period must be at least 1",
		})
	}
	if s.Indicators.MACD.FastPeriod >= s.Indicators.MACD.SlowPeriod {
		errs = append(errs, ValidationError{
			Field:   "indicators.macd.fast_period",
			Message: "MACD fast period must be less than slow period",
		})
	}
	if s.Indicators.MACD.SignalPeriod < 1 {
		errs = append(errs, ValidationError{
			Field:   "indicators.macd.signal_period",
			Message: "MACD signal period must be at least 1",
		})
	}

	// Bollinger validation
	if s.Indicators.Bollinger.Period < 1 {
		errs = append(errs, ValidationError{
			Field:   "indicators.bollinger.period",
			Message: "Bollinger period must be at least 1",
		})
	}
	if s.Indicators.Bollinger.StdDev <= 0 {
		errs = append(errs, ValidationError{
			Field:   "indicators.bollinger.std_dev",
			Message: "Bollinger std dev must be greater than 0",
		})
	}

	// ADX validation
	if s.Indicators.ADX.Period < 1 {
		errs = append(errs, ValidationError{
			Field:   "indicators.adx.period",
			Message: "ADX period must be at least 1",
		})
	}
	if s.Indicators.ADX.TrendThreshold < 0 || s.Indicators.ADX.TrendThreshold > 100 {
		errs = append(errs, ValidationError{
			Field:   "indicators.adx.trend_threshold",
			Message: "ADX trend threshold must be between 0 and 100",
		})
	}

	return errs
}

func isVersionSupported(version string) bool {
	for _, v := range SupportedSchemaVersions {
		if v == version {
			return true
		}
	}
	return false
}

// ValidateQuick performs minimal validation for quick checks
func (s *StrategyConfig) ValidateQuick() error {
	if s.Metadata.SchemaVersion == "" {
		return fmt.Errorf("%w: metadata.schema_version", ErrMissingRequiredField)
	}
	if !isVersionSupported(s.Metadata.SchemaVersion) {
		return ErrInvalidSchema
	}
	if s.Metadata.Name == "" {
		return fmt.Errorf("%w: metadata.name", ErrMissingRequiredField)
	}
	return nil
}
