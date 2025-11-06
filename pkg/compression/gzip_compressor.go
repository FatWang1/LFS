package compression

import (
	"bytes"
	"compress/gzip"
	"io"
	"strings"

	"lfs/internal/interfaces"
)

// GzipCompressor implements interfaces.Compressor using gzip compression.
type GzipCompressor struct{}

// NewGzipCompressor returns a new gzip compressor instance.
func NewGzipCompressor() interfaces.Compressor {
	return &GzipCompressor{}
}

// Compress compresses byte data.
func (g *GzipCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// CompressStream creates a streaming compression writer.
func (g *GzipCompressor) CompressStream(w io.Writer) (io.WriteCloser, error) {
	return gzip.NewWriter(w), nil
}

// Decompress decompresses byte data.
func (g *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// DecompressStream creates a streaming decompression reader.
func (g *GzipCompressor) DecompressStream(r io.Reader) (io.ReadCloser, error) {
	return gzip.NewReader(r)
}

// ContentEncoding returns the HTTP Content-Encoding header value.
func (g *GzipCompressor) ContentEncoding() string {
	return "gzip"
}

// Supports checks if the specified encoding format is supported.
func (g *GzipCompressor) Supports(encoding string) bool {
	return strings.Contains(encoding, "gzip")
}
