// Package vectorindex provides hierarchical vector tree indexing for efficient semantic search
package vectorindex

import (
	"math"
	"sort"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// VectorNode represents a single node in the Vector Tree
// Can be a Leaf (Text Chunk) or an Internal Node (Compressed Summary)
type VectorNode struct {
	NodeID      string    `json:"node_id"`
	Vector      []float64 `json:"vector"`
	ChildrenIDs []string  `json:"children_ids"`
	Text        string    `json:"text,omitempty"` // Only for leaves
	Depth       int       `json:"depth"`

	// Additional metadata
	LeafCount   int       `json:"leaf_count,omitempty"`   // Number of leaves under this node
	ClusterID   int       `json:"cluster_id,omitempty"`   // Cluster assignment
	CentroidDist float64  `json:"centroid_dist,omitempty"` // Distance to cluster centroid
}

// ToDict converts the node to a map for JSON serialization
func (n *VectorNode) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"node_id":      n.NodeID,
		"vector":       n.Vector,
		"children_ids": n.ChildrenIDs,
		"text":         n.Text,
		"depth":        n.Depth,
		"leaf_count":   n.LeafCount,
	}
}

// Chunk represents a text chunk with its embedding
type Chunk struct {
	Text      string    `json:"text"`
	Embedding []float64 `json:"embedding"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// IndexBuilder builds a Hierarchical Vector Tree from flat chunks
type IndexBuilder struct {
	BranchingFactor int
	Dim             int
	Logger          *zap.Logger
}

// NewIndexBuilder creates a new vector index builder
func NewIndexBuilder(branchingFactor, dim int, logger *zap.Logger) *IndexBuilder {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &IndexBuilder{
		BranchingFactor: branchingFactor,
		Dim:             dim,
		Logger:          logger,
	}
}

// BuildIndex creates a hierarchical vector tree from chunks
func (b *IndexBuilder) BuildIndex(chunks []Chunk) map[string]*VectorNode {
	if len(chunks) == 0 {
		return make(map[string]*VectorNode)
	}

	// 1. Create Leaf Nodes
	leaves := make([]*VectorNode, 0, len(chunks))
	for _, chunk := range chunks {
		node := &VectorNode{
			NodeID: uuid.New().String(),
			Vector: chunk.Embedding,
			Text:   chunk.Text,
			Depth:  0,
		}
		leaves = append(leaves, node)
	}

	treeMap := make(map[string]*VectorNode)
	for _, node := range leaves {
		treeMap[node.NodeID] = node
	}

	currentLayer := leaves

	// 2. Recursive Clustering & Compression
	for len(currentLayer) > 1 {
		b.Logger.Debug("Building layer",
			zap.Int("depth", currentLayer[0].Depth+1),
			zap.Int("nodes", len(currentLayer)))

		nextLayer := b.processLayer(currentLayer)

		// Add new nodes to map
		for _, node := range nextLayer {
			treeMap[node.NodeID] = node
		}

		currentLayer = nextLayer
	}

	return treeMap
}

// processLayer takes N nodes, clusters them, compresses them into M parent nodes
func (b *IndexBuilder) processLayer(nodes []*VectorNode) []*VectorNode {
	numNodes := len(nodes)

	// Determine number of clusters
	numClusters := numNodes / b.BranchingFactor
	if numClusters <= 0 {
		numClusters = 1
	}
	if numNodes%b.BranchingFactor != 0 {
		numClusters++
	}

	// Perform K-means clustering
	clusterAssignments := b.kMeansCluster(nodes, numClusters)

	// Group nodes by cluster
	clusterGroups := make(map[int][]*VectorNode)
	for i, node := range nodes {
		clusterID := clusterAssignments[i]
		clusterGroups[clusterID] = append(clusterGroups[clusterID], node)
	}

	// Create parent nodes for each cluster
	parents := make([]*VectorNode, 0, len(clusterGroups))
	for clusterID, group := range clusterGroups {
		if len(group) == 0 {
			continue
		}

		// Create parent vector via compression (mean of cluster)
		parentVector := b.compressCluster(group)

		// Collect children IDs
		childrenIDs := make([]string, len(group))
		for i, node := range group {
			childrenIDs[i] = node.NodeID
		}

		// Count total leaves under this parent
		leafCount := 0
		for _, node := range group {
			leafCount += node.LeafCount
			if leafCount == 0 {
				leafCount++ // This node is a leaf
			}
		}

		parent := &VectorNode{
			NodeID:      uuid.New().String(),
			Vector:      parentVector,
			ChildrenIDs: childrenIDs,
			Depth:       group[0].Depth + 1,
			LeafCount:   leafCount,
			ClusterID:   clusterID,
		}

		parents = append(parents, parent)
	}

	return parents
}

// kMeansCluster performs K-means clustering on nodes
func (b *IndexBuilder) kMeansCluster(nodes []*VectorNode, k int) []int {
	n := len(nodes)
	if n <= k {
		// Each node gets its own cluster
		assignments := make([]int, n)
		for i := range assignments {
			assignments[i] = i
		}
		return assignments
	}

	if k <= 0 {
		k = 1
	}

	dim := b.Dim

	// Initialize centroids using k-means++ like approach
	centroids := make([][]float64, k)
	centroids[0] = make([]float64, dim)
	copy(centroids[0], nodes[0].Vector)

	for i := 1; i < k; i++ {
		// Find the point farthest from existing centroids
		maxDist := 0.0
		farthestIdx := 0

		for j, node := range nodes {
			minCentroidDist := math.MaxFloat64
			for c := 0; c < i; c++ {
				dist := cosineDistance(node.Vector, centroids[c])
				if dist < minCentroidDist {
					minCentroidDist = dist
				}
			}

			if minCentroidDist > maxDist {
				maxDist = minCentroidDist
				farthestIdx = j
			}
		}

		centroids[i] = make([]float64, dim)
		copy(centroids[i], nodes[farthestIdx].Vector)
	}

	// Iterative k-means
	assignments := make([]int, n)
	maxIterations := 100

	for iter := 0; iter < maxIterations; iter++ {
		// Assign nodes to nearest centroid
		changed := false
		for i, node := range nodes {
			minDist := math.MaxFloat64
			bestCluster := 0

			for c := 0; c < k; c++ {
				dist := cosineDistance(node.Vector, centroids[c])
				if dist < minDist {
					minDist = dist
					bestCluster = c
				}
			}

			if assignments[i] != bestCluster {
				assignments[i] = bestCluster
				changed = true
			}
		}

		if !changed {
			break
		}

		// Update centroids
		clusterSums := make([][]float64, k)
		clusterCounts := make([]int, k)

		for i := 0; i < k; i++ {
			clusterSums[i] = make([]float64, dim)
		}

		for i, node := range nodes {
			cluster := assignments[i]
			for j := 0; j < dim; j++ {
				clusterSums[cluster][j] += node.Vector[j]
			}
			clusterCounts[cluster]++
		}

		for c := 0; c < k; c++ {
			if clusterCounts[c] > 0 {
				for j := 0; j < dim; j++ {
					centroids[c][j] = clusterSums[c][j] / float64(clusterCounts[c])
				}
			}
		}
	}

	return assignments
}

// compressCluster creates a parent vector from a cluster of nodes
func (b *IndexBuilder) compressCluster(nodes []*VectorNode) []float64 {
	if len(nodes) == 0 {
		return make([]float64, b.Dim)
	}

	if len(nodes) == 1 {
		result := make([]float64, b.Dim)
		copy(result, nodes[0].Vector)
		return result
	}

	// Mean pooling
	mean := make([]float64, b.Dim)
	for _, node := range nodes {
		for i, v := range node.Vector {
			mean[i] += v
		}
	}

	for i := range mean {
		mean[i] /= float64(len(nodes))
	}

	// Normalize
	norm := 0.0
	for _, v := range mean {
		norm += v * v
	}
	norm = math.Sqrt(norm)

	if norm > 0 {
		for i := range mean {
			mean[i] /= norm
		}
	}

	return mean
}

// cosineDistance calculates the cosine distance between two vectors
func cosineDistance(a, b []float64) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}

	dotProduct := 0.0
	normA := 0.0
	normB := 0.0

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	if normA == 0 || normB == 0 {
		return 1.0
	}

	return 1.0 - (dotProduct / (normA * normB))
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float64) float64 {
	return 1.0 - cosineDistance(a, b)
}

// euclideanDistance calculates the Euclidean distance between two vectors
func euclideanDistance(a, b []float64) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}

	sum := 0.0
	for i := 0; i < len(a); i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return math.Sqrt(sum)
}

// dotProduct calculates the dot product of two vectors
func dotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	sum := 0.0
	for i := 0; i < len(a); i++ {
		sum += a[i] * b[i]
	}

	return sum
}

// normalize normalizes a vector to unit length
func normalize(v []float64) []float64 {
	result := make([]float64, len(v))
	copy(result, v)

	norm := 0.0
	for _, val := range result {
		norm += val * val
	}
	norm = math.Sqrt(norm)

	if norm > 0 {
		for i := range result {
			result[i] /= norm
		}
	}

	return result
}

// Search performs a search in the vector tree
func (b *IndexBuilder) Search(tree map[string]*VectorNode, query []float64, topK int) []*VectorNode {
	if len(tree) == 0 {
		return nil
	}

	// Find root (node with maximum depth)
	maxDepth := 0
	for _, node := range tree {
		if node.Depth > maxDepth {
			maxDepth = node.Depth
		}
	}

	// Collect all root nodes (should be just one)
	var roots []*VectorNode
	for _, node := range tree {
		if node.Depth == maxDepth {
			roots = append(roots, node)
		}
	}

	// Search from roots
	results := make([]searchResult, 0)
	for _, root := range roots {
		b.searchRecursive(root, tree, query, &results)
	}

	// Sort by similarity
	sort.Slice(results, func(i, j int) bool {
		return results[i].similarity > results[j].similarity
	})

	// Return top K
	k := topK
	if k > len(results) {
		k = len(results)
	}

	nodes := make([]*VectorNode, k)
	for i := 0; i < k; i++ {
		nodes[i] = results[i].node
	}

	return nodes
}

type searchResult struct {
	node       *VectorNode
	similarity float64
}

func (b *IndexBuilder) searchRecursive(node *VectorNode, tree map[string]*VectorNode, query []float64, results *[]searchResult) {
	sim := cosineSimilarity(query, node.Vector)

	// If this is a leaf, add to results
	if node.Text != "" || len(node.ChildrenIDs) == 0 {
		*results = append(*results, searchResult{
			node:       node,
			similarity: sim,
		})
		return
	}

	// For internal nodes, check children
	for _, childID := range node.ChildrenIDs {
		if child, ok := tree[childID]; ok {
			// Prune if similarity is too low
			childSim := cosineSimilarity(query, child.Vector)
			if childSim > 0.5 || sim > 0.7 {
				b.searchRecursive(child, tree, query, results)
			}
		}
	}
}

// GetTreeStats returns statistics about the tree
func (b *IndexBuilder) GetTreeStats(tree map[string]*VectorNode) map[string]interface{} {
	if len(tree) == 0 {
		return map[string]interface{}{
			"total_nodes": 0,
			"depth":       0,
		}
	}

	maxDepth := 0
	leafCount := 0
	internalCount := 0

	for _, node := range tree {
		if node.Depth > maxDepth {
			maxDepth = node.Depth
		}
		if node.Text != "" || len(node.ChildrenIDs) == 0 {
			leafCount++
		} else {
			internalCount++
		}
	}

	// Calculate average branching factor
	totalChildren := 0
	internalNodes := 0
	for _, node := range tree {
		if len(node.ChildrenIDs) > 0 {
			totalChildren += len(node.ChildrenIDs)
			internalNodes++
		}
	}

	avgBranching := 0.0
	if internalNodes > 0 {
		avgBranching = float64(totalChildren) / float64(internalNodes)
	}

	return map[string]interface{}{
		"total_nodes":      len(tree),
		"depth":            maxDepth + 1,
		"leaf_count":       leafCount,
		"internal_count":   internalCount,
		"avg_branching":    avgBranching,
		"branching_factor": b.BranchingFactor,
	}
}
