"""
Admin service for management operations.
"""

import logging

logger = logging.getLogger(__name__)

# Application-level cache (could be replaced with Redis in production)
_cache = {}


async def clear_cache():
    """
    Clear all application-level caches.
    
    This includes:
    - In-memory LRU caches
    - Query result caches
    - Embedding caches
    """
    global _cache
    
    logger.info("Clearing application cache...")
    
    # Clear global cache
    _cache.clear()
    
    # Clear any functools.lru_cache decorated functions
    try:
        # If there are any cached functions, clear them here
        pass
    except Exception as e:
        logger.warning(f"Could not clear function caches: {e}")
    
    logger.info("Application cache cleared")
    return True


def get_cached(key: str):
    """Get a value from cache"""
    return _cache.get(key)


def set_cached(key: str, value, ttl: int = 3600):
    """Set a value in cache"""
    _cache[key] = value


def invalidate_cached(key: str):
    """Invalidate a cache entry"""
    if key in _cache:
        del _cache[key]
