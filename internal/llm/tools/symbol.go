package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
)

type SymbolTool struct {
	lspClients map[string]*lsp.Client
}

type SymbolParams struct {
	Query    string `json:"query"`
	FileType string `json:"file_type,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

func NewSymbolTool(lspClients map[string]*lsp.Client) BaseTool {
	return &SymbolTool{
		lspClients: lspClients,
	}
}

func (s *SymbolTool) Name() string {
	return "symbol"
}

func (s *SymbolTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "symbol",
		Description: "Search for symbols (functions, classes, variables, types, etc.) across the workspace using LSP. Provides powerful symbol search capabilities to find definitions by name or pattern.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Symbol name or pattern to search for (supports partial matching)",
				},
				"file_type": map[string]any{
					"type":        "string",
					"description": "Optional file extension to limit search scope (e.g., '.go', '.ts', '.py')",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results to return (default: 50)",
				},
			},
			"required": []string{"query"},
		},
		Required: []string{"query"},
	}
}

func (s *SymbolTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params SymbolParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Invalid parameters: %v", err)), nil
	}

	// Validate parameters
	if params.Query == "" {
		return NewTextErrorResponse("query is required"), nil
	}

	// Set default limit
	if params.Limit <= 0 {
		params.Limit = 50
	}

	// Check if we have any LSP clients
	if len(s.lspClients) == 0 {
		return NewTextResponse("No LSP clients available for symbol search"), nil
	}

	// Find appropriate LSP clients for the search
	clients := s.findLSPClientsForSearch(params.FileType)
	if len(clients) == 0 {
		if params.FileType != "" {
			return NewTextResponse(fmt.Sprintf("No LSP client available for file type: %s", params.FileType)), nil
		} else {
			return NewTextResponse("No LSP clients available for symbol search"), nil
		}
	}

	// Collect results from all relevant clients
	var allResults []SymbolResult
	for clientName, client := range clients {
		results, err := s.searchSymbolsInClient(ctx, client, params.Query, clientName)
		if err != nil {
			// Log error but continue with other clients
			continue
		}
		allResults = append(allResults, results...)
	}

	// Limit results
	if len(allResults) > params.Limit {
		allResults = allResults[:params.Limit]
	}

	// Format response
	response := s.formatSymbolResponse(allResults, params.Query, params.FileType, params.Limit)
	return NewTextResponse(response), nil
}

type SymbolResult struct {
	Name         string
	Kind         string
	Location     protocol.Location
	ContainerName string
	ClientName   string
}

func (s *SymbolTool) findLSPClientsForSearch(fileType string) map[string]*lsp.Client {
	if fileType == "" {
		// Return all clients if no file type specified
		return s.lspClients
	}

	// Find clients that handle the specified file type
	result := make(map[string]*lsp.Client)
	for name, client := range s.lspClients {
		if s.clientHandlesFileType(client, fileType) {
			result[name] = client
		}
	}

	return result
}

func (s *SymbolTool) searchSymbolsInClient(ctx context.Context, client *lsp.Client, query, clientName string) ([]SymbolResult, error) {
	// Create LSP workspace symbol request
	symbolParams := protocol.WorkspaceSymbolParams{
		Query: query,
	}

	// Call LSP server
	result, err := client.Symbol(ctx, symbolParams)
	if err != nil {
		return nil, fmt.Errorf("LSP symbol request failed: %v", err)
	}

	// Convert results
	var symbols []SymbolResult
	
	// Handle different result types
	if result.Value == nil {
		return symbols, nil
	}

	switch v := result.Value.(type) {
	case []protocol.SymbolInformation:
		for _, symbol := range v {
			symbols = append(symbols, SymbolResult{
				Name:         symbol.Name,
				Kind:         s.symbolKindToString(symbol.Kind),
				Location:     symbol.Location,
				ContainerName: symbol.ContainerName,
				ClientName:   clientName,
			})
		}
	case []protocol.WorkspaceSymbol:
		for _, symbol := range v {
			location := protocol.Location{
				URI:   symbol.Location.URI,
				Range: symbol.Location.Range,
			}
			symbols = append(symbols, SymbolResult{
				Name:         symbol.Name,
				Kind:         s.symbolKindToString(symbol.Kind),
				Location:     location,
				ContainerName: symbol.ContainerName,
				ClientName:   clientName,
			})
		}
	case []interface{}:
		// Handle generic slice
		for _, item := range v {
			if symbol, ok := item.(protocol.SymbolInformation); ok {
				symbols = append(symbols, SymbolResult{
					Name:         symbol.Name,
					Kind:         s.symbolKindToString(symbol.Kind),
					Location:     symbol.Location,
					ContainerName: symbol.ContainerName,
					ClientName:   clientName,
				})
			} else if symbol, ok := item.(protocol.WorkspaceSymbol); ok {
				location := protocol.Location{
					URI:   symbol.Location.URI,
					Range: symbol.Location.Range,
				}
				symbols = append(symbols, SymbolResult{
					Name:         symbol.Name,
					Kind:         s.symbolKindToString(symbol.Kind),
					Location:     location,
					ContainerName: symbol.ContainerName,
					ClientName:   clientName,
				})
			}
		}
	}

	return symbols, nil
}

// clientHandlesFileType checks if an LSP client handles a specific file type
func (s *SymbolTool) clientHandlesFileType(client *lsp.Client, fileExt string) bool {
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
		// For unknown clients, assume they can handle any file type
		return true
	}
}

func (s *SymbolTool) symbolKindToString(kind protocol.SymbolKind) string {
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
	case protocol.SymbolKindString:
		return "String"
	case protocol.SymbolKindNumber:
		return "Number"
	case protocol.SymbolKindBoolean:
		return "Boolean"
	case protocol.SymbolKindArray:
		return "Array"
	case protocol.SymbolKindObject:
		return "Object"
	case protocol.SymbolKindKey:
		return "Key"
	case protocol.SymbolKindNull:
		return "Null"
	case protocol.SymbolKindEnumMember:
		return "EnumMember"
	case protocol.SymbolKindStruct:
		return "Struct"
	case protocol.SymbolKindEvent:
		return "Event"
	case protocol.SymbolKindOperator:
		return "Operator"
	case protocol.SymbolKindTypeParameter:
		return "TypeParameter"
	default:
		return fmt.Sprintf("Unknown(%d)", kind)
	}
}

func (s *SymbolTool) formatSymbolResponse(results []SymbolResult, query, fileType string, limit int) string {
	var response strings.Builder
	
	response.WriteString(fmt.Sprintf("## Symbol Search Results for '%s'\n\n", query))

	if fileType != "" {
		response.WriteString(fmt.Sprintf("**File Type Filter:** %s\n", fileType))
	}
	response.WriteString(fmt.Sprintf("**Limit:** %d\n\n", limit))

	if len(results) == 0 {
		response.WriteString("No symbols found matching the query.\n")
		return response.String()
	}

	response.WriteString(fmt.Sprintf("### Found %d symbol(s):\n\n", len(results)))

	// Group results by file for better organization
	fileGroups := make(map[string][]SymbolResult)
	for _, result := range results {
		filePath := strings.TrimPrefix(string(result.Location.URI), "file://")
		fileGroups[filePath] = append(fileGroups[filePath], result)
	}

	for filePath, symbols := range fileGroups {
		response.WriteString(fmt.Sprintf("#### `%s` (%d symbol(s))\n\n", filePath, len(symbols)))
		
		for _, symbol := range symbols {
			response.WriteString(fmt.Sprintf("- **%s** `%s`", symbol.Name, symbol.Kind))
			
			if symbol.ContainerName != "" {
				response.WriteString(fmt.Sprintf(" (in %s)", symbol.ContainerName))
			}
			
			response.WriteString(fmt.Sprintf(" - Line %d:%d", 
				symbol.Location.Range.Start.Line+1, // Convert to 1-based
				symbol.Location.Range.Start.Character))
			
			if symbol.ClientName != "" {
				response.WriteString(fmt.Sprintf(" [%s]", symbol.ClientName))
			}
			
			response.WriteString("\n")
		}
		
		response.WriteString("\n")
	}

	// Add summary
	response.WriteString("### Summary:\n\n")
	response.WriteString(fmt.Sprintf("- **Total Symbols:** %d\n", len(results)))
	response.WriteString(fmt.Sprintf("- **Files:** %d\n", len(fileGroups)))
	
	// Count by symbol kind
	kindCounts := make(map[string]int)
	for _, result := range results {
		kindCounts[result.Kind]++
	}
	
	response.WriteString("- **By Type:**\n")
	for kind, count := range kindCounts {
		response.WriteString(fmt.Sprintf("  - %s: %d\n", kind, count))
	}

	return response.String()
}
