package models

import "time"

// File represents a file stored in GitHub.
type File struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
	URL         string    `json:"url"`
	SHA         string    `json:"sha"`
}

// UploadResult represents the result of an upload operation.
type UploadResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
	Error   string `json:"error,omitempty"`
}

// DownloadResult represents the result of a download operation.
type DownloadResult struct {
	Success bool   `json:"success"`
	Path    string `json:"path,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ListResult represents the result of a list operation.
type ListResult struct {
	Success bool   `json:"success"`
	Files   []File `json:"files,omitempty"`
	Error   string `json:"error,omitempty"`
}

// DeleteResult represents the result of a delete operation.
type DeleteResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}
