package precortex

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/reflective-memory-kernel/internal/graph"
	"go.uber.org/zap"
)

// ReflexEngine handles deterministic fact retrieval from DGraph
type ReflexEngine struct {
	graphClient *graph.Client
	logger      *zap.Logger

	// Fact extraction patterns
	factPatterns []factPattern

	// Response templates
	templates map[string]*template.Template
}

type factPattern struct {
	Pattern   *regexp.Regexp
	QueryType string
	Template  string
}

// NewReflexEngine creates a new reflex engine
func NewReflexEngine(graphClient *graph.Client, logger *zap.Logger) *ReflexEngine {
	re := &ReflexEngine{
		graphClient: graphClient,
		logger:      logger,
		templates:   make(map[string]*template.Template),
	}

	// Initialize fact extraction patterns
	re.factPatterns = []factPattern{
		{
			Pattern:   regexp.MustCompile(`(?i)what\s+(is|are)\s+my\s+email`),
			QueryType: "user_email",
			Template:  "Your email is {{.Email}}.",
		},
		{
			Pattern:   regexp.MustCompile(`(?i)what\s+(is|are)\s+my\s+name`),
			QueryType: "user_name",
			Template:  "Your name is {{.Name}}.",
		},
		{
			Pattern:   regexp.MustCompile(`(?i)(list|show|what are)\s+(my\s+)?groups`),
			QueryType: "user_groups",
			Template:  "{{if .Groups}}Your groups are: {{range $i, $g := .Groups}}{{if $i}}, {{end}}{{$g.Name}}{{end}}.{{else}}You don't have any groups yet.{{end}}",
		},
		{
			Pattern:   regexp.MustCompile(`(?i)what\s+(do\s+)?i\s+(like|love|prefer)`),
			QueryType: "user_preferences",
			Template:  "{{if .Preferences}}Based on what you've told me, you like: {{range $i, $p := .Preferences}}{{if $i}}, {{end}}{{$p.Name}}{{end}}.{{else}}I don't have any preferences stored for you yet.{{end}}",
		},
		{
			Pattern:   regexp.MustCompile(`(?i)what\s+(do\s+)?you\s+know\s+about\s+me`),
			QueryType: "user_facts",
			Template:  "{{if .Facts}}Here's what I know about you: {{range $i, $f := .Facts}}{{if $i}}; {{end}}{{$f.Description}}{{end}}.{{else}}I'm still learning about you. Tell me more!{{end}}",
		},
	}

	// Compile templates
	for i, fp := range re.factPatterns {
		tmpl, err := template.New(fp.QueryType).Parse(fp.Template)
		if err != nil {
			logger.Error("Failed to parse template", zap.String("type", fp.QueryType), zap.Error(err))
			continue
		}
		re.templates[fp.QueryType] = tmpl
		re.factPatterns[i].Template = "" // Clear, using templates map now
	}

	return re
}

// Handle processes a fact retrieval request
func (re *ReflexEngine) Handle(ctx context.Context, namespace, userID, query string) (string, bool) {
	// Match query against patterns
	for _, fp := range re.factPatterns {
		if fp.Pattern.MatchString(query) {
			re.logger.Debug("Reflex matched pattern", zap.String("type", fp.QueryType))

			// Execute DGraph query based on type
			data, err := re.executeDGraphQuery(ctx, namespace, userID, fp.QueryType)
			if err != nil {
				re.logger.Error("DGraph query failed", zap.Error(err))
				return "", false
			}

			// Fill template with data
			response, err := re.fillTemplate(fp.QueryType, data)
			if err != nil {
				re.logger.Error("Template fill failed", zap.Error(err))
				return "", false
			}

			return response, true
		}
	}

	return "", false
}

// QueryResult holds data from DGraph queries
type QueryResult struct {
	Name        string  `json:"name"`
	Email       string  `json:"email"`
	Groups      []Group `json:"groups"`
	Preferences []Fact  `json:"preferences"`
	Facts       []Fact  `json:"facts"`
}

type Group struct {
	Name string `json:"name"`
}

type Fact struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// executeDGraphQuery runs a DGraph query based on type
func (re *ReflexEngine) executeDGraphQuery(ctx context.Context, namespace, userID, queryType string) (*QueryResult, error) {
	result := &QueryResult{
		Name:   userID,
		Groups: []Group{},
		Facts:  []Fact{},
	}

	switch queryType {
	case "user_email", "user_name":
		// Query user node
		q := `query GetUser($ns: string, $name: string) {
			user(func: type(User)) @filter(eq(namespace, $ns) AND eq(name, $name)) {
				name
				email
			}
		}`
		resp, err := re.graphClient.Query(ctx, q, map[string]string{
			"$ns":   namespace,
			"$name": userID,
		})
		if err != nil {
			return result, err
		}
		var data struct {
			User []struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"user"`
		}
		if err := json.Unmarshal(resp, &data); err == nil && len(data.User) > 0 {
			result.Name = data.User[0].Name
			result.Email = data.User[0].Email
		}

	case "user_groups":
		// Query groups where user is a member
		q := `query GetGroups($ns: string) {
			groups(func: type(Group)) @filter(eq(namespace, $ns)) {
				name
			}
		}`
		resp, err := re.graphClient.Query(ctx, q, map[string]string{
			"$ns": fmt.Sprintf("user_%s", userID),
		})
		if err != nil {
			return result, err
		}
		var data struct {
			Groups []Group `json:"groups"`
		}
		if err := json.Unmarshal(resp, &data); err == nil {
			result.Groups = data.Groups
		}

	case "user_preferences", "user_facts":
		// Query facts for user
		q := `query GetFacts($ns: string) {
			facts(func: type(Fact)) @filter(eq(namespace, $ns)) {
				name
				description
			}
		}`
		resp, err := re.graphClient.Query(ctx, q, map[string]string{
			"$ns": namespace,
		})
		if err != nil {
			return result, err
		}
		var data struct {
			Facts []Fact `json:"facts"`
		}
		if err := json.Unmarshal(resp, &data); err == nil {
			result.Facts = data.Facts
			// Also use as preferences for now
			result.Preferences = data.Facts
		}
	}

	return result, nil
}

// fillTemplate fills a template with query result data
func (re *ReflexEngine) fillTemplate(queryType string, data *QueryResult) (string, error) {
	tmpl, ok := re.templates[queryType]
	if !ok {
		return "", fmt.Errorf("template not found: %s", queryType)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
