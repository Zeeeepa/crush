package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/charmbracelet/crush/internal/lsp/protocol"
)

// MockLSPServer provides a mock LSP server for testing
type MockLSPServer struct {
	mu sync.RWMutex

	// Server capabilities
	capabilities protocol.ServerCapabilities

	// Mock data
	definitions  map[string][]protocol.Location
	references   map[string][]protocol.Location
	symbols      map[string][]protocol.WorkspaceSymbol
	hover        map[string]protocol.Hover
	completions  map[string][]protocol.CompletionItem
	diagnostics  map[string][]protocol.Diagnostic
	callHierarchy map[string][]protocol.CallHierarchyItem

	// Request tracking
	requests []MockRequest
}

// MockRequest tracks requests made to the mock server
type MockRequest struct {
	Method string
	Params interface{}
}

// NewMockLSPServer creates a new mock LSP server with default capabilities
func NewMockLSPServer() *MockLSPServer {
	return &MockLSPServer{
		capabilities: protocol.ServerCapabilities{
			DefinitionProvider:     true,
			ReferencesProvider:     true,
			HoverProvider:          true,
			CompletionProvider:     &protocol.CompletionOptions{},
			DocumentSymbolProvider: true,
			WorkspaceSymbolProvider: &protocol.WorkspaceSymbolOptions{
				ResolveProvider: true,
			},
			CallHierarchyProvider: true,
		},
		definitions:   make(map[string][]protocol.Location),
		references:    make(map[string][]protocol.Location),
		symbols:       make(map[string][]protocol.WorkspaceSymbol),
		hover:         make(map[string]protocol.Hover),
		completions:   make(map[string][]protocol.CompletionItem),
		diagnostics:   make(map[string][]protocol.Diagnostic),
		callHierarchy: make(map[string][]protocol.CallHierarchyItem),
		requests:      make([]MockRequest, 0),
	}
}

// SetCapabilities updates the server capabilities
func (m *MockLSPServer) SetCapabilities(caps protocol.ServerCapabilities) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.capabilities = caps
}

// GetCapabilities returns the server capabilities
func (m *MockLSPServer) GetCapabilities() protocol.ServerCapabilities {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.capabilities
}

// AddDefinition adds a mock definition for a symbol
func (m *MockLSPServer) AddDefinition(symbol string, locations []protocol.Location) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.definitions[symbol] = locations
}

// AddReferences adds mock references for a symbol
func (m *MockLSPServer) AddReferences(symbol string, locations []protocol.Location) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.references[symbol] = locations
}

// AddSymbol adds a mock workspace symbol
func (m *MockLSPServer) AddSymbol(query string, symbols []protocol.WorkspaceSymbol) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.symbols[query] = symbols
}

// AddHover adds mock hover information
func (m *MockLSPServer) AddHover(symbol string, hover protocol.Hover) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hover[symbol] = hover
}

// AddCompletion adds mock completion items
func (m *MockLSPServer) AddCompletion(prefix string, items []protocol.CompletionItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completions[prefix] = items
}

// AddDiagnostics adds mock diagnostics for a file
func (m *MockLSPServer) AddDiagnostics(uri string, diagnostics []protocol.Diagnostic) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.diagnostics[uri] = diagnostics
}

// AddCallHierarchy adds mock call hierarchy items
func (m *MockLSPServer) AddCallHierarchy(symbol string, items []protocol.CallHierarchyItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callHierarchy[symbol] = items
}

// GetRequests returns all requests made to the server
func (m *MockLSPServer) GetRequests() []MockRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]MockRequest(nil), m.requests...)
}

// ClearRequests clears the request history
func (m *MockLSPServer) ClearRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = m.requests[:0]
}

// trackRequest adds a request to the tracking list
func (m *MockLSPServer) trackRequest(method string, params interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, MockRequest{
		Method: method,
		Params: params,
	})
}

// MockDefinition handles textDocument/definition requests
func (m *MockLSPServer) MockDefinition(ctx context.Context, params protocol.DefinitionParams) (protocol.Or_Result_textDocument_definition, error) {
	m.trackRequest("textDocument/definition", params)

	key := fmt.Sprintf("%s:%d:%d", params.TextDocument.URI, params.Position.Line, params.Position.Character)
	
	m.mu.RLock()
	locations, exists := m.definitions[key]
	m.mu.RUnlock()

	if !exists {
		return protocol.Or_Result_textDocument_definition{}, nil
	}

	return protocol.Or_Result_textDocument_definition{Value: locations}, nil
}

// MockReferences handles textDocument/references requests
func (m *MockLSPServer) MockReferences(ctx context.Context, params protocol.ReferenceParams) ([]protocol.Location, error) {
	m.trackRequest("textDocument/references", params)

	key := fmt.Sprintf("%s:%d:%d", params.TextDocument.URI, params.Position.Line, params.Position.Character)
	
	m.mu.RLock()
	locations, exists := m.references[key]
	m.mu.RUnlock()

	if !exists {
		return []protocol.Location{}, nil
	}

	return locations, nil
}

