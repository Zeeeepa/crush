package tui

import (
	"context"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/crush/internal/cache"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

// StreamingHelper provides utilities for TUI components to use streaming data
type StreamingHelper struct {
	cacheManager *cache.Manager
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewStreamingHelper creates a new streaming helper
func NewStreamingHelper(cacheManager *cache.Manager) *StreamingHelper {
	ctx, cancel := context.WithCancel(context.Background())
	return &StreamingHelper{
		cacheManager: cacheManager,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Close stops all streaming operations
func (h *StreamingHelper) Close() {
	if h.cancel != nil {
		h.cancel()
	}
}

// SessionsUpdatedMsg is sent when sessions are updated via streaming
type SessionsUpdatedMsg struct {
	Sessions []session.Session
	Error    error
	Cached   bool
}

// MessagesUpdatedMsg is sent when messages are updated via streaming
type MessagesUpdatedMsg struct {
	SessionID string
	Messages  []message.Message
	Error     error
	Cached    bool
}

// StreamSessions starts streaming session updates and returns a command
func (h *StreamingHelper) StreamSessions() tea.Cmd {
	if h.cacheManager == nil {
		return nil
	}

	streamingSessions := h.cacheManager.StreamingSessions()
	if streamingSessions == nil {
		return nil
	}

	return func() tea.Msg {
		sessionStream := streamingSessions.StreamList(h.ctx)
		
		// Wait for first result and return it
		select {
		case result, ok := <-sessionStream:
			if !ok {
				return SessionsUpdatedMsg{Error: context.Canceled}
			}
			
			// Start background goroutine for subsequent updates
			go h.handleSessionUpdates(sessionStream)
			
			return SessionsUpdatedMsg{
				Sessions: result.Data,
				Error:    result.Error,
				Cached:   result.Cached,
			}
			
		case <-h.ctx.Done():
			return SessionsUpdatedMsg{Error: context.Canceled}
		}
	}
}

// handleSessionUpdates processes ongoing session updates in background
func (h *StreamingHelper) handleSessionUpdates(sessionStream <-chan cache.CacheResult[[]session.Session]) {
	for {
		select {
		case result, ok := <-sessionStream:
			if !ok {
				return // Channel closed
			}
			
			// Log updates for now - in a full implementation, you'd send these
			// back to the TUI via a program.Send() mechanism
			log.Printf("Sessions updated: %d sessions (cached: %v)", 
				len(result.Data), result.Cached)
			
		case <-h.ctx.Done():
			return
		}
	}
}

// StreamMessages starts streaming message updates for a session
func (h *StreamingHelper) StreamMessages(sessionID string) tea.Cmd {
	if h.cacheManager == nil || sessionID == "" {
		return nil
	}

	streamingMessages := h.cacheManager.StreamingMessages()
	if streamingMessages == nil {
		return nil
	}

	return func() tea.Msg {
		messageStream := streamingMessages.StreamList(h.ctx, sessionID)
		
		// Wait for first result and return it
		select {
		case result, ok := <-messageStream:
			if !ok {
				return MessagesUpdatedMsg{SessionID: sessionID, Error: context.Canceled}
			}
			
			// Start background goroutine for subsequent updates
			go h.handleMessageUpdates(sessionID, messageStream)
			
			return MessagesUpdatedMsg{
				SessionID: sessionID,
				Messages:  result.Data,
				Error:     result.Error,
				Cached:    result.Cached,
			}
			
		case <-h.ctx.Done():
			return MessagesUpdatedMsg{SessionID: sessionID, Error: context.Canceled}
		}
	}
}

// handleMessageUpdates processes ongoing message updates in background
func (h *StreamingHelper) handleMessageUpdates(sessionID string, messageStream <-chan cache.CacheResult[[]message.Message]) {
	for {
		select {
		case result, ok := <-messageStream:
			if !ok {
				return // Channel closed
			}
			
			// Log updates for now - in a full implementation, you'd send these
			// back to the TUI via a program.Send() mechanism
			log.Printf("Messages updated for session %s: %d messages (cached: %v)", 
				sessionID, len(result.Data), result.Cached)
			
		case <-h.ctx.Done():
			return
		}
	}
}

// GetSessionsSync gets sessions synchronously (fallback for immediate needs)
func (h *StreamingHelper) GetSessionsSync() ([]session.Session, error) {
	if h.cacheManager == nil {
		return nil, nil
	}

	streamingSessions := h.cacheManager.StreamingSessions()
	if streamingSessions == nil {
		return nil, nil
	}

	// Use traditional service method as fallback
	return streamingSessions.List(h.ctx)
}

// GetMessagesSync gets messages synchronously (fallback for immediate needs)
func (h *StreamingHelper) GetMessagesSync(sessionID string) ([]message.Message, error) {
	if h.cacheManager == nil || sessionID == "" {
		return nil, nil
	}

	streamingMessages := h.cacheManager.StreamingMessages()
	if streamingMessages == nil {
		return nil, nil
	}

	// Use traditional service method as fallback
	return streamingMessages.List(h.ctx, sessionID)
}
