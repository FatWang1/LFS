package storage

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 常量定义
const (
	// 缓冲区大小
	DefaultBufferSize = 4 * 1024 * 1024 // 4MB
	ChunkBufferSize   = 2 * 1024 * 1024 // 2MB

	// 分片大小
	DefaultChunkSize = 5 * 1024 * 1024 // 5MB

	// MD5计算配置
	MD5ChunkSize     = 64 * 1024 * 1024 // 64MB 分块大小，适合大文件
	MD5MaxConcurrent = 3                // 最大并发计算数

	// 错误消息
	ErrFileNotFound  = "file not found"
	ErrInvalidRange  = "invalid range header"
	ErrChunkNotFound = "chunk not found"
	ErrMD5Mismatch   = "MD5 checksum mismatch"
	ErrMD5Timeout    = "MD5 calculation timeout"
	ErrMD5InProgress = "MD5 calculation in progress"
)

// FileMetadata 文件元数据结构
type FileMetadata struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	MD5     string    `json:"md5,omitempty"`
}

// FileChunkInfo 文件分片信息
type FileChunkInfo struct {
	FileName   string `json:"file_name"`
	TotalSize  int64  `json:"total_size"`
	ChunkIndex int    `json:"chunk_index"`
	ChunkSize  int64  `json:"chunk_size"`
	TotalChunk int    `json:"total_chunk"`
	MD5        string `json:"md5"`
}

// MD5CacheEntry MD5缓存条目
type MD5CacheEntry struct {
	MD5         string    `json:"md5"`
	ModTime     time.Time `json:"mod_time"`
	Size        int64     `json:"size"`
	Calculated  bool      `json:"calculated"`
	Calculating bool      `json:"calculating"`     // 是否正在计算中
	Progress    float64   `json:"progress"`        // 计算进度 0.0-1.0
	Error       string    `json:"error,omitempty"` // 计算错误信息
}

// MD5Cache MD5缓存管理器
type MD5Cache struct {
	cache     map[string]*MD5CacheEntry
	mutex     sync.RWMutex
	semaphore chan struct{} // 控制并发计算数量
}

// 全局MD5缓存实例
var md5Cache = &MD5Cache{
	cache:     make(map[string]*MD5CacheEntry),
	semaphore: make(chan struct{}, MD5MaxConcurrent),
}

// GetMD5FromCache 从缓存获取MD5
func (mc *MD5Cache) GetMD5FromCache(filePath string, modTime time.Time, size int64) (string, bool) {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	entry, exists := mc.cache[filePath]
	if !exists {
		return "", false
	}

	// 检查文件是否被修改
	if entry.ModTime != modTime || entry.Size != size {
		return "", false
	}

	return entry.MD5, entry.Calculated
}

// SetMD5ToCache 设置MD5到缓存
func (mc *MD5Cache) SetMD5ToCache(filePath, md5 string, modTime time.Time, size int64) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.cache[filePath] = &MD5CacheEntry{
		MD5:         md5,
		ModTime:     modTime,
		Size:        size,
		Calculated:  true,
		Calculating: false,
	}
}

// SetCalculating 设置正在计算状态
func (mc *MD5Cache) SetCalculating(filePath string, modTime time.Time, size int64) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.cache[filePath] = &MD5CacheEntry{
		ModTime:     modTime,
		Size:        size,
		Calculated:  false,
		Calculating: true,
		Progress:    0.0,
	}
}

// UpdateProgress 更新计算进度
func (mc *MD5Cache) UpdateProgress(filePath string, progress float64) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if entry, exists := mc.cache[filePath]; exists {
		entry.Progress = progress
	}
}

// SetError 设置计算错误
func (mc *MD5Cache) SetError(filePath string, err error) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if entry, exists := mc.cache[filePath]; exists {
		entry.Calculating = false
		entry.Error = err.Error()
	}
}

// GetProgress 获取计算进度
func (mc *MD5Cache) GetProgress(filePath string) (float64, bool, string) {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	entry, exists := mc.cache[filePath]
	if !exists {
		return 0, false, ""
	}

	return entry.Progress, entry.Calculating, entry.Error
}

