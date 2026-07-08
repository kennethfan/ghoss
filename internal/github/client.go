package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client.
type Client struct {
	client *github.Client
	owner  string
	repo   string
	branch string
	ctx    context.Context
}

// NewClient creates a new GitHub API client.
func NewClient(token, owner, repo, branch string) *Client {
	var client *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		client = oauth2.NewClient(context.Background(), ts)
	}

	return &Client{
		client: github.NewClient(client),
		owner:  owner,
		repo:   repo,
		branch: branch,
		ctx:    context.Background(),
	}
}

// UploadFile uploads a file to the GitHub repository.
// GitHub API requires Base64 encoding internally, but we handle it transparently.
func (c *Client) UploadFile(filePath, contentPath, message string) (*github.RepositoryContentResponse, *github.Response, error) {
	// Read file content as raw bytes
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	opts := &github.RepositoryContentFileOptions{
		Content: fileData,
		Message: github.String(message),
		Branch:  github.String(c.branch),
	}

	return c.client.Repositories.CreateFile(c.ctx, c.owner, c.repo, contentPath, opts)
}

// DownloadFile downloads a file from the GitHub repository.
// Returns the original raw file content using go-github library's native method.
func (c *Client) DownloadFile(contentPath string) ([]byte, *github.RepositoryContent, error) {
	// Get file metadata and content from GitHub API
	file, _, _, err := c.client.Repositories.GetContents(c.ctx, c.owner, c.repo, contentPath, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file: %w", err)
	}

	// If it's a file (not a directory)
	if file.Type != nil && *file.Type == "file" {
		// Use the library's native GetDownloadURL() method to get the raw file URL
		downloadURL := file.GetDownloadURL()
		if downloadURL == "" {
			return nil, nil, fmt.Errorf("download URL is not available")
		}

		// Download the file using the standard http client
		resp, err := http.Get(downloadURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to download: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, nil, fmt.Errorf("failed to download: HTTP %d", resp.StatusCode)
		}

		// Read the response body directly (no decoding needed)
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read content: %w", err)
		}

		return content, file, nil
	}

	return nil, nil, fmt.Errorf("path is not a file")
}

// ListFiles lists files in a directory.
func (c *Client) ListFiles(path string) ([]*github.RepositoryContent, *github.Response, error) {
	_, files, resp, err := c.client.Repositories.GetContents(c.ctx, c.owner, c.repo, path, nil)
	if err != nil {
		return nil, nil, err
	}
	return files, resp, nil
}

// ListCommits returns the latest commit for a file path (1 result).
func (c *Client) ListCommits(path string) (*github.RepositoryCommit, error) {
	opts := &github.CommitsListOptions{
		Path: path,
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

// DeleteFile deletes a file from the repository.
func (c *Client) DeleteFile(contentPath, message string) (*github.RepositoryContentResponse, *github.Response, error) {
	// First get the SHA of the file
	file, _, _, err := c.client.Repositories.GetContents(c.ctx, c.owner, c.repo, contentPath, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file: %w", err)
	}

	// Use the SHA from the file object
	sha := file.SHA
	if sha == nil {
		return nil, nil, fmt.Errorf("file SHA is nil")
	}

	opts := &github.RepositoryContentFileOptions{
		SHA:     sha,
		Message: github.String(message),
		Branch:  github.String(c.branch),
	}

	return c.client.Repositories.DeleteFile(c.ctx, c.owner, c.repo, contentPath, opts)
}

// GetFileURL returns the raw file URL using GitHub API's native GetContents method.
func (c *Client) GetFileURL(contentPath string) (string, error) {
	// Use GetContents to get file metadata
	file, _, _, err := c.client.Repositories.GetContents(c.ctx, c.owner, c.repo, contentPath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get file: %w", err)
	}

	// Return the native download URL from the library
	downloadURL := file.GetDownloadURL()
	if downloadURL == "" {
		return "", fmt.Errorf("download URL is not available for: %s", contentPath)
	}

	return downloadURL, nil
}

// IsConfigured checks if the client has required configuration.
func (c *Client) IsConfigured() bool {
	return c.owner != "" && c.repo != ""
}

// GetOwner returns the repository owner.
func (c *Client) GetOwner() string {
	return c.owner
}

// GetRepo returns the repository name.
func (c *Client) GetRepo() string {
	return c.repo
}

// GetBranch returns the branch name.
func (c *Client) GetBranch() string {
	return c.branch
}
