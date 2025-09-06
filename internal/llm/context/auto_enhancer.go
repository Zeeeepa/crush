package context

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
)

// AutoEnhancer automatically enhances AI requests with relevant LSP context
// This is the "Ferrari engine" that makes the AI dramatically smarter about code
type AutoEnhancer struct {
	lspClients map[string]*lsp.Client
	cache      *ContextCache
	mu         sync.RWMutex
}

// NewAutoEnhancer creates a new automatic context enhancer
func NewAutoEnhancer(lspClients map[string]*lsp.Client) *AutoEnhancer {
	return &AutoEnhancer{
		lspClients: lspClients,
		cache:      NewContextCache(5 * time.Minute), // 5 minute cache
	}
}

// EnhanceContent automatically enhances content with relevant LSP context
// This is called automatically by tools to make the AI smarter
func (ae *AutoEnhancer) EnhanceContent(ctx context.Context, content string, filePath string) string {
	if len(ae.lspClients) == 0 {
		return content
	}

	// Extract code symbols and positions from content
	symbols := ae.extractCodeSymbols(content, filePath)
	if len(symbols) == 0 {
		return content
	}

	// Get LSP context for the symbols
	lspContext := ae.gatherLSPContext(ctx, symbols, filePath)
	if lspContext == "" {
		return content
	}

	// Enhance the content with LSP context
	enhanced := fmt.Sprintf(`%s

## 🧠 AI Context Enhancement (LSP Intelligence)

%s

---

`, content, lspContext)

	return enhanced
}

// CodeSymbol represents a symbol found in code content
type CodeSymbol struct {
	Name     string
	Line     int
	Column   int
	Type     string // function, variable, type, etc.
	FilePath string
}

// extractCodeSymbols extracts potential code symbols from content
func (ae *AutoEnhancer) extractCodeSymbols(content string, filePath string) []CodeSymbol {
	var symbols []CodeSymbol

	// Pattern to match function calls, variable references, etc.
	patterns := map[string]*regexp.Regexp{
		"function": regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`),
		"variable": regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*[=:]`),
		"type":     regexp.MustCompile(`\btype\s+([a-zA-Z_][a-zA-Z0-9_]*)`),
		"import":   regexp.MustCompile(`import\s+.*["']([^"']+)["']`),
	}

	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		for symbolType, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) > 1 {
					symbols = append(symbols, CodeSymbol{
						Name:     match[1],
						Line:     lineNum + 1,
						Column:   strings.Index(line, match[1]),
						Type:     symbolType,
						FilePath: filePath,
					})
				}
			}
		}
	}

	return symbols
}

// gatherLSPContext gathers relevant LSP context for symbols
func (ae *AutoEnhancer) gatherLSPContext(ctx context.Context, symbols []CodeSymbol, filePath string) string {
	if filePath == "" {
		return ""
	}

	client := ae.findLSPClient(filePath)
	if client == nil {
		return ""
	}

	var contextParts []string

	// Get context for up to 5 most important symbols
	maxSymbols := 5
	if len(symbols) < maxSymbols {
		maxSymbols = len(symbols)
	}

	for i := 0; i < maxSymbols; i++ {
		symbol := symbols[i]
		symbolContext := ae.getSymbolContext(ctx, client, symbol)
		if symbolContext != "" {
			contextParts = append(contextParts, symbolContext)
		}
	}

	if len(contextParts) == 0 {
		return ""
	}

	return strings.Join(contextParts, "\n\n")
}

// getSymbolContext gets comprehensive context for a single symbol
func (ae *AutoEnhancer) getSymbolContext(ctx context.Context, client *lsp.Client, symbol CodeSymbol) string {
	uri := protocol.DocumentURI("file://" + symbol.FilePath)
	position := protocol.Position{
		Line:      uint32(symbol.Line - 1), // LSP is 0-based
		Character: uint32(symbol.Column),
	}

	var contextParts []string

	// Get hover information (documentation, type info)
	if hover := ae.getHoverInfo(ctx, client, uri, position); hover != "" {
		contextParts = append(contextParts, fmt.Sprintf("**%s** (%s):\n%s", symbol.Name, symbol.Type, hover))
	}

	// Get definition location
	if definition := ae.getDefinitionInfo(ctx, client, uri, position); definition != "" {
		contextParts = append(contextParts, fmt.Sprintf("Definition: %s", definition))
	}

	// Get references (limited to 3 for brevity)
	if references := ae.getReferencesInfo(ctx, client, uri, position, 3); references != "" {
		contextParts = append(contextParts, fmt.Sprintf("References: %s", references))
	}

	if len(contextParts) == 0 {
		return ""
	}

	return strings.Join(contextParts, "\n")
}

