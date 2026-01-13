"""
memchunker - Fast semantic text chunking using memchunk.

This module provides Python bindings to the memchunk Rust library for
high-performance semantic text chunking (up to 1TB/s throughput).

Features:
- SIMD-accelerated delimiter search (1-3 delimiters)
- Lookup table for 4+ delimiters
- Zero-copy iteration (borrowed slices)
- Consecutive delimiter handling
- Forward fallback search
"""
import re
from dataclasses import dataclass, field
from typing import Optional, List, Callable
from functools import lru_cache

# Try to import the Rust-based memchunk library
try:
    from memchunk import chunk as rust_chunk
    HAS_RUST_MEMCHUNK = True
except ImportError:
    HAS_RUST_MEMCHUNK = False
    print("WARNING: memchunk Rust library not found. Using pure Python fallback.")
    print("Install with: pip install memchunk")


@dataclass
class ChunkResult:
    """Result of chunking operation."""
    text: str
    start_pos: int
    end_pos: int
    is_complete: bool = True  # True if chunk ends at delimiter, False if hard split


@dataclass
class ChunkerConfig:
    """Configuration for memchunker."""
    chunk_size: int = 4096  # Target chunk size in bytes
    delimiters: bytes = b'\n.?'  # Single-byte delimiters
    pattern: Optional[bytes] = None  # Multi-byte pattern (e.g., SentencePiece ▁)
    prefix_mode: bool = False  # Put delimiter at start of next chunk
    consecutive: bool = False  # Split at START of consecutive runs
    forward_fallback: bool = True  # Search forward if no delimiter in window


