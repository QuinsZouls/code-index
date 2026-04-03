---
name: codeindex
description: "This skill should be used when code search is needed (whether explicitly requested or as part of completing a task), when indexing the codebase after changes, or when the user asks about codeindex, cocoindex-code, or the codebase index. Trigger phrases include 'search the codebase', 'find code related to', 'update the index', 'codeindex', 'cocoindex-code'."
---

# codeindex - Semantic Code Search & Indexing

`codeindex` is a semantic code search CLI that indexes codebases and provides fast, concept-based search over the current project.

## Ownership

The agent owns the `codeindex` lifecycle for the current project — initialization, indexing, and searching. Do not ask the user to perform these steps; handle them automatically.

- **Initialization**: If `codeindex search` or `codeindex index` fails with an initialization error (e.g., "Not in an initialized project directory"), run `codeindex init` from the project root directory, then `codeindex index` to build the index, then retry the original command.
- **Index freshness**: Keep the index up to date by running `codeindex index -path .` when the index may be stale — e.g., at the start of a session, or after making significant code changes (new files, refactors, renamed modules). There is no need to re-index between consecutive searches if no code was changed in between.
- **Installation**: If `codeindex` itself is not found (command not found), refer to [management.md](references/management.md) for installation instructions and inform the user.

## Searching the Codebase

To perform a semantic search:

```bash
codeindex search -path . <query terms>
```

The query should describe the concept, functionality, or behavior to find, not exact code syntax. For example:

```bash
codeindex search -path . database connection pooling
codeindex search -path . user authentication flow
codeindex search -path . error handling retry logic
```

### Filtering Results

- **By language** (`-lang`): restrict results to specific languages (comma-separated).

  ```bash
  codeindex search -path . -lang python,markdown database schema
  ```

- **By path** (`-glob`): restrict results to files matching glob patterns (comma-separated).

  ```bash
  codeindex search -path . -glob 'src/api/**' request validation
  ```

### Pagination

Results default to the first page (limit: 5). To retrieve additional results:

```bash
codeindex search -path . -offset 5 -limit 5 database schema
```

If all returned results look relevant, use `-offset` to fetch the next page — there are likely more useful matches beyond the first page.

### Working with Search Results

Search results include file paths and line ranges. To explore a result in more detail:

- Use the editor's built-in file reading capabilities (e.g., the `Read` tool) to load the matched file and read lines around the returned range for full context.
- When working in a terminal without a file-reading tool, use `sed -n '<start>,<end>p' <file>` to extract a specific line range.

## Settings

To view or edit embedding model configuration, include/exclude patterns, or language overrides, see [settings.md](references/settings.md).

## Management & Troubleshooting

For installation, initialization, troubleshooting, and cleanup commands, see [management.md](references/management.md).
