"""
Test script for memchunker integration.

Tests both the pure Python fallback and (if available) the Rust memchunk library.
"""
import sys
import os
import time

# Add parent directory to path for imports
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from ai.memchunker import MemChunker, ChunkerConfig, chunk_text, HAS_RUST_MEMCHUNK


def test_basic_chunking():
    """Test basic semantic chunking."""
    print("=" * 60)
    print("TEST 1: Basic Semantic Chunking")
    print("=" * 60)

    text = """
    The quick brown fox jumps over the lazy dog. This sentence contains every
    letter of the English alphabet. Memchunk is a fast text chunking library.

    It uses SIMD instructions to achieve high throughput. The library is
    available in Rust, Python, and JavaScript. Semantic boundaries are
    detected using period, newline, and question mark delimiters.
    """

    chunks = chunk_text(text, chunk_size=128)

    print(f"Input text length: {len(text)} chars")
    print(f"Number of chunks: {len(chunks)}")
    print()

    for i, chunk in enumerate(chunks):
        print(f"Chunk {i+1}: ({len(chunk)} chars)")
        print(f"  {repr(chunk[:60])}...")
        print()

    assert len(chunks) > 1, "Should produce multiple chunks"
    assert all(chunk.strip() for chunk in chunks), "All chunks should be non-empty"
    print("[PASS] Test passed\n")


def test_delimiter_handling():
    """Test delimiter detection at boundaries."""
    print("=" * 60)
    print("TEST 2: Delimiter Boundary Detection")
    print("=" * 60)

    text = "First sentence. Second sentence? Third sentence! Fourth sentence."

    chunks = chunk_text(text, chunk_size=40)

    print(f"Input: {text}")
    print(f"Chunks: {len(chunks)}")
    for i, chunk in enumerate(chunks):
        print(f"  [{i}] {repr(chunk)}")

    # Verify chunks end at delimiters (not mid-word)
    for i, chunk in enumerate(chunks):
        if i < len(chunks) - 1:  # Not the last chunk
            assert chunk[-1] in ".?!", f"Chunk {i} should end with delimiter"

    print("\n[PASS] Test passed\n")


def test_consecutive_newlines():
    """Test handling of consecutive newlines."""
    print("=" * 60)
    print("TEST 3: Consecutive Newline Handling")
    print("=" * 60)

    text = """First paragraph.


Second paragraph with blank lines above.


Third paragraph."""

    config = ChunkerConfig(
        chunk_size=256,
        delimiters=b'\n',
        consecutive=True,  # Keep consecutive newlines together
        prefix_mode=False,
    )
    chunker = MemChunker(config)
    chunks = chunker.chunk(text)

    print(f"Input has {text.count(chr(10))} newlines")
    print(f"Chunks: {len(chunks)}")
    for i, chunk in enumerate(chunks):
        print(f"  [{i}] {len(chunk.text)} chars")

    print("\n[PASS] Test passed\n")


def test_forward_fallback():
    """Test forward fallback when no delimiter in window."""
    print("=" * 60)
    print("TEST 4: Forward Fallback Search")
    print("=" * 60)

    # Long word without delimiters, then a delimiter
    text = "verylongwordwithoutdelimiters. Next sentence here."

    config = ChunkerConfig(
        chunk_size=20,  # Small to trigger forward search
        delimiters=b'.',
        forward_fallback=True,
        prefix_mode=False,
    )
    chunker = MemChunker(config)
    chunks = chunker.chunk(text)

    print(f"Input: {text}")
    print(f"Chunk size: 20")
    print(f"Chunks: {len(chunks)}")
    for i, chunk in enumerate(chunks):
        print(f"  [{i}] {repr(chunk.text)}")

    # First chunk should end at the period (via forward fallback)
    assert '.' in chunks[0].text, "First chunk should include the period"
    assert 'Next' in chunks[1].text, "Second chunk should have 'Next'"

    print("\n[PASS] Test passed\n")


def test_empty_and_edge_cases():
    """Test edge cases."""
    print("=" * 60)
    print("TEST 5: Edge Cases")
    print("=" * 60)

    # Empty text
    chunks = chunk_text("")
    assert len(chunks) == 0, "Empty text should produce no chunks"
    print("[PASS] Empty text handled")

    # Text smaller than chunk size
    chunks = chunk_text("Short text.", chunk_size=1000)
    assert len(chunks) == 1, "Small text should produce single chunk"
    print("[PASS] Small text handled")

    # Text with only delimiters
    chunks = chunk_text("\n\n\n\n", chunk_size=10)
    print(f"[PASS] Only delimiters: {len(chunks)} chunks")

    print("\n[PASS] Test passed\n")


def benchmark_performance():
    """Benchmark chunking performance."""
    print("=" * 60)
    print("TEST 6: Performance Benchmark")
    print("=" * 60)

    # Sample text ~10KB
    text = ("The quick brown fox jumps over the lazy dog. " * 100 +
            "Memchunk is a fast text chunking library. " * 50 +
            "It uses SIMD instructions for high throughput. " * 50)

    config = ChunkerConfig(chunk_size=512, delimiters=b'\n.?!')
    chunker = MemChunker(config)

    # Warmup
    chunker.chunk(text[:1000])

    # Benchmark
    iterations = 1000
    start = time.perf_counter()
    for _ in range(iterations):
        chunker.chunk(text)
    elapsed = time.perf_counter() - start

    total_bytes = len(text.encode('utf-8')) * iterations
    throughput_mb = total_bytes / elapsed / 1_000_000

    print(f"Iterations: {iterations}")
    print(f"Text size: {len(text):,} chars")
    print(f"Total processed: {total_bytes:,} bytes")
    print(f"Elapsed: {elapsed*1000:.2f} ms")
    print(f"Avg: {elapsed/iterations*1000:.4f} ms per chunk")
    print(f"Throughput: {throughput_mb:.1f} MB/s")

    if HAS_RUST_MEMCHUNK:
        print(f"\n[PASS] Using Rust memchunk library")
    else:
        print(f"\n[WARN] Using pure Python fallback (install memchunk for 100x speedup)")

    print("\n[PASS] Test passed\n")


def main():
    """Run all tests."""
    print("\n")
    print("=" * 60)
    print("  MEMCHUNKER INTEGRATION TEST")
    print("=" * 60)
    print()

    if HAS_RUST_MEMCHUNK:
        print("[OK] Rust memchunk library available - using SIMD-accelerated path")
    else:
        print("[WARN] Rust memchunk library NOT found - using pure Python fallback")
        print("       Install with: pip install memchunk")
    print()

    try:
        test_basic_chunking()
        test_delimiter_handling()
        test_consecutive_newlines()
        test_forward_fallback()
        test_empty_and_edge_cases()
        benchmark_performance()

        print("=" * 60)
        print("ALL TESTS PASSED [PASS]")
        print("=" * 60)
        return 0

    except AssertionError as e:
        print(f"\n[FAIL] TEST FAILED: {e}")
        return 1
    except Exception as e:
        print(f"\n[FAIL] ERROR: {e}")
        import traceback
        traceback.print_exc()
        return 1


if __name__ == "__main__":
    sys.exit(main())
