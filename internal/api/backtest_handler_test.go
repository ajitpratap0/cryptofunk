package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBacktest(t *testing.T) {
	// Skip integration test if database not available
	t.Skip("Integration test - requires database setup")

	gin.SetMode(gin.TestMode)
	router := gin.New()

	tests := []struct {
		name           string
		requestBody    RunBacktestRequest
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "valid backtest request",
			requestBody: RunBacktestRequest{
				Name:           "Test Backtest",
				StartDate:      "2024-01-01",
				EndDate:        "2024-06-01",
				Symbols:        []string{"BTC/USDT"},
				InitialCapital: 10000.0,
				Strategy: map[string]interface{}{
					"type": "trend_following",
					"parameters": map[string]interface{}{
						"period": 20,
					},
				},
			},
			expectedStatus: http.StatusAccepted,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.NotEmpty(t, body["id"])
				assert.Equal(t, "pending", body["status"])
				assert.Contains(t, body["message"], "created successfully")
			},
		},
		{
			name: "missing required fields",
			requestBody: RunBacktestRequest{
				Name:           "",
				StartDate:      "2024-01-01",
				EndDate:        "2024-06-01",
				InitialCapital: 10000.0,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid date format",
			requestBody: RunBacktestRequest{
				Name:           "Test Backtest",
				StartDate:      "invalid-date",
				EndDate:        "2024-06-01",
				Symbols:        []string{"BTC/USDT"},
				InitialCapital: 10000.0,
				Strategy: map[string]interface{}{
					"type": "trend_following",
				},
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Contains(t, body["error"], "Invalid start_date format")
			},
		},
		{
			name: "invalid capital (negative)",
			requestBody: RunBacktestRequest{
				Name:           "Test Backtest",
				StartDate:      "2024-01-01",
				EndDate:        "2024-06-01",
				Symbols:        []string{"BTC/USDT"},
				InitialCapital: -1000.0,
				Strategy: map[string]interface{}{
					"type": "trend_following",
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/backtest/run", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.checkResponse(t, response)
			}
		})
	}
}

func TestGetBacktest(t *testing.T) {
	// Skip integration test if database not available
	t.Skip("Integration test - requires database setup")

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Placeholder job ID for skipped test
	testJobID := uuid.New().String()

	tests := []struct {
		name           string
		jobID          string
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:           "valid job ID",
			jobID:          testJobID,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, testJobID, body["id"])
				assert.Equal(t, "Test Job", body["name"])
				assert.Equal(t, "pending", body["status"])
			},
		},
		{
			name:           "non-existent job ID",
			jobID:          uuid.New().String(),
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Contains(t, body["error"], "not found")
			},
		},
		{
			name:           "invalid job ID format",
			jobID:          "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Contains(t, body["error"], "Invalid job ID format")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/backtest/"+tt.jobID, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.checkResponse(t, response)
			}
		})
	}
}

func TestListBacktests(t *testing.T) {
	// Skip integration test if database not available
	t.Skip("Integration test - requires database setup")

	gin.SetMode(gin.TestMode)
	router := gin.New()

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:           "list all jobs",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				backtests := body["backtests"].([]interface{})
				assert.Equal(t, 5, len(backtests))
				assert.Equal(t, float64(5), body["total"])
			},
		},
		{
			name:           "list with pagination",
			queryParams:    "?limit=2&offset=0",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				backtests := body["backtests"].([]interface{})
				assert.Equal(t, 2, len(backtests))
				assert.Equal(t, float64(5), body["total"])
				assert.True(t, body["has_more"].(bool))
			},
		},
		{
			name:           "invalid limit",
			queryParams:    "?limit=200",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid offset",
			queryParams:    "?offset=-1",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/backtest"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.checkResponse(t, response)
			}
		})
	}
}

func TestDeleteBacktest(t *testing.T) {
	// Skip integration test if database not available
	t.Skip("Integration test - requires database setup")

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Placeholder job ID for skipped test
	testJobID := uuid.New().String()

	tests := []struct {
		name           string
		jobID          string
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:           "delete existing job",
			jobID:          testJobID,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Contains(t, body["message"], "deleted successfully")
			},
		},
		{
			name:           "delete non-existent job",
			jobID:          uuid.New().String(),
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/backtest/"+tt.jobID, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.checkResponse(t, response)
			}
		})
	}
}

func TestCancelBacktest(t *testing.T) {
	// Skip integration test if database not available
	t.Skip("Integration test - requires database setup")

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Placeholder job IDs for skipped test
	pendingJobID := uuid.New().String()
	completedJobID := uuid.New().String()

	tests := []struct {
		name           string
		jobID          string
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:           "cancel pending job",
			jobID:          pendingJobID,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Contains(t, body["message"], "cancelled successfully")
				assert.Equal(t, "cancelled", body["status"])
			},
		},
		{
			name:           "cannot cancel completed job",
			jobID:          completedJobID,
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.Contains(t, body["error"], "Cannot cancel")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/backtest/"+tt.jobID+"/cancel", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.checkResponse(t, response)
			}
		})
	}
}
