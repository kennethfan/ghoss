# `ghoss mv` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `ghoss mv <src> <dst>` command for moving/renaming files and directories.

**Architecture:** Two sequential tasks: (1) add `CreateFile(path, content, message)` to the GitHub client (takes raw bytes, no disk I/O), (2) add `mvCmd` cobra command with recursive directory move using download → create → delete.

**Tech Stack:** Go, go-github v17, cobra CLI

## Global Constraints

- No overwrite existing file — error if dst already exists
- No `-r` flag — directory moves always recurse (UNIX semantics)
- Each file operation creates its own commit via GitHub API
- Default commit message: `"move: <src> → <dst>"`
- Custom message via `-m`/`--message` flag
- `ghoss mv dir1/ dir2/` moves all contents recursively
- `ghoss mv file.txt dir/` infers dst = `dir/basename(file.txt)`
- New method `CreateFile` on `*github.Client`: `CreateFile(path string, content []byte, message string) (*github.RepositoryContentResponse, *github.Response, error)`

---

### Task 1: Add `CreateFile` method to GitHub client

**Files:**
- Modify: `internal/github/client.go` — add CreateFile after UploadFile

**Interfaces:**
- Produces: `Client.CreateFile(path string, content []byte, message string) (*github.RepositoryContentResponse, *github.Response, error)`
- Consumed by: Task 2's `moveFile` function

- [ ] **Step 1: Add CreateFile method**

Insert `CreateFile` after `UploadFile` (after line 58 in current file):

```go
// CreateFile creates or updates a file from raw byte content.
func (c *Client) CreateFile(path string, content []byte, message string) (*github.RepositoryContentResponse, *github.Response, error) {
	opts := &github.RepositoryContentFileOptions{
		Content: content,
		Message: github.String(message),
		Branch:  github.String(c.branch),
	}
	return c.client.Repositories.CreateFile(c.ctx, c.owner, c.repo, path, opts)
}
```

- [ ] **Step 2: Add IsDirectory method**

Used by Task 2 to determine if dst is an existing directory (vs. a file). Insert after `CreateFile`:

```go
// IsDirectory checks whether path is an existing directory in the repo.
// Returns false (no error) if path is a file.
func (c *Client) IsDirectory(path string) (bool, error) {
	_, dirs, _, err := c.client.Repositories.GetContents(c.ctx, c.owner, c.repo, path, nil)
	if err != nil {
		return false, err
	}
	return dirs != nil, nil
}
```

Note: `dirs` is the second return from go-github's GetContents — it's `nil` for files, `non-nil` slice for directories (including empty). Using `dirs != nil` handles all cases.

- [ ] **Step 3: Build and verify**

```bash
go build ./internal/github/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/github/client.go
git commit -m "feat(github): add CreateFile and IsDirectory methods"
```

---

### Task 2: Add `mvCmd` command

**Files:**
- Modify: `cmd/ghoss/main.go` — add mv command flag, mvCmd, moveFile, moveDir, resolveDest

**Interfaces:**
- Consumes: `Client.CreateFile`, `Client.IsDirectory`, `Client.DownloadFile`, `Client.DeleteFile`, `Client.ListFiles`
- Produces: `ghoss mv <src> <dst> [-m "message"]` CLI command

- [ ] **Step 1: Add `mvMessage` variable**

Insert near other command variable declarations (around line 159, after `uploadRecursive`):

```go
var mvMessage string
```

- [ ] **Step 2: Add `mvCmd` command**

Insert between `rmCmd` and `deleteRecursive` function (around line 458):

