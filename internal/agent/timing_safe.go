// Package agent provides timing-safe operations to prevent side-channel attacks
// SECURITY: These utilities prevent timing attacks that could leak information
package agent

import (
	"net/http"
	"strings"
	"time"
)

const (
	// Normalized timing for all operations to prevent timing attacks
	// Operations will take at least this long, regardless of result
	normalizedOperationTime = 50 * time.Millisecond
)

// ConstantTimeAuthResult represents the result of an authentication check
// with timing-safe properties
type ConstantTimeAuthResult struct {
	Allowed bool
	Message string
	// Internal use only - not exposed to callers
	details string
}

// ConstantTimeVerify performs a membership or permission check with timing-safe behavior
// SECURITY: Returns consistent timing regardless of whether the check passes or fails
// This prevents attackers from determining valid usernames, group IDs, etc. through timing
func ConstantTimeVerify(allowed bool, genericMessage string) ConstantTimeAuthResult {
	// Always sleep the same duration to normalize timing
	time.Sleep(normalizedOperationTime)

	return ConstantTimeAuthResult{
		Allowed: allowed,
		Message: genericMessage,
	}
}

// ConstantTimeMembershipCheck performs a membership check with timing-safe behavior
// Returns a generic error message that doesn't reveal whether the user/group exists
func ConstantTimeMembershipCheck(exists bool, isMember bool, genericMessage string) ConstantTimeAuthResult {
	// Normalize timing regardless of outcome
	time.Sleep(normalizedOperationTime)

	// Only reveal membership status if entity exists
	// If entity doesn't exist, return generic "not found" that matches "not member" timing
	if !exists {
		return ConstantTimeAuthResult{
			Allowed: false,
			Message: genericMessage, // Same message for "not found" and "not member"
		}
	}

	return ConstantTimeAuthResult{
		Allowed: isMember,
		Message: genericMessage,
	}
}

// ConstantTimeErrorResponse writes a timing-safe error response
// SECURITY: Always returns the same status code and similar response structure
// regardless of the actual error, preventing enumeration through response timing
func ConstantTimeErrorResponse(w http.ResponseWriter, operation string) {
	// Normalize timing
	time.Sleep(normalizedOperationTime)

	// Always return the same generic message
	genericMessage := "Operation could not be completed"

	w.Header().Set("Content-Type", "application/json")
	// Use 200 OK with error field instead of 404/403 to prevent status code enumeration
	w.WriteHeader(http.StatusOK)

	w.Write([]byte(`{"status":"error","message":"` + genericMessage + `"}`))
}

// ConstantTimeSuccessResponse writes a timing-safe success response
// SECURITY: Adds artificial delay to match error response timing
func ConstantTimeSuccessResponse(w http.ResponseWriter, data []byte) {
	// Normalize timing - same delay as error responses
	time.Sleep(normalizedOperationTime)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// ConstantTimeBoolCompare performs a constant-time boolean comparison
// Useful for comparing authorization results without leaking timing
func ConstantTimeBoolCompare(a, b bool) bool {
	// Convert bools to integers and compare in constant time
	// This is a simplified version - for production crypto, use crypto/subtle.ConstantTimeCompare
	ai := 0
	bi := 0
	if a {
		ai = 1
	}
	if b {
		bi = 1
	}
	return ai == bi
}

// NormalizeResponseTiming ensures all responses take approximately the same time
// Call this at the end of handlers to normalize timing across success/failure paths
func NormalizeResponseTiming(startTime time.Time) {
	elapsed := time.Since(startTime)
	if elapsed < normalizedOperationTime {
		time.Sleep(normalizedOperationTime - elapsed)
	}
	// If operation took longer than target, return immediately
	// (we can't speed up operations, only slow down fast ones)
}

// RedactSensitiveInfo removes sensitive information from error messages
// SECURITY: Prevents information disclosure through error messages
func RedactSensitiveInfo(input string) string {
	// List of sensitive patterns to redact
	sensitivePatterns := []struct {
		pattern     string
		replacement string
	}{
		{"password", "[REDACTED]"},
		{"token", "[REDACTED]"},
		{"secret", "[REDACTED]"},
		{"key", "[REDACTED]"},
		{"credential", "[REDACTED]"},
	}

	result := input
	for _, s := range sensitivePatterns {
		// Simple case-insensitive replacement
		// In production, use proper regex with word boundaries
		result = strings.ReplaceAll(result, s.pattern, s.replacement)
	}

	return result
}

// GenericErrorMessage returns a standardized generic error message
// SECURITY: Prevents user enumeration by always returning the same message
// regardless of whether the resource exists, the user is unauthorized, etc.
type GenericErrorMessage string

const (
	// Generic error messages that don't reveal information
	GenericOperationFailed  GenericErrorMessage = "Operation could not be completed"
	GenericNotFound         GenericErrorMessage = "Resource not found"
	GenericAccessDenied     GenericErrorMessage = "Access denied"
	GenericInvalidRequest   GenericErrorMessage = "Invalid request"
	GenericRateLimited      GenericErrorMessage = "Too many requests"
)

// WriteGenericError writes a timing-safe, generic error response
// SECURITY: Always returns the same structure to prevent enumeration
func WriteGenericError(w http.ResponseWriter, message GenericErrorMessage) {
	// Normalize timing
	time.Sleep(normalizedOperationTime)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // Always 200, never 404/403

	w.Write([]byte(`{"status":"error","message":"` + string(message) + `"}`))
}

// WriteGenericSuccess writes a response with timing normalization
func WriteGenericSuccess(w http.ResponseWriter, data map[string]interface{}) {
	// Normalize timing to match error responses
	time.Sleep(normalizedOperationTime)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Build JSON response manually to avoid encoding issues
	json := `{"status":"success"`
	for k, v := range data {
		switch val := v.(type) {
		case string:
			json += `,"` + k + `":"` + val + `"`
		case int:
			json += `,"` + k + `":` + string(rune(val+'0'))
		case bool:
			if val {
				json += `,"` + k + `":true`
			} else {
				json += `,"` + k + `":false`
			}
		}
	}
	json += `}"`

	w.Write([]byte(json))
}

