// Package kernel provides error message sanitization for security
// SECURITY: Prevents information disclosure through error messages and logs
package kernel

import (
	"fmt"
	"regexp"
	"strings"
)

// Sanitization patterns for sensitive information
var (
	// Patterns that might reveal sensitive data
	sensitivePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)password\s*[:=]\s*\S+`),
		regexp.MustCompile(`(?i)token\s*[:=]\s*\S+`),
		regexp.MustCompile(`(?i)secret\s*[:=]\s*\S+`),
		regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*\S+`),
		regexp.MustCompile(`(?i)credential\s*[:=]\s*\S+`),
		regexp.MustCompile(`(?i)auth[_-]?token\s*[:=]\s*\S+`),
		regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/]+=*`),
		regexp.MustCompile(`(?i)session[_-]?id\s*[:=]\s*\S+`),
		regexp.MustCompile(`(?i)jwt\s*[:=]\s*\S+`),
		// UUID-like patterns (might be internal IDs)
		regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`),
		// Email addresses
		regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
		// IP addresses
		regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),
		// File paths (might reveal system structure)
		regexp.MustCompile(`[/\\][a-zA-Z0-9_\-./\\]+`),
	}

	// Stack trace patterns
	stackTracePatterns = []*regexp.Regexp{
		regexp.MustCompile(`goroutine \d+`),
		regexp.MustCompile(`created by .*\.go:\d+`),
		regexp.MustCompile(`\.go:\d+`),
		regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`),
	}
)

// SanitizeError removes sensitive information from error messages
// SECURITY: Prevents information disclosure through error messages
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}

	return SanitizeString(err.Error())
}

// SanitizeString removes sensitive information from a string
// SECURITY: Prevents information disclosure through logs and responses
func SanitizeString(input string) string {
	if input == "" {
		return ""
	}

	result := input

	// Remove sensitive patterns
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}

	// Remove stack traces (often included in error messages)
	for _, pattern := range stackTracePatterns {
		result = pattern.ReplaceAllString(result, "")
	}

	// Clean up any resulting whitespace issues
	result = strings.TrimSpace(result)
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")

	return result
}

// SanitizeLogEntry sanitizes a log entry for safe logging
// Removes stack traces and sensitive data while preserving useful information
func SanitizeLogEntry(format string, args ...interface{}) string {
	// Build the full message
	message := fmt.Sprintf(format, args...)
	return SanitizeString(message)
}

// SafeError creates a safe error message that doesn't reveal sensitive information
// Use this when returning errors to API clients
func SafeError(operation string) string {
	return fmt.Sprintf("%s failed", operation)
}

// SafeErrorWithID creates a safe error message with an error reference ID
// The ID can be used internally to look up the actual error
func SafeErrorWithID(operation string, errorID string) string {
	return fmt.Sprintf("%s failed (ref: %s)", operation, errorID)
}

// IsSafeToExpose checks if an error message is safe to expose to clients
// Returns false if the error contains sensitive patterns
func IsSafeToExpose(err error) bool {
	if err == nil {
		return true
	}

	message := err.Error()

	// Check for sensitive patterns
	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(message) {
			return false
		}
	}

	// Check for stack trace indicators
	for _, pattern := range stackTracePatterns {
		if pattern.MatchString(message) {
			return false
		}
	}

	// Check for common internal error patterns
	dangerousPhrases := []string{
		"internal error",
		"panic",
		"fatal",
		"stack trace",
		"unexpected",
	}

	lowerMessage := strings.ToLower(message)
	for _, phrase := range dangerousPhrases {
		if strings.Contains(lowerMessage, phrase) {
			return false
		}
	}

	return true
}

// RedactUser removes user identifiers from error messages
// SECURITY: Prevents user enumeration through error messages
func RedactUser(message string, userID string) string {
	if userID == "" {
		return message
	}

	// Replace exact user ID
	result := strings.ReplaceAll(message, userID, "[USER]")

	// Replace case-insensitive matches
	lowerMessage := strings.ToLower(result)
	lowerUserID := strings.ToLower(userID)
	if strings.Contains(lowerMessage, lowerUserID) {
		// Use regex for case-insensitive replacement
		re := regexp.MustCompile(`(?i)`+regexp.QuoteMeta(userID))
		result = re.ReplaceAllString(result, "[USER]")
	}

	return result
}

// RedactNamespace removes namespace identifiers from error messages
// SECURITY: Prevents namespace enumeration through error messages
func RedactNamespace(message string, namespace string) string {
	if namespace == "" {
		return message
	}

	// Replace exact namespace
	result := strings.ReplaceAll(message, namespace, "[NAMESPACE]")

	// Case-insensitive replacement
	re := regexp.MustCompile(`(?i)`+regexp.QuoteMeta(namespace))
	result = re.ReplaceAllString(result, "[NAMESPACE]")

	return result
}

// GenerateErrorID generates a unique error reference ID
// This can be used to track errors internally without exposing details to clients
func GenerateErrorID() string {
	// Simple ID generation - in production use UUID or similar
	return fmt.Sprintf("ERR_%d", uint64(12345)) // Placeholder
}
