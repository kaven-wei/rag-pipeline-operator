"""
LLM Service for interacting with OpenAI or compatible APIs.
Handles embeddings generation and chat completions.
"""

import logging
import asyncio
from typing import List, Optional
from openai import AsyncOpenAI, APIError, RateLimitError, APIConnectionError
from app.config.settings import settings

logger = logging.getLogger(__name__)


class LLMServiceError(Exception):
    """Custom exception for LLM service errors"""
    pass


class LLMService:
    """
    Service for LLM operations with retry logic and error handling.
    """
    
    def __init__(self):
        """Initialize the LLM service"""
        self.api_key = settings.OPENAI_API_KEY
        self.model = settings.OPENAI_MODEL
        self.embedding_model = settings.OPENAI_EMBEDDING_MODEL
        self.max_tokens = settings.OPENAI_MAX_TOKENS
        self.temperature = settings.OPENAI_TEMPERATURE
        
        # Initialize client
        client_kwargs = {"api_key": self.api_key}
        if settings.OPENAI_API_BASE:
            client_kwargs["base_url"] = settings.OPENAI_API_BASE
            
        self.client = AsyncOpenAI(**client_kwargs)
        
        # Retry configuration
        self.max_retries = settings.JOB_MAX_RETRIES
        self.retry_backoff = settings.JOB_RETRY_BACKOFF
        
    async def _retry_with_backoff(self, func, *args, **kwargs):
        """
        Execute a function with exponential backoff retry.
        
        Args:
            func: Async function to execute
            *args, **kwargs: Arguments to pass to the function
            
        Returns:
            Function result
            
        Raises:
            LLMServiceError: If all retries fail
        """
        last_error = None
        
        for attempt in range(self.max_retries + 1):
            try:
                return await func(*args, **kwargs)
            except RateLimitError as e:
                last_error = e
                wait_time = self.retry_backoff * (2 ** attempt)
                logger.warning(f"Rate limit hit, waiting {wait_time}s before retry {attempt + 1}/{self.max_retries}")
                await asyncio.sleep(wait_time)
            except APIConnectionError as e:
                last_error = e
                wait_time = self.retry_backoff * (2 ** attempt)
                logger.warning(f"API connection error, waiting {wait_time}s before retry {attempt + 1}/{self.max_retries}")
                await asyncio.sleep(wait_time)
            except APIError as e:
                last_error = e
                if e.status_code >= 500:
                    # Server error, retry
                    wait_time = self.retry_backoff * (2 ** attempt)
                    logger.warning(f"API error {e.status_code}, waiting {wait_time}s before retry")
                    await asyncio.sleep(wait_time)
                else:
                    # Client error, don't retry
                    raise LLMServiceError(f"API error: {str(e)}") from e
        
        raise LLMServiceError(f"Max retries exceeded: {str(last_error)}") from last_error

    async def get_embedding(self, text: str) -> List[float]:
        """
        Generate embedding for a single text.
        
        Args:
            text: Text to embed
            
        Returns:
            Embedding vector
        """
        if not self.api_key:
            raise LLMServiceError("OPENAI_API_KEY not configured")
        
        # Clean text
        text = text.replace("\n", " ").strip()
        if not text:
            raise LLMServiceError("Empty text provided for embedding")
        
        async def _get_embedding():
            response = await self.client.embeddings.create(
                input=[text],
                model=self.embedding_model
            )
            return response.data[0].embedding
        
        return await self._retry_with_backoff(_get_embedding)

    async def get_embeddings(self, texts: List[str]) -> List[List[float]]:
        """
        Generate embeddings for multiple texts.
        
        Args:
            texts: List of texts to embed
            
        Returns:
            List of embedding vectors
        """
        if not self.api_key:
            raise LLMServiceError("OPENAI_API_KEY not configured")
        
        if not texts:
            return []
        
        # Clean texts
        cleaned_texts = [t.replace("\n", " ").strip() for t in texts]
        
        # Filter out empty texts
        non_empty_indices = [i for i, t in enumerate(cleaned_texts) if t]
        non_empty_texts = [cleaned_texts[i] for i in non_empty_indices]
        
        if not non_empty_texts:
            raise LLMServiceError("All texts are empty")
        
        async def _get_embeddings():
            response = await self.client.embeddings.create(
                input=non_empty_texts,
                model=self.embedding_model
            )
            return [data.embedding for data in response.data]
        
        embeddings = await self._retry_with_backoff(_get_embeddings)
        
        # Reconstruct full list with None for empty texts
        result = [None] * len(texts)
        for i, idx in enumerate(non_empty_indices):
            result[idx] = embeddings[i]
        
        # Replace None with zero vectors (or raise error)
        vector_dim = len(embeddings[0]) if embeddings else 1536
        result = [e if e is not None else [0.0] * vector_dim for e in result]
        
        return result

    async def generate_response(
        self, 
        prompt: str, 
        system_prompt: str = "You are a helpful assistant.",
        temperature: Optional[float] = None,
        max_tokens: Optional[int] = None
    ) -> str:
        """
        Generate a chat completion response.
        
        Args:
            prompt: User prompt
            system_prompt: System prompt for context
            temperature: Override default temperature
            max_tokens: Override default max tokens
            
        Returns:
            Generated response text
        """
        if not self.api_key:
            raise LLMServiceError("OPENAI_API_KEY not configured")
        
        async def _generate():
            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": prompt}
                ],
                temperature=temperature or self.temperature,
                max_tokens=max_tokens or self.max_tokens
            )
            return response.choices[0].message.content
        
        return await self._retry_with_backoff(_generate)

    async def health_check(self) -> bool:
        """
        Check if the LLM service is available.
        
        Returns:
            True if service is healthy
        """
        if not self.api_key:
            return False
        
        try:
            # Use a minimal request to check connectivity
            await self.client.models.list()
            return True
        except Exception as e:
            logger.warning(f"LLM health check failed: {e}")
            return False


# Global singleton
llm_service = LLMService()
