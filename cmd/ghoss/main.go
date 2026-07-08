package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghoss/internal/config"
	"github.com/ghoss/internal/github"
	"github.com/ghoss/internal/lister"
	"github.com/ghoss/internal/storage"
	"github.com/ghoss/pkg/models"
	"github.com/spf13/cobra"
)

var reader *bufio.Reader

// readInput reads a line of input from stdin.
func readInput(prompt string) string {
	if prompt != "" {
		fmt.Println(prompt)
	}
	if reader == nil {
		reader = bufio.NewReader(os.Stdin)
	}
	input, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(input)
}

var (
	// Global flags
	cfgFile string
	verbose bool
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "ghoss",
	Short: "GitHub Object Storage Tool - Use GitHub as your object storage",
	Long: `GHoss (GitHub Object Storage Service) is a tool that uses GitHub repositories
as object storage. You can upload, download, and manage files through simple CLI commands.

Warning: GitHub is not designed as object storage. There are limitations:
- Single file size limit: 100MB
- API rate limits apply
- Not suitable for high-frequency access

This tool is intended for personal/learning purposes only.`,
}

// initCmd represents the init command.
var initCmd = &cobra.Command{
	Use:   "init [config-path]",
	Short: "Initialize configuration",
	Long: `Initialize the configuration file. You will be prompted to enter:
- GitHub Personal Access Token
- Repository owner (username or organization)
- Repository name
- Branch name (default: main)

The configuration will be saved to ~/.ghoss/config.yaml or the specified path.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		homeDir, _ := os.UserHomeDir()
		configPath := filepath.Join(homeDir, ".ghoss", "config.yaml")

		if len(args) > 0 {
			configPath = args[0]
		} else if cfgFile != "" {
			configPath = cfgFile
		}

		fmt.Printf("Configuration will be saved to: %s\n\n", configPath)

		// Read user input
		token := readInput("GitHub Token: ")
		if token == "" {
			fmt.Fprintln(os.Stderr, "Error: GitHub token is required")
			os.Exit(1)
		}

		owner := readInput("Repository Owner: ")
		if owner == "" {
			fmt.Fprintln(os.Stderr, "Error: repository owner is required")
			os.Exit(1)
		}

		repo := readInput("Repository Name: ")
		if repo == "" {
			fmt.Fprintln(os.Stderr, "Error: repository name is required")
			os.Exit(1)
		}

		branch := readInput("Branch (default: main): ")
		if branch == "" {
			branch = "main"
		}

		// Create configuration
		cfg := config.DefaultConfig()
		cfg.Github.Token = token
		cfg.Github.Owner = owner
		cfg.Github.Repo = repo
		cfg.Github.Branch = branch

		if err := config.SaveConfig(cfg, configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving configuration: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✓ Configuration saved to %s\n", configPath)
		fmt.Println("✓ You can now use ghoss to manage your files!")
	},
}

// uploadCmd represents the upload command.
var uploadCmd = &cobra.Command{
	Use:   "upload [file paths...]",
	Short: "Upload files to GitHub repository",
	Long:  `Upload one or more files to your GitHub repository.`,
	Example: `  ghoss upload ./photo.jpg
  ghoss upload ./file.txt --path docs/file.txt
  ghoss upload ./images/ --recursive`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Error: please specify files to upload")
			cmd.Help()
			os.Exit(1)
		}

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		// Create GitHub client
		client := github.NewClient(cfg.Github.Token, cfg.Github.Owner, cfg.Github.Repo, cfg.Github.Branch)

		if !client.IsConfigured() {
			fmt.Fprintln(os.Stderr, "Error: GitHub repository not configured. Please run 'ghoss init' first.")
			os.Exit(1)
		}

		// Process each file
		for _, filePath := range args {
			result := uploadFile(client, filePath, cfg)
			printResult(result, verbose)
		}
	},
}

var uploadPath string
var uploadMessage string
var uploadRecursive bool

// uploadFile handles the actual file upload logic.
func uploadFile(client *github.Client, filePath string, cfg *config.Config) models.UploadResult {
	info, err := os.Stat(filePath)
	if err != nil {
		return models.UploadResult{
			Success: false,
			Message: fmt.Sprintf("Failed to access file: %s", filePath),
			Error:   err.Error(),
		}
	}

	if info.IsDir() && !uploadRecursive {
		return models.UploadResult{
			Success: false,
			Message: "Cannot upload directory without --recursive flag",
		}
	}

	if info.IsDir() && uploadRecursive {
		return uploadDirectory(client, filePath, cfg)
	}

	// Validate file path
	if err := storage.ValidateFilePath(filePath); err != nil {
		return models.UploadResult{
			Success: false,
			Message: "Invalid file path",
			Error:   err.Error(),
		}
	}

	// Determine target path in repository
	targetPath := uploadPath
	if targetPath == "" {
		targetPath = filepath.Base(filePath)
	}

	targetPath = storage.NormalizePath(targetPath)

	// Check file size
	maxSize, _ := parseFileSize(cfg.Storage.MaxFileSize)
	if info.Size() > maxSize {
		return models.UploadResult{
			Success: false,
			Message: "File size exceeds limit",
			Error:   fmt.Sprintf("File is %d bytes, maximum is %d bytes", info.Size(), maxSize),
		}
	}

	// Upload file
	message := uploadMessage
	if message == "" {
		message = fmt.Sprintf("upload: %s", filepath.Base(filePath))
	}

	_, _, err = client.UploadFile(filePath, targetPath, message)
	if err != nil {
		return models.UploadResult{
			Success: false,
			Message: "Upload failed",
			Error:   err.Error(),
		}
	}

	fileURL, err := client.GetFileURL(targetPath)
	if err != nil {
		fileURL = ""
	}
	return models.UploadResult{
		Success: true,
		Message: fmt.Sprintf("Successfully uploaded: %s", filePath),
		URL:     fileURL,
	}
}

// uploadDirectory handles recursive directory upload.
func uploadDirectory(client *github.Client, dirPath string, cfg *config.Config) models.UploadResult {
	files, err := storage.ListDirectory(dirPath)
	if err != nil {
		return models.UploadResult{
			Success: false,
			Message: "Failed to list directory",
			Error:   err.Error(),
		}
	}

	uploaded := 0
	failed := 0

	for _, file := range files {
		if file.IsDir {
			continue
		}

		relPath, _ := storage.GetRelativePath(dirPath, file.Path)
		targetPath := filepath.Join(cfg.Storage.RootPath, relPath)
		targetPath = storage.NormalizePath(targetPath)

		message := fmt.Sprintf("upload: %s", file.Name)
		_, _, err := client.UploadFile(file.Path, targetPath, message)
		if err != nil {
			failed++
			if verbose {
				fmt.Fprintf(os.Stderr, "Failed to upload %s: %v\n", file.Path, err)
			}
		} else {
			uploaded++
		}
	}

	return models.UploadResult{
		Success: failed == 0,
		Message: fmt.Sprintf("Uploaded %d files, failed %d", uploaded, failed),
	}
}

// downloadCmd represents the download command.
var downloadCmd = &cobra.Command{
	Use:   "download [file path]",
	Short: "Download files from GitHub repository",
	Long:  `Download a file from your GitHub repository to local storage.`,
	Example: `  ghoss download photos/image.jpg
  ghoss download photos/image.jpg --output ./local/path.jpg`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Error: please specify the file path to download")
			cmd.Help()
			os.Exit(1)
		}

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		// Create GitHub client
		client := github.NewClient(cfg.Github.Token, cfg.Github.Owner, cfg.Github.Repo, cfg.Github.Branch)

		if !client.IsConfigured() {
			fmt.Fprintln(os.Stderr, "Error: GitHub repository not configured. Please run 'ghoss init' first.")
			os.Exit(1)
		}

		filePath := storage.NormalizePath(args[0])
		outputPath := outputPathFlag

		if outputPath == "" {
			outputPath = filepath.Base(filePath)
		}

		result := downloadFile(client, filePath, outputPath)
		if result.Success {
			fmt.Printf("Successfully downloaded to: %s\n", result.Path)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", result.Error)
		}
	},
}

var outputPathFlag string

// downloadFile handles the actual file download logic.
func downloadFile(client *github.Client, filePath, outputPath string) models.DownloadResult {
	content, _, err := client.DownloadFile(filePath)
	if err != nil {
		return models.DownloadResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		return models.DownloadResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	return models.DownloadResult{
		Success: true,
		Path:    outputPath,
	}
}

// ls command flags
var (
	lsLong        bool
	lsHuman       bool
	lsSortBy      string
	lsSortByTime  bool
	lsSortBySize  bool
	lsReverse     bool
	lsRecursive   bool
	lsJSON        bool
)

// lsCmd represents the list command.
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

		// -t and -S override --sort (S overrides t)
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

// rmCmd represents the remove command.
var rmCmd = &cobra.Command{
	Use:   "rm [file path]",
	Short: "Delete a file from GitHub repository",
	Long:  `Delete a file from your GitHub repository.`,
	Example: `  ghoss rm photos/image.jpg
  ghoss rm docs/report.pdf`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Error: please specify the file path to delete")
			cmd.Help()
			os.Exit(1)
		}

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		// Create GitHub client
		client := github.NewClient(cfg.Github.Token, cfg.Github.Owner, cfg.Github.Repo, cfg.Github.Branch)

		if !client.IsConfigured() {
			fmt.Fprintln(os.Stderr, "Error: GitHub repository not configured. Please run 'ghoss init' first.")
			os.Exit(1)
		}

		filePath := storage.NormalizePath(args[0])
		message := "delete: " + filepath.Base(filePath)

		_, _, err = client.DeleteFile(filePath, message)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully deleted: %s\n", filePath)
	},
}

// urlCmd generates the public URL for a file.
var urlCmd = &cobra.Command{
	Use:   "url [file path]",
	Short: "Get public URL for a file",
	Long:  `Generate the public access URL for a file in your GitHub repository.`,
	Example: `  ghoss url photos/image.jpg
  ghoss url docs/report.pdf | xargs open`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Error: please specify the file path")
			cmd.Help()
			os.Exit(1)
		}

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		// Create GitHub client
		client := github.NewClient(cfg.Github.Token, cfg.Github.Owner, cfg.Github.Repo, cfg.Github.Branch)

		if !client.IsConfigured() {
			fmt.Fprintln(os.Stderr, "Error: GitHub repository not configured. Please run 'ghoss init' first.")
			os.Exit(1)
		}

		filePath := storage.NormalizePath(args[0])
		fileURL, err := client.GetFileURL(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(fileURL)
		fmt.Printf("\nAccess your file at: %s\n", fileURL)
	},
}

// Helper functions

func printResult(result models.UploadResult, verbose bool) {
	if verbose {
		fmt.Println(result.Message)
		if result.URL != "" {
			fmt.Printf("URL: %s\n", result.URL)
		}
		if result.Error != "" {
			fmt.Fprintf(os.Stderr, "Error: %s\n", result.Error)
		}
	} else {
		if result.Success {
			fmt.Println(result.Message)
			if result.URL != "" {
				fmt.Printf("URL: %s\n", result.URL)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", result.Message)
			if result.Error != "" {
				fmt.Fprintf(os.Stderr, "Details: %s\n", result.Error)
			}
		}
	}
}

func parseFileSize(sizeStr string) (int64, error) {
	// Simple parser for sizes like "100MB", "10GB", etc.
	sizeStr = strings.ToUpper(sizeStr)
	
	var multiplier int64 = 1
	switch {
	case strings.HasSuffix(sizeStr, "GB"):
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "GB")
	case strings.HasSuffix(sizeStr, "MB"):
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "MB")
	case strings.HasSuffix(sizeStr, "KB"):
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "KB")
	}

	var size int64
	fmt.Sscanf(sizeStr, "%d", &size)
	
	return size * multiplier, nil
}

func main() {
	// Add global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Config file path")
	
	// Add flags
	uploadCmd.Flags().StringVarP(&uploadPath, "path", "p", "", "Target path in repository")
	uploadCmd.Flags().StringVarP(&uploadMessage, "message", "m", "", "Git commit message")
	uploadCmd.Flags().BoolVarP(&uploadRecursive, "recursive", "r", false, "Recursively upload directory")
	
	downloadCmd.Flags().StringVarP(&outputPathFlag, "output", "o", "", "Output file path")

	// ls flags
	lsCmd.Flags().BoolVarP(&lsLong, "long", "l", false, "Long format with permissions, size, and time")
	lsCmd.Flags().BoolVar(&lsHuman, "human-readable", false, "Human-readable file sizes (implied by -l)")
	lsCmd.Flags().StringVar(&lsSortBy, "sort", "name", "Sort key: name|time|size")
	lsCmd.Flags().BoolVarP(&lsSortByTime, "sort-by-time", "t", false, "Sort by last modified time")
	lsCmd.Flags().BoolVarP(&lsSortBySize, "sort-by-size", "S", false, "Sort by file size")
	lsCmd.Flags().BoolVarP(&lsReverse, "reverse", "r", false, "Reverse sort order")
	lsCmd.Flags().BoolVarP(&lsRecursive, "recursive", "R", false, "Recursively list subdirectories")
	lsCmd.Flags().BoolVar(&lsJSON, "json", false, "JSON output format")

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Add subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(uploadCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(urlCmd)

	// Execute
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
