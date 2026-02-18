// Load test for gnet-based servers
// This tests the performance and correctness of the gnet migration
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	testServerURL = "http://localhost:8080" // Monolith (Agent + Kernel unified)
	testAgentURL  = "http://localhost:8080" // Monolith (Agent + Kernel unified)
	testAIServiceURL = "http://localhost:8000" // AI service
)

// TestResult represents the result of a single test
type TestResult struct {
	Name      string
	Success   bool
	Latency   time.Duration
	Error     string
	RequestID int
}

// LoadTestConfig configures the load test
type LoadTestConfig struct {
	ServerURL      string
	Concurrent     int
	TotalRequests  int
	Timeout        time.Duration
	WarmupRequests int
}

// DefaultLoadTestConfig returns sensible defaults
func DefaultLoadTestConfig() *LoadTestConfig {
	return &LoadTestConfig{
		ServerURL:      testServerURL,
		Concurrent:     100,
		TotalRequests:  1000,
		Timeout:        10 * time.Second,
		WarmupRequests: 10,
	}
}

// runLoadTest executes a load test against a server
func runLoadTest(cfg *LoadTestConfig, logger *zap.Logger) []TestResult {
	results := make([]TestResult, 0, cfg.TotalRequests)
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, cfg.Concurrent)

	startTime := time.Now()

	logger.Info("Starting load test",
		zap.String("server", cfg.ServerURL),
		zap.Int("concurrent", cfg.Concurrent),
		zap.Int("total_requests", cfg.TotalRequests),
	)

	for i := 0; i < cfg.TotalRequests; i++ {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire

		go func(requestID int) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release

			start := time.Now()
			result := TestResult{
				Name:      fmt.Sprintf("Request %d", requestID),
				RequestID: requestID,
			}

			// Perform health check request
			client := &http.Client{
				Timeout: cfg.Timeout,
			}

			resp, err := client.Get(cfg.ServerURL+"/health")
			if err != nil {
				result.Success = false
				result.Error = err.Error()
			} else {
				defer resp.Body.Close()

				// Check response
				if resp.StatusCode == 200 {
					result.Success = true
					// Optionally parse response body
					var health map[string]string
					if err := json.NewDecoder(resp.Body).Decode(&health); err == nil {
						// Successfully parsed health response
					}
				} else {
					result.Success = false
					result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
				}
			}

			result.Latency = time.Since(start)
			results[requestID] = result
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Analyze results
	successCount := 0
	totalLatency := time.Duration(0)
	minLatency := time.Duration(1<<63 - 1)
	maxLatency := time.Duration(0)

	for _, r := range results {
		if r.Success {
			successCount++
			totalLatency += r.Latency
			if r.Latency < minLatency {
				minLatency = r.Latency
			}
			if r.Latency > maxLatency {
				maxLatency = r.Latency
			}
		}
	}

	successRate := float64(successCount) / float64(cfg.TotalRequests) * 100
	avgLatency := time.Duration(0)
	if successCount > 0 {
		avgLatency = totalLatency / time.Duration(successCount)
	}
	throughput := float64(cfg.TotalRequests) / duration.Seconds()

	logger.Info("Load test completed",
		zap.String("server", cfg.ServerURL),
		zap.Float64("duration_seconds", duration.Seconds()),
		zap.Float64("success_rate", successRate),
		zap.Duration("avg_latency", avgLatency),
		zap.Duration("min_latency", minLatency),
		zap.Duration("max_latency", maxLatency),
		zap.Float64("requests_per_second", throughput),
		zap.Int("success", successCount),
		zap.Int("total", cfg.TotalRequests),
	)

	return results
}

// testKernelServer tests the kernel server endpoints
func testKernelServer(baseURL string, logger *zap.Logger) bool {
	logger.Info("Testing kernel server", zap.String("url", baseURL))

	client := &http.Client{Timeout: 5 * time.Second}

	// Test health endpoint
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		logger.Error("Health check failed", zap.Error(err))
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logger.Error("Health check returned non-200", zap.Int("status", resp.StatusCode))
		return false
	}

	var health map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		logger.Error("Failed to parse health response", zap.Error(err))
		return false
	}

	logger.Info("Health check passed", zap.String("status", health["status"]))

	// Test stats endpoint
	resp, err = client.Get(baseURL + "/api/stats")
	if err != nil {
		logger.Error("Stats check failed", zap.Error(err))
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logger.Error("Stats check returned non-200", zap.Int("status", resp.StatusCode))
		return false
	}

	logger.Info("Kernel server tests passed")
	return true
}

// testAIService tests the AI service endpoints
func testAIService(baseURL string, logger *zap.Logger) bool {
	logger.Info("Testing AI service", zap.String("url", baseURL))

	client := &http.Client{Timeout: 10 * time.Second}

	// Test health endpoint
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		logger.Error("AI service health check failed", zap.Error(err))
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logger.Error("AI service health check returned non-200", zap.Int("status", resp.StatusCode))
		return false
	}

	logger.Info("AI service health check passed")

	// Test extract endpoint
	extractReq := map[string]string{
		"user_query":  "What is the capital of France?",
		"ai_response": "The capital of France is Paris.",
	}

	reqBody, _ := json.Marshal(extractReq)
	resp, err = client.Post(baseURL+"/extract", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		logger.Error("Extract endpoint failed", zap.Error(err))
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logger.Error("Extract endpoint returned non-200", zap.Int("status", resp.StatusCode))
		return false
	}

	logger.Info("AI service tests passed")
	return true
}

// testAgentServer tests the agent server endpoints
func testAgentServer(baseURL string, logger *zap.Logger) bool {
	logger.Info("Testing agent server", zap.String("url", baseURL))

	client := &http.Client{Timeout: 5 * time.Second}

	// Test health endpoint
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		logger.Error("Agent health check failed", zap.Error(err))
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logger.Error("Agent health check returned non-200", zap.Int("status", resp.StatusCode))
		return false
	}

	logger.Info("Agent server tests passed")
	return true
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Info("Starting gnet migration load tests")

	// Test each server
	servers := []struct {
		Name string
		URL  string
		Test func(string, *zap.Logger) bool
	}{
		{"Kernel Server", testServerURL, testKernelServer},
		{"AI Service", testAIServiceURL, testAIService},
		{"Agent Server", testAgentURL, testAgentServer},
	}

	allPassed := true
	for _, server := range servers {
		if !server.Test(server.URL, logger) {
			allPassed = false
			logger.Error("Server tests failed", zap.String("server", server.Name))
		}
	}

	// Run load test on kernel server if basic tests passed
	if allPassed {
		logger.Info("All basic tests passed, running load test...")
		cfg := DefaultLoadTestConfig()
		results := runLoadTest(cfg, logger)

		// Count successes
		successCount := 0
		for _, r := range results {
			if r.Success {
				successCount++
			}
		}

		if successCount == cfg.TotalRequests {
			logger.Info("Load test PASSED - all requests succeeded")
		} else {
			logger.Warn("Load test completed with some failures",
				zap.Int("succeeded", successCount),
				zap.Int("failed", cfg.TotalRequests-successCount))
		}
	}

	logger.Info("Load tests completed")
}
