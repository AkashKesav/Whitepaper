// Package validation provides comprehensive file validation for secure document uploads
package validation

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ValidationErrorType represents different types of validation errors
type ValidationErrorType string

const (
	ErrorInvalidBase64    ValidationErrorType = "Invalid base64 encoding"
	ErrorFileTooLarge     ValidationErrorType = "File size exceeds maximum"
	ErrorInvalidExtension ValidationErrorType = "File extension not allowed"
	ErrorInvalidFilename  ValidationErrorType = "Invalid filename (potentially malicious)"
	ErrorMagicMismatch    ValidationErrorType = "File content does not match extension"
	ErrorSuspiciousContent ValidationErrorType = "File contains suspicious patterns"
	ErrorEmptyFile        ValidationErrorType = "File is empty"
)

// ValidationResult represents the result of file validation
type ValidationResult struct {
	Valid             bool                 `json:"valid"`
	ErrorType         ValidationErrorType   `json:"error_type,omitempty"`
	ErrorMessage      string               `json:"error_message,omitempty"`
	FileSize          int                  `json:"file_size,omitempty"`
	DetectedExtension string               `json:"detected_extension,omitempty"`
	SafeFilename      string               `json:"safe_filename,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// FileValidator provides comprehensive file validation for secure uploads
type FileValidator struct {
	maxFileSize        int
	minFileSize        int
	maxFilenameLength  int
	allowedExtensions  map[string][][]byte
	suspiciousPatterns [][]byte
	pathTraversalRegex  *regexp.Regexp
	invalidCharsRegex    *regexp.Regexp
}

// DefaultConfig returns a validator with default security settings
func DefaultConfig() *FileValidator {
	return &FileValidator{
		maxFileSize:       10 * 1024 * 1024, // 10MB
		minFileSize:       100,               // 100 bytes
		maxFilenameLength: 255,
		allowedExtensions: map[string][][]byte{
			".txt":  {}, // Text files - no magic number
			".md":   {}, // Markdown
			".json": {{byte('{'), byte('[')}},
			".csv":  {}, // CSV
			".pdf":  {{byte('%'), byte('P'), byte('D'), byte('F')}},
			".html": {{byte('<'), byte('h'), byte('t'), byte('m'), byte('l')}, {byte('<'), byte('H'), byte('T'), byte('M'), byte('L')}, {byte('<'), byte('!'), byte('D'), byte('O'), byte('C'), byte('T'), byte('Y'), byte('P'), byte('E'), byte(' ')}},
			".htm":  {{byte('<'), byte('h'), byte('t'), byte('m'), byte('l')}, {byte('<'), byte('H'), byte('T'), byte('M'), byte('L')}},
			".xml":  {{byte('<'), byte('?'), byte('x'), byte('m'), byte('l')}, {byte('<'), byte('x'), byte('m'), byte('l')}},
		},
		suspiciousPatterns: [][]byte{
			// Script injection patterns
			[]byte("<script"),
			[]byte("javascript:"),
			[]byte("vbscript:"),
			[]byte("data:text/html"),

			// PowerShell/Cmd patterns
			[]byte("powershell"),
			[]byte("cmd.exe"),
			[]byte("/c "),
			[]byte("\\c "),

			// Shell patterns
			[]byte("/bin/"),
			[]byte("/etc/"),
			[]byte("curl "),
			[]byte("wget "),

			// Macro patterns
			[]byte("AutoOpen"),
			[]byte("AutoClose"),
			[]byte("Document_Open"),
			[]byte("Workbook_Open"),

			// Binary executable patterns
			[]byte("MZ"),       // PE/Windows executable
			{0x7f, 'E', 'L', 'F'}, // Linux executable
			{'C', 'A', 0xFE, ' ', 'B', 'A', ' ', 'B', 'E'}, // Mach-O
		},
		pathTraversalRegex:  regexp.MustCompile(`\.\.[\/\\]`),
		invalidCharsRegex:    regexp.MustCompile(`[<>:"|?*\x00-\x1f]`),
	}
}

