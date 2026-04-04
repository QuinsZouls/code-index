# codeindex Settings

Configuration lives in two JSON files, both created automatically by `codeindex init`.

## User-Level Settings (`~/.codeindex/default_settings.json`)

Shared across all projects. Controls the embedding model and default configuration.

```json
{
  "version": 1,
  "embedding": {
    "provider": "openai",
    "model": "text-embedding-3-small",
    "api_key_env": "OPENAI_API_KEY"
  }
}
```

### Embedding Configuration Fields

| Field | Description |
|-------|-------------|
| `provider` | Embedding provider: `openai`, `openai-compatible`, `openrouter`, `mistral`, `gemini`, `ollama`, `lmstudio` |
| `model` | Model identifier — format depends on provider (see examples below) |
| `base_url` | Optional. API endpoint. Auto-set for known providers. |
| `api_key` | Optional. API key value (use `api_key_env` instead for security). |
| `api_key_env` | Environment variable name containing the API key (recommended). |
| `headers` | Optional. Custom headers for API requests. |
| `rate_limit` | Optional. Requests per second limit to avoid API throttling. `0` = disabled (default). |
| `timeout` | Optional. HTTP request timeout. Format: `"30s"`, `"1m"`, `"2m30s"`. Default: `"60s"`. |

### Provider Defaults

When `base_url` and `api_key_env` are omitted, these defaults apply:

| Provider | Base URL | API Key Env |
|----------|----------|-------------|
| `openai` | `https://api.openai.com/v1` | `OPENAI_API_KEY` |
| `openai-compatible` | `https://api.openai.com/v1` | `OPENAI_API_KEY` |
| `openrouter` | `https://openrouter.ai/api/v1` | `OPENROUTER_API_KEY` |
| `mistral` | `https://api.mistral.ai/v1` | `MISTRAL_API_KEY` |
| `gemini` | `https://generativelanguage.googleapis.com/v1beta` | `GEMINI_API_KEY` |
| `ollama` | `http://localhost:11434` | (none) |
| `lmstudio` | `http://localhost:1234/v1` | (none) |

### Embedding Configuration Examples

**OpenAI (default):**

```json
{
  "embedding": {
    "provider": "openai",
    "model": "text-embedding-3-small",
    "api_key_env": "OPENAI_API_KEY",
    "rate_limit": 10,
    "timeout": "60s"
  }
}
```

**OpenRouter:**

```json
{
  "embedding": {
    "provider": "openrouter",
    "model": "qwen/qwen3-embedding-8b",
    "api_key_env": "OPENROUTER_API_KEY",
    "rate_limit": 5
  }
}
```

**Mistral:**

```json
{
  "embedding": {
    "provider": "mistral",
    "model": "mistral-embed",
    "api_key_env": "MISTRAL_API_KEY"
  }
}
```

**Gemini:**

```json
{
  "embedding": {
    "provider": "gemini",
    "model": "text-embedding-004",
    "api_key_env": "GEMINI_API_KEY"
  }
}
```

**Ollama (local, no API key needed):**

```json
{
  "embedding": {
    "provider": "ollama",
    "model": "nomic-embed-text",
    "base_url": "http://localhost:11434",
    "timeout": "120s"
  }
}
```

**LM Studio (local, no API key needed):**

```json
{
  "embedding": {
    "provider": "lmstudio",
    "model": "text-embedding-nomic-embed-text-v1",
    "base_url": "http://localhost:1234/v1"
  }
}
```

**OpenAI-compatible backend:**

```json
{
  "embedding": {
    "provider": "openai-compatible",
    "model": "your-model-name",
    "base_url": "https://your-api-endpoint.com/v1",
    "api_key_env": "YOUR_API_KEY_VAR"
  }
}
```

### Important

**Rate Limiting:** Enable `rate_limit` to avoid hitting API provider limits. For example, OpenAI free tier allows ~3 requests/second, so set `"rate_limit": 3` or lower. Higher limits may work for paid tiers or local providers like Ollama.

**Timeout:** Adjust `timeout` based on your network conditions and model response time. Local models (Ollama/LM Studio) may need longer timeouts (`"120s"` or more) for large batches.

**Model Switching:** Changing embedding models changes vector dimensions — you must re-index after changing the model:

```bash
rm -rf .codeindex/index.gob
codeindex index -path .
```

## Project-Level Settings (`<project>/.codeindex/settings.json`)

Per-project. Controls which files to index and chunking parameters. Created by `codeindex init` and automatically added to `.gitignore`.

```json
{
  "version": 1,
  "include_patterns": [
    "**/*.py",
    "**/*.js",
    "**/*.ts",
    "**/*.go",
    "**/*.rs"
  ],
  "exclude_patterns": [
    "**/.git",
    "**/.codeindex",
    "**/node_modules",
    "**/dist",
    "**/build",
    "**/target",
    "**/vendor",
    "**/__pycache__"
  ],
  "language_overrides": {
    "inc": "php"
  },
  "chunk_size": 120,
  "chunk_overlap": 20,
  "min_chunk_size": 8,
  "worker_count": 0,
  "checkpoint_every": 0,
  "search_limit": 5,
  "score_threshold": 0.3,
  "embedding": {
    "provider": "openrouter",
    "model": "qwen/qwen3-embedding-8b",
    "base_url": "https://openrouter.ai/api/v1",
    "api_key_env": "OPENROUTER_API_KEY"
  }
}
```

