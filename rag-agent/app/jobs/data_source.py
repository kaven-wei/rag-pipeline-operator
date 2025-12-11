"""
Data source handlers for fetching documents from various sources.
Supports: S3, HTTP, PVC, Git
"""

import os
import logging
from abc import ABC, abstractmethod
from typing import List, Dict, Any
from pathlib import Path

logger = logging.getLogger(__name__)


class DataSource(ABC):
    """Abstract base class for data sources"""
    
    @abstractmethod
    async def fetch(self, uri: str) -> List[Dict[str, Any]]:
        """
        Fetch documents from the source.
        
        Args:
            uri: URI to the data source
            
        Returns:
            List of documents with 'id', 'text', and 'metadata' fields
        """
        pass
    
    def _read_file(self, file_path: str) -> Dict[str, Any]:
        """Read a single file and return document dict"""
        try:
            with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
                content = f.read()
            
            return {
                "id": os.path.basename(file_path),
                "text": content,
                "metadata": {
                    "source": file_path,
                    "filename": os.path.basename(file_path),
                    "extension": os.path.splitext(file_path)[1]
                }
            }
        except Exception as e:
            logger.warning(f"Failed to read file {file_path}: {e}")
            return None
    
    def _get_supported_extensions(self) -> List[str]:
        """Return list of supported file extensions"""
        return ['.txt', '.md', '.html', '.json', '.yaml', '.yml', '.rst', '.csv']


class PVCDataSource(DataSource):
    """Data source for PVC (local filesystem) paths"""
    
    async def fetch(self, uri: str) -> List[Dict[str, Any]]:
        """
        Fetch documents from PVC/local path.
        
        Args:
            uri: Path like 'pvc://pvc-name/path' or '/path/to/files'
        """
        # Parse PVC URI
        if uri.startswith("pvc://"):
            # Format: pvc://pvc-name/path
            path = "/" + "/".join(uri.replace("pvc://", "").split("/")[1:])
        else:
            path = uri
        
        logger.info(f"Fetching documents from local path: {path}")
        
        documents = []
        path_obj = Path(path)
        
        if path_obj.is_file():
            doc = self._read_file(str(path_obj))
            if doc:
                documents.append(doc)
        elif path_obj.is_dir():
            for ext in self._get_supported_extensions():
                for file_path in path_obj.rglob(f"*{ext}"):
                    doc = self._read_file(str(file_path))
                    if doc:
                        documents.append(doc)
        else:
            raise FileNotFoundError(f"Path not found: {path}")
        
        logger.info(f"Loaded {len(documents)} documents from {path}")
        return documents


class S3DataSource(DataSource):
    """Data source for S3-compatible storage"""
    
    async def fetch(self, uri: str) -> List[Dict[str, Any]]:
        """
        Fetch documents from S3.
        
        Args:
            uri: S3 URI like 's3://bucket/prefix/'
        """
        import boto3
        from io import StringIO
        
        # Parse S3 URI
        if not uri.startswith("s3://"):
            raise ValueError(f"Invalid S3 URI: {uri}")
        
        parts = uri.replace("s3://", "").split("/", 1)
        bucket = parts[0]
        prefix = parts[1] if len(parts) > 1 else ""
        
        logger.info(f"Fetching documents from S3: bucket={bucket}, prefix={prefix}")
        
        # Get AWS credentials from environment
        s3_client = boto3.client(
            's3',
            aws_access_key_id=os.getenv('AWS_ACCESS_KEY_ID'),
            aws_secret_access_key=os.getenv('AWS_SECRET_ACCESS_KEY'),
            region_name=os.getenv('AWS_REGION', 'us-east-1'),
            endpoint_url=os.getenv('S3_ENDPOINT_URL')  # For S3-compatible services
        )
        
        documents = []
        paginator = s3_client.get_paginator('list_objects_v2')
        
        for page in paginator.paginate(Bucket=bucket, Prefix=prefix):
            for obj in page.get('Contents', []):
                key = obj['Key']
                
                # Check if file has supported extension
                if any(key.endswith(ext) for ext in self._get_supported_extensions()):
                    try:
                        response = s3_client.get_object(Bucket=bucket, Key=key)
                        content = response['Body'].read().decode('utf-8', errors='ignore')
                        
                        documents.append({
                            "id": key,
                            "text": content,
                            "metadata": {
                                "source": f"s3://{bucket}/{key}",
                                "bucket": bucket,
                                "key": key,
                                "size": obj['Size'],
                                "last_modified": str(obj['LastModified'])
                            }
                        })
                    except Exception as e:
                        logger.warning(f"Failed to fetch S3 object {key}: {e}")
        
        logger.info(f"Loaded {len(documents)} documents from S3")
        return documents


