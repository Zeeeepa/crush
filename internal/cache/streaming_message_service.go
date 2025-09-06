package cache

import (
	"context"

	"github.com/charmbracelet/crush/internal/message"
)

// streamingMessageService implements StreamingMessageService
type streamingMessageService struct {
	message.Service
	cache StreamCache[message.Message]
}

// NewStreamingMessageService creates a new streaming message service
func NewStreamingMessageService(
	baseService message.Service,
	cache StreamCache[message.Message],
) StreamingMessageService {
	return &streamingMessageService{
		Service: baseService,
		cache:   cache,
	}
}

// StreamGet returns a channel that emits the message and any updates
func (s *streamingMessageService) StreamGet(ctx context.Context, id string) <-chan CacheResult[message.Message] {
	return s.cache.Get(ctx, id)
}

// StreamList returns a channel that emits messages for a session with streaming updates
func (s *streamingMessageService) StreamList(ctx context.Context, sessionID string) <-chan CacheResult[[]message.Message] {
	filter := Filter{
		Field: "session_id",
		Op:    FilterOpEquals,
		Value: sessionID,
	}
	return s.cache.List(ctx, filter)
}

// StreamListByParent returns messages by parent ID with streaming updates
func (s *streamingMessageService) StreamListByParent(ctx context.Context, parentID string) <-chan CacheResult[[]message.Message] {
	filter := Filter{
		Field: "parent_id",
		Op:    FilterOpEquals,
		Value: parentID,
	}
	return s.cache.List(ctx, filter)
}

// StreamListByRole returns messages by role with streaming updates
func (s *streamingMessageService) StreamListByRole(ctx context.Context, sessionID string, role message.Role) <-chan CacheResult[[]message.Message] {
	filters := []Filter{
		{
			Field: "session_id",
			Op:    FilterOpEquals,
			Value: sessionID,
		},
		{
			Field: "role",
			Op:    FilterOpEquals,
			Value: string(role),
		},
	}
	return s.cache.List(ctx, filters...)
}

// StreamQuery executes a query and returns streaming results
func (s *streamingMessageService) StreamQuery(ctx context.Context, query Query) <-chan CacheResult[[]message.Message] {
	return s.cache.Query(ctx, query)
}
