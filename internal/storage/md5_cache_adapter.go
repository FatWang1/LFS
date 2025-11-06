package storage

// MD5CacheAdapter implements the MD5Cache interface, providing MD5 value caching functionality.
// It delegates interface calls to the underlying MD5Cache implementation.
type MD5CacheAdapter struct {
	cache *MD5Cache
}

// NewMD5CacheAdapter creates and returns a new MD5 cache adapter instance.
func NewMD5CacheAdapter() *MD5CacheAdapter {
	return &MD5CacheAdapter{
		cache: md5Cache, // Use global instance
	}
}

// GetMD5 gets the MD5 value of a file from cache.
func (a *MD5CacheAdapter) GetMD5(filePath, fileName string, size int64) (string, bool) {
	return a.cache.GetMD5FromCache(filePath, fileName, size)
}

// SetMD5 sets the MD5 value to cache.
func (a *MD5CacheAdapter) SetMD5(filePath, fileName, md5 string, size int64) error {
	a.cache.SetMD5ToCache(filePath, fileName, md5, size)
	return nil
}

// SetCalculating marks that a file is being calculated for MD5.
func (a *MD5CacheAdapter) SetCalculating(filePath, fileName string, size int64) error {
	a.cache.SetCalculating(filePath, fileName, size)
	return nil
}

// UpdateProgress updates the MD5 calculation progress.
func (a *MD5CacheAdapter) UpdateProgress(filePath string, progress float64) error {
	a.cache.UpdateProgress(filePath, progress)
	return nil
}

// SetError records errors during MD5 calculation.
func (a *MD5CacheAdapter) SetError(filePath string, err error) error {
	a.cache.SetError(filePath, err)
	return nil
}

// GetProgress gets the MD5 calculation progress information.
func (a *MD5CacheAdapter) GetProgress(filePath string) (float64, bool, string) {
	return a.cache.GetProgress(filePath)
}
