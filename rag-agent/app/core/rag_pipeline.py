from app.core.retriever import vector_search
from app.core.generator import llm_generate

async def rag_pipeline(query: str):
    docs = await vector_search(query)
    answer = await llm_generate(query, docs)
    return {
        "query": query,
        "documents": docs,
        "answer": answer
    }
