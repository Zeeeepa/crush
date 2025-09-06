package context

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
)

// ContextEnhancer provides intelligent context enhancement for AI requests
type ContextEnhancer struct {
	lspClients map[string]*lsp.Client
	cache      *ContextCache
	mu         sync.RWMutex
}

// ContextRequest represents a request for context enhancement
type ContextRequest struct {
	FilePath string         `json:"file_path"`
	Line     int            `json:"line,omitempty"`
	Column   int            `json:"column,omitempty"`
	Options  ContextOptions `json:"options"`
}

// ContextOptions controls what types of context to include
type ContextOptions struct {
	IncludeHover         bool `json:"include_hover"`
	IncludeDefinition    bool `json:"include_definition"`
	IncludeReferences    bool `json:"include_references"`
	IncludeSymbols       bool `json:"include_symbols"`
	IncludeDiagnostics   bool `json:"include_diagnostics"`
	IncludeTypeContext   bool `json:"include_type_context"`
	IncludeErrorLists    bool `json:"include_error_lists"`
	MaxReferences        int  `json:"max_references"`
	MaxSymbols           int  `json:"max_symbols"`
}

// EnhancedContext contains all the enhanced context information
type EnhancedContext struct {
	FilePath        string                 `json:"file_path"`
	LSPContext      string                 `json:"lsp_context"`
	DiagnosticInfo  string                 `json:"diagnostic_info"`
	TypeContext     string                 `json:"type_context"`
	ErrorLists      map[string]string      `json:"error_lists"`
	Metadata        map[string]interface{} `json:"metadata"`
	GeneratedAt     time.Time              `json:"generated_at"`
	CacheHit        bool                   `json:"cache_hit"`
}

// NewContextEnhancer creates a new context enhancer
func NewContextEnhancer(lspClients map[string]*lsp.Client) *ContextEnhancer {
	return &ContextEnhancer{
		lspClients: lspClients,
		cache:      NewContextCache(),
	}
}

// EnhanceContext enriches a request with relevant LSP and diagnostic context
func (ce *ContextEnhancer) EnhanceContext(ctx context.Context, request ContextRequest) (*EnhancedContext, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	// Check cache first
	cacheKey := ce.generateCacheKey(request)
	if cached := ce.cache.Get(cacheKey); cached != nil {
		cached.CacheHit = true
		return cached, nil
	}

	// Create enhanced context
	enhanced := &EnhancedContext{
		FilePath:    request.FilePath,
		ErrorLists:  make(map[string]string),
		Metadata:    make(map[string]interface{}),
		GeneratedAt: time.Now(),
		CacheHit:    false,
	}

	// Find appropriate LSP client
	client := ce.findLSPClientForFile(request.FilePath)
	if client == nil {
		// No LSP client available, but we can still provide other context
		enhanced.LSPContext = "No LSP client available for this file type"
	} else {
		// Gather LSP context
		lspContext, err := ce.gatherLSPContext(ctx, client, request)
		if err != nil {
			enhanced.LSPContext = fmt.Sprintf("Error gathering LSP context: %v", err)
		} else {
			enhanced.LSPContext = lspContext
		}
	}

	// Gather diagnostic information if requested
	if request.Options.IncludeDiagnostics {
		diagnosticInfo := ce.gatherDiagnosticInfo(request.FilePath)
		enhanced.DiagnosticInfo = diagnosticInfo
	}

	// Gather type context if requested
	if request.Options.IncludeTypeContext {
		typeContext := ce.gatherTypeContext(request.FilePath)
		enhanced.TypeContext = typeContext
	}

	// Gather error lists if requested
	if request.Options.IncludeErrorLists {
		errorLists := ce.gatherErrorLists(request.FilePath)
		enhanced.ErrorLists = errorLists
	}

	// Add metadata
	enhanced.Metadata["lsp_clients"] = ce.getAvailableLSPClients()
	enhanced.Metadata["file_extension"] = filepath.Ext(request.FilePath)
	enhanced.Metadata["options"] = request.Options

	// Cache the result
	ce.cache.Set(cacheKey, enhanced)

	return enhanced, nil
}

