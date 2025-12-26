// Package migration provides batch processing for SQL-to-graph migration.
package migration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
)

// Processor handles batch processing of DataPoints into graph nodes
type Processor struct {
	graphClient   *graph.Client
	aiServicesURL string
	httpClient    *http.Client
	config        SQLConfig
	logger        *zap.Logger

	// Progress tracking
	mu       sync.RWMutex
	progress MigrationProgress
}

// NewProcessor creates a new migration processor
func NewProcessor(
	graphClient *graph.Client,
	aiServicesURL string,
	config SQLConfig,
	logger *zap.Logger,
) *Processor {
	return &Processor{
		graphClient:   graphClient,
		aiServicesURL: aiServicesURL,
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // Increased for LLM batch processing
		},
		config: config,
		logger: logger,
	}
}

// ProcessBatch processes a batch of DataPoints through the cognification pipeline
func (p *Processor) ProcessBatch(ctx context.Context, points []DataPoint) (*BatchResult, error) {
	result := &BatchResult{
		StartTime: time.Now(),
	}

	// Step 1: Cognify - Extract entities via AI service
	cognifyResults, err := p.cognifyBatch(ctx, points)
	if err != nil {
		return nil, fmt.Errorf("cognify failed: %w", err)
	}

	// Step 2: Build nodes from cognify results
	var allNodes []*graph.Node
	var allEdges []graph.EdgeInput

	for i, cr := range cognifyResults {
		if len(cr.Entities) == 0 {
			result.SkippedCount++
			continue
		}

		// Convert entities to graph nodes
		nodes := p.entitiesToNodes(cr.Entities, points[i])
		allNodes = append(allNodes, nodes...)

		// Build edges from relations
		edges := p.relationsToEdges(cr.Relations, points[i])
		allEdges = append(allEdges, edges...)

		result.ProcessedCount++
	}

	// Step 3: Deduplicate against existing graph
	if len(allNodes) > 0 {
		allNodes, err = p.deduplicate(ctx, allNodes)
		if err != nil {
			p.logger.Warn("deduplication warning", zap.Error(err))
		}
	}

	// Step 4: Batch upsert to DGraph
	if len(allNodes) > 0 {
		uidMap, err := p.graphClient.CreateNodes(ctx, allNodes)
		if err != nil {
			return nil, fmt.Errorf("graph upsert failed: %w", err)
		}
		result.NodesCreated = int64(len(uidMap))

		// Step 5: Infer relationships from entity attributes
		inferrer := NewRelationInferrer(p.config.Namespace)
		inferredEdges := inferrer.InferFromNodes(allNodes)
		p.logger.Info("Inferred relationships from attributes",
			zap.Int("inferred_edges", len(inferredEdges)))

		// Step 6: Resolve entity names to UIDs for edges
		// First, build a name->UID lookup from created nodes + existing
		nameToUID := make(map[string]string)
		for name, uid := range uidMap {
			nameToUID[name] = uid
		}

		// Also fetch existing nodes that might be edge targets (skills, managers)
		var targetNames []string
		for _, ie := range inferredEdges {
			targetNames = append(targetNames, ie.FromName, ie.ToName)
		}
		if len(targetNames) > 0 {
			existing, err := p.graphClient.GetNodesByNames(ctx, p.config.Namespace, targetNames)
			if err == nil {
				for name, node := range existing {
					if node != nil {
						nameToUID[name] = node.UID
					}
				}
			}
		}

		// Convert inferred edges to graph.EdgeInput with resolved UIDs
		for _, ie := range inferredEdges {
			fromUID := nameToUID[ie.FromName]
			toUID := nameToUID[ie.ToName]
			if fromUID != "" && toUID != "" {
				allEdges = append(allEdges, graph.EdgeInput{
					FromUID: fromUID,
					ToUID:   toUID,
					Type:    ie.EdgeType,
					Status:  graph.EdgeStatusCurrent,
				})
			}
		}

		// Create edges after nodes exist
		if len(allEdges) > 0 {
			if err := p.graphClient.CreateEdges(ctx, allEdges); err != nil {
				p.logger.Warn("edge creation warning", zap.Error(err))
			} else {
				result.EdgesCreated = int64(len(allEdges))
				p.logger.Info("Created relationship edges", zap.Int64("edges", result.EdgesCreated))
			}
		}
	}

	result.Duration = time.Since(result.StartTime)
	return result, nil
}

