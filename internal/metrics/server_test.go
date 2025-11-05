package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	port := 9999

	server := NewServer(port, log)

	assert.NotNil(t, server)
	assert.Equal(t, port, server.port)
	assert.NotNil(t, server.log)
	assert.Nil(t, server.server) // Server not started yet
}

func TestServerStart(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	port := 9998

	server := NewServer(port, log)
	require.NotNil(t, server)

	// Start server
	err := server.Start()
	require.NoError(t, err)
	assert.NotNil(t, server.server)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestHealthEndpoint(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	port := 9997

	server := NewServer(port, log)
	require.NotNil(t, server)

	// Start server
	err := server.Start()
	require.NoError(t, err)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test health endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Verify JSON response contains expected fields
	bodyStr := string(body)
	assert.Contains(t, bodyStr, `"status":"healthy"`)
	assert.Contains(t, bodyStr, `"timestamp"`)
	assert.Contains(t, bodyStr, `"version"`)

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestMetricsEndpoint(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	port := 9996

	// Create a test metric to ensure /metrics returns something
	testCounter := promauto.NewCounter(prometheus.CounterOpts{
		Name: "test_metrics_endpoint_counter",
		Help: "Test counter for metrics endpoint verification",
	})
	testCounter.Inc()

	server := NewServer(port, log)
	require.NotNil(t, server)

	// Start server
	err := server.Start()
	require.NoError(t, err)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test metrics endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain; version=0.0.4; charset=utf-8")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Verify Prometheus format - should contain metric name and HELP/TYPE comments
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "# HELP")
	assert.Contains(t, bodyStr, "# TYPE")
	assert.Contains(t, bodyStr, "test_metrics_endpoint_counter")

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestServerShutdown(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	port := 9995

	server := NewServer(port, log)
	require.NotNil(t, server)

	// Start server
	err := server.Start()
	require.NoError(t, err)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Shutdown(ctx)
	assert.NoError(t, err)

	// Verify server is stopped
	time.Sleep(100 * time.Millisecond)
	resp2, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if resp2 != nil {
		resp2.Body.Close()
	}
	assert.Error(t, err) // Should fail because server is stopped
}

func TestShutdownWithoutStart(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	port := 9994

	server := NewServer(port, log)
	require.NotNil(t, server)

	// Shutdown without starting should not error
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := server.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestMultipleServerInstances(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Create two servers on different ports
	server1 := NewServer(9993, log)
	server2 := NewServer(9992, log)

	// Start both servers
	err := server1.Start()
	require.NoError(t, err)
	err = server2.Start()
	require.NoError(t, err)

	// Give servers time to start
	time.Sleep(100 * time.Millisecond)

	// Test both health endpoints
	resp1, err := http.Get("http://localhost:9993/health")
	require.NoError(t, err)
	defer resp1.Body.Close()
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	resp2, err := http.Get("http://localhost:9992/health")
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	// Cleanup both servers
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server1.Shutdown(ctx)
	assert.NoError(t, err)
	err = server2.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestRegisterHandler(t *testing.T) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	port := 9991

	server := NewServer(port, log)
	require.NotNil(t, server)

	// Start server
	err := server.Start()
	require.NoError(t, err)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Register a custom handler
	customHandlerCalled := false
	server.RegisterHandler("/custom", func(w http.ResponseWriter, r *http.Request) {
		customHandlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("custom handler response"))
	})

	// Test custom endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/custom", port))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, customHandlerCalled)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "custom handler response", string(body))

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Shutdown(ctx)
	assert.NoError(t, err)
}
