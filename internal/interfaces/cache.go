package interfaces

import (
	"time"
)

// Cache 定义通用缓存操作的接口。
// 支持多种缓存实现（内存、Redis等），提供统一的缓存抽象。
type Cache interface {
	// Get 从缓存中获取值。
	// 返回值和是否存在。
	Get(key string) (interface{}, bool)

	// Set 设置缓存值，并指定过期时间。
	Set(key string, value interface{}, ttl time.Duration) error

	// Delete 删除指定的缓存项。
	Delete(key string) error

	// Clear 清空所有缓存项。
	Clear() error

	// Exists 检查指定的key是否存在。
	Exists(key string) bool
}

// MD5Cache 定义MD5值缓存的专用接口。
// 提供MD5值的缓存、计算状态跟踪和进度查询功能。
type MD5Cache interface {
	// GetMD5 从缓存中获取文件的MD5值。
	// 返回MD5值和是否存在。
	GetMD5(filePath, fileName string, size int64) (md5 string, exists bool)

	// SetMD5 将MD5值设置到缓存中。
	SetMD5(filePath, fileName, md5 string, size int64) error

	// SetCalculating 标记文件正在计算MD5。
	SetCalculating(filePath, fileName string, size int64) error

	// UpdateProgress 更新MD5计算的进度。
	// progress 为进度百分比（0-100）。
	UpdateProgress(filePath string, progress float64) error

	// SetError 记录MD5计算过程中的错误。
	SetError(filePath string, err error) error

	// GetProgress 获取MD5计算的进度信息。
	// 返回进度百分比（0-100）、是否完成、错误信息（如果有）。
	GetProgress(filePath string) (progress float64, completed bool, errMsg string)
}
