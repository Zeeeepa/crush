package cache

import (
	"context"
	"sync"

	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

// Manager coordinates all cache instances and their lifecycle
type Manager struct {
	config CacheConfig
	
	// Cache managers
	sessionManager *SessionCacheManager
	messageManager *MessageCacheManager
	
	// Services
	sessionService session.Service
	messageService message.Service
	historyService history.Service
	
	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// State
	started bool
	mu      sync.RWMutex
}

// NewManager creates a new cache manager
func NewManager(
	sessionService session.Service,
	messageService message.Service,
	historyService history.Service,
	config CacheConfig,
) *Manager {
	if config.TTL == 0 {
		config = DefaultCacheConfig()
	}
	
	return &Manager{
		config:         config,
		sessionService: sessionService,
		messageService: messageService,
		historyService: historyService,
	}
}

// Start initializes and starts all cache managers
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.started {
		return nil
	}
	
	m.ctx, m.cancel = context.WithCancel(ctx)
	
	// Initialize cache managers
	m.sessionManager = NewSessionCacheManager(m.sessionService, m.config)
	m.messageManager = NewMessageCacheManager(m.messageService, m.config)
	
	// Start cache managers
	if err := m.sessionManager.Start(m.ctx); err != nil {
		return err
	}
	
	if err := m.messageManager.Start(m.ctx); err != nil {
		return err
	}
	
	m.started = true
	return nil
}

// Stop shuts down all cache managers
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.started {
		return nil
	}
	
	// Cancel context to stop all operations
	if m.cancel != nil {
		m.cancel()
	}
	
	// Stop cache managers
	var errs []error
	
	if m.sessionManager != nil {
		if err := m.sessionManager.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	
	if m.messageManager != nil {
		if err := m.messageManager.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	
	// Wait for all goroutines to finish
	m.wg.Wait()
	
	m.started = false
	
	// Return first error if any
	if len(errs) > 0 {
		return errs[0]
	}
	
	return nil
}

// Sessions returns the session cache
func (m *Manager) Sessions() *SessionCache {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.sessionManager != nil {
		return m.sessionManager.GetCache()
	}
	return nil
}

// Messages returns the message cache
func (m *Manager) Messages() *MessageCache {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.messageManager != nil {
		return m.messageManager.GetCache()
	}
	return nil
}

// IsStarted returns whether the cache manager is started
func (m *Manager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// Stats returns statistics for all caches
func (m *Manager) Stats() map[string]CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := make(map[string]CacheStats)
	
	if m.sessionManager != nil && m.sessionManager.GetCache() != nil {
		stats["sessions"] = m.sessionManager.GetCache().Stats()
	}
	
	if m.messageManager != nil && m.messageManager.GetCache() != nil {
		stats["messages"] = m.messageManager.GetCache().Stats()
	}
	
	return stats
}