```go
var mvCmd = &cobra.Command{
	Use:   "mv <source> <destination>",
	Short: "Move or rename a file or directory",
	Long:  `Move or rename files and directories in your GitHub repository. Directory moves are always recursive.`,
	Example: `  ghoss mv docs/report.pdf docs/archive/report.pdf
  ghoss mv photos/vacation/ backup/2026/
  ghoss mv image.png images/ -m "organize images"`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		client := github.NewClient(cfg.Github.Token, cfg.Github.Owner, cfg.Github.Repo, cfg.Github.Branch)
		if !client.IsConfigured() {
			fmt.Fprintln(os.Stderr, "Error: GitHub repository not configured. Please run 'ghoss init' first.")
			os.Exit(1)
		}

		src := storage.NormalizePath(args[0])
		dst := storage.NormalizePath(args[1])

		// Check if src exists
		srcContents, _, err := client.ListFiles(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: source not found: %s\n", src)
			os.Exit(1)
		}

		// Determine effective dst path
		isDir, dErr := client.IsDirectory(dst)
		if dErr == nil && isDir {
			// dst is an existing directory — move src into it
			dst = strings.TrimSuffix(dst, "/") + "/" + filepath.Base(src)
		} else if dErr == nil {
			// dst is an existing file — error (no overwrite)
			fmt.Fprintf(os.Stderr, "Error: destination already exists: %s\n", dst)
			os.Exit(1)
		}

		message := mvMessage
		if message == "" {
			message = "move: " + src + " → " + dst
		}

		if len(srcContents) > 0 {
			// Source is a directory — recursively move
			moveDir(client, src, dst, message)
		} else {
			// Source is a file — single move
			moveFile(client, src, dst, message)
		}
	},
}
```

- [ ] **Step 3: Add `moveFile` function**

Insert after `mvCmd`:

```go
func moveFile(client *github.Client, src, dst, message string) {
	content, _, err := client.DownloadFile(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading %s: %v\n", src, err)
		os.Exit(1)
	}

	_, _, err = client.CreateFile(dst, content, message)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", dst, err)
		os.Exit(1)
	}

	_, _, err = client.DeleteFile(src, message)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", src, err)
		os.Exit(1)
	}

	fmt.Printf("Moved: %s → %s\n", src, dst)
}
```

- [ ] **Step 4: Add `moveDir` function**

Insert after `moveFile`:

```go
func moveDir(client *github.Client, src, dst, message string) {
	entries, _, err := client.ListFiles(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing %s: %v\n", src, err)
		os.Exit(1)
	}

	srcPrefix := strings.TrimSuffix(src, "/") + "/"
	dstPrefix := strings.TrimSuffix(dst, "/") + "/"

	var errors bool
	for _, e := range entries {
		if e.Type == nil {
			continue
		}
		srcPath := e.GetPath()
		relPath := strings.TrimPrefix(srcPath, srcPrefix)
		dstPath := dstPrefix + relPath

		if *e.Type == "dir" {
			moveDir(client, srcPath, dstPath, message)
		} else {
			fileMsg := mvMessage
			if fileMsg == "" {
				fileMsg = "move: " + srcPath + " → " + dstPath
			}
			content, _, err := client.DownloadFile(srcPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error downloading %s: %v\n", srcPath, err)
				errors = true
				continue
			}
			_, _, err = client.CreateFile(dstPath, content, fileMsg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", dstPath, err)
				errors = true
				continue
			}
			_, _, err = client.DeleteFile(srcPath, fileMsg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", srcPath, err)
				errors = true
				continue
			}
			fmt.Printf("Moved: %s\n", srcPath)
		}
	}
	if errors {
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Register flag `-m`**

Insert near the other flag registrations (around line 595, after rm flags):

```go
	// mv flags
	mvCmd.Flags().StringVarP(&mvMessage, "message", "m", "", "Git commit message")
```

- [ ] **Step 6: Register `mvCmd` subcommand**

Insert near other `rootCmd.AddCommand` calls (around line 604):

```go
	rootCmd.AddCommand(mvCmd)
```

- [ ] **Step 7: Build and verify**

```bash
go build -o /dev/null ./cmd/ghoss/ && echo "BUILD OK"
```

Expected: `BUILD OK`

```bash
./bin/ghoss mv --help
```

Expected: show mv usage with -m flag.

- [ ] **Step 8: Commit**

```bash
git add cmd/ghoss/main.go
git commit -m "feat(mv): add mv command for moving files and directories"
```
