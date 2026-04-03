# codeindex Management

## Installation

Install via one of these methods:

### Recommended (curl)

```bash
curl -fsSL https://raw.githubusercontent.com/QuinsZouls/code-index/master/install.sh | bash
```

Windows is detected automatically; the script downloads the `.zip` release and installs the `.exe`.

### Go users

```bash
go install github.com/QuinsZouls/code-index/src@latest
```

### Manual

Download the matching asset from GitHub Releases and copy `codeindex` into your PATH.

After installation, verify:

```bash
codeindex version
```

## Project Initialization

Run from the project root:

```bash
codeindex init -path .
```

This creates:
- `~/.codeindex/default_settings.json` (user-level settings, if it doesn't exist)
- `.codeindex/settings.json` (project-level settings)

If `.git` exists, `.codeindex/` is automatically added to `.gitignore`.

After initialization, edit `.codeindex/settings.json` if needed (see [settings.md](settings.md) for format details), then run `codeindex index` to build the initial index.

## Commands

### `version`

Prints the current version:

```bash
codeindex version
```

### `init`

Initializes the project configuration:

```bash
codeindex init -path .
```

Options:
- `-path`: Project root directory (default: `.`)

### `index`

Scans repository, chunks text, sends embeddings requests, persists index to `.codeindex/index.gob`:

```bash
codeindex index -path . --verbose
```

Options:
- `-path`: Project root directory (default: `.`)
- `--verbose`: Show detailed progress per file

Default output:
- Spinner while indexing
- Final count of indexed files

Verbose output:
- `[+]` new files
- `[~]` updated files
- `[=]` unchanged (skipped)
- Final file/chunk summary

Config options (in `settings.json`):
- `worker_count`: Override parallel file workers
- `checkpoint_every`: Override checkpoint flush frequency

### `search`

Runs vector search against the stored index:

```bash
codeindex search -path . "authentication logic"
```

Options:
- `-path`: Project root directory (default: `.`)
- `-limit`: Max results (default: 5)
- `-offset`: Offset for pagination (default: 0)
- `-lang`: Comma-separated language filter
- `-glob`: Comma-separated path glob filter

Example with filters:

```bash
codeindex search -path . -lang go,python -glob "src/**" "database connection"
```

### `status`

Shows index statistics:

```bash
codeindex status -path .
```

Output includes:
- File count
- Chunk count
- Language distribution

## Storage

All data is stored in `.codeindex/`:
- `settings.json`: Project configuration
- `index.gob`: Embeddings and chunk metadata

## Performance

- Files skipped when `size` and `modtime` match previous index
- Worker pool overlaps file IO and API calls
- Only modified files are re-embedded

## Troubleshooting

### Re-indexing After Model Change

Switching embedding models changes vector dimensions. Delete the index and re-index:

```bash
rm -rf .codeindex/index.gob
codeindex index -path .
```

### Reset Project

To remove all project data (keeps settings):

```bash
rm -rf .codeindex/index.gob
```

To remove everything including settings:

```bash
rm -rf .codeindex/
```