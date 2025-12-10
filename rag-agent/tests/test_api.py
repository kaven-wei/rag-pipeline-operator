from fastapi.testclient import TestClient
from app.main import app

client = TestClient(app)

def test_health_check():
    response = client.get("/health")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}

def test_query_api_mock():
    # We need to mock the rag_pipeline or its dependencies to test this without real services
    # For now, we just check if the endpoint exists and handles bad input
    response = client.post("/rag/query", json={})
    assert response.status_code == 422  # Missing query field

    # To test success, we'd need to mock app.core.rag_pipeline.rag_pipeline
