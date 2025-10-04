package config

import (
	"fmt"
	"os"
)

// Config 配置结构体
type Config struct {
	StoragePath string `json:"storage_path"`
}

// LoadConfig 加载配置
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
