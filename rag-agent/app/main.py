from fastapi import FastAPI
from app.api import query, health, admin
from app.startup import startup_event
from app.config.settings import settings

app = FastAPI(title=settings.APP_NAME, version=settings.APP_VERSION)

app.include_router(health.router, prefix="/health", tags=["Health"])
app.include_router(query.router, prefix="/rag", tags=["RAG"])
app.include_router(admin.router, prefix="/admin", tags=["Admin"])

@app.on_event("startup")
async def startup():
    await startup_event()

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
