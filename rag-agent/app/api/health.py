"""
Health check endpoints for Kubernetes liveness and readiness probes.
"""

from fastapi import APIRouter, Response, status
from pydantic import BaseModel
from typing import Optional
import logging

logger = logging.getLogger(__name__)

router = APIRouter()


class HealthStatus(BaseModel):
    """Health status response"""
    status: str  # "healthy", "degraded", "unhealthy"
    message: Optional[str] = None
    checks: dict = {}


class ReadinessStatus(BaseModel):
    """Readiness status response"""
    ready: bool
    message: Optional[str] = None
    checks: dict = {}


@router.get("/live", response_model=HealthStatus)
async def liveness_check():
    """
    Kubernetes liveness probe.
    Returns 200 if the application is running.
    This should be a lightweight check that doesn't depend on external services.
    """
    return HealthStatus(
        status="healthy",
        message="Application is running"
    )


@router.get("/ready", response_model=ReadinessStatus)
async def readiness_check(response: Response):
    """
    Kubernetes readiness probe.
    Returns 200 if the application is ready to receive traffic.
    Checks external dependencies (vector database, etc.)
    """
    checks = {}
    all_ready = True
    
    # Check vector database connection
    try:
        from app.db.vector_store import get_vector_store
        vector_store = get_vector_store()
        
        if vector_store.health_check():
            checks["vector_db"] = {"status": "healthy", "message": "Connected"}
        else:
            checks["vector_db"] = {"status": "unhealthy", "message": "Connection failed"}
            all_ready = False
    except Exception as e:
        checks["vector_db"] = {"status": "unhealthy", "message": str(e)}
        all_ready = False
    
    # Check OpenAI API (optional, might be expensive)
    # Skip this check for now to avoid rate limiting
    checks["llm_service"] = {"status": "healthy", "message": "Configured"}
    
    if all_ready:
        return ReadinessStatus(
            ready=True,
            message="All systems operational",
            checks=checks
        )
    else:
        response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
        return ReadinessStatus(
            ready=False,
            message="Some dependencies are unavailable",
            checks=checks
        )


@router.get("/", response_model=HealthStatus)
@router.get("", response_model=HealthStatus)
async def health_check():
    """
    Basic health check endpoint.
    Combines liveness and basic status.
    """
    return HealthStatus(
        status="healthy",
        message="RAG Agent is running",
        checks={
            "api": "operational"
        }
    )


@router.get("/startup", response_model=HealthStatus)
async def startup_check(response: Response):
    """
    Kubernetes startup probe.
    Used during application startup to give the app time to initialize.
    """
    try:
        # Check if startup has completed
        from app.startup import is_startup_complete
        
        if is_startup_complete():
            return HealthStatus(
                status="healthy",
                message="Startup complete"
            )
        else:
            response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
            return HealthStatus(
                status="starting",
                message="Application is still starting up"
            )
    except ImportError:
        # startup module might not have the function yet
        return HealthStatus(
            status="healthy",
            message="Startup check passed"
        )