// calculateFileMD5Chunked 分块计算大文件MD5（支持任意大小文件）
func calculateFileMD5Chunked(filePath string, progressCallback func(float64)) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 获取文件大小
	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}
	fileSize := fileInfo.Size()

	hash := md5.New()
	buf := make([]byte, MD5ChunkSize)
	var totalRead int64

	for {
		n, err := file.Read(buf)
		if n > 0 {
			hash.Write(buf[:n])
			totalRead += int64(n)

			// 更新进度
			if progressCallback != nil {
				progress := float64(totalRead) / float64(fileSize)
				progressCallback(progress)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// calculateFileMD5WithProgress 带进度回调的MD5计算
func calculateFileMD5WithProgress(filePath string, progressCallback func(float64)) (string, error) {
	// 对于大文件使用分块计算
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	// 如果文件大于100MB，使用分块计算
	if fileInfo.Size() > 100*1024*1024 {
		return calculateFileMD5Chunked(filePath, progressCallback)
	}

	// 小文件使用原有方法
	return calculateFileMD5(filePath)
}

// SaveFile 保存文件到指定路径，支持断点重传
func SaveFile(storagePath string, file *multipart.FileHeader, rangeHeader string) error {
	dest := filepath.Join(storagePath, file.Filename)
	err := os.MkdirAll(storagePath, os.ModePerm)
	if err != nil {
		return err
	}

	// 处理 Range 头部信息
	var start int64
	if rangeHeader != "" {
		// 解析 Range 头部信息
		parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
		if len(parts) > 0 {
			start, err = strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				return err
			}
		}
	}

	// 打开上传的文件
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	// 打开目标文件，以追加模式写入
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	// 移动文件指针到指定位置
	if start > 0 {
		_, err = out.Seek(start, io.SeekStart)
		if err != nil {
			return err
		}
	}

	// 将上传的文件内容复制到目标文件，使用更大的缓冲区提高性能
	// 使用4MB缓冲区进行复制，提高大文件传输性能
	buf := make([]byte, 4*1024*1024)
	_, err = io.CopyBuffer(out, src, buf)
	return err
}

// SaveFileWithTimeout 保存文件到指定路径，支持超时控制
func SaveFileWithTimeout(ctx context.Context, storagePath string, file *multipart.FileHeader, rangeHeader string) error {
	// 创建一个带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- SaveFile(storagePath, file, rangeHeader)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SaveFileChunk 保存文件分片
func SaveFileChunk(storagePath string, chunkInfo FileChunkInfo, file *multipart.FileHeader) error {
	chunkDir := filepath.Join(storagePath, "chunks", chunkInfo.FileName)
	err := os.MkdirAll(chunkDir, os.ModePerm)
	if err != nil {
		return err
	}

	chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%s_%d", chunkInfo.FileName, chunkInfo.ChunkIndex))

	// 打开上传的分片文件
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	// 创建分片文件
	chunkFile, err := os.Create(chunkPath)
	if err != nil {
		return err
	}
	defer chunkFile.Close()

	// 复制分片内容，使用优化的缓冲区
	buf := make([]byte, ChunkBufferSize)
	_, err = io.CopyBuffer(chunkFile, src, buf)
	if err != nil {
		return err
	}

	// 检查是否所有分片都已上传完成
	if chunkInfo.ChunkIndex == chunkInfo.TotalChunk-1 {
		// 合并所有分片
		err = mergeFileChunks(chunkDir, filepath.Join(storagePath, chunkInfo.FileName), chunkInfo.TotalChunk)
		if err != nil {
			return err
		}

		// 验证文件完整性
		md5sum, err := calculateFileMD5(filepath.Join(storagePath, chunkInfo.FileName))
		if err != nil {
			return err
		}

		if md5sum != chunkInfo.MD5 {
			// MD5校验失败，删除文件
			os.Remove(filepath.Join(storagePath, chunkInfo.FileName))
			return fmt.Errorf("file integrity check failed: expected %s, got %s", chunkInfo.MD5, md5sum)
		}
	}

	return nil
}

// mergeFileChunks 合并文件分片
func mergeFileChunks(chunkDir, targetFile string, totalChunk int) error {
	target, err := os.Create(targetFile)
	if err != nil {
		return err
	}
	defer target.Close()

	// 使用1MB缓冲区提高合并性能
	buf := make([]byte, 1024*1024)

	for i := 0; i < totalChunk; i++ {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("*_%d", i))
		matches, err := filepath.Glob(chunkPath)
		if err != nil {
			return err
		}

		if len(matches) == 0 {
			return fmt.Errorf("%s: chunk %d", ErrChunkNotFound, i)
		}

		chunkFile, err := os.Open(matches[0])
		if err != nil {
			return err
		}

		_, err = io.CopyBuffer(target, chunkFile, buf)
		chunkFile.Close()
		if err != nil {
			return err
		}
	}

	// 删除分片目录
	os.RemoveAll(chunkDir)
	return nil
}

// DownloadFile 从指定路径下载文件，支持断点重传
func DownloadFile(c *gin.Context, storagePath, filename, rangeHeader string) error {
	file := filepath.Join(storagePath, filename)
	fileInfo, err := os.Stat(file)
	if os.IsNotExist(err) {
		return err
	}
	if err != nil {
		return err
	}

	// 处理Range头信息
	if rangeHeader != "" {
		start, end, err := parseRangeHeader(rangeHeader)
		if err != nil {
			return err
		}

		// 打开文件
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		// 获取文件大小
		fileSize := fileInfo.Size()

		// 设置响应头
		c.Writer.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
		c.Writer.Header().Set("Accept-Ranges", "bytes")
		c.Writer.Header().Set("Content-Length", strconv.Itoa(end-start+1))

		// 检查客户端是否已经断开连接
		if c.Request.Context().Err() != nil {
			return c.Request.Context().Err()
		}

		c.Writer.WriteHeader(http.StatusPartialContent)

		// 移动文件指针到指定位置
		_, err = f.Seek(int64(start), io.SeekStart)
		if err != nil {
			return err
		}

		// 发送文件内容并检查连接状态
		return copyWithCancel(c.Request.Context(), c.Writer, f, int64(end-start+1))
	}

	// 对于完整文件下载，使用流式传输避免内存问题
	// 检查客户端是否已经断开连接
	if c.Request.Context().Err() != nil {
		return c.Request.Context().Err()
	}

	// 打开文件
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	// 设置响应头
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Writer.Header().Set("Content-Type", "application/octet-stream")
	c.Writer.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))

	// 检查客户端是否已经断开连接
	if c.Request.Context().Err() != nil {
		return c.Request.Context().Err()
	}

	// Copy文件内容到响应并检查连接状态
	return copyWithCancel(c.Request.Context(), c.Writer, f, fileInfo.Size())
}

