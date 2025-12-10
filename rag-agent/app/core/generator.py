from typing import List, Dict, Any
from app.services.llm_service import llm_service

async def llm_generate(query: str, context_docs: List[Dict[str, Any]]) -> str:
    context_text = "\n\n".join([doc["payload"].get("text", "") for doc in context_docs])
    
    prompt = f"""Use the following pieces of context to answer the question at the end. If you don't know the answer, just say that you don't know, don't try to make up an answer.

Context:
{context_text}

Question: {query}
Answer:"""

    return await llm_service.generate_response(prompt)
