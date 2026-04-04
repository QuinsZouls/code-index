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
    "api_key_env": "OPENROUTER_API_KEY",
    "rate_limit": 10,
    "timeout": "60s"
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

### Rate Limiting

The `rate_limit` field controls request frequency to avoid API throttling:

- `rate_limit`: requests per second (default: `0` = disabled)
- Example: `10` means maximum 10 requests per second

### Timeout

The `timeout` field sets HTTP request timeout:

- Format: duration string like `"30s"`, `"1m"`, `"2m30s"`
- Default: `"60s"`

## Provider Notes

- `api_key_env` should contain the environment variable name, not the secret itself.
- For OpenAI-compatible backends, set `base_url` to the provider endpoint.
- Default local endpoints:
  - Ollama: `http://localhost:11434`
  - LM Studio: `http://localhost:1234/v1`

## Commands

### `init`

Creates `.codeindex/settings.json` from `~/.codeindex/default_settings.json` when present, then ensures `.gitignore` excludes `.codeindex/`.

### `index`

Scans the repository, chunks text, sends embeddings requests, and persists the index in `.codeindex/index.gob`.

Default output:
- spinner while indexing
- final count of indexed files

Verbose output:
- `new`
- `updated`
- `unchanged`
- final file/chunk summary

Example:

```bash
codeindex index -path . --verbose
```

Config options:

- `worker_count`: override parallel file workers
- `checkpoint_every`: override how often the `.gob` checkpoint is flushed

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
