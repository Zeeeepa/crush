package cache

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

// TestIntegration_StreamingServices tests the complete streaming system
func TestIntegration_StreamingServices(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup in-memory database
	conn, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer conn.Close()

	// Initialize database schema (simplified for testing)
	schema := `
	CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		parent_session_id TEXT,
		title TEXT NOT NULL,
		cost REAL DEFAULT 0,
		created_at INTEGER DEFAULT (strftime('%s', 'now')),
		updated_at INTEGER DEFAULT (strftime('%s', 'now'))
	);

	CREATE TABLE messages (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT,
		created_at INTEGER DEFAULT (strftime('%s', 'now')),
		updated_at INTEGER DEFAULT (strftime('%s', 'now')),
		FOREIGN KEY (session_id) REFERENCES sessions(id)
	);
	`

	_, err = conn.Exec(schema)
	require.NoError(t, err)

	// Create services
	q := db.New(conn)
	sessionService := session.NewService(q, conn)
	messageService := message.NewService(q, conn)

	// Create cache manager
	config := DefaultCacheConfig()
	config.TTL = 30 * time.Second // Shorter TTL for testing
	config.CleanupInterval = 5 * time.Second

	manager := NewManager(sessionService, messageService, nil, config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start cache manager
	err = manager.Start(ctx)
	require.NoError(t, err)
	defer manager.Stop()

	// Get streaming services
	streamingSessions := manager.StreamingSessions()
	streamingMessages := manager.StreamingMessages()

	require.NotNil(t, streamingSessions)
	require.NotNil(t, streamingMessages)

	// Test 1: Session streaming
	t.Run("SessionStreaming", func(t *testing.T) {
		// Create a session
		sess, err := sessionService.Create(ctx, session.CreateParams{
			Title: "Test Session",
		})
		require.NoError(t, err)

		// Test StreamGet
		sessionStream := streamingSessions.StreamGet(ctx, sess.ID)
		
		select {
		case result := <-sessionStream:
			require.NoError(t, result.Error)
			assert.Equal(t, sess.ID, result.Data.ID)
			assert.Equal(t, "Test Session", result.Data.Title)
			
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for session stream")
		}

		// Test StreamList
		listStream := streamingSessions.StreamList(ctx)
		
		select {
		case result := <-listStream:
			require.NoError(t, result.Error)
			assert.GreaterOrEqual(t, len(result.Data), 1)
			
			// Find our session
			found := false
			for _, s := range result.Data {
				if s.ID == sess.ID {
					found = true
					break
				}
			}
			assert.True(t, found)
			
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for session list stream")
		}
	})

	// Test 2: Message streaming
	t.Run("MessageStreaming", func(t *testing.T) {
		// Create a session for messages
		sess, err := sessionService.Create(ctx, session.CreateParams{
			Title: "Message Test Session",
		})
		require.NoError(t, err)

		// Create a message
		msg, err := messageService.Create(ctx, message.CreateParams{
			SessionID: sess.ID,
			Role:      message.RoleUser,
			Content:   "Test message",
		})
		require.NoError(t, err)

		// Test StreamGet
		messageStream := streamingMessages.StreamGet(ctx, msg.ID)
		
		select {
		case result := <-messageStream:
			require.NoError(t, result.Error)
			assert.Equal(t, msg.ID, result.Data.ID)
			assert.Equal(t, "Test message", result.Data.Content)
			
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for message stream")
		}

		// Test StreamList by session
		listStream := streamingMessages.StreamList(ctx, sess.ID)
		
		select {
		case result := <-listStream:
			require.NoError(t, result.Error)
			assert.GreaterOrEqual(t, len(result.Data), 1)
			
			// Find our message
			found := false
			for _, m := range result.Data {
				if m.ID == msg.ID {
					found = true
					break
				}
			}
			assert.True(t, found)
			
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for message list stream")
		}
	})

	// Test 3: Real-time updates
	t.Run("RealTimeUpdates", func(t *testing.T) {
		// Start streaming session list
		listStream := streamingSessions.StreamList(ctx)
		
		// Get initial count
		var initialCount int
		select {
		case result := <-listStream:
			require.NoError(t, result.Error)
			initialCount = len(result.Data)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for initial session list")
		}

		// Create a new session in background
		go func() {
			time.Sleep(100 * time.Millisecond)
			_, err := sessionService.Create(ctx, session.CreateParams{
				Title: "Real-time Test Session",
			})
			if err != nil {
				t.Errorf("Failed to create session: %v", err)
			}
		}()

		// Wait for update
		select {
		case result := <-listStream:
			require.NoError(t, result.Error)
			assert.Equal(t, initialCount+1, len(result.Data))
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for real-time update")
		}
	})

	// Test 4: Cache performance
	t.Run("CachePerformance", func(t *testing.T) {
		// Create test session
		sess, err := sessionService.Create(ctx, session.CreateParams{
			Title: "Performance Test Session",
		})
		require.NoError(t, err)

		// First access (cache miss)
		start := time.Now()
		sessionStream1 := streamingSessions.StreamGet(ctx, sess.ID)
		select {
		case result := <-sessionStream1:
			require.NoError(t, result.Error)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout on first access")
		}
		firstAccess := time.Since(start)

		// Second access (cache hit)
		start = time.Now()
		sessionStream2 := streamingSessions.StreamGet(ctx, sess.ID)
		select {
		case result := <-sessionStream2:
			require.NoError(t, result.Error)
			assert.True(t, result.Cached, "Second access should be cached")
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout on second access")
		}
		secondAccess := time.Since(start)

		t.Logf("First access: %v, Second access: %v", firstAccess, secondAccess)
		
		// Cached access should be faster
		assert.Less(t, secondAccess, firstAccess)
	})
}
