package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"lfs/config"
	"lfs/optimization"
	"lfs/storage"

	"github.com/gin-gonic/gin"
)

// 响应结构体
type Response struct {
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// 成功响应
func successResponse(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Message: message,
		Data:    data,
	})
}

// 错误响应
func errorResponse(c *gin.Context, statusCode int, message string) {
	log.Printf("Error: %s", message)
	c.JSON(statusCode, Response{Error: message})
}

// RegisterFileHandlers 注册文件处理路由
func RegisterFileHandlers(r *gin.Engine, cfg config.Config) {
	// 先注册具体路由
	r.POST("/upload", uploadFileHandler(cfg))
	r.POST("/batch-upload", batchUploadHandler(cfg)) // 批量上传
	r.POST("/upload-chunk", uploadChunkHandler(cfg)) // 分片上传
	r.GET("/download/:filename", downloadFileHandler(cfg))
	r.GET("/download-chunk/:filename", downloadChunkHandler(cfg)) // 分片下载
	r.GET("/batch-download", batchDownloadHandler(cfg))           // 批量下载
	r.GET("/files", listFilesHandler(cfg))                        // 添加列出文件路由
	r.GET("/file-md5/:filename", getFileMD5Handler(cfg))          // 获取文件MD5
}

// uploadFileHandler 处理单个文件上传请求
func uploadFileHandler(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "Failed to get file: "+err.Error())
			return
		}

		rangeHeader := c.GetHeader("Range")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err = storage.SaveFileWithTimeout(ctx, cfg.StoragePath, file, rangeHeader)
		if err != nil {
			if c.Request.Context().Err() != nil {
				return // 客户端断开连接
			}
			errorResponse(c, http.StatusInternalServerError, "Failed to save file: "+err.Error())
			return
		}

		successResponse(c, "File uploaded successfully", nil)
	}
}

// uploadChunkHandler 处理文件分片上传请求
func uploadChunkHandler(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 解析分片信息
		fileName := c.PostForm("fileName")
		totalSize, err := strconv.ParseInt(c.PostForm("totalSize"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid totalSize"})
			return
		}

		chunkIndex, err := strconv.Atoi(c.PostForm("chunkIndex"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chunkIndex"})
			return
		}

		chunkSize, err := strconv.ParseInt(c.PostForm("chunkSize"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chunkSize"})
			return
		}

		totalChunk, err := strconv.Atoi(c.PostForm("totalChunk"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid totalChunk"})
			return
		}

		md5sum := c.PostForm("md5")

		// 获取上传的分片文件
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 构造分片信息
		chunkInfo := storage.FileChunkInfo{
			FileName:   fileName,
			TotalSize:  totalSize,
			ChunkIndex: chunkIndex,
			ChunkSize:  chunkSize,
			TotalChunk: totalChunk,
			MD5:        md5sum,
		}

		// 保存分片
		err = storage.SaveFileChunk(cfg.StoragePath, chunkInfo, file)
		if err != nil {
			// 检查是否是客户端断开连接导致的错误
			if c.Request.Context().Err() != nil {
				// 客户端断开连接，不返回错误
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":    "Chunk uploaded successfully",
			"chunkIndex": chunkIndex,
		})
	}
}

// batchUploadHandler 处理批量文件上传请求
func batchUploadHandler(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取所有上传的文件
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		files := form.File["files"]
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No files provided"})
			return
		}

		// 创建并发处理器
		processor := optimization.NewConcurrentProcessor()

		// 创建任务列表
		tasks := make([]optimization.TaskFunc, len(files))

		// 统计结果
		var (
			successCount int64
			errorCount   int64
			mutex        sync.Mutex
		)

		// 为每个文件创建上传任务
		for i, file := range files {
			// 创建局部变量以避免闭包问题
			file := file
			index := i

			tasks[index] = func() error {
				rangeHeader := c.GetHeader("Range")
				// 使用带超时的上传
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				err := storage.SaveFileWithTimeout(ctx, cfg.StoragePath, file, rangeHeader)

				mutex.Lock()
				if err != nil {
					errorCount++
				} else {
					successCount++
				}
				mutex.Unlock()

				return err
			}
		}

		// 并发执行所有上传任务
		ctx := context.Background()
		errors := processor.Process(ctx, tasks)

		// 收集错误详情
		errorDetails := make([]string, 0)
		for _, err := range errors {
			if err != nil {
				errorDetails = append(errorDetails, err.Error())
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Batch upload completed",
			"total":         len(files),
			"success_count": successCount,
			"error_count":   errorCount,
			"errors":        errorDetails,
		})
	}
}

// listFilesHandler 处理文件列表请求
func listFilesHandler(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		files, err := storage.ListFiles(cfg.StoragePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"files": files})
	}
}

