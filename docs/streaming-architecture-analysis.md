# Streaming Architecture Analysis

## Current State Analysis

### ✅ Existing Streaming Infrastructure

**pubsub.Broker[T] System:**
- Well-implemented generic broker with subscription management
- Automatic cleanup on context cancellation
- Buffer management (64 events) with overflow protection
- Thread-safe operations with proper mutex usage

**Services with Streaming:**
- `session.Service`: Publishes Created/Updated/Deleted events
- `message.Service`: Publishes Created/Updated/Deleted events  
- `history.Service`: Publishes Created/Deleted events
- `permission.Service`: Publishes Created events + notifications
- `agent.Service`: Publishes Created events for agent operations
- `lsp`: LSP events for client status changes
- `mcp`: MCP tool events

**Event Integration:**
- App.setupEvents() subscribes to all service events
- Events are routed to TUI via tea.Msg channel
- Proper cleanup and lifecycle management

### ❌ Current Limitations & Direct Database Access

**TUI Components Making Direct Calls:**
```go
// internal/tui/components/chat/chat.go
sessionMessages, err := m.app.Messages.List(context.Background(), session.ID)
nestedMessages, _ := m.app.Messages.List(context.Background(), tc.ID)

// internal/tui/tui.go  
allSessions, _ := a.app.Sessions.List(context.Background())
session, err := a.app.Sessions.Get(context.Background(), a.selectedSessionID)
```

**Agent System Direct Database Access:**
```go
// internal/llm/agent/agent.go
session, err := a.sessions.Get(ctx, sessionID)
msgs, err := a.messages.List(ctx, sessionID)
a.messages.Update(context.Background(), agentMessage)
```

**Missing Stream Utilization:**
- No caching layer that responds to stream events
- Components don't subscribe to relevant data streams
- No real-time query subscriptions
- Manual refresh patterns instead of reactive updates

## Migration Strategy

### Phase 1: Stream-Based Caching Layer

**StreamCache[T] Implementation:**
- Generic cache that subscribes to service events
- Automatic cache invalidation on Created/Updated/Deleted
- TTL-based expiration for memory management
- Query-based caching (e.g., "messages for session X")

**Cache Types Needed:**
- SessionCache: Cache sessions by ID, maintain session lists
- MessageCache: Cache messages by session, support pagination
- FileCache: Cache file history by session and path

### Phase 2: Service Layer Enhancement

**Stream-First Methods:**
```go
// Add to existing services
StreamList(ctx context.Context, filters ...Filter) <-chan []T
StreamGet(ctx context.Context, id string) <-chan T
StreamQuery(ctx context.Context, query Query) <-chan QueryResult[T]
```

**Real-Time Subscriptions:**
- Subscribe to specific data queries
- Automatic re-evaluation when underlying data changes
- Delta updates for efficiency

### Phase 3: TUI Component Migration

**Replace Direct Calls:**
```go
// Before
sessionMessages, err := m.app.Messages.List(context.Background(), session.ID)

// After  
messageStream := m.cache.Messages.StreamList(session.ID)
// Component subscribes to stream and updates automatically
```

**Reactive Component Pattern:**
- Components subscribe to relevant streams in Init()
- Handle stream updates in Update() method
- Maintain local state synchronized with streams

### Phase 4: Agent System Integration

**Stream-Based Agent Operations:**
- Agent subscribes to message streams instead of direct queries
- Real-time progress updates via agent event streams
- Execution state persistence through streams

## Implementation Plan

### Step 1: Create Stream Cache Infrastructure
- `internal/cache/stream_cache.go` - Generic stream cache
- `internal/cache/session_cache.go` - Session-specific cache
- `internal/cache/message_cache.go` - Message-specific cache
- `internal/cache/interfaces.go` - Cache interfaces

### Step 2: Enhance Service Layer
- Add StreamList/StreamGet methods to existing services
- Implement query subscription system
- Add cache integration points

### Step 3: Migrate TUI Components
- Start with chat components (highest impact)
- Migrate session management in tui.go
- Update sidebar and file components

### Step 4: Agent System Migration
- Update agent to use stream-based data access
- Implement real-time agent status streaming
- Add execution monitoring via streams

## Benefits of Migration

### Performance Improvements
- Reduced database queries through intelligent caching
- Real-time updates eliminate polling
- Efficient delta updates for large datasets

### User Experience
- Instant UI updates when data changes
- Real-time collaboration support (future)
- Responsive interface with loading states

### Architecture Benefits
- Decoupled components through event-driven design
- Better testability with stream mocking
- Scalable foundation for multi-user scenarios

## Risk Mitigation

### Backward Compatibility
- Keep existing service methods during transition
- Gradual migration with feature flags
- Fallback to direct database access if streams fail

### Memory Management
- TTL-based cache expiration
- Configurable cache sizes
- Memory pressure monitoring

### Error Handling
- Graceful degradation when streams are unavailable
- Retry mechanisms for failed stream connections
- Circuit breaker patterns for reliability

## Success Metrics

### Performance
- Reduce database query count by 70%+
- Improve UI responsiveness (< 100ms updates)
- Lower memory usage through efficient caching

### Code Quality
- Reduce direct database access in TUI by 90%+
- Improve test coverage with stream mocking
- Better separation of concerns

This analysis provides the foundation for implementing a modern, reactive streaming architecture while preserving all existing functionality.
