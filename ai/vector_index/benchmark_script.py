import time
import uuid
import numpy as np
import asyncio
from indexer import VectorIndexBuilder

def generate_dummy_data(num_chunks=1000, dim=1536):
    print(f"Generating {num_chunks} dummy chunks (dim={dim})...")
    chunks = []
    for i in range(num_chunks):
        chunks.append({
            'text': f"This is chunk number {i} with some random content to simulate a document segment.",
            'embedding': np.random.rand(dim).astype(np.float32).tolist()
        })
    return chunks

def benchmark_vector_tree(chunks):
    print("\n--- Benchmarking Vector Tree Construction ---")
    start_time = time.time()
    
    builder = VectorIndexBuilder(branching_factor=10)
    tree_map = builder.build_index(chunks)
    
    duration = time.time() - start_time
    print(f"Vector Tree Build Time: {duration:.4f} seconds")
    print(f"Total Nodes in Tree: {len(tree_map)}")
    
    # Analyze Tree
    max_depth = max(n.depth for n in tree_map.values())
    print(f"Tree Height: {max_depth}")
    
    return duration, len(tree_map)

async def benchmark_llm_simulation(chunks):
    """
    Simulates the OLD way: Tier 3 Extraction.
    Assuming 1.5 seconds per chunk for LLM API call + parsing.
    """
    print("\n--- Benchmarking LLM Extraction (Simulated) ---")
    start_time = time.time()
    
    # Simulate processing (async sleep for concurrency, but typically limited by rate limits)
    # Let's assume we can do 10 concurrent requests.
    batch_size = 10
    latency_per_batch = 1.0 # Optimistic: 1s for 10 chunks
    
    total_time = (len(chunks) / batch_size) * latency_per_batch
    
    # We won't actually sleep for full 100 seconds for 1000 chunks in this script, 
    # but we will calculate the projection.
    projection = total_time
    
    # Actual "code overhead" simulation
    # await asyncio.sleep(0.1) 
    
    print(f"Projected LLM Extraction Time: {projection:.4f} seconds")
    print(f"(Based on optimistic {latency_per_batch}s per {batch_size} chunks)")
    
    return projection

if __name__ == "__main__":
    # Test with 100 chunks
    data_small = generate_dummy_data(100)
    vt_time_small, _ = benchmark_vector_tree(data_small)
    llm_time_small = asyncio.run(benchmark_llm_simulation(data_small))
    
    print(f"\n[Small Dataset - 100 Chunks]")
    print(f"Vector-Native Speedup: {llm_time_small / vt_time_small:.1f}x")
    
    # Test with 1000 chunks
    data_large = generate_dummy_data(1000)
    vt_time_large, _ = benchmark_vector_tree(data_large)
    llm_time_large = asyncio.run(benchmark_llm_simulation(data_large))
    
    print(f"\n[Large Dataset - 1,000 Chunks]")
    print(f"Vector-Native Speedup: {llm_time_large / vt_time_large:.1f}x")
    
    print("\nCONCLUSION:")
    print("The Vector-Native approach scales O(N) with mathematical operations (microseconds),")
    print("whereas LLM-Native scales O(N) with API latency (seconds).")