// copyWithCancel 带取消功能的复制函数，支持大文件长时间传输
func copyWithCancel(ctx context.Context, dst io.Writer, src io.Reader, size int64) error {
	// 使用更大的缓冲区大小以提高传输性能
	// 使用优化的缓冲区大小
	buf := make([]byte, DefaultBufferSize)

	// 已传输的字节数
	var written int64

	for {
		// 检查是否需要取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 读取数据
		nr, er := src.Read(buf)
		if nr > 0 {
			// 写入数据
			nw, ew := dst.Write(buf[0:nr])

			// 更新已写入字节数
			written += int64(nw)

			// 检查写入错误
			if ew != nil {
				return ew
			}

			// 检查写入字节数是否匹配
			if nr != nw {
				return io.ErrShortWrite
			}
		}

		// 检查读取错误
		if er != nil {
			if er != io.EOF {
				return er
			}
			break
		}
	}

	return nil
}

// DownloadFileWithTimeout 从指定路径下载文件，支持超时控制
func DownloadFileWithTimeout(ctx context.Context, c *gin.Context, storagePath, filename, rangeHeader string) error {
	// 对于大文件下载，我们不设置固定的超时时间，而是依赖HTTP连接本身的超时机制
	// 这样可以支持长时间的大文件传输
	return DownloadFile(c, storagePath, filename, rangeHeader)
}

// DownloadFileChunk 下载文件分片，支持多线程分片下载
func DownloadFileChunk(c *gin.Context, storagePath, filename string, chunkIndex, chunkSize int64) error {
	file := filepath.Join(storagePath, filename)
	fileInfo, err := os.Stat(file)
	if err != nil {
		return err
	}

	start := chunkIndex * chunkSize
	end := start + chunkSize - 1
	fileSize := fileInfo.Size()

	if end >= fileSize {
		end = fileSize - 1
	}

	// 打开文件
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	// 设置响应头
	c.Writer.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Writer.Header().Set("Accept-Ranges", "bytes")
	c.Writer.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))

	// 检查客户端是否已经断开连接
	if c.Request.Context().Err() != nil {
		return c.Request.Context().Err()
	}

	c.Writer.WriteHeader(http.StatusPartialContent)

	// 移动文件指针到指定位置
	_, err = f.Seek(start, io.SeekStart)
	if err != nil {
		return err
	}

	// 发送文件内容并检查连接状态
	return copyWithCancel(c.Request.Context(), c.Writer, f, end-start+1)
}

