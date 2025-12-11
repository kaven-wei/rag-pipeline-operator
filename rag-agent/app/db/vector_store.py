"""
Vector store abstraction layer supporting multiple vector databases.
Currently supports: Qdrant (primary), with Milvus support planned.
"""

from typing import List, Dict, Any, Optional
import os
import logging

logger = logging.getLogger(__name__)


class VectorStore:
    """
    Vector store wrapper for Qdrant.
    Provides a unified interface for vector operations.
    """
    
    def __init__(
        self, 
        url: Optional[str] = None, 
        api_key: Optional[str] = None,
        collection_name: Optional[str] = None
    ):
        """
        Initialize vector store.
        
        Args:
            url: Vector database URL (defaults to QDRANT_URL env var)
            api_key: API key (defaults to QDRANT_API_KEY env var)
            collection_name: Collection name (defaults to QDRANT_COLLECTION_NAME env var)
        """
        from qdrant_client import QdrantClient
        
        self.url = url or os.getenv("QDRANT_URL", "http://localhost:6333")
        self.api_key = api_key or os.getenv("QDRANT_API_KEY")
        self.collection_name = collection_name or os.getenv("QDRANT_COLLECTION_NAME", "rag_collection")
        
        self.client = QdrantClient(
            url=self.url,
            api_key=self.api_key,
            timeout=60  # 60 second timeout
        )
        
        logger.info(f"Initialized VectorStore: url={self.url}, collection={self.collection_name}")

    def ensure_collection(self, vector_size: int = 1536):
        """
        Ensure collection exists, create if not.
        
        Args:
            vector_size: Dimension of vectors (default 1536 for OpenAI embeddings)
        """
        from qdrant_client.http import models
        
        try:
            if not self.client.collection_exists(self.collection_name):
                logger.info(f"Creating collection: {self.collection_name} with vector_size={vector_size}")
                
                self.client.create_collection(
                    collection_name=self.collection_name,
                    vectors_config=models.VectorParams(
                        size=vector_size, 
                        distance=models.Distance.COSINE
                    ),
                    # Optimize for batched inserts
                    optimizers_config=models.OptimizersConfigDiff(
                        indexing_threshold=20000,
                    ),
                    # HNSW index configuration
                    hnsw_config=models.HnswConfigDiff(
                        m=16,
                        ef_construct=100,
                    )
                )
                logger.info(f"Collection {self.collection_name} created successfully")
            else:
                logger.info(f"Collection {self.collection_name} already exists")
        except Exception as e:
            logger.error(f"Failed to ensure collection: {e}")
            raise

    def upsert_vectors(
        self, 
        vectors: List[List[float]], 
        payloads: List[Dict[str, Any]], 
        ids: Optional[List[str]] = None
    ):
        """
        Upsert vectors to the collection.
        
        Args:
            vectors: List of embedding vectors
            payloads: List of metadata payloads
            ids: Optional list of IDs (generated if not provided)
        """
        from qdrant_client.http import models
        import uuid
        
        if len(vectors) != len(payloads):
            raise ValueError("vectors and payloads must have same length")
        
        # Generate IDs if not provided
        if ids is None:
            ids = [str(uuid.uuid4()) for _ in range(len(vectors))]
        
        points = [
            models.PointStruct(
                id=point_id,
                vector=vector,
                payload=payload
            )
            for point_id, vector, payload in zip(ids, vectors, payloads)
        ]
        
        self.client.upsert(
            collection_name=self.collection_name,
            points=points,
            wait=True  # Wait for operation to complete
        )
        
        logger.debug(f"Upserted {len(points)} vectors to {self.collection_name}")

    def search(
        self, 
        vector: List[float], 
        limit: int = 5, 
        score_threshold: float = 0.0,
        filter: Optional[Dict] = None
    ) -> List[Dict[str, Any]]:
        """
        Search for similar vectors.
        
        Args:
            vector: Query vector
            limit: Maximum number of results
            score_threshold: Minimum similarity score
            filter: Optional filter conditions
            
        Returns:
            List of results with id, score, and payload
        """
        from qdrant_client.http import models
        
        query_filter = None
        if filter:
            query_filter = models.Filter(**filter)
        
        results = self.client.search(
            collection_name=self.collection_name,
            query_vector=vector,
            limit=limit,
            score_threshold=score_threshold,
            query_filter=query_filter
        )
        
        return [
            {
                "id": hit.id,
                "score": hit.score,
                "payload": hit.payload
            }
            for hit in results
        ]

    def get_collection_info(self) -> Dict[str, Any]:
        """
        Get collection information.
        
        Returns:
            Dict with collection stats (vectors_count, status, etc.)
        """
        try:
            info = self.client.get_collection(self.collection_name)
            return {
                "name": self.collection_name,
                "vectors_count": info.vectors_count,
                "points_count": info.points_count,
                "status": info.status.value if info.status else "unknown",
                "config": {
                    "vector_size": info.config.params.vectors.size if info.config.params.vectors else None
                }
            }
        except Exception as e:
            logger.error(f"Failed to get collection info: {e}")
            return {}

    def delete_collection(self):
        """Delete the collection"""
        try:
            self.client.delete_collection(self.collection_name)
            logger.info(f"Deleted collection: {self.collection_name}")
        except Exception as e:
            logger.error(f"Failed to delete collection: {e}")
            raise

    def create_alias(self, alias_name: str, collection_name: Optional[str] = None):
        """
        Create an alias for a collection.
        
        Args:
            alias_name: Name of the alias
            collection_name: Target collection (defaults to self.collection_name)
        """
        from qdrant_client.http import models
        
        target = collection_name or self.collection_name
        
        self.client.update_collection_aliases(
            change_aliases_operations=[
                models.CreateAliasOperation(
                    create_alias=models.CreateAlias(
                        collection_name=target,
                        alias_name=alias_name
                    )
                )
            ]
        )
        
        logger.info(f"Created alias: {alias_name} -> {target}")

    def switch_alias(self, alias_name: str, new_collection_name: str):
        """
        Atomically switch an alias to a new collection.
        This removes the alias from any existing collection and assigns it to the new one.
        
        Args:
            alias_name: Name of the alias to switch
            new_collection_name: New target collection
        """
        from qdrant_client.http import models
        
        try:
            # First try to delete existing alias (might not exist)
            operations = [
                models.DeleteAliasOperation(
                    delete_alias=models.DeleteAlias(
                        alias_name=alias_name
                    )
                ),
                models.CreateAliasOperation(
                    create_alias=models.CreateAlias(
                        collection_name=new_collection_name,
                        alias_name=alias_name
                    )
                )
            ]
            
            self.client.update_collection_aliases(
                change_aliases_operations=operations
            )
            
            logger.info(f"Switched alias: {alias_name} -> {new_collection_name}")
            
        except Exception as e:
            # If delete fails (alias doesn't exist), just create
            logger.warning(f"Alias switch failed, trying create only: {e}")
            self.create_alias(alias_name, new_collection_name)

    def list_aliases(self) -> List[Dict[str, str]]:
        """
        List all aliases.
        
        Returns:
            List of dicts with alias_name and collection_name
        """
        try:
            aliases = self.client.get_collection_aliases(self.collection_name)
            return [
                {"alias_name": alias.alias_name, "collection_name": alias.collection_name}
                for alias in aliases.aliases
            ]
        except Exception as e:
            logger.warning(f"Could not list aliases: {e}")
            return []

    def health_check(self) -> bool:
        """
        Check if the vector store is healthy.
        
        Returns:
            True if healthy, False otherwise
        """
        try:
            # Try to get collections list
            self.client.get_collections()
            return True
        except Exception as e:
            logger.error(f"Vector store health check failed: {e}")
            return False


# Global singleton for convenience
_vector_store: Optional[VectorStore] = None


def get_vector_store() -> VectorStore:
    """Get or create the global vector store instance"""
    global _vector_store
    if _vector_store is None:
        _vector_store = VectorStore()
    return _vector_store


# Backwards compatibility
vector_store = VectorStore()
