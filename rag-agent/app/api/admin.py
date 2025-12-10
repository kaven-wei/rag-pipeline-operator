from fastapi import APIRouter

router = APIRouter()

@router.post("/clear-cache")
async def clear_cache_api():
    # Placeholder for cache clearing logic
    return {"status": "ok", "message": "Cache cleared"}