// New creates a new file validator with custom configuration
func New(maxSize, minSize, maxFilenameLen int) *FileValidator {
	return &FileValidator{
		maxFileSize:       maxSize,
		minFileSize:       minSize,
		maxFilenameLength: maxFilenameLen,
		allowedExtensions: make(map[string][][]byte),
		suspiciousPatterns: [][]byte{
			[]byte("<script"),
			[]byte("javascript:"),
			[]byte("powershell"),
			[]byte("cmd.exe"),
		},
		pathTraversalRegex: regexp.MustCompile(`\.\.[\/\\]`),
		invalidCharsRegex:   regexp.MustCompile(`[<>:"|?*\x00-\x1f]`),
	}
}

// ValidateBase64Content validates a base64-encoded file content
func (v *FileValidator) ValidateBase64Content(contentB64, filename string) ValidationResult {
	// Step 1: Validate filename first
	filenameResult := v.ValidateFilename(filename)
	if !filenameResult.Valid {
		return filenameResult
	}

	// Step 2: Check base64 length before decoding
	maxB64Length := (v.maxFileSize * 4 / 3) + 100
	if len(contentB64) > maxB64Length {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorFileTooLarge,
			ErrorMessage: "Base64 content is too large",
		}
	}

	if len(contentB64) == 0 {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorEmptyFile,
			ErrorMessage: "File content is empty",
		}
	}

	// Step 3: Decode base64
	fileContent, err := base64.StdEncoding.DecodeString(contentB64)
	if err != nil {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorInvalidBase64,
			ErrorMessage: "Invalid base64 encoding: " + err.Error(),
		}
	}

	// Step 4: Check file size
	fileSize := len(fileContent)
	if fileSize > v.maxFileSize {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorFileTooLarge,
			ErrorMessage: formatString("File size (%d bytes) exceeds maximum (%d)", fileSize, v.maxFileSize),
		}
	}

	if fileSize < v.minFileSize {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorEmptyFile,
			ErrorMessage: formatString("File is too small (minimum %d bytes)", v.minFileSize),
		}
	}

	// Step 5: Validate file extension
	ext := v.getExtension(filename)
	ext = strings.ToLower(ext)
	if _, ok := v.allowedExtensions[ext]; !ok {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorInvalidExtension,
			ErrorMessage: formatString("File extension '%s' is not allowed", ext),
		}
	}

	// Step 6: Magic number verification
	if !v.verifyMagicNumber(fileContent, ext) {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorMagicMismatch,
			ErrorMessage: formatString("File content does not match the '%s' extension", ext),
		}
	}

	// Step 7: Check for suspicious content patterns
	suspicious := v.checkSuspiciousPatterns(fileContent)
	if len(suspicious) > 0 {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorSuspiciousContent,
			ErrorMessage: formatString("File contains potentially malicious content: %s", strings.Join(suspicious[:3], ", ")),
		}
	}

	// All checks passed
	return ValidationResult{
		Valid:             true,
		FileSize:          fileSize,
		DetectedExtension: ext,
		SafeFilename:      filenameResult.SafeFilename,
		Metadata:          map[string]interface{}{"content_validated": true},
	}
}

// ValidateFilename validates and sanitizes a filename
func (v *FileValidator) ValidateFilename(filename string) ValidationResult {
	if filename == "" {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorInvalidFilename,
			ErrorMessage: "Filename cannot be empty",
		}
	}

	// Check for null bytes
	if strings.Contains(filename, "\x00") {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorInvalidFilename,
			ErrorMessage: "Filename contains null bytes",
		}
	}

	// Check for path traversal
	if v.pathTraversalRegex.MatchString(filename) {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorInvalidFilename,
			ErrorMessage: "Filename contains invalid path sequences",
		}
	}

	// Check for invalid characters
	if v.invalidCharsRegex.MatchString(filename) {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorInvalidFilename,
			ErrorMessage: "Filename contains invalid characters",
		}
	}

	// Check length
	if len(filename) > v.maxFilenameLength {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorInvalidFilename,
			ErrorMessage: formatString("Filename exceeds maximum length of %d", v.maxFilenameLength),
		}
	}

	// Extract just the basename
	safeFilename := filepath.Base(filename)

	// Additional sanitization: remove control characters
	safeFilename = removeControlChars(safeFilename)

	return ValidationResult{
		Valid:        true,
		SafeFilename: safeFilename,
	}
}

