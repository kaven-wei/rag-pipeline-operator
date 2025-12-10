**使用 Kubebuilder 生成并扩展的 RAG Pipeline Operator 项目结构**。  
它是基于 `kubebuilder init` + `kubebuilder create api` 后的 scaffold，再结合三个 CRD（DocumentSet / EmbeddingJob / IndexJob）形成的完整工程结构。

---

# 📦 **RAG Pipeline Operator — Kubebuilder 项目结构**
```plain
rag-operator/
├── Makefile
├── PROJECT
├── Dockerfile
├── go.mod
├── go.sum
├── config/
│   ├── default/
│   │   ├── kustomization.yaml
│   │   ├── manager_auth_proxy_patch.yaml
│   │   └── manager_resources_patch.yaml
│   ├── manager/
│   │   └── kustomization.yaml
│   ├── rbac/
│   │   ├── documentset_editor_role.yaml
│   │   ├── documentset_viewer_role.yaml
│   │   ├── embeddingjob_editor_role.yaml
│   │   ├── embeddingjob_viewer_role.yaml
│   │   ├── indexjob_editor_role.yaml
│   │   ├── indexjob_viewer_role.yaml
│   │   ├── role.yaml
│   │   ├── leader_election_role.yaml
│   │   └── role_binding.yaml
│   ├── samples/
│   │   ├── rag_v1alpha1_documentset.yaml
│   │   ├── rag_v1alpha1_embeddingjob.yaml
│   │   └── rag_v1alpha1_indexjob.yaml
│   ├── crd/
│   │   ├── patches/
│   │   │   ├── webhook_in_documentsets.yaml
│   │   │   ├── conversion_webhook_patch.yaml
│   │   │   └── kustomization.yaml
│   │   ├── bases/
│   │   │   ├── rag.ai_documentsets.yaml
│   │   │   ├── rag.ai_embeddingjobs.yaml
│   │   │   └── rag.ai_indexjobs.yaml
│   │   └── kustomization.yaml
│   ├── webhook/
│   │   ├── kustomization.yaml
│   │   ├── documentset_mutating_webhook.yaml
│   │   ├── documentset_validating_webhook.yaml
│   │   ├── embeddingjob_validating_webhook.yaml
│   │   └── indexjob_validating_webhook.yaml
│   └── kustomization.yaml
├── api/
│   └── v1alpha1/
│       ├── documentset_types.go
│       ├── embeddingjob_types.go
│       ├── indexjob_types.go
│       ├── groupversion_info.go
│       ├── zz_generated.deepcopy.go
│       └── webhook.go (可选)
└── controllers/
    ├── documentset_controller.go
    ├── embeddingjob_controller.go
    ├── indexjob_controller.go
    ├── suite_test.go
    └── helpers/
        ├── vector_db_client.go
        ├── job_builder.go
        ├── condition_updater.go
        ├── chunking_logic.go
        └── pipeline_orchestration.go
```

---

# 🔍 **目录结构说明 + 功能设计**
下面逐层解释这个结构如何帮助你构建 RAG Operator。

---

# 1️⃣ 顶层目录
| 文件 | 功能 |
| --- | --- |
| **Makefile** | 构建 Operator、生成 CRD、运行测试 |
| **PROJECT** | Kubebuilder 工程描述文件 |
| **Dockerfile** | 构建 manager 的镜像 |
| **go.mod** | Go module |


---

# 2️⃣ config/（部署相关）
## ✓ config/crd
存放所有生成的 **CRD YAML**，包括：

```plain
rag.ai_documentsets.yaml
rag.ai_embeddingjobs.yaml
rag.ai_indexjobs.yaml
```

这些是 `make manifests` 自动生成的。

---

## ✓ config/samples
示例 CR，便于开发与测试：

```plain
rag_v1alpha1_documentset.yaml
rag_v1alpha1_embeddingjob.yaml
rag_v1alpha1_indexjob.yaml
```

你可以直接：

```plain
kubectl apply -f config/samples/
```

验证 Reconcile 流程。

---

# 3️⃣ api/v1alpha1（CRD 类型代码）
包含三个 CRD 的结构体：

```plain
documentset_types.go
embeddingjob_types.go
indexjob_types.go
```

它们定义：

+ Spec
+ Status
+ Condition
+ +kubebuilder annotations

kubebuilder 会根据这些自动生成 CRD YAML。

---

# 4️⃣ controllers/（核心逻辑）
这里是 **RAG Pipeline Operator 的大脑**：

---

## **documentset_controller.go**
执行逻辑：

+ 监听 DocumentSet
+ 创建 EmbeddingJob
+ 监控 EmbeddingJob 状态 → 决定创建 IndexJob
+ 更新 DocumentSet Status
+ 完成后标记 DocumentSet Ready

---

## **embeddingjob_controller.go**
负责：

+ 启动 embedding Pod / Job
+ 将进度写入 Status
+ 成功后更新 DocumentSet.Condition

---

## **indexjob_controller.go**
负责：

+ 调用 VectorDB 客户端（如 Milvus、Qdrant）
+ 构建索引 / 重建索引
+ 写入状态到 DocumentSet

---

## helpers/
建议你分离一些工具类：

```plain
vector_db_client.go         // 调用 Milvus/Qdrant SDK
job_builder.go              // 生成 Kubernetes Job Spec
condition_updater.go        // 更新 status 条件
chunking_logic.go           // 按 DocumentSet 配置执行 chunking
pipeline_orchestration.go   // 统一 RAG Pipeline 状态机
```

让你的 controller 代码结构更清晰。

---

# 🔥 项目初始化过程（生成方式）
你可以按以下步骤生成：

---

## (1) 初始化项目
```bash
kubebuilder init --domain=rag.ai --owner "your-company"
```

---

## (2) 创建三个 API（带 CRD + Controller）
```bash
kubebuilder create api --group rag --version v1alpha1 --kind DocumentSet
kubebuilder create api --group rag --version v1alpha1 --kind EmbeddingJob
kubebuilder create api --group rag --version v1alpha1 --kind IndexJob
```

自动生成：

+ api/v1alpha1/*.go
+ controllers/*_controller.go
+ config/crd/bases/*.yaml

---





