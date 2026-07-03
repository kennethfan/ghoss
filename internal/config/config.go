package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config  holds the application configuration.
type Config struct {
	Github  GithubConfig  `mapstructure:"github"`
	Storage StorageConfig `mapstructure:"storage"`
	Cache   CacheConfig   `mapstructure:"cache"`
}

// GithubConfig holds GitHub-related configuration.
type GithubConfig struct {
	Token string `mapstructure:"token"`
	Owner string `mapstructure:"owner"`
	Repo  string `mapstructure:"repo"`
	Branch string `mapstructure:"branch"`
}

// StorageConfig holds storage-related configuration.
type StorageConfig struct {
	RootPath    string `mapstructure:"root_path"`
	MaxFileSize string `mapstructure:"max_file_size"`
}

// CacheConfig holds cache-related configuration.
type CacheConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Dir     string `mapstructure:"dir"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		Github: GithubConfig{
			Branch: "main",
		},
		Storage: StorageConfig{
			RootPath:    "assets",
			MaxFileSize: "100MB",
		},
		Cache: CacheConfig{
			Enabled: true,
			Dir:     filepath.Join(homeDir, ".ghoss", "cache"),
		},
	}
}

// LoadConfig reads configuration from file and environment variables.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	// Get config file path - use .ghoss as hidden directory
	configDir := filepath.Join(getConfigDir(), ".ghoss")
	configPath := filepath.Join(configDir, "config.yaml")

	// Allow overriding config path via environment variable
	if envPath := os.Getenv("GHOSS_CONFIG"); envPath != "" {
		configPath = envPath
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, err
	}

	// Initialize viper
	viper.SetConfigType("yaml")
	viper.SetConfigFile(configPath)

	// Set defaults
	viper.SetDefault("github.branch", cfg.Github.Branch)
	viper.SetDefault("storage.root_path", cfg.Storage.RootPath)
	viper.SetDefault("storage.max_file_size", cfg.Storage.MaxFileSize)
	viper.SetDefault("cache.enabled", cfg.Cache.Enabled)
	viper.SetDefault("cache.dir", cfg.Cache.Dir)

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// Environment variables override config file
	viper.AutomaticEnv()

	// Unmarshal config
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	// Set file permissions on config file if it exists
	if _, err := os.Stat(configPath); err == nil {
		os.Chmod(configPath, 0600)
	}

	return cfg, nil
}

// SaveConfig saves the configuration to file.
func SaveConfig(cfg *Config, path string) error {
	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	viper.SetConfigType("yaml")
	viper.SetConfigFile(path)

	// Marshal config to viper
	viper.Set("github.token", cfg.Github.Token)
	viper.Set("github.owner", cfg.Github.Owner)
	viper.Set("github.repo", cfg.Github.Repo)
	viper.Set("github.branch", cfg.Github.Branch)
	viper.Set("storage.root_path", cfg.Storage.RootPath)
	viper.Set("storage.max_file_size", cfg.Storage.MaxFileSize)
	viper.Set("cache.enabled", cfg.Cache.Enabled)
	viper.Set("cache.dir", cfg.Cache.Dir)

	if err := viper.WriteConfig(); err != nil {
		return err
	}

	// Set restrictive permissions
	return os.Chmod(path, 0600)
}

// getConfigDir returns the configuration directory based on OS.
func getConfigDir() string {
	if envDir := os.Getenv("GHOSS_CONFIG_DIR"); envDir != "" {
		return envDir
	}

	homeDir, _ := os.UserHomeDir()
	return homeDir
}
