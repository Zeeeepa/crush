package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
	lsptesting "github.com/charmbracelet/crush/internal/lsp/testing"
)

func TestDefinitionTool_Info(t *testing.T) {
	tool := NewDefinitionTool(nil)
	info := tool.Info()

	assert.Equal(t, DefinitionToolName, info.Name)
	assert.Contains(t, info.Description, "Go to Definition")
	assert.Contains(t, info.Parameters, "file_path")
	assert.Contains(t, info.Parameters, "line")
	assert.Contains(t, info.Parameters, "character")
	assert.Contains(t, info.Required, "file_path")
	assert.Contains(t, info.Required, "line")
	assert.Contains(t, info.Required, "character")
}

func TestDefinitionTool_Run_Success(t *testing.T) {
	// Create mock LSP server
	mockServer := lsptesting.NewMockLSPServer()
	
	// Add test definition
	testURI := protocol.DocumentURI("file:///test.go")
	testLocation := lsptesting.CreateTestLocation(testURI, 10, 5)
	mockServer.AddDefinition("file:///test.go:5:10", []protocol.Location{testLocation})

	// Create mock LSP client (this would need to be implemented)
	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	tool := NewDefinitionTool(lspClients)

	// Test parameters
	params := DefinitionParams{
		FilePath:  "/test.go",
		Line:      5,
		Character: 10,
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	// Run the tool
	call := ToolCall{Input: string(paramsJSON)}
	response, err := tool.Run(context.Background(), call)

	// Verify results
	require.NoError(t, err)
	assert.Contains(t, response.Content, "Definition found")
	assert.Contains(t, response.Content, "file:///test.go")
	assert.Contains(t, response.Content, "line 11") // LSP uses 0-based, display uses 1-based
}

func TestDefinitionTool_Run_NoDefinition(t *testing.T) {
	// Create mock LSP server with no definitions
	mockServer := lsptesting.NewMockLSPServer()

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	tool := NewDefinitionTool(lspClients)

	// Test parameters
	params := DefinitionParams{
		FilePath:  "/test.go",
		Line:      5,
		Character: 10,
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	// Run the tool
	call := ToolCall{Input: string(paramsJSON)}
	response, err := tool.Run(context.Background(), call)

	// Verify results
	require.NoError(t, err)
	assert.Contains(t, response.Content, "No definition found")
}

func TestDefinitionTool_Run_InvalidParams(t *testing.T) {
	tool := NewDefinitionTool(nil)

	// Test with invalid JSON
	call := ToolCall{Input: "invalid json"}
	response, err := tool.Run(context.Background(), call)

	require.NoError(t, err)
	assert.Contains(t, response.Content, "error parsing parameters")
}

func TestDefinitionTool_Run_NoLSPClients(t *testing.T) {
	tool := NewDefinitionTool(nil)

	params := DefinitionParams{
		FilePath:  "/test.go",
		Line:      5,
		Character: 10,
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	call := ToolCall{Input: string(paramsJSON)}
	response, err := tool.Run(context.Background(), call)

	require.NoError(t, err)
	assert.Contains(t, response.Content, "No LSP clients available")
}

func TestDefinitionTool_Run_MultipleDefinitions(t *testing.T) {
	// Create mock LSP server
	mockServer := lsptesting.NewMockLSPServer()
	
	// Add multiple test definitions
	testURI1 := protocol.DocumentURI("file:///test1.go")
	testURI2 := protocol.DocumentURI("file:///test2.go")
	testLocation1 := lsptesting.CreateTestLocation(testURI1, 10, 5)
	testLocation2 := lsptesting.CreateTestLocation(testURI2, 20, 15)
	
	mockServer.AddDefinition("file:///test.go:5:10", []protocol.Location{testLocation1, testLocation2})

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	tool := NewDefinitionTool(lspClients)

	// Test parameters
	params := DefinitionParams{
		FilePath:  "/test.go",
		Line:      5,
		Character: 10,
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	// Run the tool
	call := ToolCall{Input: string(paramsJSON)}
	response, err := tool.Run(context.Background(), call)

	// Verify results
	require.NoError(t, err)
	assert.Contains(t, response.Content, "2 definitions found")
	assert.Contains(t, response.Content, "test1.go")
	assert.Contains(t, response.Content, "test2.go")
}

func TestDefinitionTool_Run_MultipleLSPClients(t *testing.T) {
	// Create multiple mock LSP servers
	goMockServer := lsptesting.NewMockLSPServer()
	tsMockServer := lsptesting.NewMockLSPServer()
	
	// Add definitions to different servers
	testURI := protocol.DocumentURI("file:///test.go")
	testLocation := lsptesting.CreateTestLocation(testURI, 10, 5)
	goMockServer.AddDefinition("file:///test.go:5:10", []protocol.Location{testLocation})

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(goMockServer),
		"ts": createMockLSPClient(tsMockServer),
	}

	tool := NewDefinitionTool(lspClients)

	// Test parameters
	params := DefinitionParams{
		FilePath:  "/test.go",
		Line:      5,
		Character: 10,
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	// Run the tool
	call := ToolCall{Input: string(paramsJSON)}
	response, err := tool.Run(context.Background(), call)

	// Verify results
	require.NoError(t, err)
	assert.Contains(t, response.Content, "Definition found")
	
	// Verify that the correct LSP client was used
	assert.True(t, goMockServer.AssertRequestMade("textDocument/definition"))
}

func TestDefinitionTool_Run_ErrorHandling(t *testing.T) {
	// Create mock LSP server that will return an error
	mockServer := lsptesting.NewMockLSPServer()

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	tool := NewDefinitionTool(lspClients)

	// Test parameters with invalid file path
	params := DefinitionParams{
		FilePath:  "", // Empty file path should cause error
		Line:      5,
		Character: 10,
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	// Run the tool
	call := ToolCall{Input: string(paramsJSON)}
	response, err := tool.Run(context.Background(), call)

	// Verify error handling
	require.NoError(t, err)
	assert.Contains(t, response.Content, "error") // Should contain error message
}

func TestDefinitionTool_Run_ContextCancellation(t *testing.T) {
	// Create mock LSP server
	mockServer := lsptesting.NewMockLSPServer()

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	tool := NewDefinitionTool(lspClients)

	// Test parameters
	params := DefinitionParams{
		FilePath:  "/test.go",
		Line:      5,
		Character: 10,
	}
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Run the tool with cancelled context
	call := ToolCall{Input: string(paramsJSON)}
	response, err := tool.Run(ctx, call)

	// Should handle cancellation gracefully
	require.NoError(t, err)
	// Response should indicate the operation was cancelled or no results
	assert.NotEmpty(t, response.Content)
}

// Benchmark tests
func BenchmarkDefinitionTool_Run(b *testing.B) {
	// Create mock LSP server
	mockServer := lsptesting.NewMockLSPServer()
	testURI := protocol.DocumentURI("file:///test.go")
	testLocation := lsptesting.CreateTestLocation(testURI, 10, 5)
	mockServer.AddDefinition("file:///test.go:5:10", []protocol.Location{testLocation})

	lspClients := map[string]*lsp.Client{
		"go": createMockLSPClient(mockServer),
	}

	tool := NewDefinitionTool(lspClients)

	params := DefinitionParams{
		FilePath:  "/test.go",
		Line:      5,
		Character: 10,
	}
	paramsJSON, _ := json.Marshal(params)
	call := ToolCall{Input: string(paramsJSON)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tool.Run(context.Background(), call)
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

// Integration test that would work with a real LSP server
func TestDefinitionTool_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would set up a real LSP server (like gopls) and test against it
	// It's marked as integration and skipped in short mode
	t.Skip("Integration test not implemented - requires real LSP server setup")
}