// ValidateBytes validates raw file bytes
func (v *FileValidator) ValidateBytes(content []byte, filename string) ValidationResult {
	// Validate filename first
	filenameResult := v.ValidateFilename(filename)
	if !filenameResult.Valid {
		return filenameResult
	}

	fileSize := len(content)

	// Check file size
	if fileSize > v.maxFileSize {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorFileTooLarge,
			ErrorMessage: formatString("File size (%d bytes) exceeds maximum (%d)", fileSize, v.maxFileSize),
		}
	}

	if fileSize < v.minFileSize {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorEmptyFile,
			ErrorMessage: formatString("File is too small (minimum %d bytes)", v.minFileSize),
		}
	}

	// Validate extension
	ext := v.getExtension(filename)
	ext = strings.ToLower(ext)
	if _, ok := v.allowedExtensions[ext]; !ok {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorInvalidExtension,
			ErrorMessage: formatString("File extension '%s' is not allowed", ext),
		}
	}

	// Magic number verification
	if !v.verifyMagicNumber(content, ext) {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorMagicMismatch,
			ErrorMessage: formatString("File content does not match the '%s' extension", ext),
		}
	}

	// Check for suspicious patterns
	suspicious := v.checkSuspiciousPatterns(content)
	if len(suspicious) > 0 {
		return ValidationResult{
			Valid:        false,
			ErrorType:    ErrorSuspiciousContent,
			ErrorMessage: formatString("File contains potentially malicious content: %s", strings.Join(suspicious[:3], ", ")),
		}
	}

	return ValidationResult{
		Valid:             true,
		FileSize:          fileSize,
		DetectedExtension: ext,
		SafeFilename:      filenameResult.SafeFilename,
	}
}

// getExtension extracts file extension from filename
func (v *FileValidator) getExtension(filename string) string {
	ext := filepath.Ext(filename)
	return ext
}

// verifyMagicNumber checks if content matches the expected magic number
func (v *FileValidator) verifyMagicNumber(content []byte, extension string) bool {
	allowedMagic, ok := v.allowedExtensions[extension]
	if !ok {
		return false
	}

	// Empty magic number list means "accept any content"
	if len(allowedMagic) == 0 {
		return true
	}

	// Check if content starts with any expected magic number
	for _, magic := range allowedMagic {
		if bytes.HasPrefix(content, magic) {
			return true
		}
	}

	return false
}

// checkSuspiciousPatterns scans content for suspicious patterns
func (v *FileValidator) checkSuspiciousPatterns(content []byte) []string {
	contentLower := bytes.ToLower(content)
	found := make([]string, 0)

	for _, pattern := range v.suspiciousPatterns {
		if bytes.Contains(contentLower, pattern) || bytes.Contains(content, pattern) {
			found = append(found, string(pattern))
		}
	}

	return found
}

