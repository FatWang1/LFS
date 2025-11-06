package optimization

import (
	"context"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// 连接池配置
const (
	MaxConcurrentUploads   = 100
	MaxConcurrentDownloads = 200
	BufferSize             = 4 * 1024 * 1024 // 4MB
)

// 性能优化器
type PerformanceOptimizer struct {
	uploadSemaphore   chan struct{}
	downloadSemaphore chan struct{}
	bufferPool        sync.Pool
}

// 全局性能优化器实例
var GlobalOptimizer = &PerformanceOptimizer{
	uploadSemaphore:   make(chan struct{}, MaxConcurrentUploads),
	downloadSemaphore: make(chan struct{}, MaxConcurrentDownloads),
	bufferPool: sync.Pool{
		New: func() interface{} {
			return make([]byte, BufferSize)
		},
	},
}

// AcquireUploadSlot 获取上传槽位
func (po *PerformanceOptimizer) AcquireUploadSlot() {
	po.uploadSemaphore <- struct{}{}
}

// ReleaseUploadSlot 释放上传槽位
func (po *PerformanceOptimizer) ReleaseUploadSlot() {
	<-po.uploadSemaphore
}

// AcquireDownloadSlot 获取下载槽位
func (po *PerformanceOptimizer) AcquireDownloadSlot() {
	po.downloadSemaphore <- struct{}{}
}

// ReleaseDownloadSlot 释放下载槽位
func (po *PerformanceOptimizer) ReleaseDownloadSlot() {
	<-po.downloadSemaphore
}

// GetBuffer 从池中获取缓冲区
func (po *PerformanceOptimizer) GetBuffer() []byte {
	return po.bufferPool.Get().([]byte)
}

// PutBuffer 将缓冲区放回池中
func (po *PerformanceOptimizer) PutBuffer(buf []byte) {
	po.bufferPool.Put(buf)
}

// OptimizedCopy 优化的复制函数，使用连接池和缓冲区池
func (po *PerformanceOptimizer) OptimizedCopy(ctx context.Context, dst io.Writer, src io.Reader, size int64) error {
	buf := po.GetBuffer()
	defer po.PutBuffer(buf)

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

			// 检查是否已传输完成
			if written >= size {
				break
			}
		}

		// 检查读取错误
		if er != nil {
			if er == io.EOF {
				break
			}
			return er
		}
	}

	return nil
}

// SetOptimalGOMAXPROCS 设置最优的GOMAXPROCS
func SetOptimalGOMAXPROCS() {
	// 获取CPU核心数
	numCPU := runtime.NumCPU()

	// 设置GOMAXPROCS为CPU核心数
	runtime.GOMAXPROCS(numCPU)
}

// 设置HTTP客户端优化参数
func GetOptimizedHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
			DisableKeepAlives:   false,
		},
		Timeout: 30 * time.Second,
	}
}

// 内存使用统计
type MemoryStats struct {
	Alloc      uint64
	TotalAlloc uint64
	Sys        uint64
	NumGC      uint32
}

// GetMemoryStats 获取内存使用统计
func GetMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		Alloc:      m.Alloc,
		TotalAlloc: m.TotalAlloc,
		Sys:        m.Sys,
		NumGC:      m.NumGC,
	}
}

// TaskFunc 定义任务函数类型
type TaskFunc func() error

// 性能监控器
type PerformanceMonitor struct {
	startTime time.Time
	metrics   map[string]interface{}
	mutex     sync.RWMutex
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		startTime: time.Now(),
		metrics:   make(map[string]interface{}),
	}
}

// RecordMetric 记录性能指标
func (pm *PerformanceMonitor) RecordMetric(key string, value interface{}) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.metrics[key] = value
}

// GetMetrics 获取性能指标
func (pm *PerformanceMonitor) GetMetrics() map[string]interface{} {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	metrics := make(map[string]interface{})
	for k, v := range pm.metrics {
		metrics[k] = v
	}

	// 添加运行时间
	metrics["uptime"] = time.Since(pm.startTime).String()

	return metrics
}

// ConcurrentProcessor 并发处理器
type ConcurrentProcessor struct {
	workerCount int
}

// NewConcurrentProcessor 创建新的并发处理器
func NewConcurrentProcessor() *ConcurrentProcessor {
	workerCount := runtime.NumCPU() * 2 // IO密集型任务使用更多协程
	return &ConcurrentProcessor{
		workerCount: workerCount,
	}
}

// Process 并发处理任务
func (cp *ConcurrentProcessor) Process(ctx context.Context, tasks []TaskFunc) []error {
	if len(tasks) == 0 {
		return nil
	}

	taskChan := make(chan TaskFunc, len(tasks))
	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)

	resultChan := make(chan error, len(tasks))
	var wg sync.WaitGroup

	workerCount := cp.workerCount
	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case task, ok := <-taskChan:
					if !ok {
						return
					}
					resultChan <- task()
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var errors []error
	for err := range resultChan {
		errors = append(errors, err)
	}

	return errors
}
