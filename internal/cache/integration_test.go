package cache

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
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
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
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

	if _, err := conn.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Create services
	q := db.New(conn)
	sessionService := session.NewService(q)
	messageService := message.NewService(q)

	// Create cache manager
	config := DefaultCacheConfig()
	config.TTL = 30 * time.Second // Shorter TTL for testing
	config.CleanupInterval = 5 * time.Second

	manager := NewManager(sessionService, messageService, nil, config)

	ctx := context.Background()

	// Start cache manager
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start cache manager: %v", err)
	}
	defer manager.Stop()

	// Get streaming services
	streamingSessions := manager.StreamingSessions()
	streamingMessages := manager.StreamingMessages()

	if streamingSessions == nil {
		t.Fatal("StreamingSessions returned nil")
	}
	if streamingMessages == nil {
		t.Fatal("StreamingMessages returned nil")
	}

	// Test 1: Create session and verify streaming
	t.Run("SessionStreaming", func(t *testing.T) {
		// Start streaming before creating session
		sessionStream := streamingSessions.StreamList(ctx)

		// Create a session
		testSession, err := streamingSessions.Create(ctx, "Test Session")
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Wait for stream update
		select {
		case result := <-sessionStream:
			if result.Error != nil {
				t.Errorf("Stream error: %v", result.Error)
			}
			
			// Check if our session is in the list
			found := false
			for _, sess := range result.Data {
				if sess.ID == testSession.ID {
					found = true
					break
				}
			}
			
			if !found {
				t.Error("Created session not found in stream")
			}

		case <-time.After(2 * time.Second):
			t.Error("Timeout waiting for session stream update")
		}
	})

	// Test 2: Create messages and verify streaming
	t.Run("MessageStreaming", func(t *testing.T) {
		// Create a session first
		testSession, err := streamingSessions.Create(ctx, "Message Test Session")
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Start streaming messages for this session
		messageStream := streamingMessages.StreamList(ctx, testSession.ID)

		// Create a message
		testMessage, err := streamingMessages.Create(ctx, testSession.ID, message.CreateMessageParams{
			Role: message.User,
			Parts: []message.Part{
				message.Text{Content: "Hello, world!"},
			},
		})
		if err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}

		// Wait for stream update
		select {
		case result := <-messageStream:
			if result.Error != nil {
				t.Errorf("Stream error: %v", result.Error)
			}
			
			// Check if our message is in the list
			found := false
			for _, msg := range result.Data {
				if msg.ID == testMessage.ID {
					found = true
					break
				}
			}
			
			if !found {
				t.Error("Created message not found in stream")
			}

		case <-time.After(2 * time.Second):
			t.Error("Timeout waiting for message stream update")
		}
	})

	// Test 3: Test cache performance
	t.Run("CachePerformance", func(t *testing.T) {
		// Create test data
		testSession, err := streamingSessions.Create(ctx, "Performance Test Session")
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Create multiple messages
		for i := 0; i < 10; i++ {
			_, err := streamingMessages.Create(ctx, testSession.ID, message.CreateMessageParams{
				Role: message.User,
				Parts: []message.Part{
					message.Text{Content: "Test message"},
				},
			})
			if err != nil {
				t.Fatalf("Failed to create message %d: %v", i, err)
			}
		}

		// Give cache time to populate
		time.Sleep(100 * time.Millisecond)

		// Test cache hit performance
		start := time.Now()
		messageStream := streamingMessages.StreamList(ctx, testSession.ID)
		
		select {
		case result := <-messageStream:
			duration := time.Since(start)
			
			if result.Error != nil {
				t.Errorf("Stream error: %v", result.Error)
			}
			
			if len(result.Data) != 10 {
				t.Errorf("Expected 10 messages, got %d", len(result.Data))
			}
			
			// Should be fast due to caching
			if duration > 100*time.Millisecond {
				t.Errorf("Cache lookup too slow: %v", duration)
			}
			
			t.Logf("Cache lookup took: %v", duration)

		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for cached message list")
		}
	})

	// Test 4: Test cache statistics
	t.Run("CacheStatistics", func(t *testing.T) {
		stats := manager.Stats()
		
		if len(stats) == 0 {
			t.Error("No cache statistics returned")
		}
		
		for cacheName, stat := range stats {
			t.Logf("Cache %s: Items=%d, Hits=%d, Misses=%d", 
				cacheName, stat.ItemCount, stat.HitCount, stat.MissCount)
			
			if stat.ItemCount < 0 {
				t.Errorf("Invalid item count for %s: %d", cacheName, stat.ItemCount)
			}
		}
	})
}