// gatherLSPContext collects relevant LSP information based on the request
func (ce *ContextEnhancer) gatherLSPContext(ctx context.Context, client *lsp.Client, request ContextRequest) (string, error) {
	var contextParts []string

	// Convert to absolute path and URI
	absPath, err := filepath.Abs(request.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}
	uri := protocol.DocumentURI("file://" + absPath)

	// If we have position information, gather position-specific context
	if request.Line > 0 && request.Column >= 0 {
		position := protocol.Position{
			Line:      uint32(request.Line - 1), // LSP uses 0-based line numbers
			Character: uint32(request.Column),
		}

		// Gather hover information
		if request.Options.IncludeHover {
			hover, err := ce.getHoverInfo(ctx, client, uri, position)
			if err == nil && hover != "" {
				contextParts = append(contextParts, fmt.Sprintf("## Hover Information\n\n%s", hover))
			}
		}

		// Gather definition information
		if request.Options.IncludeDefinition {
			definition, err := ce.getDefinitionInfo(ctx, client, uri, position)
			if err == nil && definition != "" {
				contextParts = append(contextParts, fmt.Sprintf("## Definition Information\n\n%s", definition))
			}
		}

		// Gather references information
		if request.Options.IncludeReferences {
			references, err := ce.getReferencesInfo(ctx, client, uri, position, request.Options.MaxReferences)
			if err == nil && references != "" {
				contextParts = append(contextParts, fmt.Sprintf("## References Information\n\n%s", references))
			}
		}
	}

	// Gather symbol information for the file
	if request.Options.IncludeSymbols {
		symbols, err := ce.getSymbolInfo(ctx, client, uri, request.Options.MaxSymbols)
		if err == nil && symbols != "" {
			contextParts = append(contextParts, fmt.Sprintf("## Symbol Information\n\n%s", symbols))
		}
	}

	if len(contextParts) == 0 {
		return "No LSP context available", nil
	}

	return strings.Join(contextParts, "\n\n"), nil
}

// getHoverInfo retrieves hover information for a position
func (ce *ContextEnhancer) getHoverInfo(ctx context.Context, client *lsp.Client, uri protocol.DocumentURI, position protocol.Position) (string, error) {
	hoverParams := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
	}

	result, err := client.Hover(ctx, hoverParams)
	if err != nil {
		return "", err
	}

	if len(result.Contents.Value) == 0 {
		return "", nil
	}

	var parts []string
	for _, content := range result.Contents.Value {
		switch c := content.(type) {
		case protocol.MarkedString:
			if c.Language != "" {
				parts = append(parts, fmt.Sprintf("```%s\n%s\n```", c.Language, c.Value))
			} else {
				parts = append(parts, c.Value)
			}
		case protocol.MarkupContent:
			parts = append(parts, c.Value)
		case string:
			parts = append(parts, c)
		}
	}

	return strings.Join(parts, "\n"), nil
}

// getDefinitionInfo retrieves definition information for a position
func (ce *ContextEnhancer) getDefinitionInfo(ctx context.Context, client *lsp.Client, uri protocol.DocumentURI, position protocol.Position) (string, error) {
	definitionParams := protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
	}

	result, err := client.Definition(ctx, definitionParams)
	if err != nil {
		return "", err
	}

	locations := ce.extractLocationsFromDefinition(result)
	if len(locations) == 0 {
		return "", nil
	}

	var parts []string
	for _, location := range locations {
		filePath := strings.TrimPrefix(string(location.URI), "file://")
		parts = append(parts, fmt.Sprintf("- `%s` at line %d:%d",
			filePath,
			location.Range.Start.Line+1,
			location.Range.Start.Character))
	}

	return strings.Join(parts, "\n"), nil
}

// getReferencesInfo retrieves references information for a position
func (ce *ContextEnhancer) getReferencesInfo(ctx context.Context, client *lsp.Client, uri protocol.DocumentURI, position protocol.Position, maxReferences int) (string, error) {
	referencesParams := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
	}

	result, err := client.References(ctx, referencesParams)
	if err != nil {
		return "", err
	}

	if len(result) == 0 {
		return "", nil
	}

	// Limit results
	if maxReferences > 0 && len(result) > maxReferences {
		result = result[:maxReferences]
	}

	var parts []string
	for _, location := range result {
		filePath := strings.TrimPrefix(string(location.URI), "file://")
		parts = append(parts, fmt.Sprintf("- `%s` at line %d:%d",
			filePath,
			location.Range.Start.Line+1,
			location.Range.Start.Character))
	}

	return strings.Join(parts, "\n"), nil
}

// getSymbolInfo retrieves symbol information for a file
func (ce *ContextEnhancer) getSymbolInfo(ctx context.Context, client *lsp.Client, uri protocol.DocumentURI, maxSymbols int) (string, error) {
	symbolParams := protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	}

	result, err := client.DocumentSymbol(ctx, symbolParams)
	if err != nil {
		return "", err
	}

	symbols := ce.extractSymbolsFromResult(result)
	if len(symbols) == 0 {
		return "", nil
	}

	// Limit results
	if maxSymbols > 0 && len(symbols) > maxSymbols {
		symbols = symbols[:maxSymbols]
	}

	var parts []string
	for _, symbol := range symbols {
		parts = append(parts, fmt.Sprintf("- **%s** `%s` at line %d:%d",
			symbol.Name,
			ce.symbolKindToString(symbol.Kind),
			symbol.Range.Start.Line+1,
			symbol.Range.Start.Character))
	}

	return strings.Join(parts, "\n"), nil
}

