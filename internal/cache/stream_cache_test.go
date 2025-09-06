package cache

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
)

// TestData represents test data for cache testing
type TestData struct {
	ID   string
	Name string
	Age  int
}

func TestStreamCache_Basic(t *testing.T) {
	config := CacheConfig{
		TTL:             1 * time.Minute,
		MaxItems:        100,
		CleanupInterval: 10 * time.Second,
		BufferSize:      10,
	}

	// Create a test event broker
	broker := pubsub.NewBroker[TestData]()
	defer broker.Shutdown()

	// Create cache with event subscription
	cache := NewStreamCache(config, broker.Subscribe)
	defer cache.Close()

	ctx := context.Background()

	// Test cache miss
	resultCh := cache.Get(ctx, "test-1")
	result := <-resultCh

	if result.Error != ErrCacheMiss {
		t.Errorf("Expected cache miss, got: %v", result.Error)
	}

	// Simulate an event to populate cache
	testData := TestData{ID: "test-1", Name: "Test Item", Age: 25}
	broker.Publish(pubsub.CreatedEvent, testData)

	// Give some time for event processing
	time.Sleep(100 * time.Millisecond)

	// Test cache hit
	resultCh = cache.Get(ctx, "test-1")
	result = <-resultCh

	if result.Error != nil {
		t.Errorf("Expected cache hit, got error: %v", result.Error)
	}

	if !result.Cached {
		t.Error("Expected cached result")
	}

	if result.Data.ID != "test-1" {
		t.Errorf("Expected ID 'test-1', got: %s", result.Data.ID)
	}

	if result.Data.Name != "Test Item" {
		t.Errorf("Expected Name 'Test Item', got: %s", result.Data.Name)
	}
}

func TestStreamCache_List(t *testing.T) {
	config := DefaultCacheConfig()
	config.BufferSize = 10

	broker := pubsub.NewBroker[TestData]()
	defer broker.Shutdown()

	cache := NewStreamCache(config, broker.Subscribe)
	defer cache.Close()

	ctx := context.Background()

	// Add some test data via events
	testItems := []TestData{
		{ID: "1", Name: "Alice", Age: 25},
		{ID: "2", Name: "Bob", Age: 30},
		{ID: "3", Name: "Charlie", Age: 35},
	}

	for _, item := range testItems {
		broker.Publish(pubsub.CreatedEvent, item)
	}

	// Give some time for event processing
	time.Sleep(100 * time.Millisecond)

	// Test list all
	resultCh := cache.List(ctx)
	result := <-resultCh

	if result.Error != nil {
		t.Errorf("Expected successful list, got error: %v", result.Error)
	}

	if len(result.Data) != 3 {
		t.Errorf("Expected 3 items, got: %d", len(result.Data))
	}

	// Test list with filter
	filter := Filter{
		Field:    "Age",
		Operator: FilterEquals,
		Value:    30,
	}

	resultCh = cache.List(ctx, filter)
	result = <-resultCh

	if result.Error != nil {
		t.Errorf("Expected successful filtered list, got error: %v", result.Error)
	}

	if len(result.Data) != 1 {
		t.Errorf("Expected 1 filtered item, got: %d", len(result.Data))
	}

	if result.Data[0].Name != "Bob" {
		t.Errorf("Expected filtered item to be 'Bob', got: %s", result.Data[0].Name)
	}
}

func TestStreamCache_EventHandling(t *testing.T) {
	config := DefaultCacheConfig()
	config.BufferSize = 10

	broker := pubsub.NewBroker[TestData]()
	defer broker.Shutdown()

	cache := NewStreamCache(config, broker.Subscribe)
	defer cache.Close()

	ctx := context.Background()

	// Create item
	testData := TestData{ID: "test-1", Name: "Original", Age: 25}
	broker.Publish(pubsub.CreatedEvent, testData)

	time.Sleep(50 * time.Millisecond)

	// Verify creation
	resultCh := cache.Get(ctx, "test-1")
	result := <-resultCh

	if result.Error != nil || result.Data.Name != "Original" {
		t.Errorf("Expected original data, got: %v, error: %v", result.Data, result.Error)
	}

	// Update item
	updatedData := TestData{ID: "test-1", Name: "Updated", Age: 26}
	broker.Publish(pubsub.UpdatedEvent, updatedData)

	time.Sleep(50 * time.Millisecond)

	// Verify update
	resultCh = cache.Get(ctx, "test-1")
	result = <-resultCh

	if result.Error != nil || result.Data.Name != "Updated" {
		t.Errorf("Expected updated data, got: %v, error: %v", result.Data, result.Error)
	}

	// Delete item
	broker.Publish(pubsub.DeletedEvent, testData)

	time.Sleep(50 * time.Millisecond)

	// Verify deletion
	resultCh = cache.Get(ctx, "test-1")
	result = <-resultCh

	if result.Error != ErrCacheMiss {
		t.Errorf("Expected cache miss after deletion, got: %v", result.Error)
	}
}

func TestStreamCache_Stats(t *testing.T) {
	config := DefaultCacheConfig()
	config.BufferSize = 10

	broker := pubsub.NewBroker[TestData]()
	defer broker.Shutdown()

	cache := NewStreamCache(config, broker.Subscribe)
	defer cache.Close()

	ctx := context.Background()

	// Initial stats
	stats := cache.Stats()
	if stats.ItemCount != 0 {
		t.Errorf("Expected 0 items initially, got: %d", stats.ItemCount)
	}

	// Add item
	testData := TestData{ID: "test-1", Name: "Test", Age: 25}
	broker.Publish(pubsub.CreatedEvent, testData)

	time.Sleep(50 * time.Millisecond)

	// Check stats after addition
	stats = cache.Stats()
	if stats.ItemCount != 1 {
		t.Errorf("Expected 1 item after creation, got: %d", stats.ItemCount)
	}

	// Trigger cache hit
	resultCh := cache.Get(ctx, "test-1")
	<-resultCh

	stats = cache.Stats()
	if stats.HitCount != 1 {
		t.Errorf("Expected 1 hit, got: %d", stats.HitCount)
	}

	// Trigger cache miss
	resultCh = cache.Get(ctx, "nonexistent")
	<-resultCh

	stats = cache.Stats()
	if stats.MissCount != 1 {
		t.Errorf("Expected 1 miss, got: %d", stats.MissCount)
	}
}