// AddAllowedExtension adds an allowed file extension with its magic number
func (v *FileValidator) AddAllowedExtension(ext string, magicNumbers ...[]byte) {
	ext = strings.ToLower(ext)
	if ext == "" {
		return
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	if len(magicNumbers) > 0 && magicNumbers[0] != nil {
		v.allowedExtensions[ext] = magicNumbers
	} else {
		v.allowedExtensions[ext] = [][]byte{}
	}
}

// RemoveAllowedExtension removes an allowed file extension
func (v *FileValidator) RemoveAllowedExtension(ext string) {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	delete(v.allowedExtensions, ext)
}

// AddSuspiciousPattern adds a suspicious pattern to detect
func (v *FileValidator) AddSuspiciousPattern(pattern string) {
	v.suspiciousPatterns = append(v.suspiciousPatterns, []byte(pattern))
}

// SetSizeLimits sets custom size limits
func (v *FileValidator) SetSizeLimits(minSize, maxSize int) {
	v.minFileSize = minSize
	v.maxFileSize = maxSize
}

// GetAllowedExtensions returns list of allowed extensions
func (v *FileValidator) GetAllowedExtensions() []string {
	exts := make([]string, 0, len(v.allowedExtensions))
	for ext := range v.allowedExtensions {
		exts = append(exts, ext)
	}
	return exts
}

// removeControlChars removes control characters from a string
func removeControlChars(s string) string {
	var builder strings.Builder
	for _, r := range s {
		if !unicode.IsControl(r) || r == '\t' || r == '\n' || r == '\r' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

// formatString formats a string with arguments
func formatString(format string, args ...interface{}) string {
	var builder strings.Builder
	currentArg := 0

	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) {
			next := format[i+1]
			if next == 'd' || next == 's' || next == 'v' {
				if currentArg < len(args) {
					switch val := args[currentArg].(type) {
					case int:
						builder.WriteString(strconv.Itoa(val))
					case string:
						builder.WriteString(val)
					default:
						builder.WriteString(fmt.Sprintf("%v", val))
					}
					currentArg++
					i++ // Skip the format specifier
					continue
				}
			}
		}
		builder.WriteByte(format[i])
	}

	return builder.String()
}

// ValidateUpload is a convenience function for quick validation
func ValidateUpload(contentB64, filename string) (bool, string) {
	validator := DefaultConfig()
	result := validator.ValidateBase64Content(contentB64, filename)
	if result.Valid {
		return true, ""
	}
	return false, result.ErrorMessage
}

// SanitizeFilename creates a safe filename from an input string
func SanitizeFilename(filename string) string {
	// Get base name
	filename = filepath.Base(filename)

	// Remove extension
	ext := filepath.Ext(filename)
	name := filename[:len(filename)-len(ext)]

	// Replace invalid chars with underscore
	var builder strings.Builder
	for _, r := range name {
		if r == '.' || r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		} else if unicode.IsSpace(r) {
			builder.WriteRune('_')
		}
	}

	sanitized := builder.String()

	// Ensure filename is not empty
	if sanitized == "" {
		sanitized = "file"
	}

	// Re-add extension if it was valid
	if ext != "" {
		sanitized += ext
	}

	return sanitized
}

// IsTextFile checks if the content appears to be a text file
func IsTextFile(content []byte) bool {
	if len(content) == 0 {
		return true
	}

	// Check if content is mostly printable ASCII
	printableCount := 0
	for _, b := range content {
		if b >= 32 && b <= 126 {
			printableCount++
		} else if b == '\t' || b == '\n' || b == '\r' {
			printableCount++
		}
	}

	if float64(printableCount)/float64(len(content)) > 0.9 {
		return true
	}

	return false
}

// GetMimeType returns the MIME type for a given file extension
func GetMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	mimeTypes := map[string]string{
		".txt":  "text/plain",
		".md":   "text/markdown",
		".json": "application/json",
		".csv":  "text/csv",
		".pdf":  "application/pdf",
		".html": "text/html",
		".htm":  "text/html",
		".xml":  "application/xml",
		".zip":  "application/zip",
		".gz":   "application/gzip",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}

	return "application/octet-stream"
}

// GetFileCategory returns the category of a file (document, image, archive, etc.)
func GetFileCategory(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	documentExts := map[string]bool{
		".txt": true, ".md": true, ".json": true, ".csv": true,
		".pdf": true, ".html": true, ".htm": true, ".xml": true,
		".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".ppt": true, ".pptx": true,
	}

	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".bmp": true, ".svg": true, ".webp": true,
	}

	archiveExts := map[string]bool{
		".zip": true, ".gz": true, ".tar": true, ".rar": true,
		".7z": true, ".bz2": true,
	}

	if documentExts[ext] {
		return "document"
	}
	if imageExts[ext] {
		return "image"
	}
	if archiveExts[ext] {
		return "archive"
	}

	return "unknown"
}
