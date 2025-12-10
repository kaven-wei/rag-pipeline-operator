# RAG Agent

## How to Run

### 1. Install Dependencies
```bash
pip install -r requirements.txt
```

### 2. Configure Environment
Create a `.env` file in the `rag-agent/` directory with the following content:
```env
OPENAI_API_KEY=your_openai_api_key
QDRANT_URL=http://localhost:6333
# QDRANT_API_KEY=your_qdrant_key (optional)
```

### 3. Run the API Server
```bash
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```
Access Swagger UI at `http://localhost:8000/docs`.

### 4. Run Offline Jobs
```bash
# Run Embedding Job
python scripts/run_embedding_job.py <document_set_id>

# Run Index Job
python scripts/run_index_job.py <index_id>
```
