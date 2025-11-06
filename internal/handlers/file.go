package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"lfs/internal/interfaces"

	"github.com/gin-gonic/gin"
)

// Response defines the standard format for HTTP responses.
type Response struct {
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// successResponse returns a success response.
func successResponse(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Message: message,
		Data:    data,
	})
}

// errorResponse returns an error response.
// statusCode is the HTTP status code, message is the error message.
func errorResponse(c *gin.Context, statusCode int, message string) {
	log.Printf("Error: %s", message)
	c.JSON(statusCode, Response{Error: message})
}

// FileHandlers handles file-related HTTP requests.
// It depends on FileService to handle business logic, achieving separation of concerns.
type FileHandlers struct {
	fileService interfaces.FileService
}

// NewFileHandlers creates and returns a new file handlers instance.
func NewFileHandlers(fileService interfaces.FileService) *FileHandlers {
	return &FileHandlers{
		fileService: fileService,
	}
}

// Register registers file-related HTTP routes.
func (h *FileHandlers) Register(r *gin.Engine) {
	r.POST("/upload", h.UploadFile)
	r.POST("/batch-upload", h.BatchUpload)
	r.POST("/upload-chunk", h.UploadChunk)
	r.GET("/download/:filename", h.DownloadFile)
	r.GET("/download-chunk/:filename", h.DownloadChunk)
	r.GET("/batch-download", h.BatchDownload)
	r.GET("/files", h.ListFiles)
	r.GET("/file-md5/:filename", h.GetFileMD5)
	r.GET("/file-md5-progress/:filename", h.GetFileMD5Progress)
}

// UploadFile handles single file upload requests with resumable transfer support.
func (h *FileHandlers) UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Failed to get file: "+err.Error())
		return
	}

	rangeHeader := c.GetHeader("Range")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.fileService.UploadFile(ctx, file, rangeHeader); err != nil {
		if ctx.Err() != nil {
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to save file: "+err.Error())
		return
	}

	successResponse(c, "File uploaded successfully", nil)
}

// UploadChunk handles file chunk upload requests.
func (h *FileHandlers) UploadChunk(c *gin.Context) {
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
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	chunkInfo := interfaces.FileChunkInfo{
		FileName:   fileName,
		TotalSize:  totalSize,
		ChunkIndex: chunkIndex,
		ChunkSize:  chunkSize,
		TotalChunk: totalChunk,
		MD5:        md5sum,
	}

	ctx := c.Request.Context()
	if err := h.fileService.UploadFileChunk(ctx, chunkInfo, file); err != nil {
		if ctx.Err() != nil {
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

// BatchUpload handles batch file upload requests.
func (h *FileHandlers) BatchUpload(c *gin.Context) {
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

	ctx := c.Request.Context()
	successCount, errorCount, errors := h.fileService.BatchUpload(ctx, files)

	c.JSON(http.StatusOK, gin.H{
		"message":       "Batch upload completed",
		"total":         len(files),
		"success_count": successCount,
		"error_count":   errorCount,
		"errors":        errors,
	})
}

// DownloadFile handles file download requests with resumable transfer support.
func (h *FileHandlers) DownloadFile(c *gin.Context) {
	filename := c.Param("filename")
	rangeHeader := c.GetHeader("Range")
	ctx := c.Request.Context()

	if err := h.fileService.DownloadFile(ctx, c, filename, rangeHeader); err != nil {
		if ctx.Err() != nil {
			return
		}
		if !c.Writer.Written() {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		}
	}
}

// DownloadChunk handles file chunk download requests.
func (h *FileHandlers) DownloadChunk(c *gin.Context) {
	filename := c.Param("filename")

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

	ctx := c.Request.Context()
	if err := h.fileService.DownloadFileChunk(ctx, c, filename, chunkIndex, chunkSize); err != nil {
		if ctx.Err() != nil {
			return
		}
		if !c.Writer.Written() {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		}
	}
}

// BatchDownload handles batch file download requests.
func (h *FileHandlers) BatchDownload(c *gin.Context) {
	filenames := c.QueryArray("filenames")
	if len(filenames) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No filenames provided"})
		return
	}

	// Single file: directly return download link
	if len(filenames) == 1 {
		// Check if file exists
		ctx := c.Request.Context()
		if err := h.fileService.CheckFileExists(ctx, filenames[0]); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"message":       "Batch download check completed",
				"total":         1,
				"success_count": 0,
				"error_count":   1,
				"errors":        []string{err.Error()},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":       "Batch download check completed",
			"total":         1,
			"success_count": 1,
			"error_count":   0,
			"errors":        []string{},
		})
		return
	}

	// Multiple files: concurrently check file existence
	type checkResult struct {
		filename string
		err      error
	}

	resultChan := make(chan checkResult, len(filenames))
	var wg sync.WaitGroup

	// Limit concurrency
	maxConcurrent := 20
	if maxConcurrent > len(filenames) {
		maxConcurrent = len(filenames)
	}
	semaphore := make(chan struct{}, maxConcurrent)

	for _, filename := range filenames {
		wg.Add(1)
		go func(fn string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			ctx := c.Request.Context()
			err := h.fileService.CheckFileExists(ctx, fn)
			resultChan <- checkResult{filename: fn, err: err}
		}(filename)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var successCount, errorCount int
	var errorList []string

	for result := range resultChan {
		if result.err != nil {
			errorCount++
			errorList = append(errorList, result.err.Error())
		} else {
			successCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Batch download check completed",
		"total":         len(filenames),
		"success_count": successCount,
		"error_count":   errorCount,
		"errors":        errorList,
	})
}

// ListFiles handles file list query requests.
func (h *FileHandlers) ListFiles(c *gin.Context) {
	pathParam := c.Query("path")

	if pathParam != "" && strings.Contains(pathParam, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid path"})
		return
	}

	ctx := c.Request.Context()
	files, err := h.fileService.ListFiles(ctx, pathParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": files})
}

// GetFileMD5 handles file MD5 query requests.
func (h *FileHandlers) GetFileMD5(c *gin.Context) {
	filename := c.Param("filename")
	ctx := c.Request.Context()

	md5sum, err := h.fileService.GetFileMD5(ctx, filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"filename": filename,
		"md5":      md5sum,
	})
}

// GetFileMD5Progress handles MD5 calculation progress query requests.
func (h *FileHandlers) GetFileMD5Progress(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		errorResponse(c, http.StatusBadRequest, "filename is required")
		return
	}

	progress, calculating, errorMsg := h.fileService.GetFileMD5Progress(filename)

	response := gin.H{
		"filename":    filename,
		"progress":    progress,
		"calculating": calculating,
	}

	if errorMsg != "" {
		response["error"] = errorMsg
	}

	c.JSON(http.StatusOK, response)
}
