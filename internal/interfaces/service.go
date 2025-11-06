package interfaces

import (
	"context"
	"mime/multipart"

	"github.com/gin-gonic/gin"
)

// FileService 定义文件服务的业务逻辑接口。
// 封装文件上传、下载、列表和MD5计算等核心功能。
type FileService interface {
	// UploadFile 上传单个文件，支持断点续传。
	// rangeHeader 用于指定上传范围，空字符串表示完整上传。
	UploadFile(ctx context.Context, file *multipart.FileHeader, rangeHeader string) error

	// UploadFileChunk 上传文件分片。
	// chunkInfo 包含分片的元数据信息。
	UploadFileChunk(ctx context.Context, chunkInfo FileChunkInfo, file *multipart.FileHeader) error

	// BatchUpload 批量上传多个文件。
	// 返回成功数量、失败数量和错误信息列表。
	BatchUpload(ctx context.Context, files []*multipart.FileHeader) (successCount, errorCount int, errors []string)

	// DownloadFile 下载文件，支持断点续传。
	// rangeHeader 用于指定下载范围，空字符串表示完整下载。
	DownloadFile(ctx context.Context, c *gin.Context, filename, rangeHeader string) error

	// DownloadFileChunk 下载文件分片。
	// chunkIndex 从0开始的分片索引，chunkSize 为分片大小（字节）。
	DownloadFileChunk(ctx context.Context, c *gin.Context, filename string, chunkIndex, chunkSize int64) error

	// ListFiles 列出指定路径下的所有文件。
	// path 为空字符串时列出根目录。
	ListFiles(ctx context.Context, path string) ([]FileMetadata, error)

	// GetFileMD5 获取文件的MD5校验值。
	// 如果文件正在计算中，会等待计算完成。
	GetFileMD5(ctx context.Context, filename string) (string, error)

	// GetFileMD5Progress 获取MD5计算的进度信息。
	// 返回进度百分比（0-100）、是否完成、错误信息（如果有）。
	GetFileMD5Progress(filename string) (progress float64, completed bool, errMsg string)

	// CheckFileExists 检查文件是否存在。
	// 文件不存在时返回错误。
	CheckFileExists(ctx context.Context, filename string) error
}

// ChatService 定义聊天服务的接口。
// 提供WebSocket连接处理和消息广播功能。
type ChatService interface {
	// HandleWebSocket 处理WebSocket连接升级和客户端注册。
	HandleWebSocket(c *gin.Context) error

	// BroadcastMessage 向所有连接的客户端广播消息。
	BroadcastMessage(message interface{}) error

	// GetClientCount 返回当前连接的客户端数量。
	GetClientCount() int
}

// MetricsService 定义指标服务的接口。
// 用于收集和查询系统性能指标。
type MetricsService interface {
	// GetMetrics 返回所有性能指标的映射。
	GetMetrics() map[string]interface{}

	// RecordMetric 记录一个性能指标。
	// key 为指标名称，value 为指标值。
	RecordMetric(key string, value interface{})
}
