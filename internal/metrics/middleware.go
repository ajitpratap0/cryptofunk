package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// HTTPMiddleware returns middleware that instruments HTTP requests
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			written:        false,
		}

		// Call next handler
		next.ServeHTTP(rw, r)

		// Record metrics
		duration := float64(time.Since(start).Milliseconds())
		statusCode := strconv.Itoa(rw.statusCode)

		RecordAPIRequest(r.Method, r.URL.Path, statusCode, duration)
	})
}

// GinMiddleware returns a Gin middleware that instruments HTTP requests
func GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics after request is processed
		duration := float64(time.Since(start).Milliseconds())
		statusCode := strconv.Itoa(c.Writer.Status())
		path := c.FullPath() // Use FullPath() to get the route pattern instead of actual path
		if path == "" {
			path = c.Request.URL.Path // Fallback to actual path if route pattern not available
		}

		RecordAPIRequest(c.Request.Method, path, statusCode, duration)
	}
}
