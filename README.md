# RAG Pipeline Operator

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.25+-blue.svg)](https://kubernetes.io/)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://golang.org/)
[![Python](https://img.shields.io/badge/Python-3.10+-3776AB.svg)](https://www.python.org/)

A Kubernetes Operator for automating and managing RAG (Retrieval-Augmented Generation) system pipelines, including dataset management, text chunking, embedding generation, vector database indexing, and zero-downtime service updates.

## 📋 Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [Custom Resource Definitions](#custom-resource-definitions)
- [Project Structure](#project-structure)
- [Development](#development)
- [Examples](#examples)
- [Contributing](#contributing)
- [License](#license)

## 🎯 Overview

RAG Pipeline Operator leverages Kubernetes' declarative API and control loop pattern to automate the complete lifecycle of RAG systems. It orchestrates complex workflows involving data ingestion, embedding generation, vector indexing, and intelligent service updates without downtime.

### Why RAG Pipeline Operator?

- **Declarative Management**: Define your RAG pipeline as Kubernetes Custom Resources
- **Automated Orchestration**: Automatic dependency management between data processing stages
- **Zero-Downtime Updates**: Seamless index updates using alias swap mechanism
- **Production Ready**: Built on Kubebuilder with best practices for Kubernetes operators
- **Flexible Architecture**: Support for multiple vector databases (Milvus, Qdrant, Weaviate) and embedding models

## ✨ Features

### Core Capabilities

- **📦 Dataset Management**: Declarative dataset definitions with support for S3, HTTP, Git, and PVC sources
- **✂️ Automatic Text Chunking**: Configurable text splitting with overlap support
- **🧠 Embedding Generation**: Batch processing with multiple model support (BGE, OpenAI, etc.)
- **🔍 Vector Database Integration**: Native support for Milvus, Qdrant, and Weaviate
- **🔄 Index Rebuild & Hot Reload**: Zero-downtime index updates using alias swap
- **📊 Status Tracking**: Comprehensive status reporting and condition management
- **🔐 Secret Management**: Secure credential handling via Kubernetes Secrets

### Workflow Automation

1. **DocumentSet CRD**: Define data sources and processing configurations
2. **EmbeddingJob**: Automatically triggered for text chunking and vector generation
3. **IndexJob**: Builds optimized vector indexes (HNSW, IVF, etc.)
4. **Alias Swap**: Atomic switching to new indexes without service interruption
5. **RAG Service**: Query API with automatic awareness of latest data

## 🏗️ Architecture

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                        │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │           RAG Pipeline Operator (Go)                  │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐     │  │
│  │  │ DocumentSet│  │ Embedding  │  │   Index    │     │  │
│  │  │ Controller │  │ Controller │  │ Controller │     │  │
│  │  └────────────┘  └────────────┘  └────────────┘     │  │
│  └──────────────────────────────────────────────────────┘  │
│                           │                                  │
│                           ▼                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Python RAG Agent (Jobs & Service)        │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐     │  │
│  │  │ Embedding  │  │   Index    │  │    RAG     │     │  │
│  │  │    Job     │  │    Job     │  │  Service   │     │  │
│  │  └────────────┘  └────────────┘  └────────────┘     │  │
│  └──────────────────────────────────────────────────────┘  │
│                           │                                  │
│                           ▼                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Vector Database (Milvus/Qdrant)          │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Workflow Diagram

```
User
  │
  ├─► (1) Submit DocumentSet CRD
  │         │
  │         ▼
  │    Go Operator detects DocumentSet
  │         │
  │         ├─► (2) Create EmbeddingJob
  │         │         │
  │         │         ▼
  │         │    Python EmbeddingJob Pod
  │         │         ├─► Fetch data
  │         │         ├─► Chunking
  │         │         ├─► Generate embeddings
  │         │         └─► Write to temp collection (e.g., manuals_v1_20231027)
  │         │
  │         ├─► (3) Detect embedding completion
  │         │
  │         ├─► (4) Create IndexJob
  │         │         │
  │         │         ▼
  │         │    Python IndexJob Pod
  │         │         ├─► Build index (HNSW/IVF)
  │         │         └─► Alias Swap: manuals_prod → manuals_v1_20231027
  │         │
  │         └─► (5) Update DocumentSet status to Ready
  │
  └─► (6) RAG Service queries via alias (manuals_prod)
            └─► Zero-downtime access to latest data
```

## 🚀 Quick Start

### Prerequisites

- Kubernetes cluster (v1.25+)
- kubectl configured
- Go 1.21+ (for operator development)
- Python 3.10+ (for RAG agent development)
- Vector database (Milvus/Qdrant) deployed

### Installation

1. **Install CRDs**

```bash
kubectl apply -f config/crd/bases/
```

2. **Deploy the Operator**

```bash
kubectl apply -f config/manager/
```

3. **Deploy RAG Agent Service**

```bash
kubectl apply -f rag-agent/deploy/
```

### Create Your First RAG Pipeline

1. **Create a DocumentSet**

```bash
kubectl apply -f - <<EOF
apiVersion: rag.ai/v1alpha1
kind: DocumentSet
metadata:
  name: product-manuals
spec:
  source:
    type: s3
    uri: s3://docs-bucket/manuals/
    secretRef:
      name: s3-credentials
  chunking:
    size: 512
    overlap: 100
    format: text
  embedding:
    model: bge-large-en
    batchSize: 16
  index:
    vectorDB: milvus
    collection: manuals_v1
    alias: manuals_prod
EOF
```

2. **Monitor Progress**

```bash
kubectl get documentset product-manuals -w
```

3. **Query the RAG Service**

```bash
curl -X POST http://rag-service/rag/query \
  -H "Content-Type: application/json" \
  -d '{"query": "How do I reset the device?"}'
```

## 📚 Custom Resource Definitions

### DocumentSet

Defines a dataset and its processing pipeline.

**Key Fields:**
- `source`: Data source configuration (S3, HTTP, Git, PVC)
- `chunking`: Text splitting parameters
- `embedding`: Embedding model and batch settings
- `index`: Vector database and indexing strategy

**Status Phases:**
- `Pending`: Initial state
- `Chunked`: Text chunking completed
- `Embedding`: Embedding generation in progress
- `Indexing`: Index building in progress
- `Ready`: Pipeline ready for queries
- `Failed`: Error occurred

[View Full Spec](docs/CRD%20设计.md#documentset-crd)

### EmbeddingJob

Represents a batch embedding generation task.

**Key Fields:**
- `documentSet`: Reference to parent DocumentSet
- `retryPolicy`: Retry configuration

**Status:**
- Progress tracking (chunks processed/total)
- Start/completion timestamps
- Detailed conditions

[View Full Spec](docs/CRD%20设计.md#embeddingjob-crd)

### IndexJob

Manages vector index building and optimization.

**Key Fields:**
- `documentSet`: Reference to parent DocumentSet
- `vectorDB`: Target vector database configuration
- `indexSpec`: Index type and parameters (HNSW, IVF_FLAT, IVF_PQ)

**Status:**
- Index building progress
- Optimization status
- Alias swap completion

[View Full Spec](docs/CRD%20设计.md#indexjob-crd)

## 📁 Project Structure

```
rag-pipeline-operator/
├── rag-operator/              # Go-based Kubernetes Operator
│   ├── api/v1alpha1/          # CRD type definitions
│   │   ├── documentset_types.go
│   │   ├── embeddingjob_types.go
│   │   └── indexjob_types.go
│   ├── controllers/           # Reconciliation logic
│   │   ├── documentset_controller.go
│   │   ├── embeddingjob_controller.go
│   │   ├── indexjob_controller.go
│   │   └── helpers/           # Utility functions
│   ├── config/                # Kubernetes manifests
│   │   ├── crd/               # CRD definitions
│   │   ├── rbac/              # RBAC rules
│   │   └── samples/           # Example CRs
│   └── Dockerfile
│
├── rag-agent/                 # Python RAG Agent
│   ├── app/
│   │   ├── api/               # FastAPI routes
│   │   │   ├── query.py       # /rag/query endpoint
│   │   │   ├── health.py      # Health checks
│   │   │   └── admin.py       # Admin endpoints
│   │   ├── core/              # Core RAG logic
│   │   │   ├── rag_pipeline.py
│   │   │   ├── retriever.py
│   │   │   ├── generator.py
│   │   │   └── embedder.py
│   │   ├── jobs/              # Operator-triggered jobs
│   │   │   ├── embed_job.py
│   │   │   └── index_job.py
│   │   └── db/                # Vector database clients
│   ├── scripts/               # Job entry points
│   ├── Dockerfile
│   └── requirements.txt
│
└── docs/                      # Documentation
    ├── 项目需求描述.md
    ├── CRD 设计.md
    ├── Operator 项目结构（kubebuilder 脚手架）.md
    └── Python RAG agent 结构 + FastAPI 服务模板.md
```

## 🛠️ Development

### Building the Operator

```bash
cd rag-operator

# Generate CRDs and manifests
make manifests

# Build operator binary
make build

# Build and push Docker image
make docker-build docker-push IMG=<your-registry>/rag-operator:tag
```

### Running Locally

```bash
# Install CRDs
make install

# Run operator locally
make run
```

### Building the RAG Agent

```bash
cd rag-agent

# Install dependencies
pip install -r requirements.txt

# Run locally
uvicorn app.main:app --reload

# Build Docker image
docker build -t <your-registry>/rag-agent:tag .
```

### Testing

```bash
# Operator tests
cd rag-operator
make test

# RAG Agent tests
cd rag-agent
pytest tests/
```

## 📖 Examples

### Example 1: S3 Data Source with BGE Embeddings

```yaml
apiVersion: rag.ai/v1alpha1
kind: DocumentSet
metadata:
  name: technical-docs
spec:
  source:
    type: s3
    uri: s3://my-bucket/technical-docs/
    secretRef:
      name: aws-credentials
  chunking:
    size: 512
    overlap: 100
    format: markdown
  embedding:
    model: bge-large-en
    device: gpu
    batchSize: 32
  index:
    vectorDB: milvus
    collection: tech_docs_v1
    alias: tech_docs_prod
    recreate: false
```

### Example 2: Manual EmbeddingJob

```yaml
apiVersion: rag.ai/v1alpha1
kind: EmbeddingJob
metadata:
  name: manual-embedding-job
spec:
  documentSet: technical-docs
  retryPolicy:
    maxRetries: 3
    backoffSeconds: 30
```

### Example 3: Custom Index Configuration

```yaml
apiVersion: rag.ai/v1alpha1
kind: IndexJob
metadata:
  name: hnsw-index-job
spec:
  documentSet: technical-docs
  vectorDB:
    type: milvus
    collection: tech_docs_v1_20231027
    targetAlias: tech_docs_prod
  indexSpec:
    type: HNSW
    parameters:
      efConstruction: 200
      M: 16
```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Development Workflow

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Code Style

- **Go**: Follow standard Go conventions and use `gofmt`
- **Python**: Follow PEP 8 and use `black` for formatting

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

```
Copyright 2024 RAG Pipeline Operator Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

## 🙏 Acknowledgments

- Built with [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)
- Powered by [FastAPI](https://fastapi.tiangolo.com/)
- Vector database support: [Milvus](https://milvus.io/), [Qdrant](https://qdrant.tech/)
- Embedding models: [BGE](https://huggingface.co/BAAI/bge-large-en), [OpenAI](https://openai.com/)

## 📞 Support

- 📖 [Documentation](docs/)
- 🐛 [Issue Tracker](https://github.com/kaven-wei/rag-pipeline-operator/issues)
- 💬 [Discussions](https://github.com/kaven-wei/rag-pipeline-operator/discussions)

---

**Made with ❤️ for the RAG community**
