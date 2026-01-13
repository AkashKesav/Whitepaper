// Package agent provides file security validation for uploads.
package agent

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// FileTypeConfig defines validation rules for a specific file type
type FileTypeConfig struct {
	Extensions     []string
	MagicNumbers   [][]byte
	MaxContentSize int64
}

// Default file type configurations with magic number validation
var defaultFileTypes = map[string]FileTypeConfig{
	"pdf": {
		Extensions:     []string{".pdf"},
		MagicNumbers:   [][]byte{{0x25, 0x50, 0x44, 0x46}}, // %PDF
		MaxContentSize: 100 * 1024 * 1024,                  // 100MB
	},
	"txt": {
		Extensions:     []string{".txt"},
		MagicNumbers:   nil, // Text files have no magic number
		MaxContentSize: 10 * 1024 * 1024, // 10MB
	},
	"md": {
		Extensions:     []string{".md"},
		MagicNumbers:   nil,
		MaxContentSize: 10 * 1024 * 1024,
	},
	"json": {
		Extensions:     []string{".json"},
		MagicNumbers:   nil, // JSON starts with optional BOM then { or [
		MaxContentSize: 10 * 1024 * 1024,
	},
	"csv": {
		Extensions:     []string{".csv"},
		MagicNumbers:   nil,
		MaxContentSize: 50 * 1024 * 1024,
	},
	"doc": {
		Extensions:     []string{".doc"},
		MagicNumbers:   [][]byte{{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}}, // OLE2 header
		MaxContentSize: 50 * 1024 * 1024,
	},
	"docx": {
		Extensions:     []string{".docx"},
		MagicNumbers:   [][]byte{{0x50, 0x4B, 0x03, 0x04}}, // ZIP header (DOCX is ZIP)
		MaxContentSize: 50 * 1024 * 1024,
	},
	"rtf": {
		Extensions:     []string{".rtf"},
		MagicNumbers:   [][]byte{{0x7B, 0x5C, 0x72, 0x74, 0x66}}, // {\rtf
		MaxContentSize: 20 * 1024 * 1024,
	},
	"html": {
		Extensions:     []string{".html"},
		MagicNumbers:   nil, // HTML varies too much
		MaxContentSize: 10 * 1024 * 1024,
	},
	"htm": {
		Extensions:     []string{".htm"},
		MagicNumbers:   nil,
		MaxContentSize: 10 * 1024 * 1024,
	},
}

// FileValidator provides comprehensive file validation
type FileValidator struct {
	maxFileSize    int64
	allowedTypes   map[string]FileTypeConfig
	scannerEnabled bool
}

// NewFileValidator creates a new file validator with specified limits
func NewFileValidator(maxFileSize int64, scannerEnabled bool) *FileValidator {
	return &FileValidator{
		maxFileSize:    maxFileSize,
		allowedTypes:   defaultFileTypes,
		scannerEnabled: scannerEnabled,
	}
}

// ValidateFilename performs comprehensive filename validation
func (v *FileValidator) ValidateFilename(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename is empty")
	}

	// Check for path traversal attacks (basic sequences)
	if strings.Contains(filename, "..") {
		return fmt.Errorf("filename contains path traversal sequence")
	}

	// Check for path separators
	if strings.ContainsAny(filename, "/\\:") {
		return fmt.Errorf("filename contains path separators")
	}

	// Check for null bytes
	if bytes.Contains([]byte(filename), []byte{0}) {
		return fmt.Errorf("filename contains null byte")
	}

	// Check for control characters (except tab)
	for i, r := range filename {
		if r == 0 {
			return fmt.Errorf("filename contains null byte at position %d", i)
		}
		// Check for control characters (excluding tab which is sometimes used in filenames)
		if r < 32 && r != '\t' {
			return fmt.Errorf("filename contains control character at position %d", i)
		}
		// Check for bidirectional override characters (Unicode homograph attack prevention)
		if r == '\u202E' || r == '\u202A' || r == '\u202B' || r == '\u202D' || r == '\u2066' || r == '\u2067' {
			return fmt.Errorf("filename contains bidirectional override character (possible homograph attack)")
		}
		// Check for non-printable characters (except space)
		if !unicode.IsPrint(r) && r != ' ' {
			return fmt.Errorf("filename contains non-printable character at position %d", i)
		}
	}

	// Check for suspicious patterns
	lowerFilename := strings.ToLower(filename)
	suspiciousPatterns := []string{
		"con.", "prn.", "aux.", "nul.", // Windows reserved device names
		"com1.", "com2.", "com3.", "com4.", "com5.", "com6.", "com7.", "com8.", "com9.",
		"lpt1.", "lpt2.", "lpt3.", "lpt4.", "lpt5.", "lpt6.", "lpt7.", "lpt8.", "lpt9.",
	}
	for _, pattern := range suspiciousPatterns {
		if strings.HasPrefix(lowerFilename, pattern) {
			return fmt.Errorf("filename matches Windows reserved device name pattern: %s", pattern)
		}
	}

	// Check filename length
	if len(filename) > 255 {
		return fmt.Errorf("filename too long (max 255 characters)")
	}

	// Check for leading/trailing dots and spaces (problematic on Windows)
	if strings.HasPrefix(filename, ".") || strings.HasSuffix(filename, ".") {
		return fmt.Errorf("filename cannot start or end with a period")
	}
	if len(filename) != len(strings.Trim(filename, " ")) {
		return fmt.Errorf("filename cannot start or end with spaces")
	}

	return nil
}

