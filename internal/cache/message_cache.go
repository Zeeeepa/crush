package cache

import (
	"context"

	"github.com/charmbracelet/crush/internal/message"
)

// MessageCache provides stream-based caching for messages
type MessageCache struct {
	StreamCache[message.Message]
}

// NewMessageCache creates a new message cache that subscribes to message events
func NewMessageCache(
	config CacheConfig,
	messageService message.Service,
) *MessageCache {
	streamCache := NewStreamCache(
		config,
		messageService.Subscribe,
	)
	
	return &MessageCache{
		StreamCache: streamCache,
	}
}

// GetMessage retrieves a message by ID with streaming updates
func (c *MessageCache) GetMessage(ctx context.Context, id string) <-chan CacheResult[message.Message] {
	return c.Get(ctx, id)
}

// ListMessages retrieves all messages with streaming updates
func (c *MessageCache) ListMessages(ctx context.Context) <-chan CacheResult[[]message.Message] {
	return c.List(ctx)
}

// ListMessagesBySession retrieves messages for a specific session
func (c *MessageCache) ListMessagesBySession(ctx context.Context, sessionID string) <-chan CacheResult[[]message.Message] {
	filter := Filter{
		Field:    "SessionID",
		Operator: FilterEquals,
		Value:    sessionID,
	}
	return c.List(ctx, filter)
}

// ListMessagesByRole retrieves messages by role (user, assistant, etc.)
func (c *MessageCache) ListMessagesByRole(ctx context.Context, role message.MessageRole) <-chan CacheResult[[]message.Message] {
	filter := Filter{
		Field:    "Role",
		Operator: FilterEquals,
		Value:    role,
	}
	return c.List(ctx, filter)
}

// ListMessagesBySessionAndRole retrieves messages for a session filtered by role
func (c *MessageCache) ListMessagesBySessionAndRole(ctx context.Context, sessionID string, role message.MessageRole) <-chan CacheResult[[]message.Message] {
	filters := []Filter{
		{
			Field:    "SessionID",
			Operator: FilterEquals,
			Value:    sessionID,
		},
		{
			Field:    "Role",
			Operator: FilterEquals,
			Value:    role,
		},
	}
	return c.List(ctx, filters...)
}

// MessageCacheManager manages message cache lifecycle
type MessageCacheManager struct {
	cache   *MessageCache
	service message.Service
	config  CacheConfig
}

// NewMessageCacheManager creates a new message cache manager
func NewMessageCacheManager(service message.Service, config CacheConfig) *MessageCacheManager {
	return &MessageCacheManager{
		service: service,
		config:  config,
	}
}

// Start initializes and starts the message cache
func (m *MessageCacheManager) Start(ctx context.Context) error {
	m.cache = NewMessageCache(m.config, m.service)
	return nil
}

// GetCache returns the message cache instance
func (m *MessageCacheManager) GetCache() *MessageCache {
	return m.cache
}

// Stop shuts down the message cache
func (m *MessageCacheManager) Stop() error {
	if m.cache != nil {
		return m.cache.Close()
	}
	return nil
}
