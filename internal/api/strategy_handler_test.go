package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/cryptofunk/internal/strategy"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// =============================================================================
// Helper Functions
// =============================================================================

func setupStrategyRouter() (*gin.Engine, *StrategyHandler) {
	router := gin.New()
	handler := NewStrategyHandler()

	router.GET("/strategies/current", handler.GetCurrentStrategy)
	router.PUT("/strategies/current", handler.UpdateCurrentStrategy)
	router.GET("/strategies/export", handler.ExportStrategy)
	router.POST("/strategies/export", handler.ExportStrategyWithOptions)
	router.POST("/strategies/import", handler.ImportStrategy)
	router.POST("/strategies/validate", handler.ValidateStrategy)
	router.POST("/strategies/clone", handler.CloneStrategy)
	router.POST("/strategies/merge", handler.MergeStrategies)
	router.GET("/strategies/version", handler.GetVersionInfo)
	router.GET("/strategies/schema", handler.GetSchemaInfo)
	router.POST("/strategies/default", handler.GetDefaultStrategy)

	return router, handler
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewStrategyHandler(t *testing.T) {
	handler := NewStrategyHandler()
	require.NotNil(t, handler)
	require.NotNil(t, handler.currentStrategy)
	assert.Equal(t, "Default Strategy", handler.currentStrategy.Metadata.Name)
}

func TestNewStrategyHandlerWithAudit(t *testing.T) {
	handler := NewStrategyHandlerWithAudit(nil)
	require.NotNil(t, handler)
	require.NotNil(t, handler.currentStrategy)
	assert.Nil(t, handler.auditLogger)
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal filename", "strategy.yaml", "strategy.yaml"},
		{"with path", "/etc/passwd", "passwd"},
		{"with special chars", "my<>strategy|.yaml", "my__strategy_.yaml"},
		{"empty after sanitize", "///", "strategy"},
		{"spaces", "my strategy.yaml", "my_strategy.yaml"},
		{"unicode", "策略.yaml", "__.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAllowedExtension(t *testing.T) {
	tests := []struct {
		filename string
		allowed  bool
	}{
		{"strategy.yaml", true},
		{"strategy.yml", true},
		{"strategy.json", true},
		{"strategy.YAML", true},
		{"strategy.txt", false},
		{"strategy.exe", false},
		{"strategy", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			assert.Equal(t, tt.allowed, isAllowedExtension(tt.filename))
		})
	}
}

// =============================================================================
// HTTP Handler Tests
// =============================================================================

func TestGetCurrentStrategy(t *testing.T) {
	router, _ := setupStrategyRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/strategies/current", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Response is the strategy directly, not wrapped
	metadata, ok := response["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Default Strategy", metadata["name"])
}

func TestUpdateCurrentStrategy(t *testing.T) {
	router, _ := setupStrategyRouter()

	newStrategy := strategy.NewDefaultStrategy("Updated Strategy")
	body, _ := json.Marshal(newStrategy)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/strategies/current", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Response is the updated strategy directly
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	metadata, ok := response["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Updated Strategy", metadata["name"])
}

func TestUpdateCurrentStrategy_InvalidJSON(t *testing.T) {
	router, _ := setupStrategyRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/strategies/current", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportStrategy_YAML(t *testing.T) {
	router, _ := setupStrategyRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/strategies/export", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/yaml")
	assert.Contains(t, w.Body.String(), "schema_version:")
}

func TestExportStrategy_JSON(t *testing.T) {
	router, _ := setupStrategyRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/strategies/export?format=json", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var result map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
}

func TestExportStrategyWithOptions(t *testing.T) {
	router, _ := setupStrategyRouter()

	options := map[string]interface{}{
		"format":       "json",
		"pretty":       true,
		"include_meta": true,
	}
	body, _ := json.Marshal(options)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/export", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestImportStrategy_JSON(t *testing.T) {
	router, _ := setupStrategyRouter()

	importReq := map[string]interface{}{
		"data":      `{"schema_version":"1.0","metadata":{"name":"Test Import"}}`,
		"apply_now": true,
	}
	body, _ := json.Marshal(importReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/import", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// May fail validation but should not be a 400 for bad request format
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest)
}

func TestImportStrategy_FileUpload(t *testing.T) {
	router, _ := setupStrategyRouter()

	// Create multipart form
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	fileWriter, err := writer.CreateFormFile("file", "test.yaml")
	require.NoError(t, err)

	yamlContent := `schema_version: "1.0"
metadata:
  name: "Uploaded Strategy"
  version: "1.0.0"
`
	_, err = fileWriter.Write([]byte(yamlContent))
	require.NoError(t, err)

	writer.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/import", &b)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	// The upload should be processed (may fail validation but not format)
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest)
}

func TestImportStrategy_InvalidExtension(t *testing.T) {
	router, _ := setupStrategyRouter()

	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	fileWriter, err := writer.CreateFormFile("file", "test.exe")
	require.NoError(t, err)
	fileWriter.Write([]byte("invalid"))
	writer.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/import", &b)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid file extension")
}

func TestValidateStrategy(t *testing.T) {
	router, _ := setupStrategyRouter()

	validStrategy := strategy.NewDefaultStrategy("Valid Strategy")
	data, _ := strategy.Export(validStrategy, strategy.ExportOptions{Format: "yaml"})

	validateReq := map[string]interface{}{
		"data":   string(data),
		"strict": true,
	}
	body, _ := json.Marshal(validateReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/validate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["valid"])
}

func TestValidateStrategy_Invalid(t *testing.T) {
	router, _ := setupStrategyRouter()

	validateReq := map[string]interface{}{
		"data":   `{"schema_version":"1.0","metadata":{}}`,
		"strict": true,
	}
	body, _ := json.Marshal(validateReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/validate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, false, response["valid"])
}

func TestCloneStrategy(t *testing.T) {
	router, _ := setupStrategyRouter()

	cloneReq := map[string]interface{}{
		"name": "Cloned Strategy",
	}
	body, _ := json.Marshal(cloneReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/clone", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Response is the cloned strategy directly
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	metadata, ok := response["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Cloned Strategy", metadata["name"])
}

func TestMergeStrategies(t *testing.T) {
	router, _ := setupStrategyRouter()

	override := strategy.NewDefaultStrategy("Override")
	override.Risk.MaxDrawdown = 0.15

	mergeReq := map[string]interface{}{
		"override": override,
	}
	body, _ := json.Marshal(mergeReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/merge", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetVersionInfo(t *testing.T) {
	router, _ := setupStrategyRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/strategies/version", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "current_schema_version")
	assert.Contains(t, response, "supported_versions")
}

func TestGetSchemaDocumentation(t *testing.T) {
	router, _ := setupStrategyRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/strategies/schema", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "fields")
	assert.Contains(t, response, "version")
}

func TestGetDefaultStrategy(t *testing.T) {
	router, _ := setupStrategyRouter()

	defaultReq := map[string]interface{}{
		"name": "My New Strategy",
	}
	body, _ := json.Marshal(defaultReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/default", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Response is the strategy directly
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	metadata, ok := response["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "My New Strategy", metadata["name"])
}

func TestGetDefaultStrategy_NoName(t *testing.T) {
	router, _ := setupStrategyRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/default", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Response is the strategy directly
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	metadata, ok := response["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "New Strategy", metadata["name"])
}