// downloadFileHandler 处理单个文件下载请求
func downloadFileHandler(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		filename := c.Param("filename")
		rangeHeader := c.GetHeader("Range")

		// 对于大文件下载，不设置超时，依赖HTTP连接本身的超时机制
		// 这样可以支持长时间的大文件传输
		ctx := context.Background()

		// 检查客户端是否已经断开连接
		if c.Request.Context().Err() != nil {
			return
		}

		err := storage.DownloadFileWithTimeout(ctx, c, cfg.StoragePath, filename, rangeHeader)
		if err != nil {
			// 检查是否是客户端断开连接导致的错误
			if c.Request.Context().Err() != nil {
				// 客户端断开连接，不返回错误
				return
			}
			// 只有在响应头未写入的情况下才返回错误
			if !c.Writer.Written() {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			}
			return
		}
	}
}

// downloadChunkHandler 处理文件分片下载请求
func downloadChunkHandler(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		filename := c.Param("filename")

		// 获取分片参数
		chunkIndex, err := strconv.ParseInt(c.Query("chunkIndex"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chunkIndex"})
			return
		}

		chunkSize, err := strconv.ParseInt(c.Query("chunkSize"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chunkSize"})
			return
		}

		// 检查客户端是否已经断开连接
		if c.Request.Context().Err() != nil {
			return
		}

		err = storage.DownloadFileChunk(c, cfg.StoragePath, filename, chunkIndex, chunkSize)
		if err != nil {
			// 检查是否是客户端断开连接导致的错误
			if c.Request.Context().Err() != nil {
				// 客户端断开连接，不返回错误
				return
			}
			// 只有在响应头未写入的情况下才返回错误
			if !c.Writer.Written() {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			}
			return
		}
	}
}

// batchDownloadHandler 处理批量文件下载请求
func batchDownloadHandler(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取要下载的文件名列表
		filenames := c.QueryArray("filenames")
		if len(filenames) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No filenames provided"})
			return
		}

		// 创建并发处理器
		processor := optimization.NewConcurrentProcessor()

		// 创建任务列表
		tasks := make([]optimization.TaskFunc, len(filenames))

		// 统计结果
		var (
			successCount int64
			errorCount   int64
			mutex        sync.Mutex
		)

		// 为每个文件创建下载检查任务
		for i, filename := range filenames {
			// 创建局部变量以避免闭包问题
			filename := filename
			index := i

			tasks[index] = func() error {
				// 检查文件是否存在
				err := storage.CheckFileExists(cfg.StoragePath, filename)

				mutex.Lock()
				if err != nil {
					errorCount++
				} else {
					successCount++
				}
				mutex.Unlock()

				return err
			}
		}

		// 并发执行所有下载检查任务
		ctx := context.Background()
		errors := processor.Process(ctx, tasks)

		// 收集错误详情
		errorDetails := make([]string, 0)
		for _, err := range errors {
			if err != nil {
				errorDetails = append(errorDetails, err.Error())
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Batch download check completed",
			"total":         len(filenames),
			"success_count": successCount,
			"error_count":   errorCount,
			"errors":        errorDetails,
		})
	}
}

// getFileMD5Handler 处理获取文件MD5值的请求
func getFileMD5Handler(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		filename := c.Param("filename")

		// 检查文件是否存在
		err := storage.CheckFileExists(cfg.StoragePath, filename)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
			return
		}

		// 计算文件MD5
		md5sum, err := storage.GetFileMD5(cfg.StoragePath, filename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"filename": filename,
			"md5":      md5sum,
		})
	}
}