// MockSymbol handles workspace/symbol requests
func (m *MockLSPServer) MockSymbol(ctx context.Context, params protocol.WorkspaceSymbolParams) (protocol.Or_Result_workspace_symbol, error) {
	m.trackRequest("workspace/symbol", params)

	m.mu.RLock()
	symbols, exists := m.symbols[params.Query]
	m.mu.RUnlock()

	if !exists {
		return protocol.Or_Result_workspace_symbol{}, nil
	}

	return protocol.Or_Result_workspace_symbol{Value: symbols}, nil
}

// MockHover handles textDocument/hover requests
func (m *MockLSPServer) MockHover(ctx context.Context, params protocol.HoverParams) (protocol.Hover, error) {
	m.trackRequest("textDocument/hover", params)

	key := fmt.Sprintf("%s:%d:%d", params.TextDocument.URI, params.Position.Line, params.Position.Character)
	
	m.mu.RLock()
	hover, exists := m.hover[key]
	m.mu.RUnlock()

	if !exists {
		return protocol.Hover{}, nil
	}

	return hover, nil
}

// MockCompletion handles textDocument/completion requests
func (m *MockLSPServer) MockCompletion(ctx context.Context, params protocol.CompletionParams) (protocol.Or_Result_textDocument_completion, error) {
	m.trackRequest("textDocument/completion", params)

	key := fmt.Sprintf("%s:%d:%d", params.TextDocument.URI, params.Position.Line, params.Position.Character)
	
	m.mu.RLock()
	items, exists := m.completions[key]
	m.mu.RUnlock()

	if !exists {
		return protocol.Or_Result_textDocument_completion{}, nil
	}

	return protocol.Or_Result_textDocument_completion{
		Value: protocol.CompletionList{
			IsIncomplete: false,
			Items:        items,
		},
	}, nil
}

// MockCallHierarchy handles textDocument/prepareCallHierarchy requests
func (m *MockLSPServer) MockCallHierarchy(ctx context.Context, params protocol.CallHierarchyPrepareParams) ([]protocol.CallHierarchyItem, error) {
	m.trackRequest("textDocument/prepareCallHierarchy", params)

	key := fmt.Sprintf("%s:%d:%d", params.TextDocument.URI, params.Position.Line, params.Position.Character)
	
	m.mu.RLock()
	items, exists := m.callHierarchy[key]
	m.mu.RUnlock()

	if !exists {
		return []protocol.CallHierarchyItem{}, nil
	}

	return items, nil
}

// CreateTestSymbol creates a test workspace symbol
func CreateTestSymbol(name, kind string, uri protocol.DocumentURI, line, character int) protocol.WorkspaceSymbol {
	return protocol.WorkspaceSymbol{
		Name: name,
		Kind: getSymbolKind(kind),
		Location: protocol.Or_WorkspaceSymbol_location{
			Value: protocol.Location{
				URI: uri,
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(line), Character: uint32(character)},
					End:   protocol.Position{Line: uint32(line), Character: uint32(character + len(name))},
				},
			},
		},
	}
}

// CreateTestLocation creates a test location
func CreateTestLocation(uri protocol.DocumentURI, line, character int) protocol.Location {
	return protocol.Location{
		URI: uri,
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(line), Character: uint32(character)},
			End:   protocol.Position{Line: uint32(line), Character: uint32(character + 10)},
		},
	}
}

// CreateTestHover creates test hover information
func CreateTestHover(content string) protocol.Hover {
	return protocol.Hover{
		Contents: protocol.Or_Hover_contents{
			Value: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: content,
			},
		},
	}
}

// CreateTestCompletion creates a test completion item
func CreateTestCompletion(label, detail, documentation string) protocol.CompletionItem {
	return protocol.CompletionItem{
		Label:  label,
		Detail: detail,
		Documentation: protocol.Or_CompletionItem_documentation{
			Value: documentation,
		},
		Kind: protocol.FunctionCompletion,
	}
}

// CreateTestDiagnostic creates a test diagnostic
func CreateTestDiagnostic(message string, line, character int, severity protocol.DiagnosticSeverity) protocol.Diagnostic {
	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(line), Character: uint32(character)},
			End:   protocol.Position{Line: uint32(line), Character: uint32(character + 10)},
		},
		Severity: severity,
		Message:  message,
		Source:   "test-lsp",
	}
}

// getSymbolKind converts string to SymbolKind
func getSymbolKind(kind string) protocol.SymbolKind {
	switch kind {
	case "function":
		return protocol.Function
	case "class":
		return protocol.Class
	case "variable":
		return protocol.Variable
	case "constant":
		return protocol.Constant
	case "interface":
		return protocol.Interface
	case "struct":
		return protocol.Struct
	default:
		return protocol.Function
	}
}

// AssertRequestMade checks if a specific request was made
func (m *MockLSPServer) AssertRequestMade(method string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, req := range m.requests {
		if req.Method == method {
			return true
		}
	}
	return false
}

// GetRequestCount returns the number of requests for a specific method
func (m *MockLSPServer) GetRequestCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	count := 0
	for _, req := range m.requests {
		if req.Method == method {
			count++
		}
	}
	return count
}