class HTTPDataSource(DataSource):
    """Data source for HTTP/HTTPS URLs"""
    
    async def fetch(self, uri: str) -> List[Dict[str, Any]]:
        """
        Fetch documents from HTTP URL.
        
        Args:
            uri: HTTP URL to fetch
        """
        import aiohttp
        
        logger.info(f"Fetching document from HTTP: {uri}")
        
        documents = []
        
        async with aiohttp.ClientSession() as session:
            async with session.get(uri) as response:
                response.raise_for_status()
                content = await response.text()
                
                documents.append({
                    "id": uri.split("/")[-1] or "document",
                    "text": content,
                    "metadata": {
                        "source": uri,
                        "content_type": response.headers.get('Content-Type', ''),
                        "content_length": response.headers.get('Content-Length', '')
                    }
                })
        
        logger.info(f"Loaded {len(documents)} documents from HTTP")
        return documents


class GitDataSource(DataSource):
    """Data source for Git repositories"""
    
    async def fetch(self, uri: str) -> List[Dict[str, Any]]:
        """
        Fetch documents from Git repository.
        
        Args:
            uri: Git repository URL
        """
        import subprocess
        import tempfile
        import shutil
        
        logger.info(f"Cloning Git repository: {uri}")
        
        # Create temp directory
        temp_dir = tempfile.mkdtemp(prefix="rag_git_")
        
        try:
            # Clone repository
            subprocess.run(
                ["git", "clone", "--depth", "1", uri, temp_dir],
                check=True,
                capture_output=True
            )
            
            # Read files from cloned repo
            pvc_source = PVCDataSource()
            documents = await pvc_source.fetch(temp_dir)
            
            # Update metadata
            for doc in documents:
                doc["metadata"]["git_repo"] = uri
            
            return documents
            
        finally:
            # Cleanup temp directory
            shutil.rmtree(temp_dir, ignore_errors=True)


class MockDataSource(DataSource):
    """Mock data source for testing"""
    
    async def fetch(self, uri: str) -> List[Dict[str, Any]]:
        """Return mock documents for testing"""
        logger.info(f"Using mock data source for: {uri}")
        
        return [
            {
                "id": "doc1",
                "text": "Qdrant is a vector database designed for similarity search. It supports HNSW indexing and offers high performance for AI applications.",
                "metadata": {"source": "mock", "topic": "vector-db"}
            },
            {
                "id": "doc2", 
                "text": "RAG (Retrieval-Augmented Generation) combines retrieval systems with large language models to provide accurate and contextual responses.",
                "metadata": {"source": "mock", "topic": "rag"}
            },
            {
                "id": "doc3",
                "text": "Kubernetes Operators extend Kubernetes functionality by automating complex application management tasks using custom controllers.",
                "metadata": {"source": "mock", "topic": "kubernetes"}
            }
        ]


class DataSourceFactory:
    """Factory for creating data source instances"""
    
    _sources = {
        "s3": S3DataSource,
        "http": HTTPDataSource,
        "https": HTTPDataSource,
        "git": GitDataSource,
        "pvc": PVCDataSource,
        "local": PVCDataSource,
        "mock": MockDataSource
    }
    
    @classmethod
    def create(cls, source_type: str) -> DataSource:
        """
        Create a data source instance.
        
        Args:
            source_type: Type of data source (s3, http, git, pvc, mock)
            
        Returns:
            DataSource instance
        """
        source_class = cls._sources.get(source_type.lower())
        
        if source_class is None:
            raise ValueError(f"Unknown data source type: {source_type}. Supported: {list(cls._sources.keys())}")
        
        return source_class()
    
    @classmethod
    def register(cls, source_type: str, source_class: type):
        """Register a custom data source"""
        cls._sources[source_type.lower()] = source_class