// TestIntegration_CacheInvalidation tests cache invalidation via events
func TestIntegration_CacheInvalidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup in-memory database
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Initialize database schema
	schema := `
	CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		parent_session_id TEXT,
		title TEXT NOT NULL,
		cost REAL DEFAULT 0,
		created_at INTEGER DEFAULT (strftime('%s', 'now')),
		updated_at INTEGER DEFAULT (strftime('%s', 'now'))
	);
	`

	if _, err := conn.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Create services and manager
	q := db.New(conn)
	sessionService := session.NewService(q)
	config := DefaultCacheConfig()
	manager := NewManager(sessionService, nil, nil, config)

	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start cache manager: %v", err)
	}
	defer manager.Stop()

	streamingSessions := manager.StreamingSessions()

	// Create initial session
	testSession, err := streamingSessions.Create(ctx, "Test Session")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Start streaming
	sessionStream := streamingSessions.StreamGet(ctx, testSession.ID)

	// Verify initial data
	select {
	case result := <-sessionStream:
		if result.Error != nil {
			t.Fatalf("Stream error: %v", result.Error)
		}
		if result.Data.Title != "Test Session" {
			t.Errorf("Expected title 'Test Session', got: %s", result.Data.Title)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for initial session data")
	}

	// Update session
	testSession.Title = "Updated Session"
	updatedSession, err := streamingSessions.Save(ctx, testSession)
	if err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	// Verify cache invalidation and update
	select {
	case result := <-sessionStream:
		if result.Error != nil {
			t.Errorf("Stream error: %v", result.Error)
		}
		if result.Data.Title != "Updated Session" {
			t.Errorf("Expected updated title 'Updated Session', got: %s", result.Data.Title)
		}
		if result.Data.ID != updatedSession.ID {
			t.Errorf("Session ID mismatch after update")
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for session update")
	}
}

// BenchmarkStreamingServices benchmarks the streaming service performance
func BenchmarkStreamingServices(b *testing.B) {
	// Setup in-memory database
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Initialize schema
	schema := `
	CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		parent_session_id TEXT,
		title TEXT NOT NULL,
		cost REAL DEFAULT 0,
		created_at INTEGER DEFAULT (strftime('%s', 'now')),
		updated_at INTEGER DEFAULT (strftime('%s', 'now'))
	);
	`

	if _, err := conn.Exec(schema); err != nil {
		b.Fatalf("Failed to create schema: %v", err)
	}

	// Create services and manager
	q := db.New(conn)
	sessionService := session.NewService(q)
	config := DefaultCacheConfig()
	manager := NewManager(sessionService, nil, nil, config)

	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		b.Fatalf("Failed to start cache manager: %v", err)
	}
	defer manager.Stop()

	streamingSessions := manager.StreamingSessions()

	// Create test data
	for i := 0; i < 100; i++ {
		_, err := streamingSessions.Create(ctx, "Test Session")
		if err != nil {
			b.Fatalf("Failed to create session: %v", err)
		}
	}

	// Give cache time to populate
	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()

	// Benchmark streaming list operations
	b.Run("StreamList", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sessionStream := streamingSessions.StreamList(ctx)
			select {
			case result := <-sessionStream:
				if result.Error != nil {
					b.Errorf("Stream error: %v", result.Error)
				}
			case <-time.After(1 * time.Second):
				b.Error("Timeout waiting for session list")
			}
		}
	})
}
