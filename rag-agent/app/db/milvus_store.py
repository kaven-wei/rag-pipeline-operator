"""
Milvus vector store implementation.
Provides the same interface as VectorStore for Milvus database.

Note: This is a framework implementation. To use Milvus:
1. Uncomment pymilvus in requirements.txt
2. Set VECTOR_DB_TYPE=milvus in environment
"""

from typing import List, Dict, Any, Optional
import os
import logging

logger = logging.getLogger(__name__)


class MilvusStore:
    """
    Vector store wrapper for Milvus.
    Provides a unified interface compatible with VectorStore.
    """
    
    def __init__(
        self,
        host: Optional[str] = None,
        port: Optional[int] = None,
        user: Optional[str] = None,
        password: Optional[str] = None,
        collection_name: Optional[str] = None
    ):
        """
        Initialize Milvus connection.
        
        Args:
            host: Milvus server host
            port: Milvus server port
            user: Username for authentication
            password: Password for authentication
            collection_name: Default collection name
        """
        self.host = host or os.getenv("MILVUS_HOST", "localhost")
        self.port = port or int(os.getenv("MILVUS_PORT", "19530"))
        self.user = user or os.getenv("MILVUS_USER", "")
        self.password = password or os.getenv("MILVUS_PASSWORD", "")
        self.collection_name = collection_name or os.getenv("MILVUS_COLLECTION_NAME", "rag_collection")
        
        self._connected = False
        self._connection = None
        
        logger.info(f"Initialized MilvusStore: host={self.host}, port={self.port}, collection={self.collection_name}")

    def _connect(self):
        """Establish connection to Milvus"""
        if self._connected:
            return
        
        try:
            from pymilvus import connections
            
            connections.connect(
                alias="default",
                host=self.host,
                port=self.port,
                user=self.user,
                password=self.password
            )
            self._connected = True
            logger.info("Connected to Milvus")
        except ImportError:
            raise ImportError("pymilvus is not installed. Install with: pip install pymilvus")
        except Exception as e:
            logger.error(f"Failed to connect to Milvus: {e}")
            raise

    def ensure_collection(self, vector_size: int = 1536):
        """
        Ensure collection exists with proper schema.
        
        Args:
            vector_size: Dimension of vectors
        """
        self._connect()
        
        from pymilvus import Collection, CollectionSchema, FieldSchema, DataType, utility
        
        if utility.has_collection(self.collection_name):
            logger.info(f"Collection {self.collection_name} already exists")
            return
        
        # Define schema
        fields = [
            FieldSchema(name="id", dtype=DataType.VARCHAR, max_length=64, is_primary=True),
            FieldSchema(name="embedding", dtype=DataType.FLOAT_VECTOR, dim=vector_size),
            FieldSchema(name="text", dtype=DataType.VARCHAR, max_length=65535),
            FieldSchema(name="doc_id", dtype=DataType.VARCHAR, max_length=256),
            FieldSchema(name="metadata", dtype=DataType.JSON)
        ]
        
        schema = CollectionSchema(
            fields=fields,
            description=f"RAG collection: {self.collection_name}"
        )
        
        collection = Collection(
            name=self.collection_name,
            schema=schema
        )
        
        # Create index on embedding field
        index_params = {
            "index_type": "HNSW",
            "metric_type": "COSINE",
            "params": {"M": 16, "efConstruction": 200}
        }
        
        collection.create_index(
            field_name="embedding",
            index_params=index_params
        )
        
        logger.info(f"Created collection {self.collection_name} with HNSW index")

    def upsert_vectors(
        self,
        vectors: List[List[float]],
        payloads: List[Dict[str, Any]],
        ids: Optional[List[str]] = None
    ):
        """
        Insert or update vectors in the collection.
        
        Args:
            vectors: List of embedding vectors
            payloads: List of metadata payloads
            ids: Optional list of IDs
        """
        self._connect()
        
        from pymilvus import Collection
        import uuid
        
        if ids is None:
            ids = [str(uuid.uuid4()) for _ in range(len(vectors))]
        
        collection = Collection(self.collection_name)
        
        # Prepare data
        entities = [
            ids,
            vectors,
            [p.get("text", "") for p in payloads],
            [p.get("doc_id", "") for p in payloads],
            [p.get("metadata", {}) for p in payloads]
        ]
        
        collection.insert(entities)
        collection.flush()
        
        logger.debug(f"Upserted {len(vectors)} vectors to {self.collection_name}")

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
            filter: Optional filter expression
            
        Returns:
            List of results with id, score, and payload
        """
        self._connect()
        
        from pymilvus import Collection
        
        collection = Collection(self.collection_name)
        collection.load()
        
        search_params = {
            "metric_type": "COSINE",
            "params": {"ef": 100}
        }
        
        results = collection.search(
            data=[vector],
            anns_field="embedding",
            param=search_params,
            limit=limit,
            output_fields=["text", "doc_id", "metadata"]
        )
        
        output = []
        for hits in results:
            for hit in hits:
                if hit.score >= score_threshold:
                    output.append({
                        "id": hit.id,
                        "score": hit.score,
                        "payload": {
                            "text": hit.entity.get("text"),
                            "doc_id": hit.entity.get("doc_id"),
                            "metadata": hit.entity.get("metadata", {})
                        }
                    })
        
        return output

    def get_collection_info(self) -> Dict[str, Any]:
        """Get collection information"""
        self._connect()
        
        from pymilvus import Collection, utility
        
        if not utility.has_collection(self.collection_name):
            return {}
        
        collection = Collection(self.collection_name)
        
        return {
            "name": self.collection_name,
            "vectors_count": collection.num_entities,
            "status": "green" if collection.is_empty == False else "yellow",
            "config": {
                "vector_size": collection.schema.fields[1].params.get("dim")
            }
        }

    def create_alias(self, alias_name: str, collection_name: Optional[str] = None):
        """Create an alias for a collection"""
        self._connect()
        
        from pymilvus import utility
        
        target = collection_name or self.collection_name
        utility.create_alias(target, alias_name)
        
        logger.info(f"Created alias: {alias_name} -> {target}")

    def switch_alias(self, alias_name: str, new_collection_name: str):
        """Switch an alias to a new collection"""
        self._connect()
        
        from pymilvus import utility
        
        # Alter alias to point to new collection
        utility.alter_alias(new_collection_name, alias_name)
        
        logger.info(f"Switched alias: {alias_name} -> {new_collection_name}")

    def health_check(self) -> bool:
        """Check if Milvus is healthy"""
        try:
            self._connect()
            from pymilvus import utility
            utility.list_collections()
            return True
        except Exception as e:
            logger.error(f"Milvus health check failed: {e}")
            return False


def get_vector_store_for_type(db_type: str, **kwargs):
    """
    Factory function to get the appropriate vector store.
    
    Args:
        db_type: Type of vector database (qdrant, milvus)
        **kwargs: Additional arguments for the store
        
    Returns:
        Vector store instance
    """
    if db_type.lower() == "milvus":
        return MilvusStore(**kwargs)
    else:
        from app.db.vector_store import VectorStore
        return VectorStore(**kwargs)

