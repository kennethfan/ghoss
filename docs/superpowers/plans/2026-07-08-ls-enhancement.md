# `ghoss ls` Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enhance `ghoss ls` with UNIX `ls`-compatible flags, human-readable sizes, last-commit timestamps, sort options, recursive listing, and JSON output.

**Architecture:** Extract ls logic from `cmd/ghoss/main.go` into `internal/lister/lister.go` with a clean `Options` + `Run` API. Add `ListCommits` to the existing GitHub client. Wire new flags in main.go.

**Tech Stack:** Go 1.21, cobra, go-github v17, golang.org/x/sync/errgroup

## Global Constraints

- Go 1.21 only — no generics features from later versions.
- go-github v17 (`github.com/google/go-github v17.0.0+incompatible`).
- Cobra flag naming: short flags use `BoolVarP`/`StringVarP`, long-only use `BoolVar`/`StringVar`.
- No new dependencies beyond stdlib + errgroup (`golang.org/x/sync`).
- `golang.org/x/sync/errgroup` does not need `go get` — it's already a transitive dep via go-github. Run `go get golang.org/x/sync@latest` if needed.
- All output uses `text/tabwriter` for column alignment.

---
### Task 1: Add `ListCommits` to GitHub client

**Files:**
- Modify: `internal/github/client.go` — add method after `ListFiles`

**Interfaces:**
- Consumes: `Client` struct, `*github.RepositoryCommit` type from go-github
- Produces: `func (c *Client) ListCommits(path string) ([]*github.RepositoryCommit, error)`

- [ ] **Step 1: Add import and method**

Add `"golang.org/x/sync/errgroup"` to imports (only if missing — verify first).

Add after `ListFiles` method (after line 107):

```go
// ListCommits lists commits for a specific path (1 result).
func (c *Client) ListCommits(path string) (*github.RepositoryCommit, error) {
	opts := &github.CommitsListOptions{
		Path:    path,
		ListOptions: github.ListOptions{PerPage: 1},
	}
	commits, _, err := c.client.Repositories.ListCommits(c.ctx, c.owner, c.repo, opts)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, nil
	}
	return commits[0], nil
}
```

- [ ] **Step 2: Verify build**

```bash
go build -o /dev/null ./cmd/ghoss/main.go
```

Expected: exit 0, no output.

- [ ] **Step 3: Commit**

```bash
git add internal/github/client.go
git commit -m "feat(github): add ListCommits method"
```

---

### Task 2: Create `internal/lister/lister.go`

**Files:**
- Create: `internal/lister/lister.go`

**Interfaces:**
- Consumes:
  - `client.ListFiles(path string) ([]*github.RepositoryContent, *github.Response, error)`
  - `client.ListCommits(path string) (*github.RepositoryCommit, error)`
- Produces: `lister.Options`, `lister.Entry`, `lister.Run(client, path, opts) error`

- [ ] **Step 1: Create file with Options, Entry structs and helper functions**

```go
package lister

import (
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/ghoss/internal/github"
	"github.com/google/go-github/github"
	"golang.org/x/sync/errgroup"
)

// Options controls ls output behavior.
type Options struct {
	Long      bool
	Human     bool
	SortBy    string // "name" | "time" | "size"
	Reverse   bool
	Recursive bool
	JSON      bool
}

// Entry represents a single file/directory entry for display.
type Entry struct {
	Name         string
	Path         string
	Size         int64
	Type         string // "file" | "dir"
	SHA          string
	ModTime      time.Time
	ModTimeValid bool
}

const apiTimeThreshold = 30
const dirPerm = "drwxr-xr-x"
const filePerm = "-rw-r--r--"

// humanSize converts bytes to human-readable string (1024 → 1.0K).
func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatTime formats a time for display.
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}
```

- [ ] **Step 2: Add recursive listing**

```go
// listAllRecursive walks all subdirectories via API calls.
func listAllRecursive(client *github.Client, dirPath string) ([]*github.RepositoryContent, error) {
	var all []*github.RepositoryContent
	entries, _, err := client.ListFiles(dirPath)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		all = append(all, e)
		if e.Type != nil && *e.Type == "dir" {
			sub, err := listAllRecursive(client, e.GetPath())
			if err != nil {
				return nil, err
			}
			all = append(all, sub...)
		}
	}
	return all, nil
}
```

- [ ] **Step 3: Add time fetching with auto-degradation**

