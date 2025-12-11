"""
Application constants.
"""

from app.config.settings import settings

# RAG Configuration
DEFAULT_TOP_K = settings.RAG_TOP_K
DEFAULT_SCORE_THRESHOLD = settings.RAG_SCORE_THRESHOLD
MAX_CONTEXT_LENGTH = settings.RAG_MAX_CONTEXT_LENGTH

# Chunking defaults
DEFAULT_CHUNK_SIZE = 512
DEFAULT_CHUNK_OVERLAP = 100

# Embedding dimensions (varies by model)
EMBEDDING_DIMENSIONS = {
    "text-embedding-3-small": 1536,
    "text-embedding-3-large": 3072,
    "text-embedding-ada-002": 1536,
    "bge-large-en": 1024,
    "bge-base-en": 768,
    "bge-small-en": 384,
}

# Vector database types
VECTOR_DB_QDRANT = "qdrant"
VECTOR_DB_MILVUS = "milvus"
VECTOR_DB_WEAVIATE = "weaviate"

# Index types
INDEX_TYPE_HNSW = "HNSW"
INDEX_TYPE_IVF_FLAT = "IVF_FLAT"
INDEX_TYPE_IVF_PQ = "IVF_PQ"

# Job phases
PHASE_PENDING = "Pending"
PHASE_RUNNING = "Running"
PHASE_SUCCEEDED = "Succeeded"
PHASE_FAILED = "Failed"

# Supported file extensions for document processing
SUPPORTED_EXTENSIONS = [
    '.txt', '.md', '.markdown',
    '.html', '.htm',
    '.json', '.yaml', '.yml',
    '.rst', '.csv', '.xml'
]
