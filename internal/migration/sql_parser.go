// Package migration provides SQL parsing capabilities for database migration.
package migration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SQLParser parses SQL dump files from various database dialects
type SQLParser struct {
	dialect   SQLDialect
	namespace string
	tables    map[string]TableMapping
}

// NewSQLParser creates a new SQL parser
func NewSQLParser(dialect SQLDialect, namespace string, tables []TableMapping) *SQLParser {
	tableMap := make(map[string]TableMapping)
	for _, t := range tables {
		tableMap[strings.ToLower(t.Name)] = t
	}
	return &SQLParser{
		dialect:   dialect,
		namespace: namespace,
		tables:    tableMap,
	}
}

// Parse reads a SQL dump and returns a channel of DataPoints
func (p *SQLParser) Parse(reader io.Reader) ([]DataPoint, error) {
	var dataPoints []DataPoint
	scanner := bufio.NewScanner(reader)

	// Increase buffer size for large INSERT statements
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max line size

	var currentStatement strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if p.isSkippable(line) {
			continue
		}

		// Accumulate multi-line statements
		currentStatement.WriteString(line)
		currentStatement.WriteString(" ")

		// Check if statement is complete
		if strings.HasSuffix(line, ";") {
			stmt := currentStatement.String()
			currentStatement.Reset()

			// Parse INSERT statements
			if p.isInsertStatement(stmt) {
				points, err := p.parseInsert(stmt)
				if err != nil {
					// Log but continue
					continue
				}
				dataPoints = append(dataPoints, points...)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return dataPoints, nil
}

// isSkippable returns true for lines that should be skipped
func (p *SQLParser) isSkippable(line string) bool {
	if line == "" {
		return true
	}
	// SQL comments
	if strings.HasPrefix(line, "--") || strings.HasPrefix(line, "/*") {
		return true
	}
	// PostgreSQL-specific
	if strings.HasPrefix(line, "\\") {
		return true
	}
	// MySQL-specific
	if strings.HasPrefix(line, "/*!") {
		return true
	}
	return false
}

// isInsertStatement checks if the statement is an INSERT
func (p *SQLParser) isInsertStatement(stmt string) bool {
	upper := strings.ToUpper(strings.TrimSpace(stmt))
	return strings.HasPrefix(upper, "INSERT")
}

// Regex patterns for parsing INSERT statements
var (
	// INSERT INTO table_name (columns) VALUES (values), (values)...
	insertRegex = regexp.MustCompile(`(?i)INSERT\s+INTO\s+["'\x60]?(\w+)["'\x60]?\s*\(([^)]+)\)\s*VALUES\s*(.+);?`)
	// Match individual value tuples
	valuesTupleRegex = regexp.MustCompile(`\(([^)]+)\)`)
)

// parseInsert parses an INSERT statement into DataPoints
func (p *SQLParser) parseInsert(stmt string) ([]DataPoint, error) {
	var dataPoints []DataPoint

	matches := insertRegex.FindStringSubmatch(stmt)
	if matches == nil || len(matches) < 4 {
		return nil, fmt.Errorf("invalid INSERT statement")
	}

	tableName := strings.ToLower(matches[1])
	columnsStr := matches[2]
	valuesStr := matches[3]

	// Check if we have a mapping for this table
	_, hasMappings := p.tables[tableName]
	if len(p.tables) > 0 && !hasMappings {
		// Skip tables not in our mapping
		return nil, nil
	}

	// Parse column names
	columns := p.parseColumnNames(columnsStr)

	// Parse all value tuples
	valueTuples := valuesTupleRegex.FindAllStringSubmatch(valuesStr, -1)

	for i, tuple := range valueTuples {
		if len(tuple) < 2 {
			continue
		}

		values := p.parseValues(tuple[1])
		if len(values) != len(columns) {
			continue // Column count mismatch, skip
		}

		// Build raw data map
		rawData := make(map[string]interface{})
		for j, col := range columns {
			rawData[col] = values[j]
		}

		// Create DataPoint
		dp := DataPoint{
			SourceID:    fmt.Sprintf("%s_%d", tableName, i),
			SourceTable: tableName,
			RawData:     rawData,
			Namespace:   p.namespace,
			Timestamp:   time.Now(),
		}

		// Generate text content
		dp.Content = dp.ToTextContent()

		// Try to set a better SourceID from primary key
		if mapping, ok := p.tables[tableName]; ok {
			if pk, exists := rawData[mapping.PrimaryKey]; exists && pk != nil {
				dp.SourceID = fmt.Sprintf("%s_%v", tableName, pk)
			}
		}

		dataPoints = append(dataPoints, dp)
	}

	return dataPoints, nil
}

// parseColumnNames extracts column names from the column list
func (p *SQLParser) parseColumnNames(columnsStr string) []string {
	var columns []string
	parts := strings.Split(columnsStr, ",")
	for _, part := range parts {
		col := strings.TrimSpace(part)
		// Remove quotes (", ', `)
		col = strings.Trim(col, "\"'`")
		columns = append(columns, strings.ToLower(col))
	}
	return columns
}

// parseValues extracts values from a value tuple
func (p *SQLParser) parseValues(valuesStr string) []interface{} {
	var values []interface{}
	var current strings.Builder
	inString := false
	stringChar := rune(0)
	escaped := false
	parenDepth := 0

	for _, ch := range valuesStr {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			escaped = true
			current.WriteRune(ch)
			continue
		}

		if !inString && (ch == '\'' || ch == '"') {
			inString = true
			stringChar = ch
			continue
		}

		if inString && ch == stringChar {
			inString = false
			continue
		}

		if !inString {
			if ch == '(' {
				parenDepth++
			} else if ch == ')' {
				parenDepth--
			} else if ch == ',' && parenDepth == 0 {
				values = append(values, p.parseValue(current.String()))
				current.Reset()
				continue
			}
		}

		current.WriteRune(ch)
	}

	// Add last value
	if current.Len() > 0 {
		values = append(values, p.parseValue(current.String()))
	}

	return values
}

// parseValue converts a SQL value string to Go type
func (p *SQLParser) parseValue(s string) interface{} {
	s = strings.TrimSpace(s)

	// NULL
	if strings.ToUpper(s) == "NULL" {
		return nil
	}

	// Remove surrounding quotes
	if (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
		(strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) {
		return s[1 : len(s)-1]
	}

	// Try integer
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}

	// Try float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	// Try boolean
	upper := strings.ToUpper(s)
	if upper == "TRUE" || upper == "T" {
		return true
	}
	if upper == "FALSE" || upper == "F" {
		return false
	}

	// Return as string
	return s
}

// JSONLParser parses JSONL (JSON Lines) files
type JSONLParser struct {
	namespace string
	tableName string
}

// NewJSONLParser creates a new JSONL parser
func NewJSONLParser(namespace, tableName string) *JSONLParser {
	return &JSONLParser{
		namespace: namespace,
		tableName: tableName,
	}
}

// Parse reads a JSONL file and returns DataPoints
func (p *JSONLParser) Parse(reader io.Reader) ([]DataPoint, error) {
	var dataPoints []DataPoint
	scanner := bufio.NewScanner(reader)

	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var rawData map[string]interface{}
		if err := json.Unmarshal([]byte(line), &rawData); err != nil {
			continue // Skip invalid JSON lines
		}

		// Extract ID if present
		sourceID := fmt.Sprintf("%s_%d", p.tableName, lineNum)
		if id, ok := rawData["id"]; ok {
			sourceID = fmt.Sprintf("%s_%v", p.tableName, id)
		}

		dp := DataPoint{
			SourceID:    sourceID,
			SourceTable: p.tableName,
			RawData:     rawData,
			Namespace:   p.namespace,
			Timestamp:   time.Now(),
		}
		dp.Content = dp.ToTextContent()

		dataPoints = append(dataPoints, dp)
		lineNum++
	}

	return dataPoints, scanner.Err()
}
