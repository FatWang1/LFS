package config

import (
	"fmt"
	"os"
)

// Config represents the application configuration.
type Config struct {
	StoragePath string `json:"storage_path"` // File storage path
}

// LoadConfig loads configuration from environment variables.
// If LFS_STORAGE_PATH is not set, uses default path "$HOME/Downloads/".
func LoadConfig() Config {
	storagePath := os.Getenv("LFS_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "$HOME/Downloads/"
		fmt.Printf("STORAGE_PATH not set, using default: %s\n", storagePath)
	}
	return Config{
		StoragePath: storagePath,
	}
}
