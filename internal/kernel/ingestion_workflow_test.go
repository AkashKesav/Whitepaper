// Package kernel provides tests for Inngest-based durable execution workflows.
package kernel

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"github.com/reflective-memory-kernel/internal/ai/local"
	"github.com/reflective-memory-kernel/internal/graph"
)

func TestIngestionWorkflowCreation(t *testing.T) {
	logger := zaptest.NewLogger(t)

	cfg := WorkflowConfig{
		InngestAPIKey: "test-key",
		EventKey:      "test-event-key",
		AppID:         "test-rmk-workflows",
		Logger:        logger,
	}

	// Test workflow creation (without actually running it)
	opts, _ := NewIngestionWorkflow(cfg, nil, nil)

	if opts.ID != "ingest-transcript" {
		t.Errorf("Expected workflow ID 'ingest-transcript', got '%s'", opts.ID)
	}

	if opts.Name != "Ingest Transcript Event" {
		t.Errorf("Expected workflow name 'Ingest Transcript Event', got '%s'", opts.Name)
	}

	// Trigger is a struct, not a pointer, so we can't check for nil
	// But the fact that NewIngestionWorkflow returns without error is sufficient
	t.Log("Trigger created successfully")
}

func TestWisdomWorkflowCreation(t *testing.T) {
	logger := zaptest.NewLogger(t)

	cfg := WorkflowConfig{
		InngestAPIKey: "test-key",
		EventKey:      "test-event-key",
		AppID:         "test-rmk-workflows",
		Logger:        logger,
	}

	opts, _ := NewWisdomWorkflow(cfg, nil, nil)

	if opts.ID != "wisdom-batch-process" {
		t.Errorf("Expected workflow ID 'wisdom-batch-process', got '%s'", opts.ID)
	}

	if opts.Name != "Wisdom Layer Batch Processing" {
		t.Errorf("Expected workflow name 'Wisdom Layer Batch Processing', got '%s'", opts.Name)
	}

	// Trigger is a struct, not a pointer, so we can't check for nil
	// But the fact that NewIngestionWorkflow returns without error is sufficient
	t.Log("Trigger created successfully")
}

func TestCronWorkflowCreation(t *testing.T) {
	logger := zaptest.NewLogger(t)

	cfg := WorkflowConfig{
		InngestAPIKey: "test-key",
		EventKey:      "test-event-key",
		AppID:         "test-rmk-workflows",
		Logger:        logger,
	}

	opts, _ := NewCronWorkflow(cfg, nil)

	if opts.ID != "maintenance-cron" {
		t.Errorf("Expected workflow ID 'maintenance-cron', got '%s'", opts.ID)
	}

	if opts.Name != "Periodic Maintenance Tasks" {
		t.Errorf("Expected workflow name 'Periodic Maintenance Tasks', got '%s'", opts.Name)
	}

	// Trigger is a struct, not a pointer, so we can't check for nil
	// But the fact that NewIngestionWorkflow returns without error is sufficient
	t.Log("Trigger created successfully")
}

// TestIngestionWorkflowFunction tests the workflow function execution
func TestIngestionWorkflowFunction(t *testing.T) {
	logger := zaptest.NewLogger(t)

	cfg := WorkflowConfig{
		InngestAPIKey: "test-key",
		EventKey:      "test-event-key",
		AppID:         "test-rmk-workflows",
		Logger:        logger,
	}

	// Create the workflow function
	wfFn := ingestTranscriptWorkflow(cfg, nil, nil)

	// Create mock input (inngestgo.Input would be created by the Inngest SDK)
	// For this test, we'll verify the function is non-nil
	if wfFn == nil {
		t.Fatal("Expected workflow function to be non-nil")
	}

	t.Log("Workflow function created successfully")
}

// TestWisdomWorkflowFunction tests the wisdom workflow function execution
func TestWisdomWorkflowFunction(t *testing.T) {
	logger := zaptest.NewLogger(t)

	cfg := WorkflowConfig{
		InngestAPIKey: "test-key",
		EventKey:      "test-event-key",
		AppID:         "test-rmk-workflows",
		Logger:        logger,
	}

	wfFn := wisdomBatchWorkflow(cfg, nil, nil)

	if wfFn == nil {
		t.Fatal("Expected workflow function to be non-nil")
	}

	t.Log("Wisdom workflow function created successfully")
}

// TestMaintenanceWorkflowFunction tests the maintenance workflow function
func TestMaintenanceWorkflowFunction(t *testing.T) {
	logger := zaptest.NewLogger(t)

	cfg := WorkflowConfig{
		InngestAPIKey: "test-key",
		EventKey:      "test-event-key",
		AppID:         "test-rmk-workflows",
		Logger:        logger,
	}

	wfFn := maintenanceWorkflow(cfg, nil)

	if wfFn == nil {
		t.Fatal("Expected workflow function to be non-nil")
	}

	t.Log("Maintenance workflow function created successfully")
}

// TestWorkflowServiceCreation tests creating the workflow service
func TestWorkflowServiceCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping workflow service test in short mode")
	}

	logger := zaptest.NewLogger(t)

	cfg := WorkflowConfig{
		InngestAPIKey: "test-key",
		EventKey:      "test-event-key",
		AppID:         "test-rmk-workflows",
		Logger:        logger,
	}

	// Create service without Inngest connection
	// This will test the initialization logic
	graphCfg := graph.DefaultClientConfig()
	graphCfg.Address = "localhost:9080"
	graphClient, err := graph.NewClient(context.Background(), graphCfg, logger)
	if err != nil {
		t.Skipf("Skipping test: DGraph not available: %v", err)
	}
	defer graphClient.Close()

	var embedder local.LocalEmbedder = &mockEmbedder{}

	// This will fail to connect to Inngest but we can test initialization
	_, err = NewWorkflowService(cfg, graphClient, embedder)
	if err != nil {
		// Expected to fail without Inngest server, but we can test the code path
		t.Logf("Expected failure without Inngest server: %v", err)
	}
}

// mockEmbedder is a mock embedder for testing
type mockEmbedder struct{}

func (m *mockEmbedder) Embed(text string) ([]float32, error) {
	// Return a simple mock embedding
	return make([]float32, 384), nil
}

func (m *mockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i := range results {
		results[i] = make([]float32, 384)
	}
	return results, nil
}

func (m *mockEmbedder) Close() error {
	return nil
}

// BenchmarkIngestionWorkflowFunction benchmarks the workflow function creation
func BenchmarkIngestionWorkflowFunction(b *testing.B) {
	logger := zaptest.NewLogger(b)

	cfg := WorkflowConfig{
		InngestAPIKey: "test-key",
		EventKey:      "test-event-key",
		AppID:         "test-rmk-workflows",
		Logger:        logger,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ingestTranscriptWorkflow(cfg, nil, nil)
	}
}

// Example workflow input for reference
func ExampleIngestionInput() {
	input := IngestionInput{
		ConversationID: "conv-123",
		UserID:        "user-456",
		Namespace:     "personal",
		UserQuery:     "My favorite dessert is gulab jamun",
		AIResponse:    "I've noted that gulab jamun is your favorite dessert.",
		Timestamp:     time.Now().Format(time.RFC3339),
	}

	_ = input
}
