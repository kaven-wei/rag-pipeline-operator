from fastapi import APIRouter
from pydantic import BaseModel
from app.core.rag_pipeline import rag_pipeline

router = APIRouter()

class QueryRequest(BaseModel):
    query: str

@router.post("/query")
async def query_api(req: QueryRequest):
    return await rag_pipeline(req.query)
