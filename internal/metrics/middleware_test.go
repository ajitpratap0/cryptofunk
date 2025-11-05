package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponseWriter_WriteHeader(t *testing.T) {
	tests := []struct {
		name              string
		statusCode        int
		expectedCode      int
		callMultipleTimes bool
	}{
		{
			name:              "write 200 OK",
			statusCode:        http.StatusOK,
			expectedCode:      http.StatusOK,
			callMultipleTimes: false,
		},
		{
			name:              "write 404 Not Found",
			statusCode:        http.StatusNotFound,
			expectedCode:      http.StatusNotFound,
			callMultipleTimes: false,
		},
		{
			name:              "write 500 Internal Server Error",
			statusCode:        http.StatusInternalServerError,
			expectedCode:      http.StatusInternalServerError,
			callMultipleTimes: false,
		},
		{
			name:              "multiple writes - only first should be recorded",
			statusCode:        http.StatusOK,
			expectedCode:      http.StatusOK,
			callMultipleTimes: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rw := &responseWriter{
				ResponseWriter: rec,
				statusCode:     http.StatusOK,
				written:        false,
			}

			rw.WriteHeader(tt.statusCode)
			assert.Equal(t, tt.expectedCode, rw.statusCode)
			assert.True(t, rw.written)

			if tt.callMultipleTimes {
				// Write again with different status code
				rw.WriteHeader(http.StatusBadRequest)
				// Should still have original status code
				assert.Equal(t, tt.expectedCode, rw.statusCode)
			}
		})
	}
}

func TestResponseWriter_Write(t *testing.T) {
	tests := []struct {
		name                 string
		data                 []byte
		expectedStatusCode   int
		callWriteHeaderFirst bool
		customStatusCode     int
	}{
		{
			name:                 "write without calling WriteHeader first",
			data:                 []byte("test response"),
			expectedStatusCode:   http.StatusOK,
			callWriteHeaderFirst: false,
		},
		{
			name:                 "write after calling WriteHeader",
			data:                 []byte("test response"),
			expectedStatusCode:   http.StatusCreated,
			callWriteHeaderFirst: true,
			customStatusCode:     http.StatusCreated,
		},
		{
			name:                 "write empty data",
			data:                 []byte{},
			expectedStatusCode:   http.StatusOK,
			callWriteHeaderFirst: false,
		},
		{
			name:                 "write large data",
			data:                 make([]byte, 1024),
			expectedStatusCode:   http.StatusOK,
			callWriteHeaderFirst: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rw := &responseWriter{
				ResponseWriter: rec,
				statusCode:     http.StatusOK,
				written:        false,
			}

			if tt.callWriteHeaderFirst {
				rw.WriteHeader(tt.customStatusCode)
			}

			n, err := rw.Write(tt.data)
			assert.NoError(t, err)
			assert.Equal(t, len(tt.data), n)
			assert.Equal(t, tt.expectedStatusCode, rw.statusCode)
			assert.True(t, rw.written)
		})
	}
}

func TestHTTPMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		handlerStatus  int
		handlerBody    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET request with 200 OK",
			method:         "GET",
			path:           "/api/trades",
			handlerStatus:  http.StatusOK,
			handlerBody:    `{"trades":[]}`,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"trades":[]}`,
		},
		{
			name:           "POST request with 201 Created",
			method:         "POST",
			path:           "/api/orders",
			handlerStatus:  http.StatusCreated,
			handlerBody:    `{"id":"123"}`,
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"id":"123"}`,
		},
		{
			name:           "GET request with 404 Not Found",
			method:         "GET",
			path:           "/api/unknown",
			handlerStatus:  http.StatusNotFound,
			handlerBody:    `{"error":"not found"}`,
			expectedStatus: http.StatusNotFound,
			expectedBody:   `{"error":"not found"}`,
		},
		{
			name:           "POST request with 500 Internal Server Error",
			method:         "POST",
			path:           "/api/error",
			handlerStatus:  http.StatusInternalServerError,
			handlerBody:    `{"error":"internal error"}`,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"internal error"}`,
		},
		{
			name:           "Handler doesn't write - should default to 200",
			method:         "GET",
			path:           "/api/empty",
			handlerStatus:  0, // Handler doesn't call WriteHeader
			handlerBody:    "",
			expectedStatus: http.StatusOK,
			expectedBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.handlerStatus > 0 {
					w.WriteHeader(tt.handlerStatus)
				}
				if tt.handlerBody != "" {
					w.Write([]byte(tt.handlerBody))
				}
			})

			// Wrap with middleware
			wrappedHandler := HTTPMiddleware(handler)

			// Create test request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			// Execute request
			wrappedHandler.ServeHTTP(rec, req)

			// Verify response
			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, tt.expectedBody, rec.Body.String())
		})
	}
}

func TestHTTPMiddleware_MetricsRecorded(t *testing.T) {
	// This test verifies that the middleware records metrics
	// We can't directly assert metric values, but we can verify no panics occur
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	wrappedHandler := HTTPMiddleware(handler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		wrappedHandler.ServeHTTP(rec, req)
	})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

func TestHTTPMiddleware_PreservesHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	wrappedHandler := HTTPMiddleware(handler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "test-value", rec.Header().Get("X-Custom-Header"))
}

func TestHTTPMiddleware_MultipleWrites(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("part1"))
		w.Write([]byte("part2"))
		w.Write([]byte("part3"))
	})

	wrappedHandler := HTTPMiddleware(handler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "part1part2part3", rec.Body.String())
}

func TestHTTPMiddleware_WithQueryParams(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	wrappedHandler := HTTPMiddleware(handler)

	req := httptest.NewRequest("GET", "/api/test?symbol=BTC&limit=10", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHTTPMiddleware_DifferentHTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			wrappedHandler := HTTPMiddleware(handler)

			req := httptest.NewRequest(method, "/api/test", nil)
			rec := httptest.NewRecorder()

			assert.NotPanics(t, func() {
				wrappedHandler.ServeHTTP(rec, req)
			})

			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}
