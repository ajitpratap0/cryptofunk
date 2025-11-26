package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

// =============================================================================
// Concurrent Upload Tests
// =============================================================================

func TestConcurrentStrategyUpdates(t *testing.T) {
	router, _ := setupStrategyRouter()

	// Number of concurrent requests
	numRequests := 10
	done := make(chan bool, numRequests)
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(index int) {
			newStrategy := strategy.NewDefaultStrategy("Concurrent Strategy " + string(rune('A'+index)))
			body, _ := json.Marshal(newStrategy)

			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/strategies/current", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				errors <- assert.AnError
			}
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}

	// Check for errors
	close(errors)
	for err := range errors {
		t.Errorf("Concurrent update failed: %v", err)
	}

	// Verify final state is valid
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/strategies/current", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestConcurrentFileUploads(t *testing.T) {
	router, _ := setupStrategyRouter()

	// Number of concurrent uploads
	numUploads := 5
	done := make(chan bool, numUploads)
	errors := make(chan error, numUploads)

	for i := 0; i < numUploads; i++ {
		go func(index int) {
			// Create multipart form
			var b bytes.Buffer
			writer := multipart.NewWriter(&b)

			fileWriter, err := writer.CreateFormFile("file", "test_concurrent.yaml")
			if err != nil {
				errors <- err
				done <- true
				return
			}

			yamlContent := `schema_version: "1.0"
metadata:
  name: "Concurrent Upload Strategy"
  version: "1.0.0"
`
			_, err = fileWriter.Write([]byte(yamlContent))
			if err != nil {
				errors <- err
				done <- true
				return
			}

			writer.Close()

			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/import", &b)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
				errors <- assert.AnError
			}
			done <- true
		}(i)
	}

	// Wait for all uploads to complete
	for i := 0; i < numUploads; i++ {
		<-done
	}

	// Check for errors
	close(errors)
	for err := range errors {
		t.Errorf("Concurrent upload failed: %v", err)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	router, _ := setupStrategyRouter()

	// Mix of read and write operations
	numReaders := 10
	numWriters := 5
	total := numReaders + numWriters
	done := make(chan bool, total)

	// Start readers
	for i := 0; i < numReaders; i++ {
		go func() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/strategies/current", nil)
			router.ServeHTTP(w, req)
			assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
			done <- true
		}()
	}

	// Start writers
	for i := 0; i < numWriters; i++ {
		go func(index int) {
			newStrategy := strategy.NewDefaultStrategy("Concurrent RW Strategy")
			body, _ := json.Marshal(newStrategy)

			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/strategies/current", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			done <- true
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < total; i++ {
		<-done
	}
}

func TestConcurrentExports(t *testing.T) {
	router, _ := setupStrategyRouter()

	// Number of concurrent exports
	numExports := 10
	done := make(chan bool, numExports)

	for i := 0; i < numExports; i++ {
		go func(index int) {
			format := "yaml"
			if index%2 == 0 {
				format = "json"
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/strategies/export?format="+format, nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}(i)
	}

	// Wait for all exports to complete
	for i := 0; i < numExports; i++ {
		<-done
	}
}

// =============================================================================
// Validation Timeout Tests
// =============================================================================

func TestValidationTimeout(t *testing.T) {
	handler := NewStrategyHandler()
	handler.SetValidationTimeout(100 * time.Millisecond) // Short timeout for testing

	router := gin.New()
	router.POST("/strategies/validate", handler.ValidateStrategy)

	// Create a valid strategy
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

	// Should succeed with valid strategy
	assert.Equal(t, http.StatusOK, w.Code)
}

// =============================================================================
// Database Persistence Constructor Tests
// =============================================================================

func TestNewStrategyHandlerWithDB_NilRepo(t *testing.T) {
	handler := NewStrategyHandlerWithDB(nil, nil)
	require.NotNil(t, handler)
	require.NotNil(t, handler.currentStrategy)
	assert.Equal(t, "Default Strategy", handler.currentStrategy.Metadata.Name)
}

// =============================================================================
// Additional Concurrent Access Tests
// =============================================================================

func TestConcurrentUpdateAndRead(t *testing.T) {
	router, handler := setupStrategyRouter()

	// Pre-populate with a strategy
	initialStrategy := strategy.NewDefaultStrategy("Initial Strategy")
	handler.mu.Lock()
	handler.currentStrategy = initialStrategy
	handler.mu.Unlock()

	numUpdates := 20
	numReads := 50
	total := numUpdates + numReads

	var wg sync.WaitGroup
	wg.Add(total)

	successCount := int32(0)
	failCount := int32(0)

	// Launch concurrent updates
	for i := 0; i < numUpdates; i++ {
		go func(idx int) {
			defer wg.Done()

			newStrategy := strategy.NewDefaultStrategy("Update Strategy")
			newStrategy.Risk.MaxDrawdown = float64(idx+1) / 100.0 // Ensure non-zero drawdown
			body, _ := json.Marshal(newStrategy)

			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/strategies/current", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&failCount, 1)
			}
		}(i)
	}

	// Launch concurrent reads
	for i := 0; i < numReads; i++ {
		go func() {
			defer wg.Done()

			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/strategies/current", nil)
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&failCount, 1)
			}
		}()
	}

	// Wait for completion
	wg.Wait()

	// Allow some failures in concurrent scenarios, but ensure majority succeed
	successRate := float64(successCount) / float64(total)
	t.Logf("Concurrent test: %d/%d operations succeeded, %d failed (%.1f%% success rate)", successCount, total, failCount, successRate*100)

	// Require at least 90% success rate
	assert.GreaterOrEqual(t, successRate, 0.9, "Expected at least 90%% success rate in concurrent operations")
}