// getHoverInfo gets hover information for a position
func (ae *AutoEnhancer) getHoverInfo(ctx context.Context, client *lsp.Client, uri protocol.DocumentURI, position protocol.Position) string {
	params := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
	}

	result, err := client.Hover(ctx, params)
	if err != nil || result.Contents.Value == "" {
		return ""
	}

	// Clean up the hover content
	content := strings.TrimSpace(result.Contents.Value)
	if len(content) > 200 {
		content = content[:200] + "..."
	}

	return content
}

// getDefinitionInfo gets definition information for a position
func (ae *AutoEnhancer) getDefinitionInfo(ctx context.Context, client *lsp.Client, uri protocol.DocumentURI, position protocol.Position) string {
	params := protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
	}

	result, err := client.Definition(ctx, params)
	if err != nil {
		return ""
	}

	locations := ae.extractLocationsFromDefinition(result)
	if len(locations) == 0 {
		return ""
	}

	// Return the first definition location
	loc := locations[0]
	filePath := strings.TrimPrefix(string(loc.URI), "file://")
	return fmt.Sprintf("%s:%d:%d", filepath.Base(filePath), loc.Range.Start.Line+1, loc.Range.Start.Character+1)
}

// getReferencesInfo gets reference information for a position
func (ae *AutoEnhancer) getReferencesInfo(ctx context.Context, client *lsp.Client, uri protocol.DocumentURI, position protocol.Position, maxRefs int) string {
	params := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: false,
		},
	}

	result, err := client.References(ctx, params)
	if err != nil || len(result) == 0 {
		return ""
	}

	// Limit references
	if len(result) > maxRefs {
		result = result[:maxRefs]
	}

	var refStrings []string
	for _, ref := range result {
		filePath := strings.TrimPrefix(string(ref.URI), "file://")
		refStrings = append(refStrings, fmt.Sprintf("%s:%d", filepath.Base(filePath), ref.Range.Start.Line+1))
	}

	return strings.Join(refStrings, ", ")
}

// extractLocationsFromDefinition extracts locations from definition result
func (ae *AutoEnhancer) extractLocationsFromDefinition(result protocol.Or_Result_textDocument_definition) []protocol.Location {
	var locations []protocol.Location

	if result.Location != nil {
		locations = append(locations, *result.Location)
	}

	if result.LocationSlice != nil {
		locations = append(locations, *result.LocationSlice...)
	}

	return locations
}

// findLSPClient finds the appropriate LSP client for a file
func (ae *AutoEnhancer) findLSPClient(filePath string) *lsp.Client {
	if filePath == "" {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	// Try to find a client that handles this file extension
	for _, client := range ae.lspClients {
		if ae.clientHandlesFileType(client, ext) {
			return client
		}
	}

	return nil
}

// clientHandlesFileType checks if an LSP client handles a specific file type
func (ae *AutoEnhancer) clientHandlesFileType(client *lsp.Client, fileExt string) bool {
	// This is a simplified mapping - in a real implementation,
	// you'd check the client's capabilities
	switch fileExt {
	case ".go":
		return strings.Contains(strings.ToLower(client.String()), "go")
	case ".ts", ".js", ".tsx", ".jsx":
		return strings.Contains(strings.ToLower(client.String()), "typescript") ||
			strings.Contains(strings.ToLower(client.String()), "javascript")
	case ".py":
		return strings.Contains(strings.ToLower(client.String()), "python") ||
			strings.Contains(strings.ToLower(client.String()), "pylsp")
	case ".rs":
		return strings.Contains(strings.ToLower(client.String()), "rust")
	case ".c", ".cpp", ".h", ".hpp":
		return strings.Contains(strings.ToLower(client.String()), "clang") ||
			strings.Contains(strings.ToLower(client.String()), "ccls")
	}

	return false
}

// EnhanceToolContent enhances tool content with automatic LSP context
// This is the main entry point for making tools smarter
func (ae *AutoEnhancer) EnhanceToolContent(ctx context.Context, toolName string, content string, filePath string) string {
	// Only enhance for tools that work with code
	codeTools := map[string]bool{
		"view":      true,
		"edit":      true,
		"multi_edit": true,
		"write":     true,
		"grep":      true,
		"bash":      true, // When working with code files
	}

	if !codeTools[toolName] {
		return content
	}

	return ae.EnhanceContent(ctx, content, filePath)
}
