package cache

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
)

// streamCache implements StreamCache interface with event-driven updates
type streamCache[T any] struct {
	config    CacheConfig
	items     map[string]*cacheItem[T]
	queries   map[string]*querySubscription[T]
	mu        sync.RWMutex
	stats     CacheStats
	cleanup   *time.Ticker
	done      chan struct{}
	
	// Event subscription
	eventSub  <-chan pubsub.Event[T]
	eventDone chan struct{}
}

type cacheItem[T any] struct {
	data      T
	timestamp time.Time
	version   int64
	hits      int64
}

type querySubscription[T any] struct {
	query     Query
	filters   []Filter
	results   []T
	version   int64
	timestamp time.Time
	subs      map[chan CacheResult[[]T]]struct{}
	mu        sync.RWMutex
}

// NewStreamCache creates a new stream-based cache that subscribes to events
func NewStreamCache[T any](
	config CacheConfig,
	eventSubscriber func(context.Context) <-chan pubsub.Event[T],
) StreamCache[T] {
	if config.TTL == 0 {
		config = DefaultCacheConfig()
	}
	
	cache := &streamCache[T]{
		config:    config,
		items:     make(map[string]*cacheItem[T]),
		queries:   make(map[string]*querySubscription[T]),
		done:      make(chan struct{}),
		eventDone: make(chan struct{}),
	}
	
	// Start cleanup routine
	cache.cleanup = time.NewTicker(config.CleanupInterval)
	go cache.cleanupRoutine()
	
	// Subscribe to events if subscriber provided
	if eventSubscriber != nil {
		ctx := context.Background()
		cache.eventSub = eventSubscriber(ctx)
		go cache.eventRoutine()
	}
	
	return cache
}

// Get retrieves a single item by ID
func (c *streamCache[T]) Get(ctx context.Context, id string) <-chan CacheResult[T] {
	resultCh := make(chan CacheResult[T], c.config.BufferSize)
	
	go func() {
		defer close(resultCh)
		
		c.mu.RLock()
		item, exists := c.items[id]
		c.mu.RUnlock()
		
		if exists && !c.isExpired(item) {
			// Cache hit
			c.mu.Lock()
			item.hits++
			c.stats.HitCount++
			c.mu.Unlock()
			
			select {
			case resultCh <- CacheResult[T]{
				Data:      item.data,
				Cached:    true,
				Timestamp: item.timestamp,
				Version:   item.version,
			}:
			case <-ctx.Done():
				return
			}
		} else {
			// Cache miss - would need to fetch from source
			// For now, return empty result
			c.mu.Lock()
			c.stats.MissCount++
			c.mu.Unlock()
			
			var zero T
			select {
			case resultCh <- CacheResult[T]{
				Data:   zero,
				Cached: false,
				Error:  ErrCacheMiss,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return resultCh
}

// List retrieves items matching filters
func (c *streamCache[T]) List(ctx context.Context, filters ...Filter) <-chan CacheResult[[]T] {
	resultCh := make(chan CacheResult[[]T], c.config.BufferSize)
	
	go func() {
		defer close(resultCh)
		
		c.mu.RLock()
		var results []T
		for _, item := range c.items {
			if !c.isExpired(item) && c.matchesFilters(item.data, filters) {
				results = append(results, item.data)
			}
		}
		c.mu.RUnlock()
		
		select {
		case resultCh <- CacheResult[[]T]{
			Data:      results,
			Cached:    true,
			Timestamp: time.Now(),
		}:
		case <-ctx.Done():
			return
		}
	}()
	
	return resultCh
}

// Query executes a complex query
func (c *streamCache[T]) Query(ctx context.Context, query Query) <-chan CacheResult[[]T] {
	resultCh := make(chan CacheResult[[]T], c.config.BufferSize)
	
	go func() {
		defer close(resultCh)
		
		// For now, treat query as simple filter list
		results := c.List(ctx, query.Filters...)
		for result := range results {
			select {
			case resultCh <- result:
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return resultCh
}

// Invalidate removes items from cache
func (c *streamCache[T]) Invalidate(ids ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for _, id := range ids {
		delete(c.items, id)
		c.stats.ItemCount--
	}
}

// Clear removes all items from cache
func (c *streamCache[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items = make(map[string]*cacheItem[T])
	c.stats.ItemCount = 0
}

// Stats returns cache statistics
func (c *streamCache[T]) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// Close shuts down the cache
func (c *streamCache[T]) Close() error {
	close(c.done)
	close(c.eventDone)
	if c.cleanup != nil {
		c.cleanup.Stop()
	}
	return nil
}

// Event handling routine
func (c *streamCache[T]) eventRoutine() {
	for {
		select {
		case event, ok := <-c.eventSub:
			if !ok {
				return
			}
			c.handleEvent(event)
		case <-c.eventDone:
			return
		}
	}
}

// Handle incoming events to update cache
func (c *streamCache[T]) handleEvent(event pubsub.Event[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Extract ID from the event payload
	id := c.extractID(event.Payload)
	if id == "" {
		return
	}
	
	switch event.Type {
	case pubsub.CreatedEvent, pubsub.UpdatedEvent:
		// Add or update item in cache
		item := &cacheItem[T]{
			data:      event.Payload,
			timestamp: time.Now(),
			version:   time.Now().UnixNano(),
		}
		
		if existing, exists := c.items[id]; exists {
			item.hits = existing.hits
		} else {
			c.stats.ItemCount++
		}
		
		c.items[id] = item
		
	case pubsub.DeletedEvent:
		// Remove item from cache
		if _, exists := c.items[id]; exists {
			delete(c.items, id)
			c.stats.ItemCount--
		}
	}
}

// Extract ID from payload using reflection
func (c *streamCache[T]) extractID(payload T) string {
	v := reflect.ValueOf(payload)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	if v.Kind() != reflect.Struct {
		return ""
	}
	
	// Look for ID field
	idField := v.FieldByName("ID")
	if !idField.IsValid() || idField.Kind() != reflect.String {
		return ""
	}
	
	return idField.String()
}

// Check if item is expired
func (c *streamCache[T]) isExpired(item *cacheItem[T]) bool {
	return time.Since(item.timestamp) > c.config.TTL
}

// Check if item matches filters
func (c *streamCache[T]) matchesFilters(data T, filters []Filter) bool {
	if len(filters) == 0 {
		return true
	}
	
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	for _, filter := range filters {
		field := v.FieldByName(filter.Field)
		if !field.IsValid() {
			continue
		}
		
		if !c.matchesFilter(field.Interface(), filter) {
			return false
		}
	}
	
	return true
}

// Check if value matches a single filter
func (c *streamCache[T]) matchesFilter(value interface{}, filter Filter) bool {
	switch filter.Operator {
	case FilterEquals:
		return reflect.DeepEqual(value, filter.Value)
	case FilterNotEquals:
		return !reflect.DeepEqual(value, filter.Value)
	// Add more operators as needed
	default:
		return true
	}
}

// Cleanup routine to remove expired items
func (c *streamCache[T]) cleanupRoutine() {
	for {
		select {
		case <-c.cleanup.C:
			c.performCleanup()
		case <-c.done:
			return
		}
	}
}

func (c *streamCache[T]) performCleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	for id, item := range c.items {
		if now.Sub(item.timestamp) > c.config.TTL {
			delete(c.items, id)
			c.stats.ItemCount--
		}
	}
	
	c.stats.LastCleanup = now
}

// ErrCacheMiss indicates item not found in cache
var ErrCacheMiss = &CacheError{Message: "cache miss"}

type CacheError struct {
	Message string
}

func (e *CacheError) Error() string {
	return e.Message
}