```go
// fetchModTimes fetches last commit time for each entry in parallel.
// Degrades silently when count > threshold or on errors.
func fetchModTimes(client *github.Client, entries []*Entry) {
	if len(entries) > apiTimeThreshold {
		return
	}
	var mu sync.Mutex
	g, _ := errgroup.WithContext(client.GetContext())
	g.SetLimit(20)
	for i := range entries {
		if entries[i].Type == "dir" {
			continue
		}
		i := i
		g.Go(func() error {
			commit, err := client.ListCommits(entries[i].Path)
			if err != nil || commit == nil || commit.Commit == nil || commit.Commit.Committer == nil {
				return nil // silently degrade
			}
			mu.Lock()
			entries[i].ModTime = commit.Commit.Committer.GetDate()
			entries[i].ModTimeValid = true
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait() // errors are silently degraded
}
```

Note: `client.GetContext()` doesn't exist — we need to use `client.ctx`. Since `ctx` is unexported, we either export it or add a `Context()` getter. Let's use `ListCommits` directly and pass context via the client.

Actually, the `errgroup.WithContext` needs a context. The client already has a context. Let me adjust: add a getter to the client first.

Actually, let me simplify — use a simple `sync.WaitGroup` instead of errgroup for clarity, since we degrade errors anyway:

```go
// fetchModTimes fetches last commit time for each entry in parallel.
// Degrades silently when count > threshold or on errors.
func fetchModTimes(client *github.Client, entries []*Entry) {
	if len(entries) > apiTimeThreshold {
		return
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 20)
	for i := range entries {
		if entries[i].Type == "dir" {
			continue
		}
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			commit, err := client.ListCommits(entries[i].Path)
			if err != nil || commit == nil || commit.Commit == nil || commit.Commit.Committer == nil {
				return
			}
			mu.Lock()
			entries[i].ModTime = commit.Commit.Committer.GetDate()
			entries[i].ModTimeValid = true
			mu.Unlock()
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 4: Add conversion and sorting**

```go
// toEntries converts go-github RepositoryContent slice to our Entry slice.
func toEntries(contents []*github.RepositoryContent) []*Entry {
	entries := make([]*Entry, 0, len(contents))
	for _, c := range contents {
		e := &Entry{
			Name: c.GetName(),
			Path: c.GetPath(),
			Size: int64(c.GetSize()),
			Type: c.GetType(),
			SHA:  c.GetSHA(),
		}
		entries = append(entries, e)
	}
	return entries
}

// sortEntries sorts entries by the selected key.
func sortEntries(entries []*Entry, sortBy string, reverse bool) {
	var less func(i, j int) bool
	switch sortBy {
	case "size":
		less = func(i, j int) bool {
			if entries[i].Type != entries[j].Type {
				return entries[i].Type == "dir" // dirs first
			}
			return entries[i].Size < entries[j].Size
		}
	case "time":
		less = func(i, j int) bool {
			if entries[i].Type != entries[j].Type {
				return entries[i].Type == "dir"
			}
			// entries with valid times sort before those without
			if entries[i].ModTimeValid != entries[j].ModTimeValid {
				return entries[i].ModTimeValid
			}
			return entries[i].ModTime.Before(entries[j].ModTime)
		}
	default: // "name"
		less = func(i, j int) bool {
			if entries[i].Type != entries[j].Type {
				return entries[i].Type == "dir"
			}
			return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		}
	}
	if reverse {
		orig := less
		less = func(i, j int) bool { return orig(j, i) }
	}
	sort.SliceStable(entries, less)
}
```

- [ ] **Step 5: Add JSON output**

```go
import "encoding/json"

