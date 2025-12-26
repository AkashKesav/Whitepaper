import numpy as np
import torch
from sklearn.cluster import KMeans
from dataclasses import dataclass, field
from typing import List, Dict, Optional
import uuid

# Import our new Compressor
try:
    from .compressor import VectorCompressor
except ImportError:
    from compressor import VectorCompressor # For local testing

@dataclass
class VectorNode:
    """
    A single node in the Vector Tree.
    Can be a Leaf (Text Chunk) or an Internal Node (Compressed Summary).
    """
    node_id: str
    vector: np.ndarray
    children_ids: List[str] = field(default_factory=list)
    text: Optional[str] = None # Only for leaves
    depth: int = 0
    
    def to_dict(self):
        return {
            "node_id": self.node_id,
            "vector": self.vector.tolist(), # Convert numpy to list for JSON
            "children_ids": self.children_ids,
            "text": self.text,
            "depth": self.depth
        }

class VectorIndexBuilder:
    """
    Builds a Hierarchical Vector Tree from flat chunks.
    """
    def __init__(self, branching_factor=10, dim=1536):
        self.branching_factor = branching_factor
        self.dim = dim
        self.compressor = VectorCompressor()
        self.compressor.eval() # Inference mode
        
    def build_index(self, chunks: List[Dict]) -> Dict[str, VectorNode]:
        """
        Main entry point.
        Args:
            chunks: List of dicts, each having {'text': str, 'embedding': list[float]}
            
        Returns:
            tree_nodes: A flat dictionary of all nodes {node_id: VectorNode}
                        The Root Node is identified by having depth = max_depth.
        """
        # 1. Create Leaf Nodes
        leaves = []
        for chunk in chunks:
            node = VectorNode(
                node_id=str(uuid.uuid4()),
                vector=np.array(chunk['embedding'], dtype=np.float32),
                text=chunk['text'],
                depth=0
            )
            leaves.append(node)
            
        tree_map = {node.node_id: node for node in leaves}
        current_layer = leaves
        
        # 2. Recursive Clustering & Compression
        while len(current_layer) > 1:
            print(f"Building Layer {current_layer[0].depth + 1} from {len(current_layer)} nodes...")
            next_layer = self._process_layer(current_layer)
            
            # Add new nodes to map
            for node in next_layer:
                tree_map[node.node_id] = node
                
            current_layer = next_layer
            
        return tree_map

    def _process_layer(self, nodes: List[VectorNode]) -> List[VectorNode]:
        """
        Takes N nodes, clusters them, compresses them into M parent nodes.
        M ~ N / branching_factor
        """
        # Prepare vectors for clustering
        vectors = np.stack([n.vector for n in nodes])
        num_nodes = len(nodes)
        
        # Determine number of clusters
        num_clusters = max(1, int(np.ceil(num_nodes / self.branching_factor)))
        
        # K-Means Clustering
        # Note: For huge datasets, we would use FAISS or MiniBatchKMeans. 
        # For < 50k nodes, standard KMeans is fine.
        kmeans = KMeans(n_clusters=num_clusters, n_init=10, random_state=42)
        cluster_labels = kmeans.fit_predict(vectors)
        
        parents = []
        
        # Process each cluster
        for cluster_id in range(num_clusters):
            # Get indices of nodes in this cluster
            indices = np.where(cluster_labels == cluster_id)[0]
            cluster_nodes = [nodes[i] for i in indices]
            
            # Create Parent Vector via Neural Compression
            parent_vector = self._compress_cluster(cluster_nodes)
            
            # Create Parent Node
            parent = VectorNode(
                node_id=str(uuid.uuid4()),
                vector=parent_vector,
                children_ids=[n.node_id for n in cluster_nodes],
                text=None, # Internal nodes have no text, only meaning
                depth=nodes[0].depth + 1
            )
            parents.append(parent)
            
        return parents

    def _compress_cluster(self, cluster_nodes: List[VectorNode]) -> np.ndarray:
        """
        Uses the VectorCompressor NN to merge children vectors.
        """
        # Prepare Tensor: (1, Cluster_Size, Dim)
        child_vectors = np.stack([n.vector for n in cluster_nodes])
        input_tensor = torch.tensor(child_vectors, dtype=torch.float32).unsqueeze(0)
        
        if torch.cuda.is_available():
            input_tensor = input_tensor.cuda()
            self.compressor.cuda()
            
        with torch.no_grad():
            parent_tensor = self.compressor(input_tensor)
            
        # Convert back to numpy
        return parent_tensor.cpu().numpy().flatten()

# Test Code
if __name__ == "__main__":
    print("Testing VectorIndexBuilder...")
    # Mock data
    mock_chunks = [
        {'text': f"Chunk {i}", 'embedding': np.random.rand(1536).tolist()} 
        for i in range(55) # Should result in ~6 parents, then 1 root. Depth 2.
    ]
    
    builder = VectorIndexBuilder(branching_factor=10)
    tree_map = builder.build_index(mock_chunks)
    
    print(f"Total Nodes in Tree: {len(tree_map)}")
    
    # Find root
    max_depth = max(n.depth for n in tree_map.values())
    roots = [n for n in tree_map.values() if n.depth == max_depth]
    print(f"Tree Height: {max_depth}")
    print(f"Number of Roots: {len(roots)}") # Should be 1
    print("Build Success!")