// parseRangeHeader 解析Range头信息
func parseRangeHeader(rangeHeader string) (int, int, error) {
	parts := strings.Split(rangeHeader, "=")[1]
	rangeParts := strings.Split(parts, "-")
	start, err := strconv.Atoi(rangeParts[0])
	if err != nil {
		return 0, 0, err
	}

	// 如果没有结束位置，则默认到最后
	if rangeParts[1] == "" {
		// 我们需要获取文件大小来确定结束位置，但在这里我们简单处理
		return 0, 0, fmt.Errorf("range end position required")
	}

	end, err := strconv.Atoi(rangeParts[1])
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

// ListFiles 列出存储路径下的所有文件（优化版 - 异步MD5计算）
func ListFiles(storagePath string) ([]FileMetadata, error) {
	var files []FileMetadata

	err := os.MkdirAll(storagePath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(storagePath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		filePath := filepath.Join(storagePath, info.Name())

		// 先尝试从缓存获取MD5
		md5sum, calculated := md5Cache.GetMD5FromCache(filePath, info.ModTime(), info.Size())

		// 如果缓存中没有或文件已修改，异步计算MD5
		if !calculated {
			// 检查是否已经在计算中
			_, calculating, _ := md5Cache.GetProgress(filePath)
			if !calculating {
				// 设置正在计算状态
				md5Cache.SetCalculating(filePath, info.ModTime(), info.Size())

				// 异步计算MD5（不阻塞列表响应，支持任意大小文件）
				go func(filePath string, modTime time.Time, size int64) {
					// 获取信号量，控制并发数
					md5Cache.semaphore <- struct{}{}
					defer func() { <-md5Cache.semaphore }()

					// 使用带进度回调的计算方法
					md5, err := calculateFileMD5WithProgress(filePath, func(progress float64) {
						md5Cache.UpdateProgress(filePath, progress)
					})

					if err != nil {
						// 计算失败，设置错误状态
						md5Cache.SetError(filePath, err)
						return
					}

					// 计算成功，更新缓存
					md5Cache.SetMD5ToCache(filePath, md5, modTime, size)
				}(filePath, info.ModTime(), info.Size())
			}

			// 列表响应中不包含MD5，但会异步计算
			md5sum = ""
		}

		file := FileMetadata{
			Name:    info.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			MD5:     md5sum,
		}
		files = append(files, file)
	}

	return files, nil
}

// CheckFileExists 检查文件是否存在
func CheckFileExists(storagePath string, filename string) error {
	file := filepath.Join(storagePath, filename)
	_, err := os.Stat(file)
	return err
}

// calculateFileMD5 计算文件的MD5值
func calculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	// 使用1MB缓冲区提高MD5计算性能
	buf := make([]byte, 1024*1024)
	if _, err := io.CopyBuffer(hash, file, buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GetFileMD5 获取文件的MD5值（带缓存，支持大文件）
func GetFileMD5(storagePath, filename string) (string, error) {
	filePath := filepath.Join(storagePath, filename)

	// 获取文件信息
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	// 先尝试从缓存获取
	md5sum, calculated := md5Cache.GetMD5FromCache(filePath, info.ModTime(), info.Size())
	if calculated {
		return md5sum, nil
	}

	// 检查是否正在计算中
	progress, calculating, errorMsg := md5Cache.GetProgress(filePath)
	if calculating {
		return "", fmt.Errorf("%s: progress %.1f%%", ErrMD5InProgress, progress*100)
	}

	if errorMsg != "" {
		return "", fmt.Errorf("MD5 calculation failed: %s", errorMsg)
	}

	// 缓存中没有且未在计算，同步计算MD5（支持大文件）
	md5sum, err = calculateFileMD5WithProgress(filePath, nil)
	if err != nil {
		return "", err
	}

	// 更新缓存
	md5Cache.SetMD5ToCache(filePath, md5sum, info.ModTime(), info.Size())
	return md5sum, nil
}

// GetFileMetadataWithMD5 获取包含MD5的文件元数据
func GetFileMetadataWithMD5(storagePath, filename string) (*FileMetadata, error) {
	filePath := filepath.Join(storagePath, filename)

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	// 获取MD5（带缓存）
	md5sum, err := GetFileMD5(storagePath, filename)
	if err != nil {
		// MD5计算失败，返回基本信息
		md5sum = ""
	}

	return &FileMetadata{
		Name:    info.Name(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		MD5:     md5sum,
	}, nil
}

// GetFilePath 获取文件完整路径
func GetFilePath(storagePath, filename string) string {
	return filepath.Join(storagePath, filename)
}

// GetMD5Progress 获取MD5计算进度
func GetMD5Progress(filePath string) (float64, bool, string) {
	return md5Cache.GetProgress(filePath)
}
