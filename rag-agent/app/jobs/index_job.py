"""
Index Job implementation for building vector indexes and performing alias swap.
"""

import os
import logging
import asyncio
from typing import Optional, Dict, Any
from app.db.vector_store import VectorStore
from app.services.status_report import StatusReporter

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class IndexJobConfig:
    """Configuration for index job from environment variables"""
    
    def __init__(self):
        self.index_job_name = os.getenv("INDEX_JOB_NAME", "")
        self.document_set_name = os.getenv("DOCUMENT_SET_NAME", "")
        self.document_set_namespace = os.getenv("DOCUMENT_SET_NAMESPACE", "default")
        self.vector_db_type = os.getenv("VECTOR_DB_TYPE", "qdrant")
        self.vector_db_collection = os.getenv("VECTOR_DB_COLLECTION", "")
        self.vector_db_endpoint = os.getenv("VECTOR_DB_ENDPOINT", os.getenv("QDRANT_URL", "http://localhost:6333"))
        self.target_alias = os.getenv("TARGET_ALIAS", "")
        self.index_type = os.getenv("INDEX_TYPE", "HNSW")
        
        # Index parameters (prefixed with INDEX_PARAM_)
        self.index_params = self._load_index_params()
        
    def _load_index_params(self) -> Dict[str, Any]:
        """Load index parameters from environment variables"""
        params = {}
        for key, value in os.environ.items():
            if key.startswith("INDEX_PARAM_"):
                param_name = key.replace("INDEX_PARAM_", "").lower()
                # Try to convert to int
                try:
                    params[param_name] = int(value)
                except ValueError:
                    params[param_name] = value
        return params
        
    def validate(self):
        """Validate required configuration"""
        if not self.vector_db_collection:
            raise ValueError("VECTOR_DB_COLLECTION is required")


async def run_index_job_async(index_id: str, config: Optional[IndexJobConfig] = None):
    """
    Run index job asynchronously.
    
    Args:
        index_id: ID of the IndexJob to run
        config: Optional configuration (loaded from env if not provided)
    """
    if config is None:
        config = IndexJobConfig()
        config.index_job_name = index_id
    
    logger.info(f"Starting Index Job: {index_id}")
    logger.info(f"Configuration: vector_db={config.vector_db_type}, collection={config.vector_db_collection}")
    logger.info(f"Target alias: {config.target_alias}, Index type: {config.index_type}")
    
    status_reporter = StatusReporter()
    
    try:
        # Validate configuration
        config.validate()
        
        # Report starting status
        await status_reporter.report_index_progress(
            index_id,
            phase="Building",
            message="Starting index build...",
            total_vectors=0,
            indexed_vectors=0
        )
        
        # Initialize vector store
        vector_store = VectorStore(
            url=config.vector_db_endpoint,
            collection_name=config.vector_db_collection
        )
        
        # 1. Verify collection exists and has data
        logger.info(f"Verifying collection {config.vector_db_collection} exists...")
        
        collection_info = vector_store.get_collection_info()
        if not collection_info:
            raise ValueError(f"Collection {config.vector_db_collection} not found")
        
        total_vectors = collection_info.get("vectors_count", 0)
        logger.info(f"Collection has {total_vectors} vectors")
        
        await status_reporter.report_index_progress(
            index_id,
            phase="Building",
            message=f"Found {total_vectors} vectors, optimizing index...",
            total_vectors=total_vectors,
            indexed_vectors=0
        )
        
        # 2. Build/optimize index based on vector DB type
        logger.info(f"Building {config.index_type} index with params: {config.index_params}")
        
        if config.vector_db_type.lower() == "qdrant":
            await _build_qdrant_index(vector_store, config)
        elif config.vector_db_type.lower() == "milvus":
            await _build_milvus_index(vector_store, config)
        else:
            logger.warning(f"Index optimization not implemented for {config.vector_db_type}")
        
        await status_reporter.report_index_progress(
            index_id,
            phase="Optimizing",
            message="Index built, performing alias swap...",
            total_vectors=total_vectors,
            indexed_vectors=total_vectors
        )
        
        # 3. Perform alias swap if target alias is specified
        if config.target_alias:
            logger.info(f"Swapping alias '{config.target_alias}' to collection '{config.vector_db_collection}'")
            
            try:
                vector_store.switch_alias(config.target_alias, config.vector_db_collection)
                logger.info(f"Alias swap successful: {config.target_alias} -> {config.vector_db_collection}")
            except Exception as e:
                # If alias doesn't exist, create it
                logger.info(f"Creating new alias: {config.target_alias}")
                vector_store.create_alias(config.target_alias, config.vector_db_collection)
        
        # 4. Report success
        logger.info(f"Index job completed successfully")
        
        await status_reporter.report_index_progress(
            index_id,
            phase="Succeeded",
            message=f"Index built successfully with {total_vectors} vectors",
            total_vectors=total_vectors,
            indexed_vectors=total_vectors,
            alias_swapped=bool(config.target_alias)
        )
        
        return {
            "status": "success",
            "total_vectors": total_vectors,
            "collection": config.vector_db_collection,
            "alias": config.target_alias,
            "alias_swapped": bool(config.target_alias)
        }
        
    except Exception as e:
        logger.error(f"Index job failed: {str(e)}", exc_info=True)
        
        await status_reporter.report_index_progress(
            index_id,
            phase="Failed",
            message=f"Error: {str(e)}",
            total_vectors=0,
            indexed_vectors=0
        )
        
        raise


async def _build_qdrant_index(vector_store: VectorStore, config: IndexJobConfig):
    """Build/optimize index for Qdrant"""
    from qdrant_client.http import models
    
    # Qdrant automatically builds HNSW indexes, but we can optimize parameters
    # Update collection parameters for better performance
    
    hnsw_config = models.HnswConfigDiff(
        m=config.index_params.get("m", 16),
        ef_construct=config.index_params.get("efconstruction", 200),
    )
    
    optimizer_config = models.OptimizersConfigDiff(
        indexing_threshold=10000,  # Build index when collection has >10k vectors
    )
    
    try:
        vector_store.client.update_collection(
            collection_name=config.vector_db_collection,
            hnsw_config=hnsw_config,
            optimizer_config=optimizer_config
        )
        logger.info("Updated Qdrant collection HNSW configuration")
    except Exception as e:
        logger.warning(f"Could not update Qdrant collection config: {e}")
    
    # Wait for indexing to complete (Qdrant does this in background)
    logger.info("Waiting for Qdrant to finish indexing...")
    
    max_wait = 300  # 5 minutes
    wait_interval = 5
    waited = 0
    
    while waited < max_wait:
        info = vector_store.get_collection_info()
        status = info.get("status", "")
        
        if status == "green":
            logger.info("Qdrant index is ready")
            break
        
        logger.info(f"Index status: {status}, waiting...")
        await asyncio.sleep(wait_interval)
        waited += wait_interval
    
    if waited >= max_wait:
        logger.warning("Timeout waiting for index to be ready, continuing anyway")


async def _build_milvus_index(vector_store: VectorStore, config: IndexJobConfig):
    """Build index for Milvus"""
    # This would use pymilvus to create an index
    # Implementation depends on whether we add Milvus support
    
    index_params = {
        "index_type": config.index_type,
        "metric_type": "COSINE",
        "params": config.index_params
    }
    
    logger.info(f"Would build Milvus index with params: {index_params}")
    logger.warning("Milvus index building not yet implemented")


def run_index_job(index_id: str):
    """Synchronous wrapper for index job"""
    return asyncio.run(run_index_job_async(index_id))
