package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/strategy"
)

const (
	// MaxStrategyUploadSize is the maximum allowed size for strategy file uploads (10MB)
	MaxStrategyUploadSize = 10 * 1024 * 1024
)

// StrategyHandler handles HTTP requests for strategy import/export
type StrategyHandler struct {
	// mu protects concurrent access to currentStrategy
	mu sync.RWMutex

	// currentStrategy holds the active strategy
	// TODO: In production, this should be persisted to database or config file.
	// Current in-memory storage means strategies are lost on server restart.
	// See: internal/db/ for database patterns to implement persistence.
	currentStrategy *strategy.StrategyConfig

	// TODO: Add audit.Logger integration for full audit trail when database
	// persistence is implemented. Use audit.LogStrategyChange() for tracking
	// all strategy modifications (update, import, clone, merge operations).
	// See: internal/audit/audit.go for the logging infrastructure.
}

// NewStrategyHandler creates a new strategy handler
func NewStrategyHandler() *StrategyHandler {
	return &StrategyHandler{
		currentStrategy: strategy.NewDefaultStrategy("Default Strategy"),
	}
}

// RegisterRoutes registers all strategy-related routes
func (h *StrategyHandler) RegisterRoutes(router *gin.RouterGroup) {
	strategies := router.Group("/strategies")
	{
		// Current strategy
		strategies.GET("/current", h.GetCurrentStrategy)
		strategies.PUT("/current", h.UpdateCurrentStrategy)

		// Export
		strategies.GET("/export", h.ExportStrategy)
		strategies.POST("/export", h.ExportStrategyWithOptions)

		// Import
		strategies.POST("/import", h.ImportStrategy)
		strategies.POST("/validate", h.ValidateStrategy)

		// Version info
		strategies.GET("/version", h.GetVersionInfo)
		strategies.GET("/schema", h.GetSchemaInfo)

		// Operations
		strategies.POST("/clone", h.CloneStrategy)
		strategies.POST("/merge", h.MergeStrategies)
		strategies.POST("/default", h.GetDefaultStrategy)
	}
}

// GetCurrentStrategy returns the current active strategy configuration
// @Summary Get current strategy
// @Tags Strategies
// @Produce json
// @Success 200 {object} strategy.StrategyConfig
// @Router /api/v1/strategies/current [get]
func (h *StrategyHandler) GetCurrentStrategy(c *gin.Context) {
	h.mu.RLock()
	currentStrategy := h.currentStrategy
	h.mu.RUnlock()

	if currentStrategy == nil {
		h.mu.Lock()
		// Double-check after acquiring write lock
		if h.currentStrategy == nil {
			h.currentStrategy = strategy.NewDefaultStrategy("Default Strategy")
		}
		currentStrategy = h.currentStrategy
		h.mu.Unlock()
	}

	c.JSON(http.StatusOK, currentStrategy)
}

// UpdateCurrentStrategy updates the current strategy
// @Summary Update current strategy
// @Tags Strategies
// @Accept json
// @Produce json
// @Param strategy body strategy.StrategyConfig true "Strategy configuration"
// @Success 200 {object} strategy.StrategyConfig
// @Failure 400 {object} map[string]string
// @Router /api/v1/strategies/current [put]
func (h *StrategyHandler) UpdateCurrentStrategy(c *gin.Context) {
	var newStrategy strategy.StrategyConfig
	if err := c.ShouldBindJSON(&newStrategy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid strategy format",
			"details": err.Error(),
		})
		return
	}

	// Validate the strategy
	if err := newStrategy.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Strategy validation failed",
			"details": err.Error(),
		})
		return
	}

	// Update timestamps
	newStrategy.Metadata.UpdatedAt = time.Now()

	h.mu.Lock()
	h.currentStrategy = &newStrategy
	h.mu.Unlock()

	log.Info().
		Str("strategy_name", newStrategy.Metadata.Name).
		Str("strategy_id", newStrategy.Metadata.ID).
		Msg("Strategy updated")

	c.JSON(http.StatusOK, &newStrategy)
}

