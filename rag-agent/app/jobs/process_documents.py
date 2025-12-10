from typing import List

def chunk_text(text: str, chunk_size: int = 1000, overlap: int = 100) -> List[str]:
    chunks = []
    start = 0
    while start < len(text):
        end = start + chunk_size
        chunks.append(text[start:end])
        start += chunk_size - overlap
    return chunks

def process_documents(documents: List[dict]) -> List[dict]:
    """
    Process a list of documents (dict with 'id', 'text', 'metadata').
    Returns a list of chunks (dict with 'id', 'text', 'metadata', 'doc_id').
    """
    processed_chunks = []
    for doc in documents:
        text = doc.get("text", "")
        chunks = chunk_text(text)
        for i, chunk in enumerate(chunks):
            processed_chunks.append({
                "id": f"{doc['id']}_chunk_{i}",
                "text": chunk,
                "metadata": doc.get("metadata", {}),
                "doc_id": doc['id']
            })
    return processed_chunks
