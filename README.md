# cocoindex-code

Semantic code search for codebases

## Overview

- API-first embeddings
- Incremental indexing: only modified files are re-embedded.
- Project state lives in `.codeindex/`.

## Layout

```text
src/                Go implementation
.codeindex/         Project state and index
codeindex           Built binary
```

## Quick Start

```bash
curl -fsSL https://raw.githubusercontent.com/QuinsZouls/code-index/master/install.sh | bash
codeindex version
codeindex init -path .
codeindex index -path .
codeindex search -path . "authentication logic"
codeindex status -path .
```

## Installation

### Recommended

```bash
curl -fsSL https://raw.githubusercontent.com/QuinsZouls/code-index/master/install.sh | bash
```

Windows is detected automatically; the same script will download the `.zip` release and install the `.exe`.

### Go users

```bash
go install github.com/QuinsZouls/code-index/src@latest
```

### Manual

Download the matching asset from GitHub Releases and copy `codeindex` into your PATH.

## Configuration

The project config is stored in `.codeindex/settings.json`.

Example:

```json
{
  "embedding": {
    "provider": "openrouter",
    "model": "qwen/qwen3-embedding-8b",
    "base_url": "https://openrouter.ai/api/v1",
    "api_key_env": "OPENROUTER_API_KEY"
  }
}
```

Supported providers:

- `openai`
- `openai-compatible`
- `openrouter`
- `mistral`
- `gemini`
- `ollama`
- `lmstudio`

## Provider Notes

- `api_key_env` should contain the environment variable name, not the secret itself.
- For OpenAI-compatible backends, set `base_url` to the provider endpoint.
- Default local endpoints:
  - Ollama: `http://localhost:11434`
  - LM Studio: `http://localhost:1234/v1`

## Commands

### `init`

Creates `.codeindex/settings.json` and ensures `.gitignore` excludes `.codeindex/`.

### `index`

Scans the repository, chunks text, sends embeddings requests, and persists the index in `.codeindex/index.gob`.

Output includes:
- `new`
- `updated`
- `unchanged`
- spinner while indexing

### `search`

Runs vector search against the stored index.

### `status`

Prints file count, chunk count, and language distribution.

## Storage

Embeddings are stored inside `.codeindex/index.gob` alongside chunk metadata.

## Performance

- Files are skipped when `size` and `modtime` match the previous index.
- Indexing runs with a small worker pool to overlap file IO and API calls.
- Only modified files are re-embedded.

## Tests

```bash
go test ./...
```
