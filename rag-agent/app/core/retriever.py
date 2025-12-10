from typing import List, Dict, Any
from app.core.embedder import get_embedding
from app.db.vector_store import vector_store
from app.config.constants import DEFAULT_TOP_K, DEFAULT_SCORE_THRESHOLD

async def vector_search(query: str, top_k: int = DEFAULT_TOP_K, score_threshold: float = DEFAULT_SCORE_THRESHOLD) -> List[Dict[str, Any]]:
    query_vector = await get_embedding(query)
    results = vector_store.search(query_vector, limit=top_k, score_threshold=score_threshold)
    return results
