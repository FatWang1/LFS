package services

import (
	"runtime"
	"sync"
	"time"

	"lfs/pkg/optimization"
)

// MetricsService implements metrics service business logic.
// It collects and provides system performance metrics including memory, runtime, and custom metrics.
type MetricsService struct {
	startTime time.Time
	metrics   map[string]interface{}
	mutex     sync.RWMutex
}

// NewMetricsService creates and returns a new metrics service instance.
func NewMetricsService() *MetricsService {
	return &MetricsService{
		startTime: time.Now(),
		metrics:   make(map[string]interface{}),
	}
}

// GetMetrics returns a map of all performance metrics.
// Includes memory statistics, runtime information, and custom metrics.
func (s *MetricsService) GetMetrics() map[string]interface{} {
	memStats := optimization.GetMemoryStats()

	metrics := map[string]interface{}{
		"memory": map[string]interface{}{
			"alloc":       memStats.Alloc,
			"total_alloc": memStats.TotalAlloc,
			"sys":         memStats.Sys,
			"num_gc":      memStats.NumGC,
		},
		"runtime": map[string]interface{}{
			"goroutines": runtime.NumGoroutine(),
			"cpu_cores":  runtime.NumCPU(),
			"max_procs":  runtime.GOMAXPROCS(0),
		},
		"uptime": time.Since(s.startTime).String(),
	}

	s.mutex.RLock()
	for k, v := range s.metrics {
		metrics[k] = v
	}
	s.mutex.RUnlock()

	return metrics
}

// RecordMetric records a custom performance metric.
// key is the metric name, value is the metric value.
func (s *MetricsService) RecordMetric(key string, value interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.metrics[key] = value
}
