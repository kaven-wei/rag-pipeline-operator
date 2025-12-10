import sys
import os

# Add the parent directory to sys.path to allow imports from app
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from app.jobs.index_job import run_index_job

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python run_index_job.py <index_id>")
        sys.exit(1)
        
    index_id = sys.argv[1]
    run_index_job(index_id)
