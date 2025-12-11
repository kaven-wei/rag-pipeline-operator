"""
Application startup logic.
Handles initialization of services and dependencies.
"""

import logging
from app.db.vector_store import get_vector_store, VectorStore
from app.config.settings import settings

logger = logging.getLogger(__name__)

# Global startup state
_startup_complete = False


def is_startup_complete() -> bool:
    """Check if startup has completed"""
    return _startup_complete


async def startup_event():
    """
    Main startup event handler.
    Called when the FastAPI application starts.
    """
    global _startup_complete
    
    logger.info("Starting RAG Agent...")
    logger.info(f"Configuration: app_name={settings.APP_NAME}, version={settings.APP_VERSION}")
    
    try:
        # Initialize vector store connection
        logger.info(f"Connecting to vector database at {settings.QDRANT_URL}")
        
        vector_store = get_vector_store()
        
        # Ensure default collection exists
        try:
            vector_store.ensure_collection()
            logger.info(f"Vector store collection '{settings.QDRANT_COLLECTION_NAME}' is ready")
        except Exception as e:
            logger.warning(f"Could not ensure collection (may not be critical): {e}")
        
        # Test connection
        if vector_store.health_check():
            logger.info("Vector database connection: OK")
        else:
            logger.warning("Vector database connection: FAILED (service may be unavailable)")
        
        # Log LLM configuration
        if settings.OPENAI_API_KEY:
            logger.info(f"LLM configured: model={settings.OPENAI_MODEL}")
        else:
            logger.warning("OPENAI_API_KEY not set - LLM features may not work")
        
        _startup_complete = True
        logger.info("Startup complete: RAG Agent is ready to serve requests")
        
    except Exception as e:
        logger.error(f"Startup error: {e}", exc_info=True)
        # Don't mark as complete if startup failed
        # The readiness probe will return unhealthy
        raise


async def shutdown_event():
    """
    Shutdown event handler.
    Called when the FastAPI application stops.
    """
    logger.info("Shutting down RAG Agent...")
    
    # Cleanup tasks can go here
    # e.g., close database connections, flush caches
    
    logger.info("Shutdown complete")
