# `ghoss ls` Enhancement Design

**Date:** 2026-07-08
**Status:** Approved
**Related:** [AGENTS.md](../../../AGENTS.md)

## Goal

Enhance `ghoss ls` with UNIX `ls`-compatible flags, human-readable sizes, last-commit timestamps, sort options, recursive listing, and JSON output.

## Flags

| Flag | Long | Default | Description |
|------|------|---------|-------------|
| `-l` | `--long` | off | Long format: perms, size, time, name |
| `-h` | `--human-readable` | off (on with `-l`) | Human-readable file size (1024 → 1.0K) |
| `-t` | `--sort-by-time` | off | Sort by last modified time (newest first) |
| `-S` | `--sort-by-size` | off | Sort by file size (largest first) |
| `-r` | `--reverse` | off | Reverse sort order |
| `-R` | `--recursive` | off | Recursively list subdirectories |
| | `--sort` | `name` | Sort key: `name`, `time`, `size` |
| | `--json` | off | JSON output |

## Output Formats

### Default (no flags)
```
file1.go
file2.txt
mydir/
```
- Files and directories in alphabetical order, dirs first.
- Directories suffixed with `/`.

### Long format (`-l`)
```
-rw-r--r--  1.2K 2026-07-08 12:30  file1.go
-rw-r--r--  2.5M 2026-06-01 09:00  file2.txt
drwxr-xr-x  4.0K 2026-07-07 15:20  mydir/
```
- Columns aligned via `text/tabwriter`.
- `-h` enabled implicitly with `-l`. Use `--no-human` to disable.
- Human-fallback: size column shows `-` when API returns 0.
- Time column shows `-` when mod_time unavailable (degraded or failed).

### JSON output (`--json`)
```json
{"path":"","files":[{"name":"file1.go","path":"file1.go","size":1234,"sha":"abc123...","type":"file","mod_time":"2026-07-08T12:30:00Z"}]}
```

## API Strategy

One `GetContents` call for the directory listing (same as now).

**Only when `-l` is specified**, fetch last commit times:
- For each file, call `ListCommits(path=filepath, per_page=1)`.
- Use `errgroup` with concurrency limit of 20.
- **Auto-degradation**: if file count > 30, skip time queries entirely; time column shows `-`.
- Individual failures → that file's time shows `-`, process continues.
- SHA dedup: same blob SHA → reuse cached time.

### Cost estimate
| Scenario | API calls | Latency |
|----------|-----------|---------|
| No `-l`, 20 files | 1 | < 1s |
| `-l`, 20 files | 21 (1+20 parallel) | ~1-2s |
| `-l`, 100 files | 1 (degraded) | < 1s |

## Architecture

### New file: `internal/lister/lister.go`

```go
type Options struct {
    Long      bool
    Human     bool
    SortBy    string    // "name" | "time" | "size"
    Reverse   bool
    Recursive bool
    JSON      bool
}

type Entry struct {
    Name         string
    Path         string
    Size         int64
    Type         string    // "file" | "dir"
    SHA          string
    ModTime      time.Time
    ModTimeValid bool
}

func Run(client *github.Client, path string, opts Options) error
```

`Run` handles all ls logic: listing, recursive expansion, time fetching, sorting, formatting, output.

### Changes to existing files

| File | Change |
|------|--------|
| `internal/github/client.go` | Add `ListCommits(path string, perPage int)` method |
| `cmd/ghoss/main.go` | Parse flags, call `lister.Run()`, remove inline ls logic |

## Edge Cases

- **Empty directory**: print `(empty)` to stdout, exit 0.
- **Path not found**: error to stderr, exit 1.
- **Token insufficient scope**: error message with path context, exit 1.
- **`-t` sort with no timestamps**: entries without ModTime sort last.
- **Directory size in `-l`**: show `4.0K` (GitHub API fixed value).

## Future Considerations (Out of Scope)

- Pagination for directories with 1000+ entries.
- `--format=csv`, `--format=yaml`.
- Color output.
- `.gitignore`-aware filtering.