### Fields

| Field | Description |
|-------|-------------|
| `version` | Config version (currently `1`). |
| `include_patterns` | Glob patterns for files to index. Defaults cover 20+ file types (`.go`, `.rs`, `.py`, `.js`, `.jsx`, `.ts`, `.tsx`, `.java`, `.c`, `.h`, `.cpp`, `.hpp`, `.cs`, `.sh`, `.md`, `.yaml`, `.yml`, `.toml`, `.json`, `.sql`, `.html`, `.css`). |
| `exclude_patterns` | Glob patterns for files/directories to skip. Defaults: `.git`, `.codeindex`, `node_modules`, `dist`, `build`, `target`, `vendor`, `__pycache__`. |
| `language_overrides` | Map of file extensions to language names. Example: `{"inc": "php"}` to treat `.inc` as PHP. |
| `chunk_size` | Target chunk size in lines (default: `120`). |
| `chunk_overlap` | Overlap between chunks in lines (default: `20`). |
| `min_chunk_size` | Minimum chunk size in lines (default: `8`). |
| `worker_count` | Parallel file workers. `0` = auto (default: `0`). |
| `checkpoint_every` | How often to flush `.gob` checkpoint. `0` = at end only (default: `0`). |
| `search_limit` | Default max results for search (default: `5`). Can be overridden with `-limit` flag. |
| `score_threshold` | Minimum similarity score (0-1) to display results (default: `0.3`). Results below threshold are filtered out. |
| `hybrid_search` | Enable hybrid search combining vector + keyword matching (default: `false`). Improves precision for exact term matches. |
| `vector_weight` | Weight for vector similarity score in hybrid mode (default: `0.7`). Range: 0-1. |
| `keyword_weight` | Weight for keyword TF-IDF score in hybrid mode (default: `0.3`). Range: 0-1. |
| `embedding` | Optional. Project-specific embedding config. Overrides user-level settings. |

### Default Include Patterns

The default `include_patterns` covers:

- Go: `**/*.go`
- Rust: `**/*.rs`
- Python: `**/*.py`
- JavaScript: `**/*.js`, `**/*.jsx`
- TypeScript: `**/*.ts`, `**/*.tsx`
- Java: `**/*.java`
- C/C++: `**/*.c`, `**/*.h`, `**/*.cpp`, `**/*.hpp`
- C#: `**/*.cs`
- Shell: `**/*.sh`
- Config: `**/*.yaml`, `**/*.yml`, `**/*.toml`, `**/*.json`
- Data: `**/*.sql`
- Web: `**/*.html`, `**/*.css`
- Docs: `**/*.md`

### Hybrid Search

Hybrid search combines **semantic vector search** with **keyword matching** (TF-IDF) for better precision:

```json
{
  "hybrid_search": true,
  "vector_weight": 0.7,
  "keyword_weight": 0.3
}
```

**When to use:**
- Queries with specific technical terms (function names, API endpoints, identifiers)
- Mixed queries (semantic intent + exact keywords)
- When pure vector search misses obvious keyword matches

**How it works:**
- `vector_weight`: Semantic similarity score from embeddings
- `keyword_weight`: TF-IDF score based on term frequency
- Final score = `vector_weight * vector_score + keyword_weight * keyword_score`
- Weights are automatically normalized if they don't sum to 1.0

**Example weights:**
- `0.8, 0.2`: Favor semantic matching (default)
- `0.5, 0.5`: Equal importance
- `0.3, 0.7`: Favor keyword matching (better for exact terms)

You can also enable hybrid search per-query with the `-hybrid` flag:

```bash
codeindex search -path . -hybrid "EmbeddingProvider interface"
```

### Editing Tips

- To index additional file types, append to `include_patterns`:

```json
{
  "include_patterns": [
    "**/*.proto",
    "**/*.graphql"
  ]
}
```

- To exclude a directory, append to `exclude_patterns`:

```json
{
  "exclude_patterns": [
    "**/generated",
    "**/migrations"
  ]
}
```

- After editing, run `codeindex index -path .` to re-index with the new settings.

## Full Configuration Example

User-level (`~/.codeindex/default_settings.json`):

```json
{
  "version": 1,
  "embedding": {
    "provider": "openrouter",
    "model": "qwen/qwen3-embedding-8b",
    "api_key_env": "OPENROUTER_API_KEY"
  }
}
```

Project-level (`.codeindex/settings.json`):

```json
{
  "version": 1,
  "include_patterns": [
    "**/*.go",
    "**/*.md"
  ],
  "exclude_patterns": [
    "**/vendor",
    "**/testdata"
  ],
  "chunk_size": 100,
  "chunk_overlap": 15,
  "worker_count": 4
}
```