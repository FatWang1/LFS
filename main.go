// Package main 提供高性能的LFS (Local File Storage) 服务
// 支持大文件分片上传、断点续传、完整性校验和静态文件嵌入
package main

import (
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"lfs/config"
	"lfs/handlers"
	"lfs/optimization"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
)

//go:embed static/*
var staticFiles embed.FS

// 静态文件缓存结构
type cachedFile struct {
	data        []byte
	contentType string
	etag        string
	gzipData    []byte
}

// 静态文件缓存
var (
	staticCache = make(map[string]*cachedFile)
	cacheMutex  sync.RWMutex
)

// 初始化静态文件缓存
func initStaticCache() {
	fsys, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal("Failed to create sub filesystem:", err)
	}

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		log.Fatal("Failed to read static directory:", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		if err := cacheStaticFile(fsys, fileName); err != nil {
			log.Printf("Failed to cache file %s: %v", fileName, err)
		}
	}

	log.Printf("Cached %d static files", len(staticCache))
}

// cacheStaticFile 缓存单个静态文件
func cacheStaticFile(fsys fs.FS, fileName string) error {
	data, err := fs.ReadFile(fsys, fileName)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	contentType := getMimeType(ext)
	etag := fmt.Sprintf(`"%x"`, len(data))

	var gzipData []byte
	if shouldCompress(ext) {
		gzipData = compressData(data)
	}

	cacheMutex.Lock()
	staticCache[fileName] = &cachedFile{
		data:        data,
		contentType: contentType,
		etag:        etag,
		gzipData:    gzipData,
	}
	cacheMutex.Unlock()

	return nil
}

// setupRoutes 设置所有路由
func setupRoutes() *gin.Engine {
	r := gin.New()

	// 中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(gzipMiddleware())

	// 初始化配置
	cfg := config.LoadConfig()

	// API路由
	handlers.RegisterFileHandlers(r, cfg)
	r.GET("/metrics", performanceMetricsHandler())

	// 静态文件路由
	r.GET("/static/*filepath", optimizedStaticFileHandler())
	r.GET("/", homeHandler())

	return r
}

// homeHandler 主页处理器
func homeHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cacheMutex.RLock()
		file, exists := staticCache["index.html"]
		cacheMutex.RUnlock()

		if !exists {
			c.Status(http.StatusNotFound)
			return
		}

		// 设置响应头
		c.Header("Content-Type", file.contentType)
		c.Header("ETag", file.etag)
		c.Header("Cache-Control", "public, max-age=3600")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")

		// 检查客户端是否支持Gzip
		if acceptsGzip(c) && len(file.gzipData) > 0 {
			c.Header("Content-Encoding", "gzip")
			c.Header("Content-Length", fmt.Sprintf("%d", len(file.gzipData)))
			c.Data(http.StatusOK, "", file.gzipData)
		} else {
			c.Header("Content-Length", fmt.Sprintf("%d", len(file.data)))
			c.Data(http.StatusOK, "", file.data)
		}
	}
}

// getMimeType 根据文件扩展名获取MIME类型
func getMimeType(ext string) string {
	switch ext {
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	default:
		return "application/octet-stream"
	}
}

// shouldCompress 判断文件类型是否应该压缩
func shouldCompress(ext string) bool {
	compressibleTypes := map[string]bool{
		".html": true,
		".css":  true,
		".js":   true,
		".svg":  true,
		".txt":  true,
		".json": true,
		".xml":  true,
	}
	return compressibleTypes[ext]
}

// compressData 压缩数据
func compressData(data []byte) []byte {
	var buf strings.Builder
	gz := gzip.NewWriter(&buf)
	gz.Write(data)
	gz.Close()
	return []byte(buf.String())
}

// 检查客户端是否支持Gzip
func acceptsGzip(c *gin.Context) bool {
	return strings.Contains(c.GetHeader("Accept-Encoding"), "gzip")
}

// gzipMiddleware Gzip压缩中间件
func gzipMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// 只对特定类型的响应进行压缩
		if !shouldCompressResponse(c) {
			c.Next()
			return
		}

		// 检查客户端是否支持Gzip
		if !acceptsGzip(c) {
			c.Next()
			return
		}

		// 对于静态文件缓存，跳过中间件压缩（已经预压缩）
		if strings.HasPrefix(c.Request.URL.Path, "/static/") || c.Request.URL.Path == "/" {
			c.Next()
			return
		}

		// 创建Gzip写入器
		gz := gzip.NewWriter(c.Writer)
		defer gz.Close()

		// 设置响应头
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")

		// 替换写入器
		c.Writer = &gzipResponseWriter{Writer: gz, ResponseWriter: c.Writer}
		c.Next()
	})
}

// shouldCompressResponse 判断响应是否应该压缩
func shouldCompressResponse(c *gin.Context) bool {
	// 只对静态文件和API响应进行压缩
	path := c.Request.URL.Path
	return strings.HasPrefix(path, "/static/") ||
		path == "/" ||
		strings.HasPrefix(path, "/files") ||
		strings.HasPrefix(path, "/download")
}

