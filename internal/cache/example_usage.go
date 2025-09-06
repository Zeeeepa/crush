package cache

import (
	"context"
	"fmt"
	"log"

	"github.com/charmbracelet/crush/internal/message"
)

// ExampleTUIComponent demonstrates how TUI components would use streaming services
type ExampleTUIComponent struct {
	cacheManager *Manager
	sessionID    string
	
	// Component state
	messages []message.Message
	loading  bool
}

// NewExampleTUIComponent creates a new example TUI component
func NewExampleTUIComponent(cacheManager *Manager, sessionID string) *ExampleTUIComponent {
	return &ExampleTUIComponent{
		cacheManager: cacheManager,
		sessionID:    sessionID,
		loading:      true,
	}
}

// Start begins streaming data for the component
func (c *ExampleTUIComponent) Start(ctx context.Context) {
	// Get streaming message service
	streamingMessages := c.cacheManager.StreamingMessages()
	
	// Subscribe to message stream for this session
	messageStream := streamingMessages.StreamList(ctx, c.sessionID)
	
	// Handle streaming updates
	go func() {
		for {
			select {
			case result, ok := <-messageStream:
				if !ok {
					log.Println("Message stream closed")
					return
				}
				
				if result.Error != nil {
					log.Printf("Stream error: %v", result.Error)
					continue
				}
				
				// Update component state
				c.messages = result.Data
				c.loading = false
				
				// In a real TUI, this would trigger a re-render
				c.onMessagesUpdated(result.Data, result.Cached)
				
			case <-ctx.Done():
				log.Println("Context cancelled, stopping message stream")
				return
			}
		}
	}()
}

// onMessagesUpdated handles message updates (would trigger UI re-render in real TUI)
func (c *ExampleTUIComponent) onMessagesUpdated(messages []message.Message, fromCache bool) {
	source := "database"
	if fromCache {
		source = "cache"
	}
	
	fmt.Printf("📨 Messages updated from %s: %d messages\n", source, len(messages))
	
	for i, msg := range messages {
		fmt.Printf("  %d. [%s] %s\n", i+1, msg.Role, c.getMessagePreview(msg))
	}
}

// getMessagePreview extracts a preview from message parts
func (c *ExampleTUIComponent) getMessagePreview(msg message.Message) string {
	for _, part := range msg.Parts {
		if textPart, ok := part.(message.Text); ok {
			if len(textPart.Content) > 50 {
				return textPart.Content[:50] + "..."
			}
			return textPart.Content
		}
	}
	return "[no text content]"
}

// ExampleUsage demonstrates the complete streaming workflow
func ExampleUsage() {
	// This would typically be called from main application setup
	
	// Assume we have a cache manager already set up
	// manager := cache.NewManager(sessionService, messageService, historyService, config)
	// manager.Start(ctx)
	
	// Example of how a TUI component would use streaming services:
	
	/*
	// In TUI component initialization:
	component := NewExampleTUIComponent(app.CacheManager, selectedSessionID)
	component.Start(ctx)
	
	// The component now automatically receives updates when:
	// 1. New messages are created in the session
	// 2. Existing messages are updated
	// 3. Messages are deleted
	// 4. Data is loaded from cache vs database
	
	// Benefits:
	// - No manual refresh needed
	// - Real-time updates
	// - Automatic cache optimization
	// - Reduced database load
	// - Better user experience
	*/
	
	fmt.Println("Example usage documented in comments above")
}

// ExampleMigrationPattern shows how to migrate from direct database access
func ExampleMigrationPattern() {
	fmt.Println("Migration Pattern:")
	fmt.Println()
	
	fmt.Println("BEFORE (Direct Database Access):")
	fmt.Println("```go")
	fmt.Println("// TUI component making direct database calls")
	fmt.Println("sessionMessages, err := m.app.Messages.List(context.Background(), session.ID)")
	fmt.Println("if err != nil {")
	fmt.Println("    // Handle error")
	fmt.Println("}")
	fmt.Println("// Manual refresh required for updates")
	fmt.Println("```")
	fmt.Println()
	
	fmt.Println("AFTER (Stream-Based Caching):")
	fmt.Println("```go")
	fmt.Println("// TUI component subscribing to reactive streams")
	fmt.Println("messageStream := app.CacheManager.StreamingMessages().StreamList(ctx, sessionID)")
	fmt.Println("go func() {")
	fmt.Println("    for result := range messageStream {")
	fmt.Println("        if result.Error != nil {")
	fmt.Println("            // Handle error")
	fmt.Println("            continue")
	fmt.Println("        }")
	fmt.Println("        // Automatic UI updates with latest data")
	fmt.Println("        updateUI(result.Data)")
	fmt.Println("    }")
	fmt.Println("}()")
	fmt.Println("```")
	fmt.Println()
	
	fmt.Println("Benefits of Migration:")
	fmt.Println("✅ Real-time updates - no manual refresh needed")
	fmt.Println("✅ Reduced database load - intelligent caching")
	fmt.Println("✅ Better performance - cache hits are fast")
	fmt.Println("✅ Reactive UI - updates automatically when data changes")
	fmt.Println("✅ Error resilience - graceful degradation when cache unavailable")
}