// writeJSON writes entries as JSON to w.
func writeJSON(w io.Writer, path string, entries []*Entry) error {
	out := struct {
		Path  string   `json:"path"`
		Files []*Entry `json:"files"`
	}{
		Path:  path,
		Files: entries,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
```

- [ ] **Step 6: Add human-readable column output**

```go
// writeTable writes aligned columns to w.
func writeTable(w io.Writer, entries []*Entry, opts Options) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	defer tw.Flush()

	for _, e := range entries {
		if opts.Long {
			perm := filePerm
			if e.Type == "dir" {
				perm = dirPerm
			}
			sizeStr := fmt.Sprintf("%d", e.Size)
			if opts.Human {
				sizeStr = humanSize(e.Size)
			}
			timeStr := "-"
			if e.ModTimeValid {
				timeStr = formatTime(e.ModTime)
			}
			fmt.Fprintf(tw, "%s  %s\t%s\t%s\n", perm, sizeStr, timeStr, e.Name)
		} else {
			name := e.Name
			if e.Type == "dir" {
				name += "/"
			}
			fmt.Fprintln(tw, name)
		}
	}
}
```

- [ ] **Step 7: Add the main Run function**

```go
// Run executes the ls command with given options.
// It does NOT exit the process — caller handles errors.
func Run(client *github.Client, lsPath string, opts Options) error {
	var contents []*github.RepositoryContent
	var err error

	if opts.Recursive {
		contents, err = listAllRecursive(client, lsPath)
	} else {
		_, contents, err = client.ListFiles(lsPath)
	}
	if err != nil {
		return fmt.Errorf("listing %s: %w", lsPath, err)
	}

	if len(contents) == 0 {
		fmt.Println("(empty)")
		return nil
	}

	entries := toEntries(contents)

	if opts.Long {
		fetchModTimes(client, entries)
	}

	sortEntries(entries, opts.SortBy, opts.Reverse)

	if opts.JSON {
		return writeJSON(os.Stdout, lsPath, entries)
	}

	writeTable(os.Stdout, entries, opts)
	return nil
}
```

- [ ] **Step 8: Verify build**

```bash
go build -o /dev/null ./cmd/ghoss/main.go
```

Expected: exit 0.

- [ ] **Step 9: Commit**

```bash
git add internal/lister/lister.go
git commit -m "feat(lister): add ls display engine"
```

---

### Task 3: Wire flags and refactor `lsCmd` in main.go

**Files:**
- Modify: `cmd/ghoss/main.go` — replace `lsCmd` var, add flag variables, register in init()

**Interfaces:**
- Consumes: `lister.Options`, `lister.Run(client, path, opts) error`
- Produces: working `ghoss ls` with all new flags

- [ ] **Step 1: Add ls-specific flag variables (including -t and -S)**

Add near the other flag variables (around line 34-38):

```go
var (
	lsLong        bool
	lsHuman       bool
	lsSortBy      string
	lsSortByTime  bool // -t flag
	lsSortBySize  bool // -S flag
	lsReverse     bool
	lsRecursive   bool
	lsJSON        bool
)
```

- [ ] **Step 2: Add import for lister package**

Add to the imports block in `cmd/ghoss/main.go`:

```go
"github.com/ghoss/internal/lister"
```

- [ ] **Step 2: Add ls-specific flag variables**

Add near the other flag variables (around line 34-38):

```go
var (
	lsLong      bool
	lsHuman     bool
	lsSortBy    string
	lsReverse   bool
	lsRecursive bool
	lsJSON      bool
)
```

- [ ] **Step 3: Replace the lsCmd definition (lines 349-392)**

Replace the entire `lsCmd` var:

```go
var lsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List files in GitHub repository",
	Long:  `List files and directories in your GitHub repository.`,
	Example: `  ghoss ls
  ghoss ls photos/
  ghoss ls -l
  ghoss ls --json`,
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

		lsPath := ""
		if len(args) > 0 {
			lsPath = storage.NormalizePath(args[0])
		}

		// -t and -S override --sort deterministically (S overrides t)
		sortBy := lsSortBy
		if lsSortByTime {
			sortBy = "time"
		}
		if lsSortBySize {
			sortBy = "size"
		}

		opts := lister.Options{
			Long:      lsLong,
			Human:     lsHuman || lsLong, // -l implies human-readable
			SortBy:    sortBy,
			Reverse:   lsReverse,
			Recursive: lsRecursive,
			JSON:      lsJSON,
		}

		if err := lister.Run(client, lsPath, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}
```

- [ ] **Step 4: Add flag registration in init()**

Find the `init()` function (likely around line 520+) and add ls flags:

```go
// ls flags
lsCmd.Flags().BoolVarP(&lsLong, "long", "l", false, "Long format with permissions, size, and time")
lsCmd.Flags().BoolVarP(&lsHuman, "human-readable", "h", false, "Human-readable file sizes (implied by -l)")
lsCmd.Flags().StringVar(&lsSortBy, "sort", "name", "Sort key: name|time|size")
lsCmd.Flags().BoolVarP(&lsSortByTime, "sort-by-time", "t", false, "Sort by last modified time")
lsCmd.Flags().BoolVarP(&lsSortBySize, "sort-by-size", "S", false, "Sort by file size")
lsCmd.Flags().BoolVarP(&lsReverse, "reverse", "r", false, "Reverse sort order")
lsCmd.Flags().BoolVarP(&lsRecursive, "recursive", "R", false, "Recursively list subdirectories")
lsCmd.Flags().BoolVar(&lsJSON, "json", false, "JSON output format")
```

- [ ] **Step 5: Verify build**

```bash
go build -o /dev/null ./cmd/ghoss/main.go
```

Expected: exit 0.

- [ ] **Step 6: Quick smoke test**

```bash
go build -o ./bin/ghoss ./cmd/ghoss/main.go && ./bin/ghoss ls
```

Run without any configured repo — should show "(empty)" or configured-repo contents.

- [ ] **Step 7: Commit**

```bash
git add cmd/ghoss/main.go
git commit -m "feat(ls): wire new flags and lister package"
```
