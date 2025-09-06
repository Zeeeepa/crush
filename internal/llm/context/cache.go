package context

import (
	"sync"
	"time"
)

// ContextCache provides caching for enhanced context to improve performance
type ContextCache struct {
	cache map[string]*CacheEntry
	mu    sync.RWMutex
	ttl   time.Duration
}

// CacheEntry represents a cached context entry
type CacheEntry struct {
	Context   *EnhancedContext
	ExpiresAt time.Time
}

// NewContextCache creates a new context cache with default TTL
func NewContextCache() *ContextCache {
	return &ContextCache{
		cache: make(map[string]*CacheEntry),
		ttl:   5 * time.Minute, // Default TTL of 5 minutes
	}
}

// NewContextCacheWithTTL creates a new context cache with custom TTL
func NewContextCacheWithTTL(ttl time.Duration) *ContextCache {
	return &ContextCache{
		cache: make(map[string]*CacheEntry),
		ttl:   ttl,
	}
}

// Get retrieves a cached context entry if it exists and hasn't expired
func (cc *ContextCache) Get(key string) *EnhancedContext {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	entry, exists := cc.cache[key]
	if !exists {
		return nil
	}

	// Check if entry has expired
	if time.Now().After(entry.ExpiresAt) {
		// Entry expired, remove it
		delete(cc.cache, key)
		return nil
	}

	// Return a copy to avoid concurrent modification
	contextCopy := *entry.Context
	return &contextCopy
}

// Set stores a context entry in the cache with TTL
func (cc *ContextCache) Set(key string, context *EnhancedContext) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Create a copy to avoid external modifications
	contextCopy := *context
	
	cc.cache[key] = &CacheEntry{
		Context:   &contextCopy,
		ExpiresAt: time.Now().Add(cc.ttl),
	}
}

// Delete removes a specific entry from the cache
func (cc *ContextCache) Delete(key string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	delete(cc.cache, key)
}

// Clear removes all entries from the cache
func (cc *ContextCache) Clear() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.cache = make(map[string]*CacheEntry)
}

// Size returns the current number of entries in the cache
func (cc *ContextCache) Size() int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	return len(cc.cache)
}

// Cleanup removes expired entries from the cache
func (cc *ContextCache) Cleanup() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	now := time.Now()
	for key, entry := range cc.cache {
		if now.After(entry.ExpiresAt) {
			delete(cc.cache, key)
		}
	}
}

// StartCleanupRoutine starts a background goroutine that periodically cleans up expired entries
func (cc *ContextCache) StartCleanupRoutine(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			cc.Cleanup()
		}
	}()
}

// GetStats returns cache statistics
func (cc *ContextCache) GetStats() CacheStats {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	now := time.Now()
	expired := 0
	
	for _, entry := range cc.cache {
		if now.After(entry.ExpiresAt) {
			expired++
		}
	}

	return CacheStats{
		TotalEntries:   len(cc.cache),
		ExpiredEntries: expired,
		ActiveEntries:  len(cc.cache) - expired,
		TTL:           cc.ttl,
	}
}

// CacheStats provides statistics about the cache
type CacheStats struct {
	TotalEntries   int           `json:"total_entries"`
	ExpiredEntries int           `json:"expired_entries"`
	ActiveEntries  int           `json:"active_entries"`
	TTL           time.Duration `json:"ttl"`
}

// InvalidateByFilePath removes all cache entries for a specific file path
// This is useful when a file is modified and cached context becomes stale
func (cc *ContextCache) InvalidateByFilePath(filePath string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Find and remove all entries that match the file path
	for key, entry := range cc.cache {
		if entry.Context.FilePath == filePath {
			delete(cc.cache, key)
		}
	}
}

// InvalidateByPattern removes all cache entries where the key contains the pattern
func (cc *ContextCache) InvalidateByPattern(pattern string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Find and remove all entries that match the pattern
	for key := range cc.cache {
		if contains(key, pattern) {
			delete(cc.cache, key)
		}
	}
}

// contains is a simple string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(substr) <= len(s) && s[len(s)-len(substr):] == substr) ||
		(len(substr) <= len(s) && s[:len(substr)] == substr) ||
		indexOfSubstring(s, substr) >= 0)
}

// indexOfSubstring finds the index of a substring in a string
func indexOfSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
