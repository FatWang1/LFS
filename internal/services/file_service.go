package services

import (
	"context"
	"errors"
	"mime/multipart"
	"strings"
	"sync"

	"lfs/internal/interfaces"

	"github.com/gin-gonic/gin"
)

// FileService implements file service business logic.
// It encapsulates file storage and MD5 calculation implementations, providing a unified business interface.
type FileService struct {
	storage     interfaces.Storage
	md5Calc     interfaces.MD5Calculator
	storagePath string
}

// NewFileService creates and returns a new file service instance.
// storage is used for file storage operations, md5Calc is used for MD5 calculation, storagePath is the storage path.
func NewFileService(storage interfaces.Storage, md5Calc interfaces.MD5Calculator, storagePath string) *FileService {
	return &FileService{
		storage:     storage,
		md5Calc:     md5Calc,
		storagePath: storagePath,
	}
}

// UploadFile uploads a file.
func (s *FileService) UploadFile(ctx context.Context, file *multipart.FileHeader, rangeHeader string) error {
	return s.storage.SaveFile(ctx, file, rangeHeader)
}

// UploadFileChunk uploads a file chunk.
func (s *FileService) UploadFileChunk(ctx context.Context, chunkInfo interfaces.FileChunkInfo, file *multipart.FileHeader) error {
	return s.storage.SaveFileChunk(ctx, chunkInfo, file)
}

// BatchUpload performs batch upload (reuses single file upload implementation, supports concurrent processing).
func (s *FileService) BatchUpload(ctx context.Context, files []*multipart.FileHeader) (successCount, errorCount int, errors []string) {
	if len(files) == 0 {
		return 0, 0, nil
	}

	// Single file: directly call single file upload
	if len(files) == 1 {
		if err := s.storage.SaveFile(ctx, files[0], ""); err != nil {
			return 0, 1, []string{err.Error()}
		}
		return 1, 0, nil
	}

	// Multiple files: concurrent processing
	type uploadResult struct {
		err error
	}

	resultChan := make(chan uploadResult, len(files))
	var wg sync.WaitGroup

	// Limit concurrency to avoid resource exhaustion
	maxConcurrent := 10
	if maxConcurrent > len(files) {
		maxConcurrent = len(files)
	}

	semaphore := make(chan struct{}, maxConcurrent)

	for _, file := range files {
		wg.Add(1)
		go func(f *multipart.FileHeader) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			err := s.storage.SaveFile(ctx, f, "")
			resultChan <- uploadResult{err: err}
		}(file)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	errorList := make([]string, 0)
	for result := range resultChan {
		if result.err != nil {
			errorCount++
			errorList = append(errorList, result.err.Error())
		} else {
			successCount++
		}
	}

	return successCount, errorCount, errorList
}

// DownloadFile downloads a file.
func (s *FileService) DownloadFile(ctx context.Context, c *gin.Context, filename, rangeHeader string) error {
	return s.storage.DownloadFile(ctx, c, filename, rangeHeader)
}

// DownloadFileChunk downloads a file chunk.
func (s *FileService) DownloadFileChunk(ctx context.Context, c *gin.Context, filename string, chunkIndex, chunkSize int64) error {
	return s.storage.DownloadFileChunk(ctx, c, filename, chunkIndex, chunkSize)
}

// ListFiles lists files.
func (s *FileService) ListFiles(ctx context.Context, path string) ([]interfaces.FileMetadata, error) {
	// Security check: prevent path traversal attacks
	if path != "" && strings.Contains(path, "..") {
		return nil, errors.New("invalid path")
	}

	// If a path is specified, storage path needs to be adjusted
	// This is simplified; actual implementation should support subdirectories
	return s.storage.ListFiles(ctx)
}

// GetFileMD5 gets the MD5 hash of a file.
func (s *FileService) GetFileMD5(ctx context.Context, filename string) (string, error) {
	filePath := s.storage.GetFilePath(filename)
	return s.md5Calc.GetMD5(ctx, filePath)
}

// GetFileMD5Progress gets the MD5 calculation progress.
func (s *FileService) GetFileMD5Progress(filename string) (float64, bool, string) {
	filePath := s.storage.GetFilePath(filename)
	return s.md5Calc.GetMD5Progress(filePath)
}

// CheckFileExists checks if a file exists.
func (s *FileService) CheckFileExists(ctx context.Context, filename string) error {
	return s.storage.CheckFileExists(ctx, filename)
}
