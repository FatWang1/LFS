package static

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"lfs/internal/interfaces"
)

// Service implements interfaces.StaticFileService for serving static files.
// It supports file caching, gzip compression, and ETag validation.
type Service struct {
	fs           embed.FS
	subPath      string
	cache        map[string]*cachedFile
	mutex        sync.RWMutex
	compressor   interfaces.Compressor
	mimeTypes    map[string]string
	compressible map[string]bool
}

// cachedFile represents cached file data.
type cachedFile struct {
	data        []byte
	contentType string
	etag        string
	gzipData    []byte
}

// NewService creates and returns a new static file service instance.
// fs is the embedded file system, subPath is the sub-path, compressor is used for compression.
func NewService(fs embed.FS, subPath string, compressor interfaces.Compressor) interfaces.StaticFileService {
	service := &Service{
		fs:           fs,
		subPath:      subPath,
		cache:        make(map[string]*cachedFile),
		compressor:   compressor,
		mimeTypes:    make(map[string]string),
		compressible: make(map[string]bool),
	}

	// Initialize MIME types
	service.initMimeTypes()
	service.initCompressibleTypes()

	// Preload files
	service.preloadFiles()

	return service
}

// initMimeTypes initializes MIME type mappings.
func (s *Service) initMimeTypes() {
	s.mimeTypes = map[string]string{
		".css":   "text/css; charset=utf-8",
		".js":    "application/javascript; charset=utf-8",
		".html":  "text/html; charset=utf-8",
		".png":   "image/png",
		".jpg":   "image/jpeg",
		".jpeg":  "image/jpeg",
		".gif":   "image/gif",
		".svg":   "image/svg+xml",
		".ico":   "image/x-icon",
		".woff":  "font/woff",
		".woff2": "font/woff2",
		".ttf":   "font/ttf",
		".eot":   "application/vnd.ms-fontobject",
	}
}

// initCompressibleTypes initializes the list of compressible file types.
func (s *Service) initCompressibleTypes() {
	types := []string{".html", ".css", ".js", ".svg", ".txt", ".json", ".xml"}
	for _, t := range types {
		s.compressible[t] = true
	}
}

// preloadFiles preloads all static files into memory cache.
func (s *Service) preloadFiles() {
	fsys, err := fs.Sub(s.fs, s.subPath)
	if err != nil {
		return
	}

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		s.cacheFile(fsys, entry.Name())
	}
}

// cacheFile loads a single file into memory cache.
func (s *Service) cacheFile(fsys fs.FS, fileName string) error {
	data, err := fs.ReadFile(fsys, fileName)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	contentType := s.getMimeType(ext)
	etag := fmt.Sprintf(`"%x"`, len(data))

	var gzipData []byte
	if s.compressible[ext] && s.compressor != nil {
		gzipData, err = s.compressor.Compress(data)
		if err != nil {
			gzipData = nil
		}
	}

	s.mutex.Lock()
	s.cache[fileName] = &cachedFile{
		data:        data,
		contentType: contentType,
		etag:        etag,
		gzipData:    gzipData,
	}
	s.mutex.Unlock()

	return nil
}

// getMimeType returns the corresponding MIME type based on file extension.
func (s *Service) getMimeType(ext string) string {
	if mimeType, ok := s.mimeTypes[ext]; ok {
		return mimeType
	}
	return "application/octet-stream"
}

// GetFile gets the content of a static file.
func (s *Service) GetFile(path string) ([]byte, string, error) {
	s.mutex.RLock()
	file, exists := s.cache[path]
	s.mutex.RUnlock()

	if !exists {
		return nil, "", fmt.Errorf("file not found: %s", path)
	}

	return file.data, file.contentType, nil
}

// GetFileGzip gets the compressed static file content.
func (s *Service) GetFileGzip(path string) ([]byte, string, error) {
	s.mutex.RLock()
	file, exists := s.cache[path]
	s.mutex.RUnlock()

	if !exists {
		return nil, "", fmt.Errorf("file not found: %s", path)
	}

	if len(file.gzipData) > 0 {
		return file.gzipData, file.contentType, nil
	}

	return file.data, file.contentType, nil
}

// GetETag returns the file's ETag value for cache validation.
func (s *Service) GetETag(path string) string {
	s.mutex.RLock()
	file, exists := s.cache[path]
	s.mutex.RUnlock()

	if !exists {
		return ""
	}

	return file.etag
}

// FileExists checks if the specified file exists.
func (s *Service) FileExists(path string) bool {
	s.mutex.RLock()
	_, exists := s.cache[path]
	s.mutex.RUnlock()
	return exists
}

// ListFiles lists all files.
func (s *Service) ListFiles() ([]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	files := make([]string, 0, len(s.cache))
	for name := range s.cache {
		files = append(files, name)
	}
	return files, nil
}