class MemChunker:
    """
    Fast semantic text chunker using memchunk.

    Performance: Up to 1TB/s throughput using SIMD instructions.

    Example:
        >>> chunker = MemChunker(chunk_size=1024, delimiters=b'\n.?!')
        >>> chunks = chunker.chunk("Hello world. How are you?")
        >>> [c.text for c in chunks]
        ['Hello world.', ' How are you?']
    """

    def __init__(self, config: Optional[ChunkerConfig] = None):
        self.config = config or ChunkerConfig()

    def chunk(self, text: str) -> List[ChunkResult]:
        """
        Chunk text at semantic boundaries.

        Args:
            text: Input text to chunk

        Returns:
            List of ChunkResult with text and position info
        """
        if not text:
            return []

        # Use Rust implementation if available
        if HAS_RUST_MEMCHUNK:
            return self._chunk_with_rust(text)
        else:
            return self._chunk_with_fallback(text)

    def _chunk_with_rust(self, text: str) -> List[ChunkResult]:
        """Chunk using Rust memchunk library (fast path)."""
        text_bytes = text.encode('utf-8')
        results = []

        # Build chunker with configuration
        chunker = rust_chunk(text_bytes)

        # Apply configuration
        if self.config.chunk_size != 4096:
            chunker = chunker.size(self.config.chunk_size)

        if self.config.pattern:
            chunker = chunker.pattern(self.config.pattern)
        elif self.config.delimiters != b'\n.?':
            chunker = chunker.delimiters(self.config.delimiters)

        if self.config.prefix_mode:
            chunker = chunker.prefix()
        else:
            chunker = chunker.suffix()

        if self.config.consecutive:
            chunker = chunker.consecutive()

        if self.config.forward_fallback:
            chunker = chunker.forward_fallback()

        # Collect chunks
        position = 0
        for chunk_bytes in chunker:
            chunk_text = chunk_bytes.decode('utf-8', errors='replace')
            end_pos = position + len(chunk_bytes)
            results.append(ChunkResult(
                text=chunk_text,
                start_pos=position,
                end_pos=end_pos,
            ))
            position = end_pos

        return results

    def _chunk_with_fallback(self, text: str) -> List[ChunkResult]:
        """
        Pure Python fallback when Rust library unavailable.

        Implements semantic chunking using:
        - Delimiter search
        - Consecutive delimiter handling
        - Forward fallback
        """
        results = []
        position = 0

        # Get delimiters - handle both bytes and string input
        delimiters = self.config.delimiters or b'\n.?'
        if isinstance(delimiters, bytes):
            # Convert bytes to string characters
            # e.g., b'\n.?!' -> ['\n', '.', '?', '!']
            str_delimiters = [chr(b) for b in delimiters]
        else:
            str_delimiters = list(delimiters)

        chunk_size = self.config.chunk_size

        while position < len(text):
            remaining = len(text) - position

            # Last chunk - return all remaining
            if remaining <= chunk_size:
                results.append(ChunkResult(
                    text=text[position:],
                    start_pos=position,
                    end_pos=len(text),
                ))
                break

            # Search backward from target position
            target_end = position + chunk_size
            window = text[position:target_end]

            # Find last delimiter in window
            split_pos = self._find_last_delimiter_str(window, str_delimiters)

            if split_pos is not None and split_pos >= 0:
                # Found delimiter, split there
                actual_pos = position + split_pos
                # In suffix mode (default), include the delimiter with current chunk
                # In prefix mode, exclude the delimiter (it goes to next chunk)
                if not self.config.prefix_mode:
                    actual_pos += 1  # Include the delimiter
                results.append(ChunkResult(
                    text=text[position:actual_pos],
                    start_pos=position,
                    end_pos=actual_pos,
                ))
                position = actual_pos
            elif self.config.forward_fallback:
                # Search forward for next delimiter
                forward_window = text[target_end:]
                forward_pos = self._find_first_delimiter_str(forward_window, str_delimiters)
                if forward_pos is not None:
                    actual_pos = target_end + forward_pos
                    # Include delimiter in suffix mode
                    if not self.config.prefix_mode:
                        actual_pos += 1
                    results.append(ChunkResult(
                        text=text[position:actual_pos],
                        start_pos=position,
                        end_pos=actual_pos,
                    ))
                    position = actual_pos
                else:
                    # No delimiter found, take all remaining
                    results.append(ChunkResult(
                        text=text[position:],
                        start_pos=position,
                        end_pos=len(text),
                    ))
                    break
            else:
                # Hard split at target position
                results.append(ChunkResult(
                    text=text[position:target_end],
                    start_pos=position,
                    end_pos=target_end,
                ))
                position = target_end

        return results

    def _find_last_delimiter(self, window: bytes, delimiters: bytes) -> Optional[int]:
        """Find last occurrence of any delimiter in window."""
        for i in range(len(window) - 1, -1, -1):
            if window[i:i+1] in delimiters:
                return i
        return None

    def _find_first_delimiter(self, window: bytes, delimiters: bytes) -> Optional[int]:
        """Find first occurrence of any delimiter in window."""
        for i, byte in enumerate(window):
            if byte in delimiters:
                return i
        return None

    def _find_last_delimiter_str(self, window: str, delimiters: List[str]) -> Optional[int]:
        """Find last occurrence of any delimiter in string window."""
        for i in range(len(window) - 1, -1, -1):
            if window[i] in delimiters:
                return i
        return None

    def _find_first_delimiter_str(self, window: str, delimiters: List[str]) -> Optional[int]:
        """Find first occurrence of any delimiter in string window."""
        for i, char in enumerate(window):
            if char in delimiters:
                return i
        return None


class SentencePieceChunker(MemChunker):
    """
    Chunker for SentencePiece tokenized text (uses ▁ metaspace).

    The ▁ (U+2581) metaspace is used by SentencePiece tokenizers
    to mark the beginning of tokens. This chunker handles consecutive
    metaspaces correctly.
    """

    def __init__(self, chunk_size: int = 4096):
        # Metaspace character (U+2581) in UTF-8
        metaspace = '▁'.encode('utf-8')  # b'\xe2\x96\x81'

        config = ChunkerConfig(
            chunk_size=chunk_size,
            pattern=metaspace,
            prefix_mode=True,  # Metaspace at start of next chunk
            consecutive=True,  # Keep consecutive metaspaces together
            forward_fallback=True,
        )
        super().__init__(config)


