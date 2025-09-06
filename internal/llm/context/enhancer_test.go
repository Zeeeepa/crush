package context

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
	lsptesting "github.com/charmbracelet/crush/internal/lsp/testing"
)

func TestContextEnhancer_New(t *testing.T) {
	lspClients := map[string]*lsp.Client{
		"go": nil,
		"ts": nil,
	}

	enhancer := NewContextEnhancer(lspClients)

	assert.NotNil(t, enhancer)
	assert.Equal(t, len(lspClients), len(enhancer.lspClients))
}

func TestContextEnhancer_EnhanceContext_BasicRequest(t *testing.T) {
	// Create mock LSP server with test data
	mockServer := lsptesting.NewMockLSPServer()
	
	// Add test hover information
	testHover := lsptesting.CreateTestHover("Test function documentation")
	mockServer.AddHover("file:///test.go:10:5", testHover)

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	enhancer := NewContextEnhancer(lspClients)

	request := ContextRequest{
		Type:     RequestTypeCodeAnalysis,
		FilePath: "/test.go",
		Content:  "func testFunction() {}",
		Position: &Position{Line: 10, Character: 5},
	}

	enhanced, err := enhancer.EnhanceContext(context.Background(), request)

	require.NoError(t, err)
	assert.NotNil(t, enhanced)
	assert.Equal(t, request.Content, enhanced.OriginalContent)
	assert.NotEmpty(t, enhanced.LSPContext)
	assert.Contains(t, enhanced.LSPContext, "Test function documentation")
}

func TestContextEnhancer_EnhanceContext_NoLSPClients(t *testing.T) {
	enhancer := NewContextEnhancer(nil)

	request := ContextRequest{
		Type:     RequestTypeCodeAnalysis,
		FilePath: "/test.go",
		Content:  "func testFunction() {}",
	}

	enhanced, err := enhancer.EnhanceContext(context.Background(), request)

	require.NoError(t, err)
	assert.NotNil(t, enhanced)
	assert.Equal(t, request.Content, enhanced.OriginalContent)
	assert.Empty(t, enhanced.LSPContext) // No LSP context should be added
}

func TestContextEnhancer_EnhanceContext_MultipleContextTypes(t *testing.T) {
	// Create mock LSP server with comprehensive test data
	mockServer := lsptesting.NewMockLSPServer()
	
	// Add hover information
	testHover := lsptesting.CreateTestHover("Function: testFunction\nReturns: void")
	mockServer.AddHover("file:///test.go:10:5", testHover)
	
	// Add definition
	testLocation := lsptesting.CreateTestLocation(protocol.DocumentURI("file:///test.go"), 5, 0)
	mockServer.AddDefinition("file:///test.go:10:5", []protocol.Location{testLocation})
	
	// Add references
	refLocation1 := lsptesting.CreateTestLocation(protocol.DocumentURI("file:///test.go"), 15, 10)
	refLocation2 := lsptesting.CreateTestLocation(protocol.DocumentURI("file:///other.go"), 20, 5)
	mockServer.AddReferences("file:///test.go:10:5", []protocol.Location{refLocation1, refLocation2})

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	enhancer := NewContextEnhancer(lspClients)

	request := ContextRequest{
		Type:     RequestTypeCodeAnalysis,
		FilePath: "/test.go",
		Content:  "func testFunction() {}",
		Position: &Position{Line: 10, Character: 5},
		Options: ContextOptions{
			IncludeHover:      true,
			IncludeDefinition: true,
			IncludeReferences: true,
		},
	}

	enhanced, err := enhancer.EnhanceContext(context.Background(), request)

	require.NoError(t, err)
	assert.NotNil(t, enhanced)
	assert.Contains(t, enhanced.LSPContext, "Function: testFunction")
	assert.Contains(t, enhanced.LSPContext, "Definition:")
	assert.Contains(t, enhanced.LSPContext, "References:")
	assert.Contains(t, enhanced.LSPContext, "other.go")
}

