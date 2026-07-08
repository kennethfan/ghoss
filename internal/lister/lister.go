package lister

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/ghoss/internal/github"
	gogithub "github.com/google/go-github/github"
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
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	Type         string    `json:"type"` // "file" | "dir"
	SHA          string    `json:"sha"`
	ModTime      time.Time `json:"mod_time,omitempty"`
	ModTimeValid bool      `json:"-"`
}

const apiTimeThreshold = 30

const dirPerm = "drwxr-xr-x"
const filePerm = "-rw-r--r--"

// humanSize converts bytes to a human-readable string.
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

// listAllRecursive walks all subdirectories recursively via API calls.
func listAllRecursive(client *github.Client, dirPath string) ([]*gogithub.RepositoryContent, error) {
	var all []*gogithub.RepositoryContent
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

// fetchModTimes fetches the last commit time for each entry in parallel.
// Degrades silently when count exceeds threshold or on individual failures.
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

// toEntries converts go-github RepositoryContent slice to our Entry slice.
func toEntries(contents []*gogithub.RepositoryContent) []*Entry {
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
// Directories always sort before files within the same sort key.
func sortEntries(entries []*Entry, sortBy string, reverse bool) {
	var less func(i, j int) bool
	switch sortBy {
	case "size":
		less = func(i, j int) bool {
			if entries[i].Type != entries[j].Type {
				return entries[i].Type == "dir"
			}
			return entries[i].Size < entries[j].Size
		}
	case "time":
		less = func(i, j int) bool {
			if entries[i].Type != entries[j].Type {
				return entries[i].Type == "dir"
			}
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

// writeTable writes aligned column output to w.
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

// Run executes the ls command with given options.
// It does NOT exit the process — caller handles errors.
func Run(client *github.Client, lsPath string, opts Options) error {
	var contents []*gogithub.RepositoryContent
	var err error

	if opts.Recursive {
		contents, err = listAllRecursive(client, lsPath)
	} else {
		contents, _, err = client.ListFiles(lsPath)
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