// BatchResult holds results from a batch operation
type BatchResult struct {
	ProcessedCount int64
	SkippedCount   int64
	NodesCreated   int64
	EdgesCreated   int64
	StartTime      time.Time
	Duration       time.Duration
}

// CognifyRequest is sent to the AI service
type CognifyRequest struct {
	Items []CognifyItem `json:"items"`
}

// CognifyItem represents one item to cognify
type CognifyItem struct {
	SourceID    string                 `json:"source_id"`
	SourceTable string                 `json:"source_table"`
	Content     string                 `json:"content"`
	RawData     map[string]interface{} `json:"raw_data"`
}

// CognifyResult is the response from AI service
type CognifyResult struct {
	SourceID  string              `json:"source_id"`
	Entities  []ExtractedEntity   `json:"entities"`
	Relations []ExtractedRelation `json:"relations"`
}

// ExtractedEntity from AI cognification
type ExtractedEntity struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	Attributes  map[string]string `json:"attributes"`
}

// ExtractedRelation from AI cognification
type ExtractedRelation struct {
	FromName string `json:"from_name"`
	ToName   string `json:"to_name"`
	Type     string `json:"type"`
}

// cognifyBatch sends a batch to the AI service for entity extraction
func (p *Processor) cognifyBatch(ctx context.Context, points []DataPoint) ([]CognifyResult, error) {
	// Build request
	items := make([]CognifyItem, len(points))
	for i, dp := range points {
		items[i] = CognifyItem{
			SourceID:    dp.SourceID,
			SourceTable: dp.SourceTable,
			Content:     dp.Content,
			RawData:     dp.RawData,
		}
	}

	reqBody, err := json.Marshal(CognifyRequest{Items: items})
	if err != nil {
		return nil, err
	}

	// Call AI service
	url := p.aiServicesURL + "/cognify-batch"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AI service unavailable - NO FALLBACK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI service returned status %d - NO FALLBACK", resp.StatusCode)
	}

	var results []CognifyResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode AI response - NO FALLBACK: %w", err)
	}

	return results, nil
}

// entitiesToNodes converts extracted entities to graph nodes
func (p *Processor) entitiesToNodes(entities []ExtractedEntity, source DataPoint) []*graph.Node {
	nodes := make([]*graph.Node, 0, len(entities))

	for _, e := range entities {
		node := &graph.Node{
			Name:                 e.Name,
			Description:          e.Description,
			Tags:                 e.Tags,
			Namespace:            p.config.Namespace,
			Confidence:           0.8,
			Activation:           0.5,
			SourceConversationID: source.SourceID,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		}

		// Set node type
		switch e.Type {
		case "Person", "User":
			node.SetType(graph.NodeTypeEntity)
		case "Preference":
			node.SetType(graph.NodeTypePreference)
		case "Fact":
			node.SetType(graph.NodeTypeFact)
		case "Event":
			node.SetType(graph.NodeTypeEvent)
		default:
			node.SetType(graph.NodeTypeEntity)
		}

		// Copy attributes
		if len(e.Attributes) > 0 {
			node.Attributes = e.Attributes
		}

		nodes = append(nodes, node)
	}

	return nodes
}

// relationsToEdges converts extracted relations to graph edges
func (p *Processor) relationsToEdges(relations []ExtractedRelation, source DataPoint) []graph.EdgeInput {
	edges := make([]graph.EdgeInput, 0, len(relations))

	for _, r := range relations {
		edge := graph.EdgeInput{
			Type:   graph.EdgeType(r.Type),
			Status: graph.EdgeStatusCurrent,
		}
		// Note: FromUID and ToUID will be resolved after node creation
		edges = append(edges, edge)
	}

	return edges
}

// deduplicate removes nodes that already exist in the graph
func (p *Processor) deduplicate(ctx context.Context, nodes []*graph.Node) ([]*graph.Node, error) {
	if len(nodes) == 0 {
		return nodes, nil
	}

	// Extract names for lookup
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}

	// Query existing nodes
	existing, err := p.graphClient.GetNodesByNames(ctx, p.config.Namespace, names)
	if err != nil {
		return nodes, err // Return all if lookup fails
	}

	// Filter out duplicates, update existing ones
	var newNodes []*graph.Node
	for _, node := range nodes {
		if existingNode, found := existing[node.Name]; found {
			// Merge: update existing node's description if richer
			if len(node.Description) > len(existingNode.Description) {
				p.graphClient.UpdateDescription(ctx, existingNode.UID, node.Description)
			}
			// Add new tags
			if len(node.Tags) > 0 {
				p.graphClient.AddTags(ctx, existingNode.UID, node.Tags)
			}
		} else {
			newNodes = append(newNodes, node)
		}
	}

	return newNodes, nil
}

