# `ghoss mv` Command Design

> Move/rename files and directories in a GitHub repository.

**Goal:** Add `ghoss mv <src> <dst>` command for moving and renaming files and directories, with recursive directory support.

**Architecture:** GitHub Contents API has no native move operation — mv is implemented as download → create → delete. A new `CreateFile` method on the GitHub client accepts raw bytes (no local disk I/O). Directory mv uses recursive listing + per-file move.

**Tech Stack:** Go, go-github v17, cobra CLI

---

## Behavior

- `ghoss mv a.txt b.txt` → Rename file
- `ghoss mv a.txt dir/` → Move file into directory
- `ghoss mv dir1/ dir2/` → Recursively move directory to directory
- `ghoss mv src dst -m "my message"` → Custom commit message per file operation

**No `-r` flag needed** — directory moves always recurse (UNIX `mv` semantics).

## Guard rails

- If `src` does not exist → error with exit code 1
- If `dst` already exists → error with exit code 1 (no overwrite)
- If `src` is a single file, move it directly
- If `src` is a directory, recursively move all contents

## Commit strategy

GitHub Contents API creates one commit per file operation. Each file moved generates its own commit with message `"move: src/path → dst/path"`. On partial failure, already-moved files are not rolled back (error is printed, process exits 1).

---

## Components

### 1. `internal/github/client.go` — Add `CreateFile` method

```go
func (c *Client) CreateFile(path string, content []byte, message string) (*github.RepositoryContentResponse, *github.Response, error)
```

- Similar to existing `UploadFile` but takes raw `[]byte` instead of reading from disk
- Uses `c.client.Repositories.CreateFile(...)`
- Adds `"os"` import is not needed (no file I/O)

### 2. `cmd/ghoss/main.go` — Add `mvCmd`

**Flags:**
- `--message, -m` — Custom commit message (default: `"move: <src> → <dst>"`)

**Variables:**
- `mvMessage string`

**Functions:**
- `moveFile(client, src, dst, message)` — Download → Create → Delete for a single file
- `moveDir(client, src, dst, message)` — List files recursively under src, compute dst paths, call `moveFile` for each
- `isDir(client, path)` — Check if path is a directory via `ListFiles` result

**Directory move algorithm:**

```
1. List all entries under src
2. For each entry:
   - If file: compute dstPath (replace src prefix with dst), moveFile
   - If dir: recursively call moveDir
```

**dst path computation when dst is an existing directory:**
- Check if dst ends with `/` or if `ListFiles(dst)` returns entries (it's a dir)
- If dst is a dir, the effective dst is `dst/basename(src)`

Example:
- `mv images/photos/ backup/` → moves to `backup/photos/`
- `mv file.txt backup/` → moves to `backup/file.txt`

DST detection logic:
1. Try `ListFiles(dst)` — if it returns entries (directory) or succeeds without error, treat dst as an existing directory
2. If dst starts with an existing directory prefix + `basename(src)` resolves — same as above

Simpler rule: **if dst ends with `/` or is an existing directory, treat it as a directory target**. Otherwise treat it as an exact target path (rename).

### 3. dst path resolution

```
func resolveDest(src, dst string, client *github.Client) (targetPath string, isDirTarget bool)
```

- If dst ends with `/` → `dst + basename(src)`, dir target
- If `ListFiles(dst)` succeeds and returns non-file → `dst + "/" + basename(src)`, dir target
- Otherwise → dst as-is (exact rename)

---

## Errors

| Condition | Message |
|-----------|---------|
| `src` not found | `"Error: source not found: <src>"` |
| `dst` already exists | `"Error: destination already exists: <dst>"` |
| File move API error | `"Error moving <src> → <dst>: <error>"` |
| Dir move partial failure | Print errors per file, exit 1 at end |

## Testing considerations

- No test files exist in project yet — out of scope for this feature
- Manual testing: `ghoss mv <src> <dst>` and verify via `ghoss ls`
