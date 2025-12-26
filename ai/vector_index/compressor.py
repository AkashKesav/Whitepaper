import torch
import torch.nn as nn
import torch.nn.functional as F
import numpy as np

class VectorCompressor(nn.Module):
    """
    A lightweight Neural Network to compress a cluster of vectors into a single
    representative 'Parent Vector'.
    
    Architecture:
    - Input: (Batch_Size, Cluster_Size, Vector_Dim)
    - Operation: Attention-based Aggregation (Transformer Encoder Layer simplified)
    - Output: (Batch_Size, Vector_Dim)
    
    This replaces the "LLM Summarization" step with O(1) Matrix Math.
    """
    def __init__(self, input_dim=1536, hidden_dim=512, num_heads=4, dropout=0.1):
        super().__init__()
        
        self.input_dim = input_dim
        
        # 1. Attention Mechanism to find the "center of gravity" of meaning
        self.attention = nn.MultiheadAttention(embed_dim=input_dim, num_heads=num_heads, batch_first=True)
        
        # 2. Feed Forward Network for feature extraction
        self.norm1 = nn.LayerNorm(input_dim)
        self.ffn = nn.Sequential(
            nn.Linear(input_dim, hidden_dim),
            nn.GELU(),
            nn.Linear(hidden_dim, input_dim),
            nn.Dropout(dropout)
        )
        self.norm2 = nn.LayerNorm(input_dim)
        
        # 3. Final projection to ensure robust output
        self.compressor_head = nn.Linear(input_dim, input_dim)

    def forward(self, cluster_vectors):
        """
        Args:
            cluster_vectors: Tensor of shape (Batch, Cluster_Size, Vector_Dim)
                             e.g., (1, 10, 1536) for one cluster of 10 chunks.
                             
        Returns:
            parent_vector: Tensor of shape (Batch, Vector_Dim)
        """
        # Self-Attention over the cluster elements
        # This allows the model to "weigh" important chunks higher than noise.
        attn_out, _ = self.attention(cluster_vectors, cluster_vectors, cluster_vectors)
        
        # Add & Norm
        x = self.norm1(cluster_vectors + attn_out)
        
        # Feed Forward
        ffn_out = self.ffn(x)
        x = self.norm2(x + ffn_out)
        
        # Pooling: We need 1 vector from N vectors.
        # We use Mean Pooling on the enriched vectors.
        # (The attention step already aligned them, so mean is safe now).
        pooled = torch.mean(x, dim=1)
        
        # Final projection
        parent_vector = self.compressor_head(pooled)
        
        # Normalize to unit length (crucial for cosine similarity later)
        parent_vector = F.normalize(parent_vector, p=2, dim=1)
        
        return parent_vector

    @classmethod
    def load_model(cls, path=None):
        """
        Load pretrained weights. If path is None, initializes a random model 
        (useful for testing or initial structure).
        """
        model = cls()
        if path:
            try:
                model.load_state_dict(torch.load(path))
                print(f"Loaded VectorCompressor from {path}")
            except Exception as e:
                print(f"Failed to load model from {path}, using random init: {e}")
        return model

# Validation code
if __name__ == "__main__":
    # Test shape correctness
    model = VectorCompressor()
    dummy_cluster = torch.randn(1, 10, 1536) # 1 cluster, 10 vectors, 1536 dim
    output = model(dummy_cluster)
    print(f"Input Shape: {dummy_cluster.shape}")
    print(f"Output Shape: {output.shape}") # Should be (1, 1536)
    assert output.shape == (1, 1536)
    print("VectorCompressor forward pass successful.")
