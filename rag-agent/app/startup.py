from app.db.vector_store import vector_store

async def startup_event():
    # Ensure Qdrant collection exists
    vector_store.ensure_collection()
    print("Startup complete: Vector store collection ensured.")
