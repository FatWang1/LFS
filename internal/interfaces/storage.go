package interfaces

import (
	"context"
	"io"
	"mime/multipart"
	"time"

	"github.com/gin-gonic/gin"
)

// FileChunkInfo 表示文件分片的元数据信息。
type FileChunkInfo struct {
	FileName   string `json:"file_name"`   // 文件名
	TotalSize  int64  `json:"total_size"`  // 文件总大小（字节）
	ChunkIndex int    `json:"chunk_index"` // 分片索引（从0开始）
	ChunkSize  int64  `json:"chunk_size"`  // 分片大小（字节）
	TotalChunk int    `json:"total_chunk"` // 总分片数
	MD5        string `json:"md5"`         // 分片的MD5值
}

// FileMetadata 表示文件或目录的元数据信息。
type FileMetadata struct {
	Name     string         `json:"name"`              // 文件或目录名
	Path     string         `json:"path"`              // 完整路径
	Size     int64          `json:"size"`               // 文件大小（字节），目录为0
	ModTime  time.Time      `json:"mod_time"`          // 修改时间
	MD5      string         `json:"md5,omitempty"`     // MD5值（仅文件）
	IsDir    bool           `json:"is_dir"`             // 是否为目录
	Children []FileMetadata `json:"children,omitempty"` // 子项列表（仅目录）
}

// Storage 定义文件存储的核心操作接口。
// 支持多种存储实现（本地文件系统、云存储等），提供统一的存储抽象。
type Storage interface {
	// SaveFile 保存文件，支持断点续传。
	// rangeHeader 用于指定保存范围，空字符串表示完整保存。
	SaveFile(ctx context.Context, file *multipart.FileHeader, rangeHeader string) error

	// SaveFileChunk 保存文件分片。
	// chunkInfo 包含分片的元数据信息。
	SaveFileChunk(ctx context.Context, chunkInfo FileChunkInfo, file *multipart.FileHeader) error

	// DownloadFile 下载文件，支持断点续传。
	// rangeHeader 用于指定下载范围，空字符串表示完整下载。
	DownloadFile(ctx context.Context, c *gin.Context, filename, rangeHeader string) error

	// DownloadFileChunk 下载文件分片。
	// chunkIndex 从0开始的分片索引，chunkSize 为分片大小（字节）。
	DownloadFileChunk(ctx context.Context, c *gin.Context, filename string, chunkIndex, chunkSize int64) error

	// ListFiles 列出所有文件和文件夹，支持递归遍历。
	ListFiles(ctx context.Context) ([]FileMetadata, error)

	// CheckFileExists 检查文件是否存在。
	// 文件不存在时返回错误。
	CheckFileExists(ctx context.Context, filename string) error

	// GetFilePath 返回文件的完整路径。
	GetFilePath(filename string) string
}

// MD5Calculator 定义MD5计算的接口。
// 支持异步计算、进度追踪和缓存机制。
type MD5Calculator interface {
	// GetMD5 获取文件的MD5值，优先从缓存读取。
	// 如果缓存不存在，会触发异步计算。
	GetMD5(ctx context.Context, filePath string) (string, error)

	// GetMD5Progress 获取MD5计算的进度信息。
	// 返回进度百分比（0-100）、是否完成、错误信息（如果有）。
	GetMD5Progress(filePath string) (progress float64, completed bool, errMsg string)

	// CalculateMD5 计算文件的MD5值。
	// progressCallback 用于报告计算进度，可以为nil。
	CalculateMD5(ctx context.Context, filePath string, progressCallback func(float64)) (string, error)
}

// FileReader 定义文件读取操作的接口。
type FileReader interface {
	// ReadFile 读取文件的全部内容。
	ReadFile(ctx context.Context, filePath string) (io.ReadCloser, error)

	// ReadFileRange 读取文件的指定范围。
	// start 和 end 分别表示起始和结束位置（字节偏移）。
	ReadFileRange(ctx context.Context, filePath string, start, end int64) (io.ReadCloser, error)
}

// FileWriter 定义文件写入操作的接口。
type FileWriter interface {
	// WriteFile 写入文件的全部内容。
	WriteFile(ctx context.Context, filePath string, data io.Reader) error

	// WriteFileRange 写入文件的指定范围，支持断点续传。
	// start 表示写入的起始位置（字节偏移）。
	WriteFileRange(ctx context.Context, filePath string, start int64, data io.Reader) error
}