// gzipResponseWriter Gzip响应写入器
type gzipResponseWriter struct {
	io.Writer
	gin.ResponseWriter
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	return w.Writer.Write(data)
}

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Range")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// main 主函数 - LFS服务器入口点
// 初始化所有组件并启动高性能HTTP服务器
func main() {
	// 设置Gin为发布模式以获得更好的性能
	gin.SetMode(gin.ReleaseMode)

	// 初始化性能优化 - 设置最优的GOMAXPROCS
	optimization.SetOptimalGOMAXPROCS()

	// 初始化静态文件缓存 - 预加载和压缩静态资源
	initStaticCache()

	// 设置路由 - 配置所有API和静态文件路由
	r := setupRoutes()

	// 创建HTTP服务器
	server := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	// 配置HTTP/2
	http2.ConfigureServer(server, &http2.Server{
		MaxConcurrentStreams: 1000,
		MaxReadFrameSize:     1048576, // 1MB
	})

	log.Println("LFS Server starting on :8080")
	log.Println("Static files embedded and cached successfully")
	log.Println("HTTP/2 and Gzip compression enabled")
	log.Fatal(server.ListenAndServe())
}

// optimizedStaticFileHandler 优化的静态文件处理器
func optimizedStaticFileHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("filepath")
		path = strings.TrimPrefix(path, "/")

		// 安全检查，防止路径遍历攻击
		if strings.Contains(path, "..") {
			c.Status(http.StatusBadRequest)
			return
		}

		// 如果路径为空，默认返回index.html
		if path == "" {
			path = "index.html"
		}

		// 从缓存中获取文件
		cacheMutex.RLock()
		file, exists := staticCache[path]
		cacheMutex.RUnlock()

		if !exists {
			c.Status(http.StatusNotFound)
			return
		}

		// 检查ETag，支持条件请求
		if c.GetHeader("If-None-Match") == file.etag {
			c.Status(http.StatusNotModified)
			return
		}

		// 设置响应头
		c.Header("Content-Type", file.contentType)
		c.Header("ETag", file.etag)
		c.Header("Cache-Control", "public, max-age=31536000") // 1年缓存
		c.Header("Vary", "Accept-Encoding")

		// 检查客户端是否支持Gzip
		if acceptsGzip(c) && len(file.gzipData) > 0 {
			c.Header("Content-Encoding", "gzip")
			c.Header("Content-Length", fmt.Sprintf("%d", len(file.gzipData)))
			c.Data(http.StatusOK, "", file.gzipData)
		} else {
			c.Header("Content-Length", fmt.Sprintf("%d", len(file.data)))
			c.Data(http.StatusOK, "", file.data)
		}
	}
}

// performanceMetricsHandler 性能监控处理器
func performanceMetricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取内存统计
		memStats := optimization.GetMemoryStats()

		// 获取Goroutine数量
		numGoroutines := runtime.NumGoroutine()

		// 获取CPU核心数
		numCPU := runtime.NumCPU()

		// 获取GOMAXPROCS设置
		maxProcs := runtime.GOMAXPROCS(0)

		metrics := map[string]interface{}{
			"memory": map[string]interface{}{
				"alloc":       memStats.Alloc,
				"total_alloc": memStats.TotalAlloc,
				"sys":         memStats.Sys,
				"num_gc":      memStats.NumGC,
			},
			"runtime": map[string]interface{}{
				"goroutines": numGoroutines,
				"cpu_cores":  numCPU,
				"max_procs":  maxProcs,
			},
			"cache": map[string]interface{}{
				"static_files": len(staticCache),
			},
		}

		c.JSON(http.StatusOK, metrics)
	}
}

// staticFileHandler 处理嵌入的静态文件
func staticFileHandler() gin.HandlerFunc {
	// 创建子文件系统
	fsys, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal("Failed to create sub filesystem:", err)
	}

	return func(c *gin.Context) {
		path := c.Param("filepath")
		path = strings.TrimPrefix(path, "/")

		// 安全检查，防止路径遍历攻击
		if strings.Contains(path, "..") {
			c.Status(http.StatusBadRequest)
			return
		}

		// 如果路径为空，默认返回index.html
		if path == "" {
			path = "index.html"
		}

		// 设置适当的MIME类型
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".css":
			c.Header("Content-Type", "text/css; charset=utf-8")
		case ".js":
			c.Header("Content-Type", "application/javascript; charset=utf-8")
		case ".html":
			c.Header("Content-Type", "text/html; charset=utf-8")
		case ".png":
			c.Header("Content-Type", "image/png")
		case ".jpg", ".jpeg":
			c.Header("Content-Type", "image/jpeg")
		case ".gif":
			c.Header("Content-Type", "image/gif")
		case ".svg":
			c.Header("Content-Type", "image/svg+xml")
		case ".ico":
			c.Header("Content-Type", "image/x-icon")
		case ".woff":
			c.Header("Content-Type", "font/woff")
		case ".woff2":
			c.Header("Content-Type", "font/woff2")
		case ".ttf":
			c.Header("Content-Type", "font/ttf")
		case ".eot":
			c.Header("Content-Type", "application/vnd.ms-fontobject")
		default:
			c.Header("Content-Type", "application/octet-stream")
		}

		// 设置缓存头
		c.Header("Cache-Control", "public, max-age=31536000") // 1年缓存
		c.Header("ETag", `"embedded-static"`)

		// 服务文件
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		// 设置Content-Length
		c.Header("Content-Length", fmt.Sprintf("%d", len(data)))

		// 直接写入文件内容
		c.Data(http.StatusOK, "", data)
	}
}
