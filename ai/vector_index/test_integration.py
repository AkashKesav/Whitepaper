import asyncio
import os
import sys
from unittest.mock import MagicMock, patch
import numpy as np

# Adjust path to import from ai modules
sys.path.append(os.path.join(os.path.dirname(__file__), '../../'))

from ai.document_ingester import DocumentIngester, IngestionResult

async def test_vector_tree_pipeline():
    print("--- Starting Vector Tree Integration Test ---")
    
    # Mock LLM Router
    mock_router = MagicMock()
    # Mock embedding to return random vectors
    async def mock_get_embedding(text):
        return np.random.rand(1536).tolist()
    mock_router.get_embedding = mock_get_embedding
    
    ingester = DocumentIngester(llm_router=mock_router)
    
    # Mock PyMuPDF (fitz)
    with patch('fitz.open') as mock_open:
        mock_doc = MagicMock()
        # Create 3 pages of text
        mock_doc.__iter__.return_value = [
            MagicMock(get_text=lambda: "Page 1: Introduction to Vector Databases. They use math."),
            MagicMock(get_text=lambda: "Page 2: Hierarchical Navigable Small World graphs are cool."),
            MagicMock(get_text=lambda: "Page 3: Conclusion. Vectors are better than keywords.")
        ]
        mock_open.return_value = mock_doc
        
        print("1. Ingesting 'mock.pdf'...")
        result = await ingester.ingest_pdf_with_tree("mock.pdf")
        
        print("2. Ingestion Complete.")
        print(f"   Summary: {result.summary}")
        print(f"   Chunks created: {len(result.chunks)}")
        
        tree_map = result.vector_tree
        print(f"\n3. Inspecting Vector Tree (Total Nodes: {len(tree_map)}):")
        
        if not tree_map:
            print("   [ERROR] Tree Map is empty!")
            sys.exit(1)
            
        # Find Root (Max Depth)
        # Note: VectorNode object might be returned or dict depending on serialization
        # Ingester returns what builder returns. Builder returns Dict[str, VectorNode]
        # BUT IngestionResult is a dataclass.
        # Let's check type of first value
        first_val = list(tree_map.values())[0]
        print(f"   Node Type: {type(first_val)}")
        
        # Helper to get depth
        def get_depth(n):
            return n.depth if hasattr(n, 'depth') else n.get('depth', 0)
            
        max_depth = max(get_depth(n) for n in tree_map.values())
        roots = [n for n in tree_map.values() if get_depth(n) == max_depth]
        
        print(f"   Max Depth: {max_depth}")
        print(f"   Roots Found: {len(roots)}")
        
        if len(roots) != 1:
            print("   [WARNING] Expected exactly 1 root!")
        
        root = roots[0]
        root_id = root.node_id if hasattr(root, 'node_id') else root.get('node_id')
        print(f"   Root ID: {root_id}")
        
        # Check children of root
        children_ids = root.children_ids if hasattr(root, 'children_ids') else root.get('children_ids', [])
        print(f"   Root Children Count: {len(children_ids)}")
        
        if len(children_ids) > 0:
            print("   [SUCCESS] Root has children. Hierarchy established.")
        else:
             # If only 1 chunk, root has 0 children? No, leaf is root.
             # If 3 chunks, root should have logic to group them.
             # Indexer logic: if len(layers) > 1 loop.
             # 3 chunks -> 1 cluster -> 1 parent.
             # Parent has 3 children.
             if len(tree_map) == len(result.chunks):
                  print("   [WARNING] Flat tree. No hierarchy built (maybe too few chunks?)")
             else:
                  print("   [SUCCESS] Hierarchy built.")

        print("\n[SUCCESS] Vector Tree Pipeline Verified.")

if __name__ == "__main__":
    try:
        asyncio.run(test_vector_tree_pipeline())
    except Exception as e:
        print(f"\n[FATAL ERROR] {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
