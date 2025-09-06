package cache

import (
	"context"

	"github.com/charmbracelet/crush/internal/session"
)

// streamingSessionService implements StreamingSessionService
type streamingSessionService struct {
	session.Service
	cache StreamCache[session.Session]
}

// NewStreamingSessionService creates a new streaming session service
func NewStreamingSessionService(
	baseService session.Service,
	cache StreamCache[session.Session],
) StreamingSessionService {
	return &streamingSessionService{
		Service: baseService,
		cache:   cache,
	}
}

// StreamGet returns a channel that emits the session and any updates
func (s *streamingSessionService) StreamGet(ctx context.Context, id string) <-chan CacheResult[session.Session] {
	return s.cache.Get(ctx, id)
}

// StreamList returns a channel that emits the session list and any updates
func (s *streamingSessionService) StreamList(ctx context.Context) <-chan CacheResult[[]session.Session] {
	return s.cache.List(ctx)
}

// StreamListByParent returns sessions by parent ID with streaming updates
func (s *streamingSessionService) StreamListByParent(ctx context.Context, parentID string) <-chan CacheResult[[]session.Session] {
	filter := Filter{
		Field: "parent_id",
		Op:    FilterOpEquals,
		Value: parentID,
	}
	return s.cache.List(ctx, filter)
}

// StreamQuery executes a query and returns streaming results
func (s *streamingSessionService) StreamQuery(ctx context.Context, query Query) <-chan CacheResult[[]session.Session] {
	return s.cache.Query(ctx, query)
}
