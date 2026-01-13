# -*- coding: utf-8 -*-
"""
File Validator for Document Upload Security

This module provides comprehensive file validation to prevent:
- Path traversal attacks
- Malicious file uploads
- DoS via large files
- File type spoofing
- Basic malware pattern detection

SECURITY: This module implements defense-in-depth for file uploads.
"""

import base64
import binascii
import logging
import os
import re
from typing import Optional, Tuple, List
from dataclasses import dataclass
from enum import Enum


# Configure logging
logger = logging.getLogger(__name__)


class ValidationError(Enum):
    """Enumeration of validation error types"""
    INVALID_BASE64 = "Invalid base64 encoding"
    FILE_TOO_LARGE = "File size exceeds maximum"
    INVALID_EXTENSION = "File extension not allowed"
    INVALID_FILENAME = "Invalid filename (potentially malicious)"
    MAGIC_MISMATCH = "File content does not match extension"
    SUSPICIOUS_CONTENT = "File contains suspicious patterns"
    EMPTY_FILE = "File is empty"


@dataclass
class ValidationResult:
    """Result of file validation"""
    valid: bool
    error_type: Optional[ValidationError] = None
    error_message: str = ""
    file_size: int = 0
    detected_extension: str = ""
    safe_filename: str = ""


class FileValidator:
    """
    Comprehensive file validator for secure document uploads.

    SECURITY FEATURES:
    1. Base64 decoding with size limits
    2. Filename sanitization (path traversal, null byte prevention)
    3. Extension whitelist
    4. Magic number verification (content-based file type detection)
    5. Suspicious content pattern detection
    6. Size limits to prevent DoS
    """

    # Configuration
    MAX_FILE_SIZE = 10 * 1024 * 1024  # 10MB
    MIN_FILE_SIZE = 100  # 100 bytes - reject suspiciously small files
    MAX_FILENAME_LENGTH = 255

    # Allowed file extensions with their magic numbers
    ALLOWED_EXTENSIONS = {
        '.txt': [b''],  # Text files have no specific magic number
        '.md': [b''],   # Markdown
        '.json': [b'{', b'['],  # JSON starts with { or [
        '.csv': [b''],  # CSV is text
        '.pdf': [b'%PDF-'],
        '.html': [b'<html', b'<HTML', b'<!DOCTYPE html'],
        '.htm': [b'<html', b'<HTML', b'<!DOCTYPE html'],
        '.xml': [b'<?xml', b'<xml'],
    }

    # Suspicious patterns that may indicate malware or exploit attempts
    SUSPICIOUS_PATTERNS = [
        # Script injection patterns
        rb'<script',
        rb'javascript:',
        rb'vbscript:',
        rb'data:text/html',

        # PowerShell/Cmd patterns (suspicious in documents)
        rb'powershell',
        rb'cmd\.exe',
        rb'/c\s+',

        # Shell patterns
        rb'/bin/',
        rb'/etc/',
        rb'curl\s+',
        rb'wget\s+',

        # Macro patterns (suspicious in office docs)
        rb'AutoOpen',
        rb'AutoClose',
        rb'Document_Open',
        rb'Workbook_Open',

        # Binary executable patterns
        rb'MZ',  # PE/Windows executable
        rb'\x7fELF',  # Linux executable
        rb'CA FE BA BE',  # Mach-O (macOS) - with spaces for readability
    ]

    # Filename sanitization patterns
    NULL_BYTE = '\x00'
    PATH_TRAVERSAL = re.compile(r'\.\.[/\\]')
    INVALID_CHARS = re.compile(r'[<>:"|?*\x00-\x1f]')

    @classmethod
    def validate_base64_content(
        cls,
        content_b64: str,
        filename: str
    ) -> ValidationResult:
        """
        Validate a base64-encoded file content.

        Args:
            content_b64: Base64-encoded file content
            filename: Original filename (for validation)

        Returns:
            ValidationResult with validation status and details
        """
        # Step 1: Validate filename first (before decoding)
        filename_result = cls.validate_filename(filename)
        if not filename_result.valid:
            return filename_result

        # Step 2: Check base64 length before decoding (prevent DoS)
        if len(content_b64) > (cls.MAX_FILE_SIZE * 4 / 3 + 100):
            logger.warning("Base64 content exceeds maximum length",
                          extra={'filename': filename})
            return ValidationResult(
                valid=False,
                error_type=ValidationError.FILE_TOO_LARGE,
                error_message=f"Base64 content is too large"
            )

        if len(content_b64) == 0:
            return ValidationResult(
                valid=False,
                error_type=ValidationError.EMPTY_FILE,
                error_message="File content is empty"
            )

        # Step 3: Decode base64 with error handling
        try:
            file_content = base64.b64decode(content_b64, validate=True)
        except binascii.Error as e:
            logger.warning("Invalid base64 encoding",
                          extra={'filename': filename, 'error': str(e)})
            return ValidationResult(
                valid=False,
                error_type=ValidationError.INVALID_BASE64,
                error_message="Invalid base64 encoding"
            )

        # Step 4: Check file size
        file_size = len(file_content)
        if file_size > cls.MAX_FILE_SIZE:
            logger.warning("File exceeds maximum size",
                          extra={'filename': filename, 'size': file_size})
            return ValidationResult(
                valid=False,
                error_type=ValidationError.FILE_TOO_LARGE,
                error_message=f"File size ({file_size} bytes) exceeds maximum ({cls.MAX_FILE_SIZE})"
            )

        if file_size < cls.MIN_FILE_SIZE:
            logger.warning("File is suspiciously small",
                          extra={'filename': filename, 'size': file_size})
            return ValidationResult(
                valid=False,
                error_type=ValidationError.EMPTY_FILE,
                error_message=f"File is too small (minimum {cls.MIN_FILE_SIZE} bytes)"
            )

        # Step 5: Validate file extension
        ext = cls._get_extension(filename)
        if ext.lower() not in cls.ALLOWED_EXTENSIONS:
            logger.warning("File extension not allowed",
                          extra={'filename': filename, 'extension': ext})
            return ValidationResult(
                valid=False,
                error_type=ValidationError.INVALID_EXTENSION,
                error_message=f"File extension '{ext}' is not allowed"
            )

        # Step 6: Magic number verification (content-based type check)
        if not cls._verify_magic_number(file_content, ext.lower()):
            logger.warning("File content does not match extension",
                          extra={'filename': filename, 'extension': ext})
            return ValidationResult(
                valid=False,
                error_type=ValidationError.MAGIC_MISMATCH,
                error_message=f"File content does not match the '{ext}' extension"
            )

        # Step 7: Check for suspicious content patterns
        suspicious = cls._check_suspicious_patterns(file_content)
        if suspicious:
            logger.warning("File contains suspicious patterns",
                          extra={'filename': filename, 'patterns': suspicious})
            return ValidationResult(
                valid=False,
                error_type=ValidationError.SUSPICIOUS_CONTENT,
                error_message=f"File contains potentially malicious content: {', '.join(suspicious[:3])}"
            )

        # All checks passed
        return ValidationResult(
            valid=True,
            file_size=file_size,
            detected_extension=ext,
            safe_filename=filename_result.safe_filename
        )

    @classmethod
    def validate_filename(cls, filename: str) -> ValidationResult:
        """
        Validate and sanitize a filename.

        Checks for:
        - Null bytes (injection attacks)
        - Path traversal (../..)
        - Invalid characters
        - Excessive length

        Args:
            filename: The filename to validate

        Returns:
            ValidationResult with safe_filename if valid
        """
        if not filename:
            return ValidationResult(
                valid=False,
                error_type=ValidationError.INVALID_FILENAME,
                error_message="Filename cannot be empty"
            )

        # Check for null bytes
        if cls.NULL_BYTE in filename:
            logger.warning("Filename contains null bytes",
                          extra={'filename': filename})
            return ValidationResult(
                valid=False,
                error_type=ValidationError.INVALID_FILENAME,
                error_message="Filename contains invalid characters"
            )

        # Check for path traversal
        if cls.PATH_TRAVERSAL.search(filename):
            logger.warning("Filename contains path traversal patterns",
                          extra={'filename': filename})
            return ValidationResult(
                valid=False,
                error_type=ValidationError.INVALID_FILENAME,
                error_message="Filename contains invalid path sequences"
            )

        # Check for invalid characters
        if cls.INVALID_CHARS.search(filename):
            logger.warning("Filename contains invalid characters",
                          extra={'filename': filename})
            return ValidationResult(
                valid=False,
                error_type=ValidationError.INVALID_FILENAME,
                error_message="Filename contains invalid characters"
            )

        # Check length
        if len(filename) > cls.MAX_FILENAME_LENGTH:
            return ValidationResult(
                valid=False,
                error_type=ValidationError.INVALID_FILENAME,
                error_message=f"Filename exceeds maximum length of {cls.MAX_FILENAME_LENGTH}"
            )

        # Extract just the basename (remove any path components)
        safe_filename = os.path.basename(filename)

        return ValidationResult(
            valid=True,
            safe_filename=safe_filename
        )

    @classmethod
    def _get_extension(cls, filename: str) -> str:
        """Extract file extension from filename."""
        _, ext = os.path.splitext(filename.lower())
        return ext

    @classmethod
    def _verify_magic_number(cls, content: bytes, extension: str) -> bool:
        """
        Verify that file content matches the expected magic number for the extension.

        Args:
            content: The file content as bytes
            extension: The file extension (e.g., '.pdf')

        Returns:
            True if content matches expected type, False otherwise
        """
        allowed_magic = cls.ALLOWED_EXTENSIONS.get(extension, [])
        if not allowed_magic:
            return False

        # Empty magic number list means "accept any content" (for text files)
        if not allowed_magic or allowed_magic == [b'']:
            return True

        # Check if content starts with any of the expected magic numbers
        for magic in allowed_magic:
            if content.startswith(magic):
                return True

        return False

    @classmethod
    def _check_suspicious_patterns(cls, content: bytes) -> List[str]:
        """
        Scan content for suspicious patterns that may indicate malware.

        Args:
            content: File content to scan

        Returns:
            List of suspicious patterns found (empty if none)
        """
        found = []
        content_lower = content.lower()

        for pattern in cls.SUSPICIOUS_PATTERNS:
            pattern_str = pattern.decode('utf-8', errors='ignore')
            if pattern in content_lower or pattern.replace(b' ', b'') in content:
                found.append(pattern_str)

        return found

    @classmethod
    def get_allowed_extensions(cls) -> List[str]:
        """Return list of allowed file extensions."""
        return list(cls.ALLOWED_EXTENSIONS.keys())

    @classmethod
    def configure(cls, **kwargs):
        """
        Configure validation parameters.

        Args:
            max_file_size: Maximum file size in bytes
            min_file_size: Minimum file size in bytes
            allowed_extensions: Dict of extension -> magic numbers
        """
        if 'max_file_size' in kwargs:
            cls.MAX_FILE_SIZE = kwargs['max_file_size']
        if 'min_file_size' in kwargs:
            cls.MIN_FILE_SIZE = kwargs['min_file_size']
        if 'allowed_extensions' in kwargs:
            cls.ALLOWED_EXTENSIONS = kwargs['allowed_extensions']
        logger.info("FileValidator configured",
                   extra={'max_size': cls.MAX_FILE_SIZE,
                         'min_size': cls.MIN_FILE_SIZE,
                         'extensions': list(cls.ALLOWED_EXTENSIONS.keys())})


# Convenience function for quick validation
def validate_upload(content_b64: str, filename: str) -> Tuple[bool, str]:
    """
    Quick validation wrapper for file uploads.

    Args:
        content_b64: Base64-encoded file content
        filename: Original filename

    Returns:
        Tuple of (is_valid, error_message)
    """
    result = FileValidator.validate_base64_content(content_b64, filename)
    if result.valid:
        return True, ""
    return False, result.error_message


# Export for use in main.py
__all__ = ['FileValidator', 'ValidationResult', 'ValidationError', 'validate_upload']