// ExportStrategy exports the current strategy as YAML
// @Summary Export strategy
// @Tags Strategies
// @Produce text/yaml
// @Param format query string false "Export format (yaml or json)" default(yaml)
// @Success 200 {string} string "Strategy YAML/JSON"
// @Router /api/v1/strategies/export [get]
func (h *StrategyHandler) ExportStrategy(c *gin.Context) {
	h.mu.RLock()
	currentStrategy := h.currentStrategy
	h.mu.RUnlock()

	if currentStrategy == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No strategy configured",
		})
		return
	}

	format := c.DefaultQuery("format", "yaml")
	opts := strategy.DefaultExportOptions()

	switch strings.ToLower(format) {
	case "json":
		opts.Format = strategy.FormatJSON
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=strategy_%s.json", currentStrategy.Metadata.ID))
	default:
		opts.Format = strategy.FormatYAML
		c.Header("Content-Type", "text/yaml")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=strategy_%s.yaml", currentStrategy.Metadata.ID))
	}

	data, err := strategy.Export(currentStrategy, opts)
	if err != nil {
		log.Err(err).Msg("Failed to export strategy")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to export strategy",
		})
		return
	}

	c.Data(http.StatusOK, c.Writer.Header().Get("Content-Type"), data)
}

// ExportOptions defines options for strategy export
type ExportOptions struct {
	Format          string `json:"format"`
	IncludeComments bool   `json:"include_comments"`
	PrettyPrint     bool   `json:"pretty_print"`
}

// ExportStrategyWithOptions exports strategy with custom options
// @Summary Export strategy with options
// @Tags Strategies
// @Accept json
// @Produce text/yaml
// @Param options body ExportOptions true "Export options"
// @Success 200 {string} string "Strategy YAML/JSON"
// @Router /api/v1/strategies/export [post]
func (h *StrategyHandler) ExportStrategyWithOptions(c *gin.Context) {
	h.mu.RLock()
	currentStrategy := h.currentStrategy
	h.mu.RUnlock()

	if currentStrategy == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No strategy configured",
		})
		return
	}

	var exportOpts ExportOptions
	if err := c.ShouldBindJSON(&exportOpts); err != nil {
		// Use defaults if no options provided
		exportOpts = ExportOptions{
			Format:          "yaml",
			IncludeComments: true,
			PrettyPrint:     true,
		}
	}

	opts := strategy.ExportOptions{
		IncludeMetadata: true,
		PrettyPrint:     exportOpts.PrettyPrint,
		AddComments:     exportOpts.IncludeComments,
	}

	switch strings.ToLower(exportOpts.Format) {
	case "json":
		opts.Format = strategy.FormatJSON
		c.Header("Content-Type", "application/json")
	default:
		opts.Format = strategy.FormatYAML
		c.Header("Content-Type", "text/yaml")
	}

	data, err := strategy.Export(currentStrategy, opts)
	if err != nil {
		log.Err(err).Msg("Failed to export strategy")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to export strategy",
		})
		return
	}

	c.Data(http.StatusOK, c.Writer.Header().Get("Content-Type"), data)
}

