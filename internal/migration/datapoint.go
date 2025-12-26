// Package migration provides SQL database migration capabilities for the
// Reflective Memory Kernel. It transforms SQL dumps into Knowledge Graph nodes.
package migration

import (
	"fmt"
	"time"

	"github.com/reflective-memory-kernel/internal/graph"
)

// SQLDialect represents the SQL database dialect
type SQLDialect string

const (
	DialectPostgres SQLDialect = "postgres"
	DialectMySQL    SQLDialect = "mysql"
	DialectGeneric  SQLDialect = "generic"
)

// DataPoint represents a normalized data unit for migration.
// Every piece of data (SQL row, JSON object) is converted to this format
// before being processed by the cognification pipeline.
type DataPoint struct {
	// Source identification
	SourceID    string `json:"source_id"`
	SourceTable string `json:"source_table"`

	// Content for LLM processing
	Content string `json:"content"` // Human-readable text representation

	// Raw data preservation
	RawData map[string]interface{} `json:"raw_data"`

	// Migration metadata
	Namespace string    `json:"namespace"`
	Timestamp time.Time `json:"timestamp"`

	// Optional hierarchy
	ParentID string   `json:"parent_id,omitempty"`
	ChildIDs []string `json:"child_ids,omitempty"`
}

// ToTextContent converts raw data fields to LLM-readable text
func (dp *DataPoint) ToTextContent() string {
	var content string
	for key, value := range dp.RawData {
		if value != nil {
			content += key + ": " + toString(value) + "\n"
		}
	}
	return content
}

// SQLConfig holds the migration configuration
type SQLConfig struct {
	// Source configuration
	Dialect    SQLDialect `yaml:"dialect"`
	SourcePath string     `yaml:"source_path"`

	// Processing configuration
	Namespace string `yaml:"namespace"`
	BatchSize int    `yaml:"batch_size"`
	Workers   int    `yaml:"workers"`

	// Table mappings
	Tables []TableMapping `yaml:"tables"`

	// AI Service
	AIServicesURL string `yaml:"ai_services_url"`
}

// DefaultSQLConfig returns sensible defaults
func DefaultSQLConfig() SQLConfig {
	return SQLConfig{
		Dialect:       DialectGeneric,
		Namespace:     "import",
		BatchSize:     100,
		Workers:       4,
		AIServicesURL: "http://localhost:8000",
	}
}

// TableMapping defines how a SQL table maps to graph nodes
type TableMapping struct {
	// SQL table name
	Name string `yaml:"name"`

	// Target node type
	NodeType graph.NodeType `yaml:"node_type"`

	// Column mappings
	NameColumn  string   `yaml:"name_column"`  // Column for node.Name
	DescColumns []string `yaml:"desc_columns"` // Columns for node.Description
	TagColumns  []string `yaml:"tag_columns"`  // Columns to convert to tags

	// Primary key for source tracking
	PrimaryKey string `yaml:"primary_key"`

	// Relationship mappings (foreign keys)
	Relations []RelationMapping `yaml:"relations"`

	// Column exclusions
	ExcludeColumns []string `yaml:"exclude_columns"`
}

// RelationMapping defines how foreign keys become graph edges
type RelationMapping struct {
	// Source column (foreign key)
	ForeignKey string `yaml:"foreign_key"`

	// Target table reference
	TargetTable string `yaml:"target_table"`

	// Edge type to create
	EdgeType graph.EdgeType `yaml:"edge_type"`

	// Optional: column in target table that matches (defaults to PrimaryKey)
	TargetColumn string `yaml:"target_column,omitempty"`
}

// MigrationResult holds the results of a migration run
type MigrationResult struct {
	TotalRecords   int64         `json:"total_records"`
	ProcessedCount int64         `json:"processed_count"`
	SkippedCount   int64         `json:"skipped_count"`
	ErrorCount     int64         `json:"error_count"`
	NodesCreated   int64         `json:"nodes_created"`
	EdgesCreated   int64         `json:"edges_created"`
	Duration       time.Duration `json:"duration"`
	Errors         []string      `json:"errors,omitempty"`
}

// MigrationProgress tracks real-time migration progress
type MigrationProgress struct {
	CurrentTable    string  `json:"current_table"`
	CurrentRecord   int64   `json:"current_record"`
	TotalRecords    int64   `json:"total_records"`
	PercentComplete float64 `json:"percent_complete"`
	RecordsPerSec   float64 `json:"records_per_sec"`
	EstimatedETA    string  `json:"estimated_eta"`
}

// Helper function to convert interface{} to string
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int, int64, float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}
