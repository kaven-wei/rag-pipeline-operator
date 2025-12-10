**涵盖**：
+ **DocumentSet**（数据集管理）
+ **EmbeddingJob**（向量生成）
+ **IndexJob**（索引构建）
+ 各 CRD 的字段设计、状态机、事件流

设计遵循 **Kubebuilder + Kubernetes API 最佳实践**，字段命名、Status、Condition、Phase 完整规范，可直接用来实现 Operator。

---

# ✅ **1. DocumentSet CRD（数据集）**
DocumentSet 是 RAG Pipeline 的输入，负责表述：

+ 数据来源（S3、OSS、本地 PVC）
+ 文本切片方式
+ 使用哪个 embedding 模型
+ 索引策略

适合定义为 **上游 CR**，Operator 监听它后生成后续的 EmbeddingJob 和 IndexJob。

---

## 🚀 DocumentSet YAML 示例
```yaml
apiVersion: rag.ai/v1alpha1
kind: DocumentSet
metadata:
  name: product-manuals
spec:
  source:
    type: s3
    uri: s3://docs-bucket/manuals/
  chunking:
    size: 512
    overlap: 100
  embedding:
    model: bge-large-en
    batchSize: 16
  index:
    vectorDB: milvus
    collection: manuals_v1
status:
  phase: Pending
  conditions: []
  observedGeneration: 1
```

---

## 🧩 DocumentSet Spec 字段设计（详细版）
```yaml
spec:
  source:
    type: s3 | http | git | pvc
    uri: "s3://bucket/path"
    secretRef:
      name: "my-s3-creds" # 引用 K8s Secret 包含 AWS_ACCESS_KEY_ID 等
  
  chunking:
    size: 512
    overlap: 100
    format: text | markdown | html
  
  embedding:
    model: bge-large-en
    device: cpu | gpu
    batchSize: 16
    autoRetry: true
  
  index:
    vectorDB: milvus | qdrant | weaviate
    collection: "manuals_v1" # 基础名称，实际会创建 manuals_v1_timestamp
    alias: "manuals_prod"    # RAG Service 使用的稳定别名
    recreate: false   # 是否强制重建
```

---

## 🧠 DocumentSet Status 字段
```yaml
status:
  phase: Pending | Chunked | Embedding | Indexing | Ready | Failed
  message: optional text message
  conditions:
    - type: ChunkingCompleted
      status: True/False
      reason:
      message:
    - type: EmbeddingCompleted
    - type: IndexingCompleted
  lastEmbeddingJobRef: embedding-job-xxxx
  lastIndexJobRef: index-job-xxxx
  observedGeneration: int
```

---

---

# ✅ **2. EmbeddingJob CRD（向量生成 Job）**
EmbeddingJob 由 Operator 根据 DocumentSet 自动创建，也可以用户手动创建：

+ 从 DocumentSet 取 chunk 文本
+ 调用 Python embedding 程序生成向量
+ 向向量数据库写入（如 milvus / qdrant）

类似 Kubernetes 的 Job，但状态更细致。

---

## 🚀 EmbeddingJob YAML 示例
```yaml
apiVersion: rag.ai/v1alpha1
kind: EmbeddingJob
metadata:
  name: embedding-job-manuals
spec:
  documentSet: product-manuals
  embeddingModel: bge-large-en
  vectorDB:
    type: milvus
    collection: manuals_v1
status:
  phase: Running
```

---

## 🧩 EmbeddingJob Spec 字段设计（详细版）
```yaml
spec:
  documentSet: string   # DocumentSet 名称
  
  # 下面的配置从 DocumentSet 继承，Controller 负责注入到 Pod 环境变量
  # 保持 Spec 简洁，避免数据不一致
  
  retryPolicy:
    maxRetries: 3
    backoffSeconds: 30
```

---

## 🧠 EmbeddingJob Status 字段
```yaml
status:
  phase: Pending | Running | Succeeded | Failed
  progress:
    totalChunks: 2000
    processedChunks: 800
  startTime: ...
  completionTime: ...
  message: "embedding in progress"
  conditions:
    - type: JobStarted
      status: "True"
    - type: VectorUpserted
      status: "True"
```

---

---

# ✅ **3. IndexJob CRD（构建向量索引）**
IndexJob 在向量写入后执行：

+ 建立 IVF / HNSW / PQ 等索引
+ 负责索引 rebuild / optimize
+ 更新向量库 collection 状态

---

## 🚀 IndexJob YAML 示例
```yaml
apiVersion: rag.ai/v1alpha1
kind: IndexJob
metadata:
  name: index-job-manuals
spec:
  documentSet: product-manuals
  vectorDB:
    type: milvus
    collection: manuals_v1
  indexSpec:
    type: HNSW
    parameters:
      efConstruction: 200
      M: 16
status:
  phase: Running
```

---

## 🧩 IndexJob Spec 字段设计
```yaml
spec:
  documentSet: string

  vectorDB:
    type: milvus | qdrant | weaviate
    collection: string      # 实际操作的临时集合 (e.g. manuals_v1_20231027)
    targetAlias: string     # 完成后需要切换的别名 (e.g. manuals_prod)
  
  indexSpec:
    type: HNSW | IVF_FLAT | IVF_PQ
    parameters:
      any: map

  retryPolicy:
    maxRetries: 3
```

---

## 🧠 IndexJob Status 字段
```yaml
status:
  phase: Pending | Building | Optimizing | Succeeded | Failed
  progress:
    indexedVectors: 0
    totalVectors: 0
  message: "Building HNSW index..."
  startTime: ...
  completionTime: ...
  conditions:
    - type: IndexCreated
      status: "True"
    - type: IndexOptimized
      status: "True"
```

---

# 🔄 **4. 三个 CRD 的生命周期关系（重要）**
```plain
DocumentSet
   ╰── creates → EmbeddingJob
                 ╰── creates → IndexJob
                        ╰── updates → DocumentSet.status.phase = Ready
```

也就是：

1. 用户提交 DocumentSet
2. Operator 检查 DocumentSet → 发起 EmbeddingJob
3. EmbeddingJob 成功 → 自动发起 IndexJob
4. IndexJob 完成 → DocumentSet 转为 Ready
5. 下游模型服务或 RAG 服务即可热加载最新索引

---

# 📦 **5. 事件触发流（Reconcile 逻辑）**
### **DocumentSet Controller**
| DocumentSet Phase | 操作 |
| --- | --- |
| Pending | 创建 EmbeddingJob |
| Chunked | 标记 embedding 准备 |
| Embedding | 等待 EmbeddingJob 完成 |
| ReadyForIndex | 创建 IndexJob |
| Indexing | 等待 IndexJob 完成 |
| Ready | 通知模型热更新 |


---

### **EmbeddingJob Controller**
+ 启动 embedding Pod / Job
+ 更新进度
+ 完成后更新 DocumentSet.status

---

### **IndexJob Controller**
+ 调用向量库 SDK 创建索引
+ 更新 DocumentSet.status.phase = Ready



