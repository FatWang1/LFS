// Package main provides a high-performance LFS (Local File Storage) service.
// It supports large file chunked uploads, resumable transfers, integrity checks, and embedded static files.
// Uses interface isolation and dependency injection for high readability, extensibility, and performance.
package main

import (
	"embed"
	"log"

	"lfs/config"
	"lfs/internal/app"
	"lfs/pkg/optimization"

	"github.com/gin-gonic/gin"
)

//go:embed web/static/*
var staticFiles embed.FS

// main is the application entry point.
// It initializes configuration, creates an application instance, and starts the HTTP server.
func main() {
	// Set Gin to release mode for better performance
	gin.SetMode(gin.ReleaseMode)

	// Initialize performance optimization - set optimal GOMAXPROCS
	optimization.SetOptimalGOMAXPROCS()

	// Load configuration
	cfg := config.LoadConfig()

	// Create application instance (using dependency injection)
	application := app.NewApp(cfg, staticFiles)

	// Run application
	if err := application.Run(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
