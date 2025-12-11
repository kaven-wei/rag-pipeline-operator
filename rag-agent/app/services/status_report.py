"""
Status reporting service for communicating job progress to the Kubernetes Operator.

This service reports status by:
1. Writing status to a shared file (for sidecar pattern)
2. Optionally patching CRD status via Kubernetes API (if running in-cluster)
3. Logging for visibility
"""

import os
import json
import logging
from typing import Optional
from datetime import datetime

logger = logging.getLogger(__name__)


class StatusReporter:
    """
    Reports job status to the Kubernetes Operator.
    
    The status can be reported via:
    - File: Write to /tmp/job-status.json (for sidecar to read)
    - K8s API: Patch the CRD status directly (requires RBAC)
    - Logs: Always log status for debugging
    """
    
    def __init__(self):
        self.status_file_path = os.getenv("STATUS_FILE_PATH", "/tmp/job-status.json")
        self.use_k8s_api = os.getenv("USE_K8S_API", "false").lower() == "true"
        self.namespace = os.getenv("DOCUMENT_SET_NAMESPACE", os.getenv("POD_NAMESPACE", "default"))
        
        # Kubernetes client (lazy loaded)
        self._k8s_client = None
        
    @property
    def k8s_client(self):
        """Lazy load Kubernetes client"""
        if self._k8s_client is None and self.use_k8s_api:
            try:
                from kubernetes import client, config
                
                # Try in-cluster config first, fall back to kubeconfig
                try:
                    config.load_incluster_config()
                except config.ConfigException:
                    config.load_kube_config()
                
                self._k8s_client = client.CustomObjectsApi()
            except Exception as e:
                logger.warning(f"Could not initialize Kubernetes client: {e}")
                self._k8s_client = None
                
        return self._k8s_client
    
    async def report_embedding_progress(
        self,
        job_name: str,
        phase: str,
        message: str,
        total_chunks: int = 0,
        processed_chunks: int = 0
    ):
        """
        Report embedding job progress.
        
        Args:
            job_name: Name of the EmbeddingJob or DocumentSet
            phase: Current phase (Pending, Running, Succeeded, Failed)
            message: Status message
            total_chunks: Total number of chunks to process
            processed_chunks: Number of chunks processed
        """
        percentage = 0
        if total_chunks > 0:
            percentage = int((processed_chunks / total_chunks) * 100)
        
        status = {
            "kind": "EmbeddingJob",
            "name": job_name,
            "phase": phase,
            "message": message,
            "progress": {
                "totalChunks": total_chunks,
                "processedChunks": processed_chunks,
                "percentage": percentage
            },
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }
        
        await self._report_status(status)
        
    async def report_index_progress(
        self,
        job_name: str,
        phase: str,
        message: str,
        total_vectors: int = 0,
        indexed_vectors: int = 0,
        alias_swapped: bool = False
    ):
        """
        Report index job progress.
        
        Args:
            job_name: Name of the IndexJob
            phase: Current phase (Pending, Building, Optimizing, Succeeded, Failed)
            message: Status message
            total_vectors: Total number of vectors
            indexed_vectors: Number of vectors indexed
            alias_swapped: Whether alias has been swapped
        """
        percentage = 0
        if total_vectors > 0:
            percentage = int((indexed_vectors / total_vectors) * 100)
        
        status = {
            "kind": "IndexJob",
            "name": job_name,
            "phase": phase,
            "message": message,
            "progress": {
                "totalVectors": total_vectors,
                "indexedVectors": indexed_vectors,
                "percentage": percentage
            },
            "aliasSwapped": alias_swapped,
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }
        
        await self._report_status(status)
        
    async def _report_status(self, status: dict):
        """Internal method to report status via all channels"""
        
        # Always log
        logger.info(f"Status update: {status['kind']}/{status['name']} - {status['phase']}: {status['message']}")
        
        # Write to file
        self._write_status_file(status)
        
        # Optionally patch K8s resource
        if self.use_k8s_api:
            await self._patch_k8s_status(status)
    
    def _write_status_file(self, status: dict):
        """Write status to file for sidecar to read"""
        try:
            # Ensure directory exists
            os.makedirs(os.path.dirname(self.status_file_path), exist_ok=True)
            
            with open(self.status_file_path, 'w') as f:
                json.dump(status, f, indent=2)
                
            logger.debug(f"Wrote status to {self.status_file_path}")
        except Exception as e:
            logger.warning(f"Could not write status file: {e}")
    
    async def _patch_k8s_status(self, status: dict):
        """Patch the Kubernetes CRD status"""
        if not self.k8s_client:
            return
            
        try:
            kind = status["kind"]
            name = status["name"]
            
            # Build the status patch
            if kind == "EmbeddingJob":
                patch = {
                    "status": {
                        "phase": status["phase"],
                        "message": status["message"],
                        "progress": status["progress"]
                    }
                }
                plural = "embeddingjobs"
            elif kind == "IndexJob":
                patch = {
                    "status": {
                        "phase": status["phase"],
                        "message": status["message"],
                        "progress": status["progress"],
                        "aliasSwapped": status.get("aliasSwapped", False)
                    }
                }
                plural = "indexjobs"
            else:
                logger.warning(f"Unknown kind: {kind}")
                return
            
            # Patch the resource
            self.k8s_client.patch_namespaced_custom_object_status(
                group="rag.ai",
                version="v1alpha1",
                namespace=self.namespace,
                plural=plural,
                name=name,
                body=patch
            )
            
            logger.debug(f"Patched K8s status for {kind}/{name}")
            
        except Exception as e:
            logger.warning(f"Could not patch K8s status: {e}")


def report_status(kind: str, name: str, status: dict):
    """
    Synchronous helper function for reporting status.
    
    Args:
        kind: Resource kind (EmbeddingJob, IndexJob)
        name: Resource name
        status: Status dict with phase, message, etc.
    """
    import asyncio
    
    reporter = StatusReporter()
    
    full_status = {
        "kind": kind,
        "name": name,
        **status,
        "timestamp": datetime.utcnow().isoformat() + "Z"
    }
    
    # Run async method
    try:
        loop = asyncio.get_event_loop()
    except RuntimeError:
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
    
    loop.run_until_complete(reporter._report_status(full_status))