// ValidateFileContent validates file content against declared type using magic numbers
func (v *FileValidator) ValidateFileContent(content []byte, filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))

	// Find matching file type
	var matchedType *FileTypeConfig
	for _, fileType := range v.allowedTypes {
		for _, allowedExt := range fileType.Extensions {
			if ext == allowedExt {
				matchedType = &fileType
				break
			}
		}
		if matchedType != nil {
			break
		}
	}

	if matchedType == nil {
		return fmt.Errorf("unknown file type for extension '%s'", ext)
	}

	// If magic numbers are defined, verify them
	if len(matchedType.MagicNumbers) > 0 {
		if len(content) < 4 {
			return fmt.Errorf("file too small to validate content type")
		}

		valid := false
		for _, magic := range matchedType.MagicNumbers {
			if len(content) >= len(magic) && bytes.Equal(content[:len(magic)], magic) {
				valid = true
				break
			}
		}

		if !valid {
			return fmt.Errorf("file content does not match extension '%s' (magic number mismatch)", ext)
		}
	}

	return nil
}

// ScanForMalware performs basic heuristic security scanning
func (v *FileValidator) ScanForMalware(content []byte, filename string) error {
	if !v.scannerEnabled {
		return nil
	}

	// Convert to lowercase for case-insensitive matching
	contentLower := bytes.ToLower(content)

	// Check for embedded scripts (XSS potential)
	scriptPatterns := [][]byte{
		[]byte("<script"),
		[]byte("javascript:"),
		[]byte("vbscript:"),
		[]byte("onload="),
		[]byte("onerror="),
		[]byte("onclick="),
	}
	ext := strings.ToLower(filepath.Ext(filename))

	// For non-HTML files, script tags are suspicious
	skipScriptCheck := ext == ".html" || ext == ".htm"
	if !skipScriptCheck {
		for _, pattern := range scriptPatterns {
			if bytes.Contains(contentLower, pattern) {
				return fmt.Errorf("suspicious content: script tag or event handler detected")
			}
		}
	}

	// Check for executable file markers
	executableMarkers := [][]byte{
		[]byte("MZ"),              // Windows executable
		[]byte("\x7fELF"),         // Linux executable
		[]byte("PE\x00\x00"),      // Portable Executable
		[]byte("\xfe\xed\xfa"),    // Mach-O (macOS)
		[]byte("\xca\xfe\xba\xbe"), // Mach-O universal binary
	}

	for _, marker := range executableMarkers {
		if bytes.Contains(content, marker) {
			return fmt.Errorf("suspicious content: executable file signature detected")
		}
	}

	// Check for PowerShell scripts
	if bytes.Contains(contentLower, []byte("powershell")) || bytes.Contains(contentLower, []byte("invoke-expression")) {
		return fmt.Errorf("suspicious content: PowerShell command detected")
	}

	// Check for shell script patterns (in non-script files)
	shellPatterns := [][]byte{
		[]byte("#!/bin/"),
		[]byte("#!/usr/bin/"),
		[]byte("eval("),
		[]byte("system("),
		[]byte("exec("),
	}

	skipShellCheck := ext == ".txt" || ext == ".md" || ext == ".json" || ext == ".csv" || ext == ".html" || ext == ".htm" || ext == ".rtf"
	if !skipShellCheck {
		for _, pattern := range shellPatterns {
			if bytes.Contains(content, pattern) {
				return fmt.Errorf("suspicious content: shell script pattern detected")
			}
		}
	}

	// Check for suspicious long strings (potential payload obfuscation)
	words := bytes.Fields(content)
	for _, word := range words {
		if len(word) > 1000 {
			return fmt.Errorf("suspicious content: unusually long string detected (possible obfuscation)")
		}
	}

	return nil
}

// IsAllowedExtension checks if the file extension is allowed
func (v *FileValidator) IsAllowedExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))

	for _, fileType := range v.allowedTypes {
		for _, allowedExt := range fileType.Extensions {
			if ext == allowedExt {
				return true
			}
		}
	}

	return false
}

// ValidateFileSize checks if the file size is within allowed limits
func (v *FileValidator) ValidateFileSize(size int64) error {
	if size == 0 {
		return fmt.Errorf("file is empty")
	}
	if size > v.maxFileSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size of %d bytes", size, v.maxFileSize)
	}
	return nil
}

// GetMaxSizeForType returns the maximum size for a specific file type
func (v *FileValidator) GetMaxSizeForType(filename string) int64 {
	ext := strings.ToLower(filepath.Ext(filename))

	for _, fileType := range v.allowedTypes {
		for _, allowedExt := range fileType.Extensions {
			if ext == allowedExt {
				return fileType.MaxContentSize
			}
		}
	}

	return v.maxFileSize // Default max
}
