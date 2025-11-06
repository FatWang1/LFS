package optimization

import (
	"runtime"
)

// SetOptimalGOMAXPROCS sets the optimal GOMAXPROCS.
func SetOptimalGOMAXPROCS() {
	// Get CPU core count
	numCPU := runtime.NumCPU()

	// Set GOMAXPROCS to CPU core count
	runtime.GOMAXPROCS(numCPU)
}

// MemoryStats represents memory usage statistics.
type MemoryStats struct {
	Alloc      uint64
	TotalAlloc uint64
	Sys        uint64
	NumGC      uint32
}

// GetMemoryStats gets memory usage statistics.
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
