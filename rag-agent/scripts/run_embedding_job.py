import sys
import os

# Add the parent directory to sys.path to allow imports from app
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from app.jobs.embed_job import run_embedding_job

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python run_embedding_job.py <document_set_id>")
        sys.exit(1)
        
    document_set_id = sys.argv[1]
    run_embedding_job(document_set_id)
