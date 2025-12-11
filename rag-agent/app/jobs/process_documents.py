"""
Document processing utilities for chunking and text preparation.
"""

from typing import List, Dict, Any, Optional
import re
import logging

logger = logging.getLogger(__name__)


def chunk_text(
    text: str, 
    chunk_size: int = 512, 
    overlap: int = 100,
    separator: str = "\n\n"
) -> List[str]:
    """
    Split text into chunks with overlap.
    
    Args:
        text: Text to split
        chunk_size: Maximum size of each chunk in characters
        overlap: Number of overlapping characters between chunks
        separator: Preferred separator for splitting (tries to split on this first)
        
    Returns:
        List of text chunks
    """
    if not text or not text.strip():
        return []
    
    # Clean the text
    text = text.strip()
    
    # If text is smaller than chunk_size, return as is
    if len(text) <= chunk_size:
        return [text]
    
    chunks = []
    
    # Try to split on natural boundaries first
    paragraphs = text.split(separator)
    current_chunk = ""
    
    for para in paragraphs:
        para = para.strip()
        if not para:
            continue
            
        # If adding this paragraph exceeds chunk size
        if len(current_chunk) + len(para) + len(separator) > chunk_size:
            if current_chunk:
                chunks.append(current_chunk.strip())
                # Start new chunk with overlap from end of previous
                if overlap > 0 and len(current_chunk) > overlap:
                    current_chunk = current_chunk[-overlap:] + separator + para
                else:
                    current_chunk = para
            else:
                # Paragraph itself is too long, split it
                para_chunks = _split_long_text(para, chunk_size, overlap)
                chunks.extend(para_chunks[:-1])
                current_chunk = para_chunks[-1] if para_chunks else ""
        else:
            if current_chunk:
                current_chunk += separator + para
            else:
                current_chunk = para
    
    # Don't forget the last chunk
    if current_chunk:
        chunks.append(current_chunk.strip())
    
    return [c for c in chunks if c.strip()]


def _split_long_text(text: str, chunk_size: int, overlap: int) -> List[str]:
    """Split a long text that doesn't have natural separators"""
    chunks = []
    start = 0
    
    while start < len(text):
        end = start + chunk_size
        
        # Try to find a good breaking point (sentence end, word boundary)
        if end < len(text):
            # Look for sentence end
            best_break = -1
            for punct in ['. ', '! ', '? ', '\n']:
                idx = text.rfind(punct, start, end)
                if idx > best_break:
                    best_break = idx + len(punct)
            
            # If no sentence break, try word boundary
            if best_break <= start:
                space_idx = text.rfind(' ', start, end)
                if space_idx > start:
                    best_break = space_idx + 1
                else:
                    best_break = end
            
            end = best_break
        
        chunk = text[start:end].strip()
        if chunk:
            chunks.append(chunk)
        
        # Move start with overlap
        start = max(start + 1, end - overlap)
    
    return chunks


def process_documents(
    documents: List[Dict[str, Any]], 
    chunk_size: int = 512, 
    overlap: int = 100,
    format: str = "text"
) -> List[Dict[str, Any]]:
    """
    Process a list of documents into chunks suitable for embedding.
    
    Args:
        documents: List of documents with 'id', 'text', 'metadata' fields
        chunk_size: Size of each chunk in characters
        overlap: Overlap between chunks
        format: Document format (text, markdown, html) for preprocessing
        
    Returns:
        List of chunks with 'id', 'text', 'metadata', 'doc_id' fields
    """
    processed_chunks = []
    
    for doc in documents:
        doc_id = doc.get("id", "unknown")
        text = doc.get("text", "")
        metadata = doc.get("metadata", {})
        
        # Preprocess based on format
        if format == "html":
            text = _strip_html(text)
        elif format == "markdown":
            text = _process_markdown(text)
        
        # Clean the text
        text = _clean_text(text)
        
        if not text:
            logger.warning(f"Document {doc_id} has no text content after processing")
            continue
        
        # Chunk the text
        chunks = chunk_text(text, chunk_size=chunk_size, overlap=overlap)
        
        # Create chunk documents
        for i, chunk_text_content in enumerate(chunks):
            chunk_id = f"{doc_id}_chunk_{i}"
            processed_chunks.append({
                "id": chunk_id,
                "text": chunk_text_content,
                "metadata": {
                    **metadata,
                    "chunk_index": i,
                    "total_chunks": len(chunks),
                },
                "doc_id": doc_id,
                "chunk_index": i
            })
    
    logger.info(f"Processed {len(documents)} documents into {len(processed_chunks)} chunks")
    return processed_chunks


def _clean_text(text: str) -> str:
    """Clean and normalize text"""
    # Remove excessive whitespace
    text = re.sub(r'\s+', ' ', text)
    # Remove control characters except newlines
    text = re.sub(r'[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\x9f]', '', text)
    return text.strip()


def _strip_html(text: str) -> str:
    """Remove HTML tags from text"""
    # Remove script and style elements
    text = re.sub(r'<script[^>]*>.*?</script>', '', text, flags=re.DOTALL | re.IGNORECASE)
    text = re.sub(r'<style[^>]*>.*?</style>', '', text, flags=re.DOTALL | re.IGNORECASE)
    # Remove HTML tags
    text = re.sub(r'<[^>]+>', ' ', text)
    # Decode HTML entities
    import html
    text = html.unescape(text)
    return text


def _process_markdown(text: str) -> str:
    """Process markdown to plain text while preserving structure"""
    # Remove code blocks (keep content)
    text = re.sub(r'```[\s\S]*?```', '', text)
    text = re.sub(r'`([^`]+)`', r'\1', text)
    
    # Remove markdown links but keep text
    text = re.sub(r'\[([^\]]+)\]\([^)]+\)', r'\1', text)
    
    # Remove images
    text = re.sub(r'!\[([^\]]*)\]\([^)]+\)', '', text)
    
    # Remove headers markers but keep text
    text = re.sub(r'^#{1,6}\s+', '', text, flags=re.MULTILINE)
    
    # Remove bold/italic markers
    text = re.sub(r'\*\*([^*]+)\*\*', r'\1', text)
    text = re.sub(r'\*([^*]+)\*', r'\1', text)
    text = re.sub(r'__([^_]+)__', r'\1', text)
    text = re.sub(r'_([^_]+)_', r'\1', text)
    
    return text
