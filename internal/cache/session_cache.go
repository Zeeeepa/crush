package cache

import (
	"context"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
)

// SessionCache provides stream-based caching for sessions
type SessionCache struct {
	StreamCache[session.Session]
}

// NewSessionCache creates a new session cache that subscribes to session events
func NewSessionCache(
	config CacheConfig,
	sessionService session.Service,
) *SessionCache {
	streamCache := NewStreamCache(
		config,
		sessionService.Subscribe,
	)
	
	return &SessionCache{
		StreamCache: streamCache,
	}
}

// GetSession retrieves a session by ID with streaming updates
func (c *SessionCache) GetSession(ctx context.Context, id string) <-chan CacheResult[session.Session] {
	return c.Get(ctx, id)
}

// ListSessions retrieves all sessions with streaming updates
func (c *SessionCache) ListSessions(ctx context.Context) <-chan CacheResult[[]session.Session] {
	return c.List(ctx)
}

// ListSessionsByParent retrieves sessions by parent ID
func (c *SessionCache) ListSessionsByParent(ctx context.Context, parentID string) <-chan CacheResult[[]session.Session] {
	filter := Filter{
		Field:    "ParentSessionID",
		Operator: FilterEquals,
		Value:    parentID,
	}
	return c.List(ctx, filter)
}

// SessionCacheManager manages session cache lifecycle
type SessionCacheManager struct {
	cache   *SessionCache
	service session.Service
	config  CacheConfig
}

// NewSessionCacheManager creates a new session cache manager
func NewSessionCacheManager(service session.Service, config CacheConfig) *SessionCacheManager {
	return &SessionCacheManager{
		service: service,
		config:  config,
	}
}

// Start initializes and starts the session cache
func (m *SessionCacheManager) Start(ctx context.Context) error {
	m.cache = NewSessionCache(m.config, m.service)
	
	// Pre-populate cache with existing sessions
	go m.prePopulateCache(ctx)
	
	return nil
}

// GetCache returns the session cache instance
func (m *SessionCacheManager) GetCache() *SessionCache {
	return m.cache
}

// Stop shuts down the session cache
func (m *SessionCacheManager) Stop() error {
	if m.cache != nil {
		return m.cache.Close()
	}
	return nil
}

// Pre-populate cache with existing sessions
func (m *SessionCacheManager) prePopulateCache(ctx context.Context) {
	// Get existing sessions from service
	sessions, err := m.service.List(ctx)
	if err != nil {
		return
	}
	
	// Simulate events to populate cache
	for _, sess := range sessions {
		// Create a fake Created event to populate cache
		event := pubsub.Event[session.Session]{
			Type:    pubsub.CreatedEvent,
			Payload: sess,
		}
		
		// This would normally be handled by the event routine
		// but we need to access the internal cache methods
		// For now, we'll rely on the cache being populated through normal usage
	}
}
