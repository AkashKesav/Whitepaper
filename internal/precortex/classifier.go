package precortex

import (
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// IntentClassifier classifies user messages into intent categories
type IntentClassifier struct {
	logger *zap.Logger

	// Compiled patterns for each intent
	greetingPatterns   []*regexp.Regexp
	navigationPatterns []*regexp.Regexp
	factPatterns       []*regexp.Regexp
}

// NewIntentClassifier creates a new intent classifier
// Currently uses rule-based classification; can be upgraded to ONNX later
func NewIntentClassifier(logger *zap.Logger) *IntentClassifier {
	ic := &IntentClassifier{
		logger: logger,
	}

	// Greeting patterns
	ic.greetingPatterns = compilePatterns([]string{
		`^(hi|hello|hey|yo|sup|greetings)[\s!.?]*$`,
		`^good\s+(morning|afternoon|evening|day)[\s!.?]*$`,
		`^(howdy|hiya|what'?s\s+up)[\s!.?]*$`,
		`^(bye|goodbye|see\s+you|later|cya|farewell)[\s!.?]*$`,
		`^(thanks|thank\s+you|thx|ty)[\s!.?]*$`,
	})

	// Navigation patterns
	ic.navigationPatterns = compilePatterns([]string{
		`^(go\s+to|open|show|take\s+me\s+to|navigate\s+to)\s+`,
		`^(open|show)\s+(my\s+)?(settings|profile|dashboard|groups|memory)`,
		`^(settings|profile|dashboard|home|groups)\s*$`,
	})

	// Fact retrieval patterns
	ic.factPatterns = compilePatterns([]string{
		`^what\s+(is|are|was|were)\s+(my|the)`,
		`^(tell|show)\s+me\s+(my|about)`,
		`^(list|get|fetch|find)\s+(my|all)`,
		`^who\s+(is|are|was)`,
		`^when\s+(did|was|is)`,
		`^where\s+(is|are|did)`,
		`^(do|did)\s+i\s+(have|know|like|say)`,
		`\?(my|email|name|phone|address|age|birthday)`,
	})

	return ic
}

// Classify determines the intent of a user message
func (ic *IntentClassifier) Classify(message string) Intent {
	// Normalize message
	msg := strings.ToLower(strings.TrimSpace(message))

	// Empty or very short messages
	if len(msg) < 2 {
		return IntentGreeting
	}

	// Check greeting patterns
	for _, pattern := range ic.greetingPatterns {
		if pattern.MatchString(msg) {
			return IntentGreeting
		}
	}

	// Check navigation patterns
	for _, pattern := range ic.navigationPatterns {
		if pattern.MatchString(msg) {
			return IntentNavigation
		}
	}

	// Check fact retrieval patterns
	for _, pattern := range ic.factPatterns {
		if pattern.MatchString(msg) {
			return IntentFactRetrieval
		}
	}

	// Default to complex (requires LLM)
	return IntentComplex
}

// compilePatterns compiles a list of regex patterns
func compilePatterns(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile("(?i)" + p)
		if err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}