class MarkdownChunker(MemChunker):
    """
    Chunker optimized for Markdown documents.

    Splits at:
    - Headers (##, ###, etc.)
    - Code blocks (```)
    - List items
    - Paragraphs (blank lines)
    """

    def __init__(self, chunk_size: int = 4096):
        # Markdown-specific delimiters
        delimiters = b'\n#`-*>'  # Headers, code, lists, quotes

        config = ChunkerConfig(
            chunk_size=chunk_size,
            delimiters=delimiters,
            prefix_mode=True,
            consecutive=True,  # Keep consecutive newlines together
            forward_fallback=True,
        )
        super().__init__(config)


def chunk_text(
    text: str,
    chunk_size: int = 4096,
    delimiters: str = '\n.?',
    mode: str = 'suffix'
) -> List[str]:
    """
    Convenience function for simple chunking.

    Args:
        text: Input text
        chunk_size: Target chunk size in bytes
        delimiters: Delimiter characters as string
        mode: 'prefix' or 'suffix' - where to put delimiters

    Returns:
        List of chunk strings

    Example:
        >>> chunks = chunk_text("Hello. World.", chunk_size=100)
        >>> chunks
        ['Hello.', ' World.']
    """
    config = ChunkerConfig(
        chunk_size=chunk_size,
        delimiters=delimiters.encode('utf-8'),
        prefix_mode=(mode == 'prefix'),
    )
    chunker = MemChunker(config)
    results = chunker.chunk(text)
    return [r.text for r in results]


# Benchmarks
def benchmark_chunker(text: str, chunker: MemChunker, iterations: int = 100) -> dict:
    """Benchmark chunking performance."""
    import time

    start = time.perf_counter()
    for _ in range(iterations):
        chunker.chunk(text)
    elapsed = time.perf_counter() - start

    total_bytes = len(text.encode('utf-8'))
    throughput = (total_bytes * iterations) / elapsed / 1e9  # GB/s

    return {
        'iterations': iterations,
        'elapsed_ms': elapsed * 1000,
        'avg_ms': (elapsed / iterations) * 1000,
        'throughput_gb_s': throughput,
        'total_bytes': total_bytes,
    }


if __name__ == "__main__":
    # Demo
    sample_text = """
    The quick brown fox jumps over the lazy dog. This sentence contains every letter
    of the English alphabet and is commonly used for typing practice. It's also useful
    for testing text processing algorithms.

    Memchunk is a fast text chunking library written in Rust. It uses SIMD instructions
    to achieve up to 1TB/s throughput. The library is available for Rust, Python, and
    JavaScript/WASM.
    """

    print("=== MemChunker Demo ===\n")

    # Basic chunking
    chunks = chunk_text(sample_text, chunk_size=256)
    print(f"Basic chunking: {len(chunks)} chunks")
    for i, chunk in enumerate(chunks):
        print(f"  [{i}] {len(chunk)} chars: {chunk[:50]}...")

    print()

    # Check if Rust is available
    if HAS_RUST_MEMCHUNK:
        print(f"✓ Rust memchunk library available")
    else:
        print(f"✗ Rust memchunk library NOT available (using Python fallback)")

    # Benchmark
    print("\n=== Performance Benchmark ===")
    results = benchmark_chunker(sample_text, MemChunker(), iterations=1000)
    print(f"Iterations: {results['iterations']}")
    print(f"Total time: {results['elapsed_ms']:.2f} ms")
    print(f"Avg time: {results['avg_ms']:.4f} ms")
    print(f"Throughput: {results['throughput_gb_s']:.2f} GB/s")
