package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/audit"
	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/strategy"
)

const (
	// MaxStrategyUploadSize is the maximum allowed size for strategy file uploads (10MB)
	MaxStrategyUploadSize = 10 * 1024 * 1024

	// DefaultAuditTimeout is the default timeout for audit logging operations
	DefaultAuditTimeout = 5 * time.Second

	// DefaultValidationTimeout is the default timeout for validation operations
	DefaultValidationTimeout = 30 * time.Second

	// ExportFormatJSON is the JSON export format identifier
	ExportFormatJSON = "json"

	// ExportFormatYAML is the YAML export format identifier
	ExportFormatYAML = "yaml"
)

// AllowedStrategyExtensions defines valid file extensions for strategy uploads
var AllowedStrategyExtensions = map[string]bool{
	".yaml": true,
	".yml":  true,
	".json": true,
}

// filenameRegex matches characters that are safe for filenames
var filenameRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// StrategyHandler handles HTTP requests for strategy import/export
type StrategyHandler struct {
	// mu protects concurrent access to currentStrategy
	mu sync.RWMutex

	// currentStrategy holds the active strategy (in-memory cache)
	currentStrategy *strategy.StrategyConfig

	// repo handles database persistence (optional - if nil, uses in-memory only)
	repo *db.StrategyRepository

	// auditLogger logs strategy operations for audit trail (optional)
	auditLogger *audit.Logger

	// validationTimeout is the timeout for validation operations
	validationTimeout time.Duration
}

// NewStrategyHandler creates a new strategy handler (in-memory only, for testing)
func NewStrategyHandler() *StrategyHandler {
	return &StrategyHandler{
		currentStrategy:   strategy.NewDefaultStrategy("Default Strategy"),
		validationTimeout: DefaultValidationTimeout,
	}
}

// NewStrategyHandlerWithAudit creates a new strategy handler with audit logging
func NewStrategyHandlerWithAudit(auditLogger *audit.Logger) *StrategyHandler {
	return &StrategyHandler{
		currentStrategy:   strategy.NewDefaultStrategy("Default Strategy"),
		auditLogger:       auditLogger,
		validationTimeout: DefaultValidationTimeout,
	}
}

// NewStrategyHandlerWithDB creates a new strategy handler with database persistence
func NewStrategyHandlerWithDB(repo *db.StrategyRepository, auditLogger *audit.Logger) *StrategyHandler {
	h := &StrategyHandler{
		repo:              repo,
		auditLogger:       auditLogger,
		validationTimeout: DefaultValidationTimeout,
	}

	// Try to load active strategy from database
	if repo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		activeStrategy, err := repo.GetActive(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to load active strategy from database, using default")
			h.currentStrategy = strategy.NewDefaultStrategy("Default Strategy")
		} else if activeStrategy != nil {
			h.currentStrategy = activeStrategy
			log.Info().
				Str("strategy_id", activeStrategy.Metadata.ID).
				Str("strategy_name", activeStrategy.Metadata.Name).
				Msg("Loaded active strategy from database")
		} else {
			h.currentStrategy = strategy.NewDefaultStrategy("Default Strategy")
			log.Info().Msg("No active strategy in database, using default")
		}
	} else {
		h.currentStrategy = strategy.NewDefaultStrategy("Default Strategy")
	}

	return h
}

// SetValidationTimeout sets the timeout for validation operations
func (h *StrategyHandler) SetValidationTimeout(timeout time.Duration) {
	h.validationTimeout = timeout
}

// sanitizeFilename removes potentially dangerous characters from a filename
func sanitizeFilename(filename string) string {
	// Get base name to remove any path components
	filename = filepath.Base(filename)
	// Replace unsafe characters with underscores
	sanitized := filenameRegex.ReplaceAllString(filename, "_")
	// Ensure it's not empty
	if sanitized == "" || sanitized == "_" {
		sanitized = "strategy"
	}
	return sanitized
}

