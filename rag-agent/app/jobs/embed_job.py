import asyncio
from typing import List
from app.jobs.process_documents import process_documents
from app.core.embedder import get_embeddings
from app.db.vector_store import vector_store

# Mock function to fetch documents
def fetch_documents(document_set_id: str) -> List[dict]:
    # In a real scenario, this would fetch from S3 or a database
    print(f"Fetching documents for set {document_set_id}...")
    return [
        {"id": "doc1", "text": "Qdrant is a vector database.", "metadata": {"source": "manual"}},
        {"id": "doc2", "text": "RAG stands for Retrieval-Augmented Generation.", "metadata": {"source": "manual"}}
    ]

async def run_embedding_job_async(document_set_id: str):
    print(f"Starting embedding job for DocumentSet: {document_set_id}")
    
    # 1. Fetch documents
    docs = fetch_documents(document_set_id)
    
    # 2. Process (chunk) documents
    chunks = process_documents(docs)
    print(f"Generated {len(chunks)} chunks.")
    
    # 3. Generate embeddings
    texts = [chunk["text"] for chunk in chunks]
    embeddings = await get_embeddings(texts)
    
    # 4. Save to vector store
    payloads = [{"text": chunk["text"], "metadata": chunk["metadata"], "doc_id": chunk["doc_id"]} for chunk in chunks]
    ids = [chunk["id"] for chunk in chunks] # Qdrant uses UUIDs or integers, or string UUIDs. 
    # Note: Qdrant IDs must be int or UUID. If strings are used, they should be UUIDs. 
    # For simplicity, we'll let Qdrant generate IDs or use UUIDs.
    # But here we are passing string IDs. Qdrant client handles UUID hashing if needed or we should ensure they are UUIDs.
    # Let's generate UUIDs for simplicity or rely on Qdrant's auto-id if we pass None.
    # But to be safe with upsert, we usually need IDs.
    # Let's use uuid5 or just let Qdrant handle it if we don't pass IDs? 
    # upsert requires points with IDs.
    
    import uuid
    point_ids = [str(uuid.uuid5(uuid.NAMESPACE_DNS, chunk_id)) for chunk_id in ids]

    vector_store.ensure_collection()
    vector_store.upsert_vectors(embeddings, payloads, ids=point_ids)
    
    print(f"Successfully indexed {len(chunks)} chunks.")

def run_embedding_job(document_set_id: str):
    asyncio.run(run_embedding_job_async(document_set_id))