// ImportRequest defines the request for importing a strategy
type ImportRequest struct {
	// Strategy data as a string (YAML or JSON)
	Data string `json:"data"`

	// Override metadata
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`

	// Options
	ValidateStrict bool `json:"validate_strict"`
	ApplyNow       bool `json:"apply_now"`
}

// ImportStrategy imports a strategy from YAML or JSON
// @Summary Import strategy
// @Tags Strategies
// @Accept json,multipart/form-data
// @Produce json
// @Param request body ImportRequest false "Import request with strategy data"
// @Param file formData file false "Strategy file (YAML or JSON)"
// @Success 200 {object} strategy.StrategyConfig
// @Failure 400 {object} map[string]string
// @Failure 413 {object} map[string]string "File too large"
// @Router /api/v1/strategies/import [post]
func (h *StrategyHandler) ImportStrategy(c *gin.Context) {
	var data []byte
	var err error

	contentType := c.GetHeader("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		// Handle file upload
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "No file provided",
			})
			return
		}
		defer file.Close()

		// Check file size before reading
		if header.Size > MaxStrategyUploadSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":    "File too large",
				"details":  fmt.Sprintf("Maximum file size is %d bytes (%d MB)", MaxStrategyUploadSize, MaxStrategyUploadSize/1024/1024),
				"max_size": MaxStrategyUploadSize,
			})
			return
		}

		// Use LimitReader for additional safety
		limitedReader := io.LimitReader(file, MaxStrategyUploadSize+1)
		data, err = io.ReadAll(limitedReader)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Failed to read file",
			})
			return
		}

		// Double-check size after reading
		if len(data) > MaxStrategyUploadSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":    "File too large",
				"details":  fmt.Sprintf("Maximum file size is %d bytes (%d MB)", MaxStrategyUploadSize, MaxStrategyUploadSize/1024/1024),
				"max_size": MaxStrategyUploadSize,
			})
			return
		}

		log.Info().
			Str("filename", header.Filename).
			Int("size", len(data)).
			Msg("Importing strategy from file")
	} else {
		// Handle JSON request body
		var req ImportRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request format",
				"details": err.Error(),
			})
			return
		}

		if req.Data == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Strategy data is required",
			})
			return
		}

		// Check data size
		if len(req.Data) > MaxStrategyUploadSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":    "Strategy data too large",
				"details":  fmt.Sprintf("Maximum size is %d bytes (%d MB)", MaxStrategyUploadSize, MaxStrategyUploadSize/1024/1024),
				"max_size": MaxStrategyUploadSize,
			})
			return
		}

		data = []byte(req.Data)

		// Set up import options with overrides
		opts := strategy.DefaultImportOptions()
		opts.ValidateStrict = req.ValidateStrict

		if req.Name != "" || req.Description != "" || len(req.Tags) > 0 {
			opts.OverrideMetadata = &strategy.StrategyMetadata{
				Name:        req.Name,
				Description: req.Description,
				Tags:        req.Tags,
			}
		}

		imported, err := strategy.Import(data, opts)
		if err != nil {
			log.Err(err).Msg("Failed to import strategy")
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Failed to import strategy",
				"details": err.Error(),
			})
			return
		}

		// Apply if requested
		if req.ApplyNow {
			h.mu.Lock()
			h.currentStrategy = imported
			h.mu.Unlock()
			log.Info().
				Str("strategy_name", imported.Metadata.Name).
				Str("strategy_id", imported.Metadata.ID).
				Msg("Strategy imported and applied")
		}

		c.JSON(http.StatusOK, gin.H{
			"strategy": imported,
			"applied":  req.ApplyNow,
		})
		return
	}

	// For file upload, use default options
	opts := strategy.DefaultImportOptions()
	imported, err := strategy.Import(data, opts)
	if err != nil {
		log.Err(err).Msg("Failed to import strategy from file")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to import strategy",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"strategy": imported,
		"applied":  false,
	})
}

// ValidateRequest defines the request for validating a strategy
type ValidateRequest struct {
	Data   string `json:"data" binding:"required"`
	Strict bool   `json:"strict"`
}

// ValidateStrategy validates a strategy without importing it
// @Summary Validate strategy
// @Tags Strategies
// @Accept json
// @Produce json
// @Param request body ValidateRequest true "Validate request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /api/v1/strategies/validate [post]
func (h *StrategyHandler) ValidateStrategy(c *gin.Context) {
	var req ValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	opts := strategy.ImportOptions{
		ValidateStrict:     req.Strict,
		AllowUnknownFields: true,
		GenerateNewID:      false,
	}

	imported, err := strategy.Import([]byte(req.Data), opts)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	// Get version info
	versionInfo, _ := strategy.GetVersionInfo(imported)

	c.JSON(http.StatusOK, gin.H{
		"valid":          true,
		"name":           imported.Metadata.Name,
		"schema_version": imported.Metadata.SchemaVersion,
		"version_info":   versionInfo,
	})
}

// GetVersionInfo returns version information
// @Summary Get version info
// @Tags Strategies
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/strategies/version [get]
func (h *StrategyHandler) GetVersionInfo(c *gin.Context) {
	info := gin.H{
		"current_schema_version": strategy.GetSchemaVersion(),
		"supported_versions":     strategy.SupportedSchemaVersions,
	}

	h.mu.RLock()
	currentStrategy := h.currentStrategy
	h.mu.RUnlock()

	if currentStrategy != nil {
		strategyInfo, _ := strategy.GetVersionInfo(currentStrategy)
		info["strategy"] = strategyInfo
	}

	c.JSON(http.StatusOK, info)
}

// GetSchemaInfo returns the strategy schema documentation
// @Summary Get schema info
// @Tags Strategies
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/strategies/schema [get]
func (h *StrategyHandler) GetSchemaInfo(c *gin.Context) {
	// Return a description of the schema
	schema := gin.H{
		"version": strategy.GetSchemaVersion(),
		"fields": gin.H{
			"metadata": gin.H{
				"required": []string{"schema_version", "name"},
				"optional": []string{"id", "description", "author", "version", "tags", "created_at", "updated_at", "source"},
			},
			"agents": gin.H{
				"weights": gin.H{
					"description": "Voting weights for each agent (0-1)",
					"fields":      []string{"technical", "orderbook", "sentiment", "trend", "reversion", "arbitrage"},
				},
				"enabled": gin.H{
					"description": "Which agents are active",
					"fields":      []string{"technical", "orderbook", "sentiment", "trend", "reversion", "arbitrage", "risk"},
				},
			},
			"risk": gin.H{
				"required": []string{"max_portfolio_exposure", "max_position_size", "max_positions", "max_daily_loss", "max_drawdown", "min_strategy_confidence", "min_consensus_votes", "default_stop_loss", "default_take_profit"},
				"optional": []string{"max_correlation", "min_sharpe_ratio", "max_var_95", "max_leverage", "circuit_breakers", "kelly_fraction", "min_position_usd", "max_position_usd"},
			},
			"orchestration": gin.H{
				"required": []string{"voting_enabled", "voting_method", "min_votes", "quorum", "step_interval", "max_signal_age", "min_consensus", "min_confidence"},
				"optional": []string{"llm_reasoning_enabled", "llm_temperature"},
			},
			"indicators": gin.H{
				"description": "Technical indicator configurations",
				"fields":      []string{"rsi", "macd", "bollinger", "ema", "adx"},
			},
		},
	}

	c.JSON(http.StatusOK, schema)
}

// CloneRequest defines the request for cloning a strategy
type CloneRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CloneStrategy creates a copy of the current strategy
// @Summary Clone strategy
// @Tags Strategies
// @Accept json
// @Produce json
// @Param request body CloneRequest true "Clone request"
// @Success 200 {object} strategy.StrategyConfig
// @Failure 400 {object} map[string]string
// @Router /api/v1/strategies/clone [post]
func (h *StrategyHandler) CloneStrategy(c *gin.Context) {
	h.mu.RLock()
	currentStrategy := h.currentStrategy
	h.mu.RUnlock()

	if currentStrategy == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No strategy to clone",
		})
		return
	}

	var req CloneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Use default name if not provided
		req.Name = currentStrategy.Metadata.Name + " (Copy)"
	}

	cloned, err := strategy.Clone(currentStrategy)
	if err != nil {
		log.Err(err).Msg("Failed to clone strategy")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to clone strategy",
		})
		return
	}

	if req.Name != "" {
		cloned.Metadata.Name = req.Name
	}
	if req.Description != "" {
		cloned.Metadata.Description = req.Description
	}

	c.JSON(http.StatusOK, cloned)
}

// MergeRequest defines the request for merging strategies
type MergeRequest struct {
	// Base strategy (optional, uses current if not provided)
	Base *strategy.StrategyConfig `json:"base,omitempty"`

	// Override values to apply
	Override *strategy.StrategyConfig `json:"override" binding:"required"`

	// Apply the result to current strategy
	Apply bool `json:"apply"`
}

// MergeStrategies merges two strategies
// @Summary Merge strategies
// @Tags Strategies
// @Accept json
// @Produce json
// @Param request body MergeRequest true "Merge request"
// @Success 200 {object} strategy.StrategyConfig
// @Failure 400 {object} map[string]string
// @Router /api/v1/strategies/merge [post]
func (h *StrategyHandler) MergeStrategies(c *gin.Context) {
	var req MergeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	base := req.Base
	if base == nil {
		h.mu.RLock()
		base = h.currentStrategy
		h.mu.RUnlock()
	}

	if base == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No base strategy available",
		})
		return
	}

	merged, err := strategy.Merge(base, req.Override)
	if err != nil {
		log.Err(err).Msg("Failed to merge strategies")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to merge strategies",
			"details": err.Error(),
		})
		return
	}

	// Generate new ID for merged strategy
	merged.Metadata.ID = uuid.New().String()
	merged.Metadata.Name = base.Metadata.Name + " (Merged)"

	if req.Apply {
		h.mu.Lock()
		h.currentStrategy = merged
		h.mu.Unlock()
	}

	c.JSON(http.StatusOK, gin.H{
		"strategy": merged,
		"applied":  req.Apply,
	})
}

// GetDefaultStrategy returns the default strategy configuration
// @Summary Get default strategy
// @Tags Strategies
// @Produce json
// @Param name query string false "Strategy name"
// @Success 200 {object} strategy.StrategyConfig
// @Router /api/v1/strategies/default [post]
func (h *StrategyHandler) GetDefaultStrategy(c *gin.Context) {
	name := c.DefaultQuery("name", "New Strategy")

	// Parse request body for name if provided
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err == nil && req.Name != "" {
		name = req.Name
	}

	defaultStrategy := strategy.NewDefaultStrategy(name)

	c.JSON(http.StatusOK, defaultStrategy)
}

// RegisterStrategyRoutes registers strategy routes with rate limiting
func (h *StrategyHandler) RegisterRoutesWithRateLimiter(router *gin.RouterGroup, readMiddleware, writeMiddleware gin.HandlerFunc) {
	// Helper to conditionally apply middleware
	applyRead := func(handlers ...gin.HandlerFunc) []gin.HandlerFunc {
		if readMiddleware != nil {
			return append([]gin.HandlerFunc{readMiddleware}, handlers...)
		}
		return handlers
	}
	applyWrite := func(handlers ...gin.HandlerFunc) []gin.HandlerFunc {
		if writeMiddleware != nil {
			return append([]gin.HandlerFunc{writeMiddleware}, handlers...)
		}
		return handlers
	}

	strategies := router.Group("/strategies")
	{
		// Read operations
		strategies.GET("/current", applyRead(h.GetCurrentStrategy)...)
		strategies.GET("/export", applyRead(h.ExportStrategy)...)
		strategies.GET("/version", applyRead(h.GetVersionInfo)...)
		strategies.GET("/schema", applyRead(h.GetSchemaInfo)...)

		// Write operations
		strategies.PUT("/current", applyWrite(h.UpdateCurrentStrategy)...)
		strategies.POST("/export", applyWrite(h.ExportStrategyWithOptions)...)
		strategies.POST("/import", applyWrite(h.ImportStrategy)...)
		strategies.POST("/validate", applyWrite(h.ValidateStrategy)...)
		strategies.POST("/clone", applyWrite(h.CloneStrategy)...)
		strategies.POST("/merge", applyWrite(h.MergeStrategies)...)
		strategies.POST("/default", applyRead(h.GetDefaultStrategy)...)
	}
}
