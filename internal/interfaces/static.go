package interfaces

import (
	"io/fs"
)

// StaticFileService 定义静态文件服务的接口。
// 支持嵌入式和文件系统两种方式提供静态文件。
type StaticFileService interface {
	// GetFile 获取静态文件的内容。
	// 返回文件内容、Content-Type和错误。
	GetFile(path string) ([]byte, string, error)

	// GetFileGzip 获取压缩后的静态文件内容。
	// 返回压缩后的内容、Content-Type和错误。
	GetFileGzip(path string) ([]byte, string, error)

	// GetETag 返回文件的ETag值，用于缓存验证。
	GetETag(path string) string

	// FileExists 检查指定的文件是否存在。
	FileExists(path string) bool

	// ListFiles 列出所有可用的静态文件路径。
	ListFiles() ([]string, error)
}

// FileSystem 定义文件系统操作的接口。
// 提供文件读取、目录读取和文件打开功能。
type FileSystem interface {
	// ReadFile 读取文件的全部内容。
	ReadFile(name string) ([]byte, error)

	// ReadDir 读取目录下的所有条目。
	ReadDir(name string) ([]fs.DirEntry, error)

	// Open 打开文件并返回文件句柄。
	Open(name string) (fs.File, error)
}
