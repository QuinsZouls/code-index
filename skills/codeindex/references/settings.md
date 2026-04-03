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
    "api_key_env": "OPENAI_API_KEY"
  }
}
```

**OpenRouter:**

```json
{
  "embedding": {
    "provider": "openrouter",
    "model": "qwen/qwen3-embedding-8b",
    "api_key_env": "OPENROUTER_API_KEY"
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
    "base_url": "http://localhost:11434"
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

Switching embedding models changes vector dimensions — you must re-index after changing the model:

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