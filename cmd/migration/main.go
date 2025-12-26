// Migration CLI - Import SQL databases into the Reflective Memory Kernel
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/migration"
)

func main() {
	// Define flags
	source := flag.String("source", "", "Path to SQL dump or JSONL file (required)")
	dialect := flag.String("dialect", "generic", "SQL dialect: postgres, mysql, generic")
	namespace := flag.String("namespace", "", "Namespace for imported data (default: import-{timestamp})")
	batchSize := flag.Int("batch-size", 100, "Records per batch")
	configPath := flag.String("config", "", "Path to table mapping config (YAML)")
	dryRun := flag.Bool("dry-run", false, "Parse only, don't insert to database")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	dgraphURL := flag.String("dgraph", "localhost:9080", "DGraph Alpha address")
	aiURL := flag.String("ai-url", "http://localhost:8001", "AI Services URL")

	flag.Parse()

	// Validate required flags
	if *source == "" {
		fmt.Println("Error: --source is required")
		flag.Usage()
		os.Exit(1)
	}

	// Setup logger
	var logger *zap.Logger
	var err error
	if *verbose {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Generate namespace if not provided
	if *namespace == "" {
		*namespace = fmt.Sprintf("import-%d", time.Now().Unix())
	}

	// Load table mapping config if provided
	var tables []migration.TableMapping
	if *configPath != "" {
		tables, err = loadTableConfig(*configPath)
		if err != nil {
			logger.Fatal("Failed to load config", zap.Error(err))
		}
	}

	// Create migration config
	config := migration.SQLConfig{
		Dialect:       migration.SQLDialect(*dialect),
		SourcePath:    *source,
		Namespace:     *namespace,
		BatchSize:     *batchSize,
		Tables:        tables,
		AIServicesURL: *aiURL,
	}

	// Open source file
	file, err := os.Open(*source)
	if err != nil {
		logger.Fatal("Failed to open source file", zap.Error(err))
	}
	defer file.Close()

	// Parse the file
	logger.Info("Parsing source file",
		zap.String("source", *source),
		zap.String("dialect", *dialect),
		zap.String("namespace", *namespace),
	)

	var dataPoints []migration.DataPoint
	ext := strings.ToLower(filepath.Ext(*source))

	if ext == ".jsonl" || ext == ".json" {
		// JSONL parser
		parser := migration.NewJSONLParser(*namespace, filepath.Base(*source))
		dataPoints, err = parser.Parse(file)
	} else {
		// SQL parser
		parser := migration.NewSQLParser(config.Dialect, *namespace, tables)
		dataPoints, err = parser.Parse(file)
	}

	if err != nil {
		logger.Fatal("Failed to parse source", zap.Error(err))
	}

	logger.Info("Parsed records",
		zap.Int("count", len(dataPoints)),
	)

	// Dry run - just report stats
	if *dryRun {
		printDryRunStats(dataPoints)
		return
	}

	// Initialize DGraph client
	ctx := context.Background()
	graphCfg := graph.DefaultClientConfig()
	graphCfg.Address = *dgraphURL
	graphClient, err := graph.NewClient(ctx, graphCfg, logger)
	if err != nil {
		logger.Fatal("Failed to connect to DGraph", zap.Error(err))
	}
	defer graphClient.Close()

	// Create processor
	processor := migration.NewProcessor(graphClient, *aiURL, config, logger)

	// Process in batches
	result := processInBatches(ctx, processor, dataPoints, *batchSize, logger)

	// Print final results
	printResults(result)
}

// loadTableConfig loads table mapping from YAML
func loadTableConfig(path string) ([]migration.TableMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config struct {
		Tables []migration.TableMapping `yaml:"tables"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config.Tables, nil
}

// processInBatches processes DataPoints in batches
func processInBatches(
	ctx context.Context,
	processor *migration.Processor,
	points []migration.DataPoint,
	batchSize int,
	logger *zap.Logger,
) *migration.MigrationResult {
	result := &migration.MigrationResult{
		TotalRecords: int64(len(points)),
	}
	startTime := time.Now()

	totalBatches := (len(points) + batchSize - 1) / batchSize

	for i := 0; i < len(points); i += batchSize {
		end := i + batchSize
		if end > len(points) {
			end = len(points)
		}

		batch := points[i:end]
		batchNum := (i / batchSize) + 1

		logger.Info("Processing batch",
			zap.Int("batch", batchNum),
			zap.Int("total_batches", totalBatches),
			zap.Int("records", len(batch)),
		)

		batchResult, err := processor.ProcessBatch(ctx, batch)
		if err != nil {
			logger.Error("Batch failed", zap.Error(err))
			result.ErrorCount += int64(len(batch))
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		result.ProcessedCount += batchResult.ProcessedCount
		result.SkippedCount += batchResult.SkippedCount
		result.NodesCreated += batchResult.NodesCreated
		result.EdgesCreated += batchResult.EdgesCreated

		// Progress update
		progress := float64(i+len(batch)) / float64(len(points)) * 100
		logger.Info("Progress",
			zap.Float64("percent", progress),
			zap.Int64("nodes_created", result.NodesCreated),
		)
	}

	result.Duration = time.Since(startTime)
	return result
}

// printDryRunStats prints statistics for dry run
func printDryRunStats(points []migration.DataPoint) {
	fmt.Println("\n=== DRY RUN RESULTS ===")
	fmt.Printf("Total records parsed: %d\n", len(points))

	// Count by table
	tableCounts := make(map[string]int)
	for _, p := range points {
		tableCounts[p.SourceTable]++
	}

	fmt.Println("\nRecords by table:")
	for table, count := range tableCounts {
		fmt.Printf("  %s: %d\n", table, count)
	}

	// Sample data
	if len(points) > 0 {
		fmt.Println("\nSample record:")
		fmt.Printf("  SourceID: %s\n", points[0].SourceID)
		fmt.Printf("  Table: %s\n", points[0].SourceTable)
		fmt.Printf("  Content preview: %.100s...\n", points[0].Content)
	}
}

// printResults prints final migration results
func printResults(result *migration.MigrationResult) {
	fmt.Println("\n=== MIGRATION COMPLETE ===")
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Total Records: %d\n", result.TotalRecords)
	fmt.Printf("Processed: %d\n", result.ProcessedCount)
	fmt.Printf("Skipped: %d\n", result.SkippedCount)
	fmt.Printf("Errors: %d\n", result.ErrorCount)
	fmt.Printf("Nodes Created: %d\n", result.NodesCreated)
	fmt.Printf("Edges Created: %d\n", result.EdgesCreated)

	if result.TotalRecords > 0 {
		rate := float64(result.ProcessedCount) / result.Duration.Seconds()
		fmt.Printf("Rate: %.2f records/sec\n", rate)
	}

	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}
	}
}