// GetProgress returns current migration progress
func (p *Processor) GetProgress() MigrationProgress {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.progress
}

// UpdateProgress updates the migration progress
func (p *Processor) UpdateProgress(table string, current, total int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.progress.CurrentTable = table
	p.progress.CurrentRecord = current
	p.progress.TotalRecords = total
	if total > 0 {
		p.progress.PercentComplete = float64(current) / float64(total) * 100
	}
}

// =============================================================================
// LAYER 2: Community Summarization
// =============================================================================

// CommunityRequest is sent to AI service for L2 summarization
type CommunityRequest struct {
	CommunityName    string                   `json:"community_name"`
	CommunityType    string                   `json:"community_type"`
	Entities         []map[string]interface{} `json:"entities"`
	MaxSummaryLength int                      `json:"max_summary_length"`
}

// CommunitySummary is the response from L2 summarization
type CommunitySummary struct {
	CommunityName string   `json:"community_name"`
	CommunityType string   `json:"community_type"`
	MemberCount   int      `json:"member_count"`
	KeyMembers    []string `json:"key_members"`
	Summary       string   `json:"summary"`
	KeyFacts      []string `json:"key_facts"`
	CommonSkills  []string `json:"common_skills"`
}

// SummarizeCommunity generates a summary for a group of entities
func (p *Processor) SummarizeCommunity(ctx context.Context, communityName, communityType string, entities []map[string]interface{}) (*CommunitySummary, error) {
	reqBody, err := json.Marshal(CommunityRequest{
		CommunityName:    communityName,
		CommunityType:    communityType,
		Entities:         entities,
		MaxSummaryLength: 500,
	})
	if err != nil {
		return nil, err
	}

	url := p.aiServicesURL + "/summarize-community"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AI service unavailable for community summary - NO FALLBACK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI service returned status %d for community summary - NO FALLBACK", resp.StatusCode)
	}

	var result CommunitySummary
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode community summary - NO FALLBACK: %w", err)
	}

	return &result, nil
}

// =============================================================================
// LAYER 3: Global Overview Summarization
// =============================================================================

// GlobalOverviewRequest is sent to AI service for L3 summarization
type GlobalOverviewRequest struct {
	Namespace          string             `json:"namespace"`
	CommunitySummaries []CommunitySummary `json:"community_summaries"`
	TotalEntities      int                `json:"total_entities"`
	TotalRelationships int                `json:"total_relationships"`
}

// GlobalOverview is the response from L3 summarization
type GlobalOverview struct {
	Namespace        string   `json:"namespace"`
	Title            string   `json:"title"`
	ExecutiveSummary string   `json:"executive_summary"`
	TotalEntities    int      `json:"total_entities"`
	TotalCommunities int      `json:"total_communities"`
	KeyInsights      []string `json:"key_insights"`
	TopSkills        []string `json:"top_skills"`
	CompressionRatio float64  `json:"compression_ratio"`
}

// SummarizeGlobal generates a global overview from community summaries
func (p *Processor) SummarizeGlobal(ctx context.Context, namespace string, communities []CommunitySummary, totalEntities int) (*GlobalOverview, error) {
	reqBody, err := json.Marshal(GlobalOverviewRequest{
		Namespace:          namespace,
		CommunitySummaries: communities,
		TotalEntities:      totalEntities,
	})
	if err != nil {
		return nil, err
	}

	url := p.aiServicesURL + "/summarize-global"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AI service unavailable for global overview - NO FALLBACK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI service returned status %d for global overview - NO FALLBACK", resp.StatusCode)
	}

	var result GlobalOverview
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode global overview - NO FALLBACK: %w", err)
	}

	return &result, nil
}

// GroupByField groups DataPoints by a field value for community detection
func GroupByField(points []DataPoint, field string) map[string][]DataPoint {
	groups := make(map[string][]DataPoint)
	for _, p := range points {
		key := "unknown"
		if val, ok := p.RawData[field].(string); ok {
			key = val
		}
		groups[key] = append(groups[key], p)
	}
	return groups
}
