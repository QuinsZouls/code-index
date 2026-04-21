# code-index

Semantic code search for codebases

## Overview

- API-first embeddings
- Incremental indexing: only modified files are re-embedded.
- Background daemon: automatic re-indexing on file changes.
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
codeindex daemon start -path . --interval 2s
codeindex daemon list
codeindex daemon stop
```

## Install skill

```bash
npx skills add QuinsZouls/code-index
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
- `llamacpp`

### Rate Limiting

The `rate_limit` field controls request frequency to avoid API throttling:

- `rate_limit`: requests per second (default: `0` = disabled)
- Example: `10` means maximum 10 requests per second

### Timeout

The `timeout` field sets HTTP request timeout:

- Format: duration string like `"30s"`, `"1m"`, `"2m30s"`
- Default: `"60s"`

### Retry System

The retry system handles transient HTTP errors automatically:

- `max_retries`: maximum retry attempts (default: `0` = disabled)
- `retry_initial_delay`: first retry delay (default: `"1s"`)
- `retry_max_delay`: maximum delay cap (default: `"30s"`)
- Uses exponential backoff (delay doubles each retry)
- If all retries fail for a file, indexer skips it and continues with next file

Example with retry enabled:

```json
{
  "embedding": {
    "provider": "openai",
    "model": "text-embedding-3-small",
    "max_retries": 3,
    "retry_initial_delay": "1s",
    "retry_max_delay": "30s"
  }
}
```

Retryable errors include: rate limits (429), server errors (502, 503), timeouts, and connection issues.

### Chunking Options

The indexer splits files into chunks for embedding. By default, it uses `chunk_size` (lines) to determine chunk boundaries:

- `chunk_size`: maximum lines per chunk (default: `120`)
- `chunk_overlap`: lines to overlap between chunks (default: `20`)
- `min_chunk_size`: minimum lines for a valid chunk (default: `8`)

#### Context Size Limit

Some embedding models have small context windows (e.g., 512 tokens). Use `context_size` to limit chunks by character count instead of lines:

- `context_size`: maximum characters per chunk (default: `0` = disabled)

When `context_size` is set and a chunk would exceed this limit, the indexer switches to character-based chunking:

- Chunks are split at line boundaries (no mid-line splits)
- Overlap is respected
- A single long line exceeding `context_size` remains as one chunk

Example for a 512-token model:

```json
{
  "chunk_size": 120,
  "chunk_overlap": 20,
  "context_size": 2048
}
```

This ensures chunks fit within the model's context window while respecting line boundaries for readability.

## Provider Notes

- `api_key_env` should contain the environment variable name, not the secret itself.
- For OpenAI-compatible backends, set `base_url` to the provider endpoint.
- Default local endpoints:
  - Ollama: `http://localhost:11434`
  - LM Studio: `http://localhost:1234/v1`
  - llama.cpp Server: `http://localhost:8080/v1`

### llama.cpp Server

To use llama.cpp server for embeddings, start the server with:

```bash
llama-server -m embedding-model.gguf --embedding --pooling cls
```

Example configuration:

```json
{
  "embedding": {
    "provider": "llamacpp",
    "model": "nomic-embed-text"
  }
}
```

The llama.cpp server exposes an OpenAI-compatible `/v1/embeddings` endpoint. No API key is required by default.

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

### `daemon`

Manages background daemon for automatic re-indexing.

#### Subcommands

| Command | Description |
|---------|-------------|
| `daemon start` | Start daemon for a project |
| `daemon stop` | Stop daemon (by PID or project) |
| `daemon list` | List all running daemons |
| `daemon status` | Show daemon status for current project |

#### `daemon start`

Starts a background daemon that monitors file changes and re-indexes automatically.

```bash
codeindex daemon start [--path .] [--interval 2s] [--debounce 500ms] [--verbose]
```

Options:

| Flag | Default | Description |
|------|---------|-------------|
| `--path` | `.` | Project root directory |
| `--interval` | `2s` | Polling interval for file scanning |
| `--debounce` | `500ms` | Wait time before processing batch |
| `--verbose` | `false` | Show re-indexing activity |

The daemon:
- Polls for file changes (no external dependencies)
- Debounces rapid changes to process in batches
- Re-indexes only modified files (partial updates)
- Removes deleted files from the index
- Stores PID registry in `~/.codeindex/daemons.json`

#### `daemon stop`

Stops a running daemon.

```bash
codeindex daemon stop [--path .] [pid]
```

If no PID is provided, stops the daemon for the current project.

#### `daemon list`

Lists all running daemons across projects.

```bash
codeindex daemon list
```

Output example:

```text
PID      PROJECT                                  STATUS     STARTED
12345    /home/user/projects/myapp                running    2026-04-05 10:30:00
```

#### `daemon status`

Shows detailed status for the current project's daemon.

```bash
codeindex daemon status [--path .]
```

Output example:

```text
PID:      12345
Project:  /home/user/projects/myapp
Status:   running
Started:  2026-04-05 10:30:00
Interval: 2s
Debounce: 500ms
Files:    150
Chunks:   500
```

## Storage

Embeddings are stored inside `.codeindex/index.gob` alongside chunk metadata.

Daemon registry is stored in `~/.codeindex/daemons.json` with PID and project information.

Lock files are stored in `/tmp/codeindex-<hash>.lock` to prevent duplicate daemons per project.

## Performance

- Files are skipped when `size` and `modtime` match the previous index.
- Indexing runs with a small worker pool to overlap file IO and API calls.
- Only modified files are re-embedded.

## Tests

```bash
go test ./...
```
