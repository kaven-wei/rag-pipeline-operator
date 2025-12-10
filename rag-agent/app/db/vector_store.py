from typing import List, Dict, Any, Optional
from qdrant_client import QdrantClient
from qdrant_client.http import models
from app.config.settings import settings

class VectorStore:
    def __init__(self):
        self.client = QdrantClient(
            url=settings.QDRANT_URL, 
            api_key=settings.QDRANT_API_KEY
        )
        self.collection_name = settings.QDRANT_COLLECTION_NAME

    def ensure_collection(self, vector_size: int = 1536):
        if not self.client.collection_exists(self.collection_name):
            self.client.create_collection(
                collection_name=self.collection_name,
                vectors_config=models.VectorParams(size=vector_size, distance=models.Distance.COSINE),
            )

    def upsert_vectors(self, vectors: List[List[float]], payloads: List[Dict[str, Any]], ids: Optional[List[str]] = None):
        import uuid
        points = [
            models.PointStruct(
                id=ids[i] if ids else str(uuid.uuid4()),
                vector=vector,
                payload=payload
            )
            for i, (vector, payload) in enumerate(zip(vectors, payloads))
        ]
        self.client.upsert(
            collection_name=self.collection_name,
            points=points
        )

    def search(self, vector: List[float], limit: int = 5, score_threshold: float = 0.0) -> List[Dict[str, Any]]:
        results = self.client.search(
            collection_name=self.collection_name,
            query_vector=vector,
            limit=limit,
            score_threshold=score_threshold
        )
        return [
            {
                "id": hit.id,
                "score": hit.score,
                "payload": hit.payload
            }
            for hit in results
        ]

    def create_alias(self, alias_name: str, collection_name: str):
        self.client.update_collection_aliases(
            change_aliases_operations=[
                models.CreateAliasOperation(
                    create_alias=models.CreateAlias(
                        collection_name=collection_name,
                        alias_name=alias_name
                    )
                )
            ]
        )

    def switch_alias(self, alias_name: str, new_collection_name: str):
        """
        Atomically switch an alias to a new collection.
        This drops the alias from the old collection (if any) and assigns it to the new one.
        """
        self.client.update_collection_aliases(
            change_aliases_operations=[
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
        )

vector_store = VectorStore()
