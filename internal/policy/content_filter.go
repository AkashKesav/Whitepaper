package policy

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"
)

// ContentFilterType represents the type of content filter
type ContentFilterType string

const (
	FilterTypePII       ContentFilterType = "PII"
	FilterTypeProfanity ContentFilterType = "PROFANITY"
	FilterTypeSensitive ContentFilterType = "SENSITIVE"
	FilterTypeCustom    ContentFilterType = "CUSTOM"
)

// Input length limits for security (prevent DoS via oversized inputs)
const (
	MaxContentLength     = 10 * 1024 * 1024 // 10MB for documents
	MaxCommentLength     = 10 * 1024        // 10KB for comments/messages
	MaxUsernameLength    = 100              // Username limit
	MaxGroupNameLength   = 100              // Group name limit
	MaxQueryLength       = 2000             // Search/query limit
	MaxFilenameLength    = 255              // Filename limit
	MaxEmbeddingLength   = 8000             // Text for embedding limit
)

// ContentFilterAction represents what to do when content is flagged
type ContentFilterAction string

const (
	ActionBlock ContentFilterAction = "BLOCK"
	ActionMask  ContentFilterAction = "MASK"
	ActionWarn  ContentFilterAction = "WARN"
	ActionLog   ContentFilterAction = "LOG"
)

// ContentFilter provides content filtering and PII detection
type ContentFilter struct {
	logger      *zap.Logger
	auditLogger *AuditLogger
	enabled     bool
	patterns    map[ContentFilterType][]*regexp.Regexp
	actions     map[ContentFilterType]ContentFilterAction
	customWords []string
}

// ContentFilterResult contains the result of content filtering
type ContentFilterResult struct {
	IsClean      bool
	FilterType   ContentFilterType
	Action       ContentFilterAction
	MatchedTerms []string
	MaskedText   string
}

// NewContentFilter creates a new content filter
func NewContentFilter(logger *zap.Logger, auditLogger *AuditLogger, enabled bool) *ContentFilter {
	cf := &ContentFilter{
		logger:      logger,
		auditLogger: auditLogger,
		enabled:     enabled,
		patterns:    make(map[ContentFilterType][]*regexp.Regexp),
		actions:     make(map[ContentFilterType]ContentFilterAction),
		customWords: make([]string, 0),
	}

	cf.initializePatterns()
	cf.initializeDefaultActions()

	return cf
}

// initializePatterns sets up default PII detection patterns
func (cf *ContentFilter) initializePatterns() {
	cf.patterns[FilterTypePII] = []*regexp.Regexp{
		// Email addresses
		regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
		// Phone numbers (various formats)
		regexp.MustCompile(`(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`),
		// SSN
		regexp.MustCompile(`\d{3}[-.\s]?\d{2}[-.\s]?\d{4}`),
		// Credit card numbers (basic pattern)
		regexp.MustCompile(`\d{4}[-.\s]?\d{4}[-.\s]?\d{4}[-.\s]?\d{4}`),
		// IP addresses
		regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		// Passport numbers (basic pattern - varies by country)
		regexp.MustCompile(`[A-Z]{1,2}\d{6,9}`),
	}

	cf.patterns[FilterTypeSensitive] = []*regexp.Regexp{
		// Passwords mentioned in text
		regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*\S+`),
		// API keys (generic pattern)
		regexp.MustCompile(`(?i)(api[-_]?key|apikey|secret[-_]?key)\s*[:=]\s*[a-zA-Z0-9]{20,}`),
		// Bearer tokens
		regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9._-]+`),
	}
}

// initializeDefaultActions sets default actions for each filter type
func (cf *ContentFilter) initializeDefaultActions() {
	cf.actions[FilterTypePII] = ActionMask
	cf.actions[FilterTypeProfanity] = ActionBlock
	cf.actions[FilterTypeSensitive] = ActionMask
	cf.actions[FilterTypeCustom] = ActionWarn
}

// SetAction sets the action for a filter type
func (cf *ContentFilter) SetAction(filterType ContentFilterType, action ContentFilterAction) {
	cf.actions[filterType] = action
}

// AddCustomPattern adds a custom filter pattern
func (cf *ContentFilter) AddCustomPattern(pattern string) error {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	cf.patterns[FilterTypeCustom] = append(cf.patterns[FilterTypeCustom], regex)
	return nil
}

// AddBlockedWords adds words to the custom block list
func (cf *ContentFilter) AddBlockedWords(words ...string) {
	cf.customWords = append(cf.customWords, words...)
}

// Filter checks content and applies filtering
func (cf *ContentFilter) Filter(ctx context.Context, userID, content string) (*ContentFilterResult, error) {
	if !cf.enabled || content == "" {
		return &ContentFilterResult{IsClean: true, MaskedText: content}, nil
	}

	result := &ContentFilterResult{
		IsClean:      true,
		MatchedTerms: make([]string, 0),
		MaskedText:   content,
	}

	// Check each filter type
	for filterType, patterns := range cf.patterns {
		for _, pattern := range patterns {
			matches := pattern.FindAllString(content, -1)
			if len(matches) > 0 {
				result.IsClean = false
				result.FilterType = filterType
				result.Action = cf.actions[filterType]
				result.MatchedTerms = append(result.MatchedTerms, matches...)

				// Apply masking if configured
				if cf.actions[filterType] == ActionMask {
					result.MaskedText = cf.maskMatches(result.MaskedText, pattern)
				}
			}
		}
	}

	// Check custom blocked words
	for _, word := range cf.customWords {
		if strings.Contains(strings.ToLower(content), strings.ToLower(word)) {
			result.IsClean = false
			result.FilterType = FilterTypeCustom
			result.Action = cf.actions[FilterTypeCustom]
			result.MatchedTerms = append(result.MatchedTerms, word)
		}
	}

	// Log if content was flagged
	if !result.IsClean && cf.auditLogger != nil {
		cf.auditLogger.Log(ctx, AuditEvent{
			EventType: AuditEventAccess,
			UserID:    userID,
			Action:    "CONTENT_FILTERED",
			Effect:    EffectDeny,
			Reason:    string(result.FilterType) + ": " + strings.Join(result.MatchedTerms[:min(3, len(result.MatchedTerms))], ", "),
			Metadata: map[string]string{
				"filter_type": string(result.FilterType),
				"action":      string(result.Action),
			},
		})
	}

	return result, nil
}

