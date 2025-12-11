"""
Application settings loaded from environment variables.
"""

from pydantic_settings import BaseSettings
from typing import Optional


class Settings(BaseSettings):
    """Application settings with defaults"""
    
    # Application
    APP_NAME: str = "RAG Agent"
    APP_VERSION: str = "0.1.0"
    DEBUG: bool = False
    LOG_LEVEL: str = "INFO"
    
    # Vector Database - Qdrant
    QDRANT_URL: str = "http://localhost:6333"
    QDRANT_API_KEY: Optional[str] = None
    QDRANT_COLLECTION_NAME: str = "rag_collection"
    
    # Vector Database - Milvus (for future use)
    MILVUS_HOST: str = "localhost"
    MILVUS_PORT: int = 19530
    MILVUS_USER: Optional[str] = None
    MILVUS_PASSWORD: Optional[str] = None
    
    # OpenAI / LLM
    OPENAI_API_KEY: str = ""
    OPENAI_API_BASE: Optional[str] = None  # For custom endpoints
    OPENAI_MODEL: str = "gpt-3.5-turbo"
    OPENAI_EMBEDDING_MODEL: str = "text-embedding-3-small"
    OPENAI_MAX_TOKENS: int = 1024
    OPENAI_TEMPERATURE: float = 0.7
    
    # RAG Configuration
    RAG_TOP_K: int = 5
    RAG_SCORE_THRESHOLD: float = 0.7
    RAG_MAX_CONTEXT_LENGTH: int = 4000
    
    # Job Configuration
    JOB_BATCH_SIZE: int = 16
    JOB_MAX_RETRIES: int = 3
    JOB_RETRY_BACKOFF: int = 30
    
    # Kubernetes
    POD_NAMESPACE: str = "default"
    USE_K8S_API: bool = False
    
    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"
        case_sensitive = True


# Global settings instance
settings = Settings()