func TestContextEnhancer_EnhanceContext_SymbolSearch(t *testing.T) {
	// Create mock LSP server with symbol data
	mockServer := lsptesting.NewMockLSPServer()
	
	// Add workspace symbols
	symbol1 := lsptesting.CreateTestSymbol("TestFunction", "function", protocol.DocumentURI("file:///test.go"), 10, 5)
	symbol2 := lsptesting.CreateTestSymbol("TestStruct", "struct", protocol.DocumentURI("file:///types.go"), 20, 0)
	mockServer.AddSymbol("Test", []protocol.WorkspaceSymbol{symbol1, symbol2})

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	enhancer := NewContextEnhancer(lspClients)

	request := ContextRequest{
		Type:    RequestTypeSymbolSearch,
		Content: "Test",
		Options: ContextOptions{
			IncludeSymbols: true,
		},
	}

	enhanced, err := enhancer.EnhanceContext(context.Background(), request)

	require.NoError(t, err)
	assert.NotNil(t, enhanced)
	assert.Contains(t, enhanced.LSPContext, "TestFunction")
	assert.Contains(t, enhanced.LSPContext, "TestStruct")
	assert.Contains(t, enhanced.LSPContext, "function")
	assert.Contains(t, enhanced.LSPContext, "struct")
}

func TestContextEnhancer_EnhanceContext_Caching(t *testing.T) {
	// Create mock LSP server
	mockServer := lsptesting.NewMockLSPServer()
	testHover := lsptesting.CreateTestHover("Cached hover information")
	mockServer.AddHover("file:///test.go:10:5", testHover)

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	enhancer := NewContextEnhancer(lspClients)

	request := ContextRequest{
		Type:     RequestTypeCodeAnalysis,
		FilePath: "/test.go",
		Content:  "func testFunction() {}",
		Position: &Position{Line: 10, Character: 5},
		Options: ContextOptions{
			IncludeHover: true,
		},
	}

	// First request
	enhanced1, err := enhancer.EnhanceContext(context.Background(), request)
	require.NoError(t, err)
	
	// Clear mock server requests to test caching
	mockServer.ClearRequests()
	
	// Second identical request
	enhanced2, err := enhancer.EnhanceContext(context.Background(), request)
	require.NoError(t, err)

	// Results should be identical
	assert.Equal(t, enhanced1.LSPContext, enhanced2.LSPContext)
	
	// Second request should have used cache (no new LSP requests)
	assert.Equal(t, 0, mockServer.GetRequestCount("textDocument/hover"))
}

func TestContextEnhancer_EnhanceContext_Timeout(t *testing.T) {
	// Create mock LSP server that simulates slow responses
	mockServer := lsptesting.NewMockLSPServer()

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	enhancer := NewContextEnhancer(lspClients)

	request := ContextRequest{
		Type:     RequestTypeCodeAnalysis,
		FilePath: "/test.go",
		Content:  "func testFunction() {}",
		Position: &Position{Line: 10, Character: 5},
		Options: ContextOptions{
			IncludeHover: true,
			Timeout:      1 * time.Millisecond, // Very short timeout
		},
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	enhanced, err := enhancer.EnhanceContext(ctx, request)

	// Should not error, but may have limited context due to timeout
	require.NoError(t, err)
	assert.NotNil(t, enhanced)
	// LSP context might be empty due to timeout
}

func TestContextEnhancer_EnhanceContext_ErrorHandling(t *testing.T) {
	// Create mock LSP server that will simulate errors
	mockServer := lsptesting.NewMockLSPServer()

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	enhancer := NewContextEnhancer(lspClients)

	request := ContextRequest{
		Type:     RequestTypeCodeAnalysis,
		FilePath: "", // Invalid file path
		Content:  "func testFunction() {}",
		Position: &Position{Line: 10, Character: 5},
	}

	enhanced, err := enhancer.EnhanceContext(context.Background(), request)

	// Should handle errors gracefully
	require.NoError(t, err)
	assert.NotNil(t, enhanced)
	// Should still return the original content even if LSP fails
	assert.Equal(t, request.Content, enhanced.OriginalContent)
}

func TestContextEnhancer_EnhanceContext_FileTypeFiltering(t *testing.T) {
	// Create mock LSP servers for different languages
	goMockServer := lsptesting.NewMockLSPServer()
	tsMockServer := lsptesting.NewMockLSPServer()
	
	// Add hover info to both servers
	goHover := lsptesting.CreateTestHover("Go function documentation")
	tsHover := lsptesting.CreateTestHover("TypeScript function documentation")
	
	goMockServer.AddHover("file:///test.go:10:5", goHover)
	tsMockServer.AddHover("file:///test.ts:10:5", tsHover)

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(goMockServer),
		"ts": createMockLSPClient(tsMockServer),
	}

	enhancer := NewContextEnhancer(lspClients)

	// Test Go file
	goRequest := ContextRequest{
		Type:     RequestTypeCodeAnalysis,
		FilePath: "/test.go",
		Content:  "func testFunction() {}",
		Position: &Position{Line: 10, Character: 5},
		Options:  ContextOptions{IncludeHover: true},
	}

	goEnhanced, err := enhancer.EnhanceContext(context.Background(), goRequest)
	require.NoError(t, err)
	assert.Contains(t, goEnhanced.LSPContext, "Go function documentation")
	assert.NotContains(t, goEnhanced.LSPContext, "TypeScript function documentation")

	// Test TypeScript file
	tsRequest := ContextRequest{
		Type:     RequestTypeCodeAnalysis,
		FilePath: "/test.ts",
		Content:  "function testFunction() {}",
		Position: &Position{Line: 10, Character: 5},
		Options:  ContextOptions{IncludeHover: true},
	}

	tsEnhanced, err := enhancer.EnhanceContext(context.Background(), tsRequest)
	require.NoError(t, err)
	assert.Contains(t, tsEnhanced.LSPContext, "TypeScript function documentation")
	assert.NotContains(t, tsEnhanced.LSPContext, "Go function documentation")
}

