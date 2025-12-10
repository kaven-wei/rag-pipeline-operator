from typing import List
from app.services.llm_service import llm_service

async def get_embedding(text: str) -> List[float]:
    return await llm_service.get_embedding(text)

async def get_embeddings(texts: List[str]) -> List[List[float]]:
    return await llm_service.get_embeddings(texts)
