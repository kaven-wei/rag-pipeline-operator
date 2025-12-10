from openai import AsyncOpenAI
from app.config.settings import settings
from typing import List

class LLMService:
    def __init__(self):
        self.client = AsyncOpenAI(api_key=settings.OPENAI_API_KEY)
        self.model = settings.OPENAI_MODEL
        self.embedding_model = settings.OPENAI_EMBEDDING_MODEL

    async def get_embedding(self, text: str) -> List[float]:
        text = text.replace("\n", " ")
        response = await self.client.embeddings.create(input=[text], model=self.embedding_model)
        return response.data[0].embedding

    async def get_embeddings(self, texts: List[str]) -> List[List[float]]:
        texts = [text.replace("\n", " ") for text in texts]
        response = await self.client.embeddings.create(input=texts, model=self.embedding_model)
        return [data.embedding for data in response.data]

    async def generate_response(self, prompt: str, system_prompt: str = "You are a helpful assistant.") -> str:
        response = await self.client.chat.completions.create(
            model=self.model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": prompt}
            ]
        )
        return response.choices[0].message.content

llm_service = LLMService()
