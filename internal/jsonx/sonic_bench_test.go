// Package jsonx provides benchmarks comparing Sonic to encoding/json.
package jsonx

import (
	"encoding/json"
	"testing"
)

// Benchmarked data structures
type SmallStruct struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Flag  bool   `json:"flag"`
	Value float64 `json:"value"`
}

type MediumStruct struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Tags        []string               `json:"tags"`
	Attributes  map[string]interface{} `json:"attributes"`
	Metadata    map[string]string      `json:"metadata"`
}

type LargeStruct struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Tags        []string               `json:"tags"`
	Attributes  map[string]interface{} `json:"attributes"`
	Metadata    map[string]string      `json:"metadata"`
	History     []map[string]interface{} `json:"history"`
	Relations   []Relation             `json:"relations"`
	Config      BenchmarkConfig        `json:"config"`
}

type Relation struct {
	Type     string                 `json:"type"`
	TargetID int                    `json:"target_id"`
	Weight   float64                `json:"weight"`
	Data     map[string]interface{} `json:"data"`
}

type BenchmarkConfig struct {
	Enabled     bool              `json:"enabled"`
	Priority    int               `json:"priority"`
	Settings    map[string]string `json:"settings"`
	Preferences []string          `json:"preferences"`
}

var (
	smallStruct = SmallStruct{
		ID:    123,
		Name:  "test-entity",
		Flag:  true,
		Value: 45.67,
	}

	mediumStruct = MediumStruct{
		ID:          456,
		Name:        "medium-entity",
		Description: "A medium-sized entity with several fields",
		Tags:        []string{"tag1", "tag2", "tag3"},
		Attributes: map[string]interface{}{
			"color":  "blue",
			"size":   100,
			"active": true,
		},
		Metadata: map[string]string{
			"created":  "2024-01-01",
			"modified": "2024-01-15",
		},
	}

	largeStruct = LargeStruct{
		ID:          789,
		Name:        "large-entity",
		Description: "A large entity with many nested structures",
		Tags:        []string{"a", "b", "c", "d", "e"},
		Attributes: map[string]interface{}{
			"color":     "red",
			"size":      200,
			"active":    false,
			"nested":    map[string]string{"key": "value"},
			"primitives": []interface{}{1, "two", 3.0, true},
		},
		Metadata: map[string]string{
			"created":   "2024-01-01T00:00:00Z",
			"modified":  "2024-01-15T12:30:45Z",
			"author":    "system",
			"version":   "2.1.0",
			"checksum":  "abc123",
		},
		History: []map[string]interface{}{
			{"action": "created", "timestamp": "2024-01-01", "user": "admin"},
			{"action": "modified", "timestamp": "2024-01-15", "user": "user1"},
			{"action": "approved", "timestamp": "2024-01-16", "user": "admin"},
		},
		Relations: []Relation{
			{Type: "parent", TargetID: 100, Weight: 1.0, Data: map[string]interface{}{"note": "primary"}},
			{Type: "child", TargetID: 200, Weight: 0.5, Data: map[string]interface{}{"note": "secondary"}},
		},
		Config: BenchmarkConfig{
			Enabled:     true,
			Priority:    5,
			Settings:    map[string]string{"mode": "fast", "level": "debug"},
			Preferences: []string{"opt1", "opt2", "opt3"},
		},
	}
)

// Benchmark Sonic vs encoding/json for Marshal operations
func BenchmarkSonicMarshalSmall(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = Marshal(smallStruct)
	}
}

func BenchmarkJSONMarshalSmall(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(smallStruct)
	}
}

func BenchmarkSonicMarshalMedium(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = Marshal(mediumStruct)
	}
}

func BenchmarkJSONMarshalMedium(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(mediumStruct)
	}
}

func BenchmarkSonicMarshalLarge(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = Marshal(largeStruct)
	}
}

func BenchmarkJSONMarshalLarge(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(largeStruct)
	}
}

// Benchmark Unmarshal operations
func BenchmarkSonicUnmarshalSmall(b *testing.B) {
	data, _ := json.Marshal(smallStruct)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var result SmallStruct
		_ = Unmarshal(data, &result)
	}
}

func BenchmarkJSONUnmarshalSmall(b *testing.B) {
	data, _ := json.Marshal(smallStruct)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var result SmallStruct
		_ = json.Unmarshal(data, &result)
	}
}

func BenchmarkSonicUnmarshalMedium(b *testing.B) {
	data, _ := json.Marshal(mediumStruct)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var result MediumStruct
		_ = Unmarshal(data, &result)
	}
}

func BenchmarkJSONUnmarshalMedium(b *testing.B) {
	data, _ := json.Marshal(mediumStruct)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var result MediumStruct
		_ = json.Unmarshal(data, &result)
	}
}

func BenchmarkSonicUnmarshalLarge(b *testing.B) {
	data, _ := json.Marshal(largeStruct)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var result LargeStruct
		_ = Unmarshal(data, &result)
	}
}

func BenchmarkJSONUnmarshalLarge(b *testing.B) {
	data, _ := json.Marshal(largeStruct)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var result LargeStruct
		_ = json.Unmarshal(data, &result)
	}
}

// Benchmark arrays
type EntityArray struct {
	Entities []SmallStruct `json:"entities"`
	Count    int           `json:"count"`
}

func BenchmarkSonicMarshalArray(b *testing.B) {
	entities := make([]SmallStruct, 100)
	for i := range entities {
		entities[i] = smallStruct
	}
	data := EntityArray{Entities: entities, Count: 100}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = Marshal(data)
	}
}

func BenchmarkJSONMarshalArray(b *testing.B) {
	entities := make([]SmallStruct, 100)
	for i := range entities {
		entities[i] = smallStruct
	}
	data := EntityArray{Entities: entities, Count: 100}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(data)
	}
}
