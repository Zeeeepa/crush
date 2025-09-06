package cache

import (
	"context"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

// StreamingSessionService extends session.Service with streaming capabilities
type StreamingSessionService interface {
	session.Service
	
	// StreamGet returns a channel that emits the session and any updates
	StreamGet(ctx context.Context, id string) <-chan CacheResult[session.Session]
	
	// StreamList returns a channel that emits the session list and any updates
	StreamList(ctx context.Context) <-chan CacheResult[[]session.Session]
	
	// StreamListByParent returns sessions by parent ID with streaming updates
	StreamListByParent(ctx context.Context, parentID string) <-chan CacheResult[[]session.Session]
}

// StreamingMessageService extends message.Service with streaming capabilities
type StreamingMessageService interface {
	message.Service
	
	// StreamGet returns a channel that emits the message and any updates
	StreamGet(ctx context.Context, id string) <-chan CacheResult[message.Message]
	
	// StreamList returns a channel that emits messages for a session with updates
	StreamList(ctx context.Context, sessionID string) <-chan CacheResult[[]message.Message]
	
	// StreamListByRole returns messages filtered by role with streaming updates
	StreamListByRole(ctx context.Context, sessionID string, role message.MessageRole) <-chan CacheResult[[]message.Message]
}

// streamingSessionService wraps a session.Service with caching capabilities
type streamingSessionService struct {
	session.Service
	cache *SessionCache
}

// NewStreamingSessionService creates a streaming session service
func NewStreamingSessionService(service session.Service, cache *SessionCache) StreamingSessionService {
	return &streamingSessionService{
		Service: service,
		cache:   cache,
	}
}

func (s *streamingSessionService) StreamGet(ctx context.Context, id string) <-chan CacheResult[session.Session] {
	return s.cache.GetSession(ctx, id)
}

func (s *streamingSessionService) StreamList(ctx context.Context) <-chan CacheResult[[]session.Session] {
	return s.cache.ListSessions(ctx)
}

func (s *streamingSessionService) StreamListByParent(ctx context.Context, parentID string) <-chan CacheResult[[]session.Session] {
	return s.cache.ListSessionsByParent(ctx, parentID)
}

// streamingMessageService wraps a message.Service with caching capabilities
type streamingMessageService struct {
	message.Service
	cache *MessageCache
}

// NewStreamingMessageService creates a streaming message service
func NewStreamingMessageService(service message.Service, cache *MessageCache) StreamingMessageService {
	return &streamingMessageService{
		Service: service,
		cache:   cache,
	}
}

func (s *streamingMessageService) StreamGet(ctx context.Context, id string) <-chan CacheResult[message.Message] {
	return s.cache.GetMessage(ctx, id)
}

func (s *streamingMessageService) StreamList(ctx context.Context, sessionID string) <-chan CacheResult[[]message.Message] {
	return s.cache.ListMessagesBySession(ctx, sessionID)
}

func (s *streamingMessageService) StreamListByRole(ctx context.Context, sessionID string, role message.MessageRole) <-chan CacheResult[[]message.Message] {
	return s.cache.ListMessagesBySessionAndRole(ctx, sessionID, role)
}