// maskMatches replaces matched patterns with asterisks
func (cf *ContentFilter) maskMatches(content string, pattern *regexp.Regexp) string {
	return pattern.ReplaceAllStringFunc(content, func(match string) string {
		if len(match) <= 4 {
			return strings.Repeat("*", len(match))
		}
		// Keep first and last 2 characters for context
		return match[:2] + strings.Repeat("*", len(match)-4) + match[len(match)-2:]
	})
}

// MaskPII masks all PII in the content
func (cf *ContentFilter) MaskPII(content string) string {
	result := content
	for _, pattern := range cf.patterns[FilterTypePII] {
		result = cf.maskMatches(result, pattern)
	}
	return result
}

// ContainsPII checks if content contains PII without modifying it
func (cf *ContentFilter) ContainsPII(content string) bool {
	for _, pattern := range cf.patterns[FilterTypePII] {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// SanitizeForStorage sanitizes content before storing in the database
func (cf *ContentFilter) SanitizeForStorage(ctx context.Context, userID, content string) (string, error) {
	if !cf.enabled {
		return content, nil
	}

	result, err := cf.Filter(ctx, userID, content)
	if err != nil {
		return "", err
	}

	// Return masked content for storage
	return result.MaskedText, nil
}

// ValidateForExport validates content before exporting
func (cf *ContentFilter) ValidateForExport(ctx context.Context, userID, content string) error {
	result, err := cf.Filter(ctx, userID, content)
	if err != nil {
		return err
	}

	if !result.IsClean && result.Action == ActionBlock {
		return &ContentBlockedError{
			FilterType:   result.FilterType,
			MatchedTerms: result.MatchedTerms,
		}
	}

	return nil
}

// ContentBlockedError is returned when content is blocked
type ContentBlockedError struct {
	FilterType   ContentFilterType
	MatchedTerms []string
}

func (e *ContentBlockedError) Error() string {
	return "content blocked due to " + string(e.FilterType) + " filter"
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ValidateInput performs comprehensive input validation including length, encoding, and content checks
// SECURITY: Prevents DoS via oversized inputs and injection via malformed content
func (cf *ContentFilter) ValidateInput(inputType, content string) error {
	if !cf.enabled {
		return nil
	}

	// Validate content is not empty
	if content == "" {
		return fmt.Errorf("%s cannot be empty", inputType)
	}

	// Determine maximum length based on input type
	var maxLength int
	switch inputType {
	case "content", "document":
		maxLength = MaxContentLength
	case "comment", "message":
		maxLength = MaxCommentLength
	case "username":
		maxLength = MaxUsernameLength
	case "group_name":
		maxLength = MaxGroupNameLength
	case "query", "search":
		maxLength = MaxQueryLength
	case "filename":
		maxLength = MaxFilenameLength
	case "embedding":
		maxLength = MaxEmbeddingLength
	default:
		// Default to comment length for unknown types
		maxLength = MaxCommentLength
	}

	// Check byte length
	if len(content) > maxLength {
		return fmt.Errorf("%s exceeds maximum length of %d bytes (got %d bytes)", inputType, maxLength, len(content))
	}

	// Check for null bytes (injection attempt)
	if strings.Contains(content, "\x00") {
		return fmt.Errorf("%s contains null byte (possible injection attempt)", inputType)
	}

	// Validate UTF-8 encoding
	if !utf8.ValidString(content) {
		return fmt.Errorf("%s contains invalid UTF-8 sequences", inputType)
	}

	// For usernames and group names, validate character set
	if inputType == "username" || inputType == "group_name" {
		// Only allow alphanumeric, underscore, hyphen, and period
		matched, _ := regexp.MatchString(`^[a-zA-Z0-9._-]+$`, content)
		if !matched {
			return fmt.Errorf("%s can only contain letters, numbers, dots, hyphens, and underscores", inputType)
		}
	}

	// For queries and searches, check for common injection patterns
	if inputType == "query" || inputType == "search" {
		lowerContent := strings.ToLower(content)
		suspiciousPatterns := []string{
			"<script", "javascript:", "vbscript:", "onload=", "onerror=", "onclick=",
			"<iframe", "eval(", "exec(", "system(",
		}
		for _, pattern := range suspiciousPatterns {
			if strings.Contains(lowerContent, pattern) {
				cf.logger.Warn("Suspicious query pattern detected",
					zap.String("input_type", inputType),
					zap.String("pattern", pattern))
				return fmt.Errorf("%s contains suspicious content pattern", inputType)
			}
		}
	}

	return nil
}