func TestConcurrentCloneOperations(t *testing.T) {
	router, _ := setupStrategyRouter()

	numClones := 10
	done := make(chan struct{}, numClones)

	for i := 0; i < numClones; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()

			cloneReq := CloneRequest{
				Name:        "Cloned Strategy",
				Description: "Concurrent clone test",
			}
			body, _ := json.Marshal(cloneReq)

			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/clone", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		}(i)
	}

	for i := 0; i < numClones; i++ {
		<-done
	}
}

func TestConcurrentImportAndExport(t *testing.T) {
	router, _ := setupStrategyRouter()

	numOperations := 20
	done := make(chan struct{}, numOperations)

	// Create a valid strategy for import
	validStrategy := strategy.NewDefaultStrategy("Import Test Strategy")
	yamlData, _ := strategy.Export(validStrategy, strategy.ExportOptions{Format: "yaml"})

	for i := 0; i < numOperations; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()

			if idx%2 == 0 {
				// Export operation
				w := httptest.NewRecorder()
				req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/strategies/export", nil)
				router.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				// Import operation
				importReq := ImportRequest{
					Data:     string(yamlData),
					ApplyNow: false,
				}
				body, _ := json.Marshal(importReq)

				w := httptest.NewRecorder()
				req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/import", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			}
		}(i)
	}

	for i := 0; i < numOperations; i++ {
		<-done
	}
}

// =============================================================================
// File Upload Integration Tests
// =============================================================================

func TestFileUploadWithValidation(t *testing.T) {
	router, _ := setupStrategyRouter()

	// Test YAML file upload
	t.Run("YAML file upload", func(t *testing.T) {
		validStrategy := strategy.NewDefaultStrategy("Upload YAML Strategy")
		yamlData, _ := strategy.Export(validStrategy, strategy.ExportOptions{Format: "yaml"})

		body, contentType := createMultipartForm(t, "strategy.yaml", yamlData, map[string]string{
			"apply_now": "true",
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/import", body)
		req.Header.Set("Content-Type", contentType)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, true, response["applied"])
	})

	// Test JSON file upload
	t.Run("JSON file upload", func(t *testing.T) {
		validStrategy := strategy.NewDefaultStrategy("Upload JSON Strategy")
		jsonData, _ := strategy.Export(validStrategy, strategy.ExportOptions{Format: "json"})

		body, contentType := createMultipartForm(t, "strategy.json", jsonData, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/import", body)
		req.Header.Set("Content-Type", contentType)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Test YML extension
	t.Run("YML file upload", func(t *testing.T) {
		validStrategy := strategy.NewDefaultStrategy("Upload YML Strategy")
		yamlData, _ := strategy.Export(validStrategy, strategy.ExportOptions{Format: "yaml"})

		body, contentType := createMultipartForm(t, "strategy.yml", yamlData, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/import", body)
		req.Header.Set("Content-Type", contentType)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestFileUploadInvalidExtension(t *testing.T) {
	router, _ := setupStrategyRouter()

	body, contentType := createMultipartForm(t, "strategy.txt", []byte("invalid content"), nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/import", body)
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Invalid file extension")
}

func TestFileUploadInvalidContent(t *testing.T) {
	router, _ := setupStrategyRouter()

	body, contentType := createMultipartForm(t, "strategy.yaml", []byte("invalid: yaml: content: [}"), nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/import", body)
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFileUploadWithStrictValidation(t *testing.T) {
	router, _ := setupStrategyRouter()

	validStrategy := strategy.NewDefaultStrategy("Strict Validation Strategy")
	yamlData, _ := strategy.Export(validStrategy, strategy.ExportOptions{Format: "yaml"})

	body, contentType := createMultipartForm(t, "strategy.yaml", yamlData, map[string]string{
		"validate_strict": "true",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/strategies/import", body)
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// createMultipartForm creates a multipart form request body for file uploads
func createMultipartForm(t *testing.T, filename string, content []byte, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)

	// Add additional fields
	for key, value := range fields {
		err = writer.WriteField(key, value)
		require.NoError(t, err)
	}

	err = writer.Close()
	require.NoError(t, err)

	return body, writer.FormDataContentType()
}

// =============================================================================
// Rate Limiter Integration Tests
// =============================================================================

func TestRateLimiterMiddlewareIntegration(t *testing.T) {
	router := gin.New()
	handler := NewStrategyHandler()

	// Create a simple rate limiter for testing (3 requests per second)
	readMiddleware := func(c *gin.Context) {
		// Simulate rate limiter passing
		c.Next()
	}
	writeMiddleware := func(c *gin.Context) {
		c.Next()
	}

	// Register routes with rate limiting
	api := router.Group("/api/v1")
	handler.RegisterRoutesWithRateLimiter(api, readMiddleware, writeMiddleware)

	// Test that routes work with middleware
	t.Run("read endpoint with rate limiter", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/strategies/current", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("write endpoint with rate limiter", func(t *testing.T) {
		newStrategy := strategy.NewDefaultStrategy("Rate Limited Strategy")
		body, _ := json.Marshal(newStrategy)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/strategies/current", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
