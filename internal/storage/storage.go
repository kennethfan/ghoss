package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// File represents a file in the storage.
type File struct {
	Path    string
	Name    string
	Size    int64
	IsDir   bool
	Modified string
}

// ValidateFilePath validates the file path.
func ValidateFilePath(path string) error {
	// Clean the path
	cleaned := filepath.Clean(path)

	// Check for directory traversal attacks
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "../") {
		return fmt.Errorf("invalid file path: %s", path)
	}

	return nil
}

// GetFileSize returns the size of a file.
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// ListDirectory lists all files in a directory recursively.
func ListDirectory(root string) ([]*File, error) {
	files := make([]*File, 0)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(root, path)
		if relPath == "." {
			return nil
		}

		files = append(files, &File{
			Path:     filepath.Join(root, relPath),
			Name:     info.Name(),
			Size:     info.Size(),
			IsDir:    info.IsDir(),
			Modified: info.ModTime().Format("2006-01-02 15:04:05"),
		})

		return nil
	})

	return files, err
}

// NormalizePath normalizes the file path for GitHub storage.
func NormalizePath(path string) string {
	// Remove leading/trailing slashes
	path = strings.Trim(path, "/")
	// Replace backslashes with forward slashes
	path = strings.ReplaceAll(path, "\\", "/")
	return path
}

// GetRelativePath gets the relative path from a base directory.
func GetRelativePath(base, fullPath string) (string, error) {
	rel, err := filepath.Rel(base, fullPath)
	if err != nil {
		return "", err
	}
	return NormalizePath(rel), nil
}
