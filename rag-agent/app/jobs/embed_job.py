import asyncio
import os
import logging
from typing import List, Optional
from app.jobs.process_documents import process_documents
from app.core.embedder import get_embeddings
from app.db.vector_store import VectorStore
from app.services.status_report import StatusReporter
from app.jobs.data_source import DataSourceFactory

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class EmbeddingJobConfig:
    """Configuration for embedding job from environment variables"""
    
    def __init__(self):
        self.document_set_name = os.getenv("DOCUMENT_SET_NAME", "")
        self.document_set_namespace = os.getenv("DOCUMENT_SET_NAMESPACE", "default")
        self.embedding_model = os.getenv("EMBEDDING_MODEL", "text-embedding-3-small")
        self.vector_db_type = os.getenv("VECTOR_DB_TYPE", "qdrant")
        self.vector_db_collection = os.getenv("VECTOR_DB_COLLECTION", "default_collection")
        self.vector_db_endpoint = os.getenv("VECTOR_DB_ENDPOINT", os.getenv("QDRANT_URL", "http://localhost:6333"))
        self.source_type = os.getenv("SOURCE_TYPE", "pvc")
        self.source_uri = os.getenv("SOURCE_URI", "")
        self.chunk_size = int(os.getenv("CHUNK_SIZE", "512"))
        self.chunk_overlap = int(os.getenv("CHUNK_OVERLAP", "100"))
        self.batch_size = int(os.getenv("BATCH_SIZE", "16"))
        
    def validate(self):
        """Validate required configuration"""
        if not self.document_set_name:
            raise ValueError("DOCUMENT_SET_NAME is required")
        if not self.source_uri:
            raise ValueError("SOURCE_URI is required")
        if not self.vector_db_collection:
            raise ValueError("VECTOR_DB_COLLECTION is required")


async def run_embedding_job_async(document_set_id: str, config: Optional[EmbeddingJobConfig] = None):
    """
    Run embedding job asynchronously.
    
    Args:
        document_set_id: ID of the DocumentSet to process
        config: Optional configuration (loaded from env if not provided)
    """
    if config is None:
        config = EmbeddingJobConfig()
        config.document_set_name = document_set_id
    
    logger.info(f"Starting embedding job for DocumentSet: {document_set_id}")
    logger.info(f"Configuration: vector_db={config.vector_db_type}, collection={config.vector_db_collection}")
    
    status_reporter = StatusReporter()
    
    try:
        # Validate configuration
        config.validate()
        
        # Report starting status
        await status_reporter.report_embedding_progress(
            document_set_id,
            phase="Running",
            message="Fetching documents...",
            total_chunks=0,
            processed_chunks=0
        )
        
        # 1. Fetch documents from source
        logger.info(f"Fetching documents from {config.source_type}: {config.source_uri}")
        data_source = DataSourceFactory.create(config.source_type)
        docs = await data_source.fetch(config.source_uri)
        logger.info(f"Fetched {len(docs)} documents")
        
        if not docs:
            raise ValueError("No documents found at the source")
        
        # 2. Process (chunk) documents
        logger.info(f"Chunking documents with size={config.chunk_size}, overlap={config.chunk_overlap}")
        chunks = process_documents(docs, chunk_size=config.chunk_size, overlap=config.chunk_overlap)
        total_chunks = len(chunks)
        logger.info(f"Generated {total_chunks} chunks")
        
        await status_reporter.report_embedding_progress(
            document_set_id,
            phase="Running",
            message=f"Processing {total_chunks} chunks...",
            total_chunks=total_chunks,
            processed_chunks=0
        )
        
        # 3. Initialize vector store
        vector_store = VectorStore(
            url=config.vector_db_endpoint,
            collection_name=config.vector_db_collection
        )
        vector_store.ensure_collection()
        
        # 4. Generate embeddings and upsert in batches
        texts = [chunk["text"] for chunk in chunks]
        processed_count = 0
        
        for i in range(0, len(texts), config.batch_size):
            batch_texts = texts[i:i + config.batch_size]
            batch_chunks = chunks[i:i + config.batch_size]
            
            logger.info(f"Processing batch {i // config.batch_size + 1}, chunks {i} to {i + len(batch_texts)}")
            
            # Generate embeddings for batch
            embeddings = await get_embeddings(batch_texts)
            
            # Prepare payloads and IDs
            payloads = [
                {
                    "text": chunk["text"],
                    "metadata": chunk["metadata"],
                    "doc_id": chunk["doc_id"],
                    "chunk_index": chunk.get("chunk_index", 0)
                }
                for chunk in batch_chunks
            ]
            
            # Generate deterministic IDs based on chunk content
            import hashlib
            ids = [
                hashlib.sha256(f"{chunk['doc_id']}_{chunk['id']}".encode()).hexdigest()[:32]
                for chunk in batch_chunks
            ]
            
            # Upsert to vector store
            vector_store.upsert_vectors(embeddings, payloads, ids=ids)
            
            processed_count += len(batch_texts)
            
            # Report progress
            await status_reporter.report_embedding_progress(
                document_set_id,
                phase="Running",
                message=f"Processed {processed_count}/{total_chunks} chunks",
                total_chunks=total_chunks,
                processed_chunks=processed_count
            )
        
        # 5. Report success
        logger.info(f"Successfully indexed {total_chunks} chunks to collection {config.vector_db_collection}")
        
        await status_reporter.report_embedding_progress(
            document_set_id,
            phase="Succeeded",
            message=f"Successfully processed {total_chunks} chunks",
            total_chunks=total_chunks,
            processed_chunks=total_chunks
        )
        
        return {
            "status": "success",
            "total_chunks": total_chunks,
            "collection": config.vector_db_collection
        }
        
    except Exception as e:
        logger.error(f"Embedding job failed: {str(e)}", exc_info=True)
        
        await status_reporter.report_embedding_progress(
            document_set_id,
            phase="Failed",
            message=f"Error: {str(e)}",
            total_chunks=0,
            processed_chunks=0
        )
        
        raise


def run_embedding_job(document_set_id: str):
    """Synchronous wrapper for embedding job"""
    return asyncio.run(run_embedding_job_async(document_set_id))