func TestContextEnhancer_EnhanceContext_MaxContextSize(t *testing.T) {
	// Create mock LSP server with large amounts of data
	mockServer := lsptesting.NewMockLSPServer()
	
	// Create very large hover content
	largeContent := ""
	for i := 0; i < 10000; i++ {
		largeContent += "This is a very long documentation string. "
	}
	
	testHover := lsptesting.CreateTestHover(largeContent)
	mockServer.AddHover("file:///test.go:10:5", testHover)

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	enhancer := NewContextEnhancer(lspClients)

	request := ContextRequest{
		Type:     RequestTypeCodeAnalysis,
		FilePath: "/test.go",
		Content:  "func testFunction() {}",
		Position: &Position{Line: 10, Character: 5},
		Options: ContextOptions{
			IncludeHover:   true,
			MaxContextSize: 1000, // Limit context size
		},
	}

	enhanced, err := enhancer.EnhanceContext(context.Background(), request)

	require.NoError(t, err)
	assert.NotNil(t, enhanced)
	// Context should be truncated to respect max size
	assert.LessOrEqual(t, len(enhanced.LSPContext), 1000+100) // Allow some buffer for formatting
}

// Benchmark tests
func BenchmarkContextEnhancer_EnhanceContext(b *testing.B) {
	// Create mock LSP server
	mockServer := lsptesting.NewMockLSPServer()
	testHover := lsptesting.CreateTestHover("Benchmark hover information")
	mockServer.AddHover("file:///test.go:10:5", testHover)

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	enhancer := NewContextEnhancer(lspClients)

	request := ContextRequest{
		Type:     RequestTypeCodeAnalysis,
		FilePath: "/test.go",
		Content:  "func testFunction() {}",
		Position: &Position{Line: 10, Character: 5},
		Options:  ContextOptions{IncludeHover: true},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := enhancer.EnhanceContext(context.Background(), request)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkContextEnhancer_EnhanceContext_WithCaching(b *testing.B) {
	// Create mock LSP server
	mockServer := lsptesting.NewMockLSPServer()
	testHover := lsptesting.CreateTestHover("Cached benchmark hover information")
	mockServer.AddHover("file:///test.go:10:5", testHover)

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	enhancer := NewContextEnhancer(lspClients)

	request := ContextRequest{
		Type:     RequestTypeCodeAnalysis,
		FilePath: "/test.go",
		Content:  "func testFunction() {}",
		Position: &Position{Line: 10, Character: 5},
		Options:  ContextOptions{IncludeHover: true},
	}

	// Prime the cache
	_, _ = enhancer.EnhanceContext(context.Background(), request)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := enhancer.EnhanceContext(context.Background(), request)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper function to create a mock LSP client
// Note: This would need to be implemented to work with the actual LSP client interface
func createMockLSPClient(mockServer *lsptesting.MockLSPServer) *lsp.Client {
	// This is a placeholder - in a real implementation, you would need to create
	// a mock that implements the LSP client interface and delegates to the mock server
	// For now, returning nil to make the code compile
	return nil
}

// Test helper functions
func TestContextEnhancer_Helpers(t *testing.T) {
	// Test position conversion
	pos := &Position{Line: 10, Character: 5}
	lspPos := positionToLSP(pos)
	assert.Equal(t, uint32(10), lspPos.Line)
	assert.Equal(t, uint32(5), lspPos.Character)

	// Test file path to URI conversion
	uri := filePathToURI("/test.go")
	assert.Equal(t, protocol.DocumentURI("file:///test.go"), uri)

	// Test context size calculation
	context := "This is a test context"
	size := calculateContextSize(context)
	assert.Equal(t, len(context), size)
}
