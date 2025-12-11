"""
Admin API endpoints for management operations.
"""

from fastapi import APIRouter, HTTPException, status
from pydantic import BaseModel
from typing import Optional, Dict, Any
import logging

logger = logging.getLogger(__name__)

router = APIRouter()


class ClearCacheResponse(BaseModel):
    """Response for cache clear operation"""
    status: str
    message: str


class CollectionInfoResponse(BaseModel):
    """Response for collection info"""
    name: str
    vectors_count: int
    status: str
    config: Dict[str, Any] = {}


class AliasSwapRequest(BaseModel):
    """Request for alias swap operation"""
    alias_name: str
    collection_name: str


class AliasSwapResponse(BaseModel):
    """Response for alias swap operation"""
    status: str
    message: str
    alias_name: str
    collection_name: str


@router.post("/clear-cache", response_model=ClearCacheResponse)
async def clear_cache_api():
    """
    Clear application-level caches.
    This can be called by the Operator after index updates.
    """
    try:
        from app.services.admin_service import clear_cache
        
        await clear_cache()
        
        return ClearCacheResponse(
            status="ok",
            message="Cache cleared successfully"
        )
    except Exception as e:
        logger.error(f"Failed to clear cache: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to clear cache: {str(e)}"
        )


@router.get("/collection-info", response_model=CollectionInfoResponse)
async def get_collection_info(collection_name: Optional[str] = None):
    """
    Get information about a vector database collection.
    
    Args:
        collection_name: Optional collection name (uses default if not provided)
    """
    try:
        from app.db.vector_store import VectorStore
        from app.config.settings import settings
        
        target_collection = collection_name or settings.QDRANT_COLLECTION_NAME
        vector_store = VectorStore(collection_name=target_collection)
        
        info = vector_store.get_collection_info()
        
        if not info:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"Collection '{target_collection}' not found"
            )
        
        return CollectionInfoResponse(
            name=info.get("name", target_collection),
            vectors_count=info.get("vectors_count", 0),
            status=info.get("status", "unknown"),
            config=info.get("config", {})
        )
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Failed to get collection info: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to get collection info: {str(e)}"
        )


@router.post("/alias-swap", response_model=AliasSwapResponse)
async def swap_alias(request: AliasSwapRequest):
    """
    Swap an alias to point to a new collection.
    This enables zero-downtime updates.
    
    Args:
        request: Alias swap request with alias_name and collection_name
    """
    try:
        from app.db.vector_store import VectorStore
        
        vector_store = VectorStore(collection_name=request.collection_name)
        vector_store.switch_alias(request.alias_name, request.collection_name)
        
        logger.info(f"Swapped alias '{request.alias_name}' to collection '{request.collection_name}'")
        
        return AliasSwapResponse(
            status="ok",
            message="Alias swapped successfully",
            alias_name=request.alias_name,
            collection_name=request.collection_name
        )
    except Exception as e:
        logger.error(f"Failed to swap alias: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to swap alias: {str(e)}"
        )


@router.get("/aliases")
async def list_aliases(collection_name: Optional[str] = None):
    """
    List all aliases for a collection.
    
    Args:
        collection_name: Optional collection name
    """
    try:
        from app.db.vector_store import VectorStore
        from app.config.settings import settings
        
        target_collection = collection_name or settings.QDRANT_COLLECTION_NAME
        vector_store = VectorStore(collection_name=target_collection)
        
        aliases = vector_store.list_aliases()
        
        return {
            "collection": target_collection,
            "aliases": aliases
        }
    except Exception as e:
        logger.error(f"Failed to list aliases: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to list aliases: {str(e)}"
        )


@router.get("/stats")
async def get_stats():
    """
    Get service statistics.
    """
    try:
        from app.db.vector_store import get_vector_store
        from app.config.settings import settings
        
        vector_store = get_vector_store()
        
        # Get collection info
        collection_info = vector_store.get_collection_info()
        
        return {
            "service": {
                "name": settings.APP_NAME,
                "version": settings.APP_VERSION
            },
            "vector_db": {
                "type": "qdrant",
                "url": settings.QDRANT_URL,
                "collection": settings.QDRANT_COLLECTION_NAME,
                "vectors_count": collection_info.get("vectors_count", 0),
                "status": collection_info.get("status", "unknown")
            },
            "llm": {
                "model": settings.OPENAI_MODEL,
                "embedding_model": settings.OPENAI_EMBEDDING_MODEL
            }
        }
    except Exception as e:
        logger.error(f"Failed to get stats: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to get stats: {str(e)}"
        )