// Helper methods
func (ce *ContextEnhancer) findLSPClientForFile(filePath string) *lsp.Client {
	ext := filepath.Ext(filePath)
	
	// Try to find a client that handles this file extension
	for _, client := range ce.lspClients {
		if ce.clientHandlesFileType(client, ext) {
			return client
		}
	}
	
	// If no specific client found, return the first available client
	for _, client := range ce.lspClients {
		return client
	}
	
	return nil
}

func (ce *ContextEnhancer) clientHandlesFileType(client *lsp.Client, fileExt string) bool {
	clientName := client.GetName()
	
	switch clientName {
	case "gopls", "go":
		return fileExt == ".go" || fileExt == ".mod"
	case "typescript-language-server", "tsserver", "ts":
		return fileExt == ".ts" || fileExt == ".tsx" || fileExt == ".js" || fileExt == ".jsx"
	case "rust-analyzer", "rust":
		return fileExt == ".rs"
	case "pylsp", "pyright", "python":
		return fileExt == ".py"
	case "clangd", "ccls", "c":
		return fileExt == ".c" || fileExt == ".cpp" || fileExt == ".cc" || fileExt == ".h" || fileExt == ".hpp"
	default:
		return true
	}
}

func (ce *ContextEnhancer) extractLocationsFromDefinition(result protocol.Or_Result_textDocument_definition) []protocol.Location {
	var locations []protocol.Location

	if result.Value == nil {
		return locations
	}

	switch v := result.Value.(type) {
	case protocol.Location:
		locations = append(locations, v)
	case []protocol.Location:
		locations = append(locations, v...)
	case []protocol.LocationLink:
		for _, link := range v {
			location := protocol.Location{
				URI:   link.TargetURI,
				Range: link.TargetRange,
			}
			locations = append(locations, location)
		}
	}

	return locations
}

func (ce *ContextEnhancer) extractSymbolsFromResult(result protocol.Or_Result_textDocument_documentSymbol) []protocol.DocumentSymbol {
	var symbols []protocol.DocumentSymbol

	if result.Value == nil {
		return symbols
	}

	switch v := result.Value.(type) {
	case []protocol.DocumentSymbol:
		symbols = append(symbols, v...)
	case []protocol.SymbolInformation:
		// Convert SymbolInformation to DocumentSymbol
		for _, info := range v {
			symbol := protocol.DocumentSymbol{
				Name:   info.Name,
				Kind:   info.Kind,
				Range:  info.Location.Range,
				SelectionRange: info.Location.Range,
			}
			symbols = append(symbols, symbol)
		}
	}

	return symbols
}

func (ce *ContextEnhancer) symbolKindToString(kind protocol.SymbolKind) string {
	switch kind {
	case protocol.SymbolKindFile:
		return "File"
	case protocol.SymbolKindModule:
		return "Module"
	case protocol.SymbolKindNamespace:
		return "Namespace"
	case protocol.SymbolKindPackage:
		return "Package"
	case protocol.SymbolKindClass:
		return "Class"
	case protocol.SymbolKindMethod:
		return "Method"
	case protocol.SymbolKindProperty:
		return "Property"
	case protocol.SymbolKindField:
		return "Field"
	case protocol.SymbolKindConstructor:
		return "Constructor"
	case protocol.SymbolKindEnum:
		return "Enum"
	case protocol.SymbolKindInterface:
		return "Interface"
	case protocol.SymbolKindFunction:
		return "Function"
	case protocol.SymbolKindVariable:
		return "Variable"
	case protocol.SymbolKindConstant:
		return "Constant"
	default:
		return fmt.Sprintf("Unknown(%d)", kind)
	}
}

func (ce *ContextEnhancer) gatherDiagnosticInfo(filePath string) string {
	// This will be implemented to gather diagnostic information
	// from LSP clients and other sources
	return "Diagnostic information gathering not yet implemented"
}

func (ce *ContextEnhancer) gatherTypeContext(filePath string) string {
	// This will be implemented to gather type context from TY project
	return "Type context gathering not yet implemented"
}

func (ce *ContextEnhancer) gatherErrorLists(filePath string) map[string]string {
	// This will be implemented to gather error lists from various tools
	return map[string]string{
		"ruff":  "Ruff error list gathering not yet implemented",
		"mypy":  "Mypy error list gathering not yet implemented",
		"biome": "Biome error list gathering not yet implemented",
	}
}

func (ce *ContextEnhancer) getAvailableLSPClients() []string {
	var clients []string
	for name := range ce.lspClients {
		clients = append(clients, name)
	}
	return clients
}

func (ce *ContextEnhancer) generateCacheKey(request ContextRequest) string {
	return fmt.Sprintf("%s:%d:%d:%+v", request.FilePath, request.Line, request.Column, request.Options)
}
