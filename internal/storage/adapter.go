package storage

import (
	"context"
	"mime/multipart"
	"path/filepath"
	"strings"

	"lfs/internal/interfaces"

	"github.com/gin-gonic/gin"
)

// StorageAdapter implements the Storage interface, providing file storage operations.
// It delegates interface calls to the underlying file storage implementation.
type StorageAdapter struct {
	storagePath string
	md5Cache    interfaces.MD5Cache
}

// NewStorageAdapter creates and returns a new storage adapter instance.
// storagePath is the file storage path, md5Cache is used for MD5 value caching.
func NewStorageAdapter(storagePath string, md5Cache interfaces.MD5Cache) *StorageAdapter {
	return &StorageAdapter{
		storagePath: storagePath,
		md5Cache:    md5Cache,
	}
}

// SaveFile saves a file with resumable transfer support.
func (a *StorageAdapter) SaveFile(ctx context.Context, file *multipart.FileHeader, rangeHeader string) error {
	return SaveFileWithTimeout(ctx, a.storagePath, file, rangeHeader)
}

// SaveFileChunk saves a file chunk.
func (a *StorageAdapter) SaveFileChunk(ctx context.Context, chunkInfo interfaces.FileChunkInfo, file *multipart.FileHeader) error {
	// Convert to internal type
	internalChunkInfo := FileChunkInfo{
		FileName:   chunkInfo.FileName,
		TotalSize:  chunkInfo.TotalSize,
		ChunkIndex: chunkInfo.ChunkIndex,
		ChunkSize:  chunkInfo.ChunkSize,
		TotalChunk: chunkInfo.TotalChunk,
		MD5:        chunkInfo.MD5,
	}
	return SaveFileChunk(a.storagePath, internalChunkInfo, file)
}

// DownloadFile downloads a file with resumable transfer support.
func (a *StorageAdapter) DownloadFile(ctx context.Context, c *gin.Context, filename, rangeHeader string) error {
	return DownloadFileWithTimeout(ctx, c, a.storagePath, filename, rangeHeader)
}

// DownloadFileChunk downloads a file chunk.
func (a *StorageAdapter) DownloadFileChunk(ctx context.Context, c *gin.Context, filename string, chunkIndex, chunkSize int64) error {
	return DownloadFileChunk(c, a.storagePath, filename, chunkIndex, chunkSize)
}

// ListFiles lists all files and directories with recursive traversal support.
func (a *StorageAdapter) ListFiles(ctx context.Context) ([]interfaces.FileMetadata, error) {
	files, err := ListFiles(a.storagePath)
	if err != nil {
		return nil, err
	}
	// Convert to interface type
	result := make([]interfaces.FileMetadata, len(files))
	for i, f := range files {
		result[i] = interfaces.FileMetadata{
			Name:     f.Name,
			Path:     f.Path,
			Size:     f.Size,
			ModTime:  f.ModTime,
			MD5:      f.MD5,
			IsDir:    f.IsDir,
			Children: convertChildren(f.Children),
		}
	}
	return result, nil
}

// convertChildren converts internal file metadata list to interface type.
func convertChildren(children []FileMetadata) []interfaces.FileMetadata {
	if len(children) == 0 {
		return nil
	}
	result := make([]interfaces.FileMetadata, len(children))
	for i, c := range children {
		result[i] = interfaces.FileMetadata{
			Name:     c.Name,
			Path:     c.Path,
			Size:     c.Size,
			ModTime:  c.ModTime,
			MD5:      c.MD5,
			IsDir:    c.IsDir,
			Children: convertChildren(c.Children),
		}
	}
	return result
}

// CheckFileExists checks if a file exists.
func (a *StorageAdapter) CheckFileExists(ctx context.Context, filename string) error {
	return CheckFileExists(a.storagePath, filename)
}

// GetFilePath returns the full path of a file.
func (a *StorageAdapter) GetFilePath(filename string) string {
	return GetFilePath(a.storagePath, filename)
}

// MD5CalculatorAdapter implements the MD5Calculator interface, providing MD5 calculation functionality.
// It delegates interface calls to the underlying MD5 calculation implementation.
type MD5CalculatorAdapter struct {
	storagePath string
	md5Cache    interfaces.MD5Cache
}

// NewMD5CalculatorAdapter creates and returns a new MD5 calculator adapter instance.
// storagePath is the file storage path, md5Cache is used for MD5 value caching.
func NewMD5CalculatorAdapter(storagePath string, md5Cache interfaces.MD5Cache) *MD5CalculatorAdapter {
	return &MD5CalculatorAdapter{
		storagePath: storagePath,
		md5Cache:    md5Cache,
	}
}

// GetMD5 gets the MD5 value of a file, prioritizing cache reads.
func (a *MD5CalculatorAdapter) GetMD5(ctx context.Context, filePath string) (string, error) {
	// filePath may be a full path or relative path
	if !strings.HasPrefix(filePath, a.storagePath) {
		filePath = GetFilePath(a.storagePath, filePath)
	}
	return GetFileMD5(a.storagePath, filepath.Base(filePath))
}

// GetMD5Progress gets the MD5 calculation progress information.
func (a *MD5CalculatorAdapter) GetMD5Progress(filePath string) (float64, bool, string) {
	return GetMD5Progress(filePath)
}

// CalculateMD5 calculates the MD5 value of a file.
// progressCallback is used to report calculation progress, can be nil.
func (a *MD5CalculatorAdapter) CalculateMD5(ctx context.Context, filePath string, progressCallback func(float64)) (string, error) {
	return calculateFileMD5WithProgress(filePath, progressCallback)
}
