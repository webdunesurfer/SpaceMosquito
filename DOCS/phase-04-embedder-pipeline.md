# Phase 4: Embedder Pipeline

## Objective
Implement vector embedding generation for extracted page content and store embeddings in PostgreSQL with pgvector for semantic search.

## Deliverables
- Embedder interface with pluggable implementations
- nomic-embed-text ONNX implementation (default)
- OpenAI text-embedding-3 fallback
- pgvector integration for storage and similarity search
- Batch embedding for full-space scans

## Tasks

### 4.1 — Embedder Interface
- `internal/embedder/embedder.go`:
  ```go
  type Embedder interface {
      Embed(text string) ([]float32, error)
      Dim() int
  }
  ```
  - Factory: `NewEmbedder(config) Embedder` based on config.model setting
  - Config options: `nomic-embed-text`, `openai`, `bge-m3`

### 4.2 — nomic-embed-text ONNX Implementation
- `internal/embedder/onnx.go`:
  - Use `github.com/yaoapp/gontf` or `github.com/edwingeng/golearn` for ONNX runtime
  - Load model from `models/nomic-embed-text.onnx` (~250MB)
  - Tokenization: use sentencepiece tokenizer (bundled with model)
  - `Embed(text string) ([]float32, error)`:
    - Tokenize input (max 512 tokens)
    - Run inference via ONNX
    - Normalize output vector (L2 normalization for cosine similarity)
  - `Dim() int` returns 768
  - Model download: first-run check, download from HuggingFace if missing

### 4.3 — OpenAI Implementation
- `internal/embedder/openai.go`:
  - Use OpenAI Go client: `github.com/openai/openai-go`
  - Call `/v1/embeddings` with `model: text-embedding-3-small`
  - `Embed(text string) ([]float32, error)`:
    - Send text to OpenAI API
    - Return embedding vector (1536 dims)
  - `Dim() int` returns 1536
  - Config: `embedder.openai.api_key`, `embedder.openai.model`

### 4.4 — bge-m3 Implementation (Optional)
- `internal/embedder/bge.go`:
  - Similar ONNX implementation for bge-m3 model
  - Model file: `models/bge-m3.onnx` (~1.9GB)
  - `Dim() int` returns 1024
  - Config-gated: only loaded if `embedder.model == "bge-m3"`
  - Not included in default Docker image (user downloads separately)

### 4.5 — pgvector Integration
- `internal/db/models.go`:
  - `CreateEmbedding(pageID uuid.UUID, embedding []float32)` — INSERT into page_embeddings
  - `SearchEmbeddings(query []float32, spaceKey string, limit int) ([]Page, error)` — cosine similarity search
  - `UpsertEmbedding(pageID uuid.UUID, embedding []float32)` — update existing embedding
- Query for semantic search:
  ```sql
  SELECT p.*, pe.embedding <=> $1 AS similarity
  FROM pages p
  JOIN page_embeddings pe ON pe.page_id = p.id
  WHERE $2::text = '' OR p.space_id IN (
      SELECT id FROM spaces WHERE key = $2
  )
  ORDER BY similarity
  LIMIT $3
  ```

### 4.6 — Batch Processing
- `internal/embedder/embedder.go`:
  - `EmbedBatch(pages []Page) error` — process pages in batches of 32
  - For each batch:
    - Extract text content from pages
    - Generate embeddings (parallel if model supports it)
    - Store embeddings in DB
  - Progress tracking: "Embedding page 50/500..."
  - Error handling: failed embeddings skipped, logged, retry on next crawl
  - Deduplication: only embed pages that don't have embeddings or were updated

### 4.7 — Model Management
- First-run model download:
  - Check for model file in `~/.config/spacemosquito/models/`
  - Download from HuggingFace: `nomic-ai/nomic-embed-text-v1`
  - Extract model and tokenizer files
  - Cache model path in config
- Model validation: verify file integrity (checksum)

## Acceptance Criteria
- nomic-embed-text generates embeddings for text input
- OpenAI embedding works with API key configured
- Embeddings stored in pgvector with correct dimensions
- Semantic search returns relevant results
- Batch embedding processes 100+ pages efficiently
- Model downloads automatically on first run