// isAllowedExtension checks if the file extension is valid for strategy uploads
func isAllowedExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return AllowedStrategyExtensions[ext]
}

// logAuditEvent is a helper to log audit events if the logger is configured
func (h *StrategyHandler) logAuditEvent(c *gin.Context, eventType audit.EventType, strategyID, strategyName string, metadata map[string]interface{}, success bool, errorMsg string) {
	if h.auditLogger == nil {
		return
	}

	// Extract user info from context (if authentication is implemented)
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	ipAddress := c.ClientIP()

	// Use request context with timeout for audit logging
	ctx, cancel := context.WithTimeout(c.Request.Context(), DefaultAuditTimeout)
	defer cancel()

	if err := h.auditLogger.LogStrategyChange(ctx, eventType, userID, ipAddress, strategyID, strategyName, metadata, success, errorMsg); err != nil {
		log.Warn().Err(err).Msg("Failed to log audit event")
	}
}

// persistStrategy saves the strategy to database if repository is configured
func (h *StrategyHandler) persistStrategy(ctx context.Context, s *strategy.StrategyConfig, setActive bool) error {
	if h.repo == nil {
		return nil // No persistence configured
	}

	if setActive {
		return h.repo.SaveAndActivate(ctx, s)
	}
	return h.repo.Save(ctx, s)
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

	// Validate the strategy with timeout
	validateCtx, validateCancel := context.WithTimeout(c.Request.Context(), h.validationTimeout)
	defer validateCancel()

	// Run validation in a goroutine to respect context cancellation
	validationDone := make(chan error, 1)
	go func() {
		validationDone <- newStrategy.Validate()
	}()

	select {
	case err := <-validationDone:
		if err != nil {
			h.logAuditEvent(c, audit.EventTypeStrategyUpdated, newStrategy.Metadata.ID, newStrategy.Metadata.Name, nil, false, err.Error())
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Strategy validation failed",
				"details": err.Error(),
			})
			return
		}
	case <-validateCtx.Done():
		c.JSON(http.StatusRequestTimeout, gin.H{
			"error": "Validation timeout exceeded",
		})
		return
	}

	// Update timestamps
	newStrategy.Metadata.UpdatedAt = time.Now()

	// Deep copy the strategy to avoid returning a reference to local variable
	// and to ensure thread-safe access
	strategyCopy := newStrategy

	h.mu.Lock()
	// Store pointer to our copy
	h.currentStrategy = &strategyCopy

	// Persist to database while still holding the lock to ensure consistency
	// between in-memory state and database
	if h.repo != nil {
		if err := h.repo.SaveAndActivate(c.Request.Context(), &strategyCopy); err != nil {
			h.mu.Unlock()
			log.Error().Err(err).Msg("Failed to persist strategy to database")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to persist strategy",
				"details": err.Error(),
			})
			return
		}
	}
	h.mu.Unlock()

	// Log successful update (outside lock to avoid holding it during I/O)
	h.logAuditEvent(c, audit.EventTypeStrategyUpdated, strategyCopy.Metadata.ID, strategyCopy.Metadata.Name, nil, true, "")

	log.Info().
		Str("strategy_name", strategyCopy.Metadata.Name).
		Str("strategy_id", strategyCopy.Metadata.ID).
		Msg("Strategy updated")

	// Return a fresh copy to avoid any potential data races
	c.JSON(http.StatusOK, strategyCopy)
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

	format := c.DefaultQuery("format", ExportFormatYAML)
	opts := strategy.DefaultExportOptions()

	// Sanitize strategy ID for use in filename
	safeID := sanitizeFilename(currentStrategy.Metadata.ID)

	switch strings.ToLower(format) {
	case ExportFormatJSON:
		opts.Format = strategy.FormatJSON
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"strategy_%s.json\"", safeID))
	default:
		opts.Format = strategy.FormatYAML
		c.Header("Content-Type", "text/yaml")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"strategy_%s.yaml\"", safeID))
	}

	data, err := strategy.Export(currentStrategy, opts)
	if err != nil {
		h.logAuditEvent(c, audit.EventTypeStrategyExported, currentStrategy.Metadata.ID, currentStrategy.Metadata.Name, nil, false, err.Error())
		log.Err(err).Msg("Failed to export strategy")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to export strategy",
		})
		return
	}

	// Log successful export
	h.logAuditEvent(c, audit.EventTypeStrategyExported, currentStrategy.Metadata.ID, currentStrategy.Metadata.Name, map[string]interface{}{
		"format": format,
	}, true, "")

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
			Format:          ExportFormatYAML,
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
	case ExportFormatJSON:
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
	var opts strategy.ImportOptions
	var applyNow bool
	var sourceFilename string

	contentType := c.GetHeader("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		// Handle file upload
		file, header, fileErr := c.Request.FormFile("file")
		if fileErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "No file provided",
			})
			return
		}
		defer file.Close()

		// Validate file extension
		if !isAllowedExtension(header.Filename) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid file extension",
				"details": fmt.Sprintf("Allowed extensions: .yaml, .yml, .json. Got: %s", filepath.Ext(header.Filename)),
			})
			return
		}

		sourceFilename = header.Filename

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
		var readErr error
		data, readErr = io.ReadAll(limitedReader)
		if readErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Failed to read file",
				"details": readErr.Error(),
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

		// File uploads use default options, but can specify apply_now via form field
		opts = strategy.DefaultImportOptions()

		// Check for apply_now form field (accepts "true", "1", "yes")
		applyNowField := c.PostForm("apply_now")
		applyNow = applyNowField == "true" || applyNowField == "1" || applyNowField == "yes"

		// Check for validate_strict form field
		validateStrictField := c.PostForm("validate_strict")
		opts.ValidateStrict = validateStrictField == "true" || validateStrictField == "1" || validateStrictField == "yes"
	} else if strings.Contains(contentType, "application/json") || contentType == "" {
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
		opts = strategy.DefaultImportOptions()
		opts.ValidateStrict = req.ValidateStrict

		if req.Name != "" || req.Description != "" || len(req.Tags) > 0 {
			opts.OverrideMetadata = &strategy.StrategyMetadata{
				Name:        req.Name,
				Description: req.Description,
				Tags:        req.Tags,
			}
		}

		applyNow = req.ApplyNow
	} else {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{
			"error":   "Unsupported content type",
			"details": fmt.Sprintf("Expected application/json or multipart/form-data, got %s", contentType),
		})
		return
	}

	// Import the strategy with timeout (shared path for both file upload and JSON)
	importCtx, importCancel := context.WithTimeout(c.Request.Context(), h.validationTimeout)
	defer importCancel()

	importDone := make(chan struct {
		strategy *strategy.StrategyConfig
		err      error
	}, 1)

	go func() {
		imported, err := strategy.Import(data, opts)
		importDone <- struct {
			strategy *strategy.StrategyConfig
			err      error
		}{imported, err}
	}()

	var imported *strategy.StrategyConfig
	select {
	case result := <-importDone:
		if result.err != nil {
			h.logAuditEvent(c, audit.EventTypeStrategyImported, "", "", map[string]interface{}{
				"source_filename": sourceFilename,
			}, false, result.err.Error())
			log.Err(result.err).Msg("Failed to import strategy")
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Failed to import strategy",
				"details": result.err.Error(),
			})
			return
		}
		imported = result.strategy
	case <-importCtx.Done():
		c.JSON(http.StatusRequestTimeout, gin.H{
			"error": "Import/validation timeout exceeded",
		})
		return
	}

	// Apply if requested
	if applyNow {
		h.mu.Lock()
		h.currentStrategy = imported
		h.mu.Unlock()

		// Persist to database if configured
		if err := h.persistStrategy(c.Request.Context(), imported, true); err != nil {
			log.Warn().Err(err).Msg("Failed to persist imported strategy to database")
			// Continue anyway - in-memory update succeeded
		}

		log.Info().
			Str("strategy_name", imported.Metadata.Name).
			Str("strategy_id", imported.Metadata.ID).
			Msg("Strategy imported and applied")
	} else if h.repo != nil {
		// Save to database but don't set as active
		if err := h.persistStrategy(c.Request.Context(), imported, false); err != nil {
			log.Warn().Err(err).Msg("Failed to persist imported strategy to database")
		}
	}

	// Log successful import
	h.logAuditEvent(c, audit.EventTypeStrategyImported, imported.Metadata.ID, imported.Metadata.Name, map[string]interface{}{
		"source_filename": sourceFilename,
		"applied":         applyNow,
	}, true, "")

	c.JSON(http.StatusOK, gin.H{
		"strategy": imported,
		"applied":  applyNow,
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
// @Failure 408 {object} map[string]string "Validation timeout"
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

	// Validate with timeout
	validateCtx, validateCancel := context.WithTimeout(c.Request.Context(), h.validationTimeout)
	defer validateCancel()

	validateDone := make(chan struct {
		strategy *strategy.StrategyConfig
		err      error
	}, 1)

	go func() {
		imported, err := strategy.Import([]byte(req.Data), opts)
		validateDone <- struct {
			strategy *strategy.StrategyConfig
			err      error
		}{imported, err}
	}()

	select {
	case result := <-validateDone:
		if result.err != nil {
			c.JSON(http.StatusOK, gin.H{
				"valid": false,
				"error": result.err.Error(),
			})
			return
		}

		// Get version info
		versionInfo, _ := strategy.GetVersionInfo(result.strategy)

		c.JSON(http.StatusOK, gin.H{
			"valid":          true,
			"name":           result.strategy.Metadata.Name,
			"schema_version": result.strategy.Metadata.SchemaVersion,
			"version_info":   versionInfo,
		})
	case <-validateCtx.Done():
		c.JSON(http.StatusRequestTimeout, gin.H{
			"valid": false,
			"error": "Validation timeout exceeded",
		})
	}
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

	// Try to parse request body if present
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request format",
				"details": err.Error(),
			})
			return
		}
	}

	// Use default name if not provided
	if req.Name == "" {
		req.Name = currentStrategy.Metadata.Name + " (Copy)"
	}

	cloned, err := strategy.Clone(currentStrategy)
	if err != nil {
		h.logAuditEvent(c, audit.EventTypeStrategyCloned, currentStrategy.Metadata.ID, currentStrategy.Metadata.Name, nil, false, err.Error())
		log.Err(err).Msg("Failed to clone strategy")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to clone strategy",
		})
		return
	}

	// Apply requested name and description
	cloned.Metadata.Name = req.Name
	if req.Description != "" {
		cloned.Metadata.Description = req.Description
	}

	// Validate the cloned strategy to ensure it's valid
	if err := cloned.Validate(); err != nil {
		h.logAuditEvent(c, audit.EventTypeStrategyCloned, cloned.Metadata.ID, cloned.Metadata.Name, nil, false, err.Error())
		log.Err(err).Msg("Cloned strategy validation failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Cloned strategy validation failed",
			"details": err.Error(),
		})
		return
	}

	// Log successful clone
	h.logAuditEvent(c, audit.EventTypeStrategyCloned, cloned.Metadata.ID, cloned.Metadata.Name, map[string]interface{}{
		"source_strategy_id":   currentStrategy.Metadata.ID,
		"source_strategy_name": currentStrategy.Metadata.Name,
	}, true, "")

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
		h.logAuditEvent(c, audit.EventTypeStrategyMerged, base.Metadata.ID, base.Metadata.Name, nil, false, err.Error())
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

	// Log successful merge
	h.logAuditEvent(c, audit.EventTypeStrategyMerged, merged.Metadata.ID, merged.Metadata.Name, map[string]interface{}{
		"base_strategy_id":   base.Metadata.ID,
		"base_strategy_name": base.Metadata.Name,
		"applied":            req.Apply,
	}, true, "")

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
