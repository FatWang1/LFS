package interfaces

import (
	"io"
)

// Compressor 定义数据压缩和解压的接口。
// 支持多种压缩算法（gzip、brotli等），提供统一的压缩抽象。
type Compressor interface {
	// Compress 压缩字节数据。
	Compress(data []byte) ([]byte, error)

	// CompressStream 创建流式压缩写入器。
	CompressStream(w io.Writer) (io.WriteCloser, error)

	// Decompress 解压字节数据。
	Decompress(data []byte) ([]byte, error)

	// DecompressStream 创建流式解压读取器。
	DecompressStream(r io.Reader) (io.ReadCloser, error)

	// ContentEncoding 返回HTTP Content-Encoding头的值。
	ContentEncoding() string

	// Supports 检查是否支持指定的编码格式。
	Supports(encoding string) bool
}
