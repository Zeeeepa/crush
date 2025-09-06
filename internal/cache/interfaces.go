package cache

import (
	"context"
	"time"
)

// StreamCache provides a generic interface for stream-based caching
type StreamCache[T any] interface {
	// Get retrieves a single item by ID, returns channel that emits current value and updates
	Get(ctx context.Context, id string) <-chan CacheResult[T]
	
	// List retrieves items matching filters, returns channel that emits current list and updates
	List(ctx context.Context, filters ...Filter) <-chan CacheResult[[]T]
	
	// Query executes a query and returns channel that emits results and updates
	Query(ctx context.Context, query Query) <-chan CacheResult[[]T]
	
	// Invalidate removes items from cache
	Invalidate(ids ...string)
	
	// Clear removes all items from cache
	Clear()
	
	// Stats returns cache statistics
	Stats() CacheStats
	
	// Close shuts down the cache and cleans up resources
	Close() error
}

// CacheResult wraps cached data with metadata
type CacheResult[T any] struct {
	Data      T
	Error     error
	Cached    bool      // true if data came from cache, false if from source
	Timestamp time.Time // when data was cached/updated
	Version   int64     // version for optimistic updates
}

// Filter represents a filter condition for cache queries
type Filter struct {
	Field    string
	Operator FilterOperator
	Value    interface{}
}

type FilterOperator string

const (
	FilterEquals    FilterOperator = "eq"
	FilterNotEquals FilterOperator = "ne"
	FilterIn        FilterOperator = "in"
	FilterNotIn     FilterOperator = "nin"
	FilterGreater   FilterOperator = "gt"
	FilterLess      FilterOperator = "lt"
	FilterContains  FilterOperator = "contains"
)

// Query represents a complex query with filters, sorting, and pagination
type Query struct {
	Filters []Filter
	Sort    []SortField
	Limit   int
	Offset  int
}

type SortField struct {
	Field string
	Desc  bool
}

// CacheStats provides cache performance metrics
type CacheStats struct {
	HitCount    int64
	MissCount   int64
	ItemCount   int64
	MemoryUsage int64
	LastCleanup time.Time
}

// CacheConfig configures cache behavior
type CacheConfig struct {
	// TTL is the time-to-live for cached items
	TTL time.Duration
	
	// MaxItems is the maximum number of items to cache
	MaxItems int
	
	// CleanupInterval is how often to run cleanup
	CleanupInterval time.Duration
	
	// BufferSize is the channel buffer size for streams
	BufferSize int
}

// DefaultCacheConfig returns sensible defaults
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		TTL:             5 * time.Minute,
		MaxItems:        1000,
		CleanupInterval: 1 * time.Minute,
		BufferSize:      64,
	}
}
