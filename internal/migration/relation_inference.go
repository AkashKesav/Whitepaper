// Package migration provides relation inference for automatic edge creation.
package migration

import (
	"strings"

	"github.com/reflective-memory-kernel/internal/graph"
)

// RelationInferrer infers relationships from entity Tags and names
type RelationInferrer struct {
	namespace string
}

// NewRelationInferrer creates a new relation inferrer
func NewRelationInferrer(namespace string) *RelationInferrer {
	return &RelationInferrer{namespace: namespace}
}

// InferredEdge represents an edge to be created with entity names
type InferredEdge struct {
	FromName string
	ToName   string
	EdgeType graph.EdgeType
}

// InferFromNodes infers all relationships from a batch of nodes using Tags
func (r *RelationInferrer) InferFromNodes(nodes []*graph.Node) []InferredEdge {
	var edges []InferredEdge

	// Classify nodes by type using tags
	var people []*graph.Node
	var departments []*graph.Node
	var skills []*graph.Node
	var locations []*graph.Node
	var roles []*graph.Node

	for _, node := range nodes {
		nodeClass := r.classifyNode(node)
		switch nodeClass {
		case "person":
			people = append(people, node)
		case "department":
			departments = append(departments, node)
		case "skill":
			skills = append(skills, node)
		case "location":
			locations = append(locations, node)
		case "role":
			roles = append(roles, node)
		}
	}

	// 1. Connect people to their skills (from description/tags)
	edges = append(edges, r.inferPersonSkillEdges(people, skills)...)

	// 2. Connect people to their departments
	edges = append(edges, r.inferPersonDeptEdges(people, departments)...)

	// 3. Connect people to their locations
	edges = append(edges, r.inferPersonLocationEdges(people, locations)...)

	// 4. Connect people in the same batch (colleagues)
	edges = append(edges, r.inferColleagueEdges(people)...)

	return edges
}

// classifyNode determines if a node is a person, department, skill, etc based on tags
func (r *RelationInferrer) classifyNode(node *graph.Node) string {
	tags := strings.Join(node.Tags, " ")
	tagsLower := strings.ToLower(tags)
	nameLower := strings.ToLower(node.Name)
	descLower := strings.ToLower(node.Description)

	// Person detection
	if strings.Contains(tagsLower, "person") || strings.Contains(tagsLower, "name") ||
		strings.Contains(tagsLower, "employee") || strings.Contains(descLower, "user's name") ||
		strings.Contains(descLower, "person's name") {
		return "person"
	}

	// Skill detection
	if strings.Contains(tagsLower, "skill") || strings.Contains(tagsLower, "technology") ||
		strings.Contains(tagsLower, "programming") || strings.Contains(descLower, "skill") {
		return "skill"
	}

	// Department detection
	if strings.Contains(tagsLower, "department") || strings.Contains(tagsLower, "team") ||
		strings.Contains(descLower, "department") || strings.Contains(descLower, "team") {
		return "department"
	}

	// Role/position detection
	if strings.Contains(tagsLower, "role") || strings.Contains(tagsLower, "position") ||
		strings.Contains(tagsLower, "job") || strings.Contains(descLower, "role") {
		return "role"
	}

	// Location detection
	if strings.Contains(tagsLower, "city") || strings.Contains(tagsLower, "location") ||
		strings.Contains(tagsLower, "geography") {
		return "location"
	}

	// Default - check if name looks like a person
	parts := strings.Fields(nameLower)
	if len(parts) == 2 {
		// Two-word name might be a person
		return "person"
	}

	return "other"
}

// inferPersonSkillEdges connects people to skill nodes they mention
func (r *RelationInferrer) inferPersonSkillEdges(people, skills []*graph.Node) []InferredEdge {
	var edges []InferredEdge

	// For each skill, connect to all people in the same batch
	// (assumption: entities extracted together are related)
	for _, person := range people {
		for _, skill := range skills {
			edges = append(edges, InferredEdge{
				FromName: person.Name,
				ToName:   skill.Name,
				EdgeType: graph.EdgeTypeHasInterest,
			})
		}
	}

	return edges
}

// inferPersonDeptEdges connects people to department nodes
func (r *RelationInferrer) inferPersonDeptEdges(people, departments []*graph.Node) []InferredEdge {
	var edges []InferredEdge

	for _, person := range people {
		for _, dept := range departments {
			edges = append(edges, InferredEdge{
				FromName: person.Name,
				ToName:   dept.Name,
				EdgeType: graph.EdgeTypeWorksAt,
			})
		}
	}

	return edges
}

// inferPersonLocationEdges connects people to location nodes
func (r *RelationInferrer) inferPersonLocationEdges(people, locations []*graph.Node) []InferredEdge {
	var edges []InferredEdge

	for _, person := range people {
		for _, loc := range locations {
			edges = append(edges, InferredEdge{
				FromName: person.Name,
				ToName:   loc.Name,
				EdgeType: graph.EdgeTypeWorksAt,
			})
		}
	}

	return edges
}

// inferColleagueEdges connects people extracted in the same batch as colleagues
func (r *RelationInferrer) inferColleagueEdges(people []*graph.Node) []InferredEdge {
	var edges []InferredEdge

	// Only connect if there are multiple people (rare in single record)
	if len(people) < 2 {
		return edges
	}

	maxConnections := 3
	for i, from := range people {
		count := 0
		for j, to := range people {
			if i >= j {
				continue
			}
			if count >= maxConnections {
				break
			}
			edges = append(edges, InferredEdge{
				FromName: from.Name,
				ToName:   to.Name,
				EdgeType: graph.EdgeTypeColleague,
			})
			count++
		}
	}

	return edges
}
