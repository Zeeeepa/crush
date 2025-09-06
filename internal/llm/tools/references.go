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

type ReferencesTool struct {
	lspClients map[string]*lsp.Client
}

type ReferencesParams struct {
	FilePath           string `json:"file_path"`
	Line               int    `json:"line"`
	Column             int    `json:"column"`
	IncludeDeclaration bool   `json:"include_declaration,omitempty"`
}

func NewReferencesTool(lspClients map[string]*lsp.Client) BaseTool {
	return &ReferencesTool{
		lspClients: lspClients,
	}
}

func (r *ReferencesTool) Name() string {
	return "references"
}

func (r *ReferencesTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "references",
		Description: "Find all references to a symbol at a specific position in a file using LSP. Shows where a symbol (function, variable, type, etc.) is used throughout the codebase.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "Path to the file containing the symbol",
				},
				"line": map[string]any{
					"type":        "integer",
					"description": "Line number (1-based) where the symbol is located",
				},
				"column": map[string]any{
					"type":        "integer",
					"description": "Column number (0-based) where the symbol is located",
				},
				"include_declaration": map[string]any{
					"type":        "boolean",
					"description": "Whether to include the declaration/definition in the results (default: true)",
				},
			},
			"required": []string{"file_path", "line", "column"},
		},
		Required: []string{"file_path", "line", "column"},
	}
}

func (r *ReferencesTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params ReferencesParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Invalid parameters: %v", err)), nil
	}

	// Validate parameters
	if params.FilePath == "" {
		return NewTextErrorResponse("file_path is required"), nil
	}
	if params.Line < 1 {
		return NewTextErrorResponse("line must be >= 1"), nil
	}
	if params.Column < 0 {
		return NewTextErrorResponse("column must be >= 0"), nil
	}

	// Default to including declaration
	if params.IncludeDeclaration == false {
		params.IncludeDeclaration = true
	}

	// Check if we have any LSP clients
	if len(r.lspClients) == 0 {
		return NewTextResponse("No LSP clients available for finding references"), nil
	}

	// Find appropriate LSP client for this file
	client := r.findLSPClientForFile(params.FilePath)
	if client == nil {
		return NewTextResponse(fmt.Sprintf("No LSP client available for file type: %s", filepath.Ext(params.FilePath))), nil
	}

	// Convert to absolute path and URI
	absPath, err := filepath.Abs(params.FilePath)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to get absolute path: %v", err)), nil
	}
	
	uri := protocol.DocumentURI("file://" + absPath)

	// Create LSP references request
	referencesParams := protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: uri,
			},
			Position: protocol.Position{
				Line:      uint32(params.Line - 1), // LSP uses 0-based line numbers
				Character: uint32(params.Column),
			},
		},
		Context: protocol.ReferenceContext{
			IncludeDeclaration: params.IncludeDeclaration,
		},
	}

	// Call LSP server
	result, err := client.References(ctx, referencesParams)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("LSP references request failed: %v", err)), nil
	}

	// Format response
	response := r.formatReferencesResponse(result, params.FilePath, params.Line, params.Column, params.IncludeDeclaration)
	return NewTextResponse(response), nil
}

func (r *ReferencesTool) findLSPClientForFile(filePath string) *lsp.Client {
	ext := filepath.Ext(filePath)
	
	// Try to find a client that handles this file extension
	for _, client := range r.lspClients {
		if r.clientHandlesFileType(client, ext) {
			return client
		}
	}
	
	// If no specific client found, return the first available client
	// This allows for fallback behavior
	for _, client := range r.lspClients {
		return client
	}
	
	return nil
}

// clientHandlesFileType checks if an LSP client handles a specific file type
func (r *ReferencesTool) clientHandlesFileType(client *lsp.Client, fileExt string) bool {
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

func (r *ReferencesTool) formatReferencesResponse(result []protocol.Location, originalFile string, line, column int, includeDeclaration bool) string {
	var response strings.Builder
	
	response.WriteString(fmt.Sprintf("## References for symbol at %s:%d:%d\n\n", originalFile, line, column))

	if len(result) == 0 {
		response.WriteString("No references found for this symbol.\n")
		return response.String()
	}

	// Group references by file for better organization
	fileGroups := make(map[string][]protocol.Location)
	for _, location := range result {
		filePath := strings.TrimPrefix(string(location.URI), "file://")
		fileGroups[filePath] = append(fileGroups[filePath], location)
	}

	response.WriteString(fmt.Sprintf("### Found %d reference(s) in %d file(s):\n\n", len(result), len(fileGroups)))

	// Sort files for consistent output
	var sortedFiles []string
	for filePath := range fileGroups {
		sortedFiles = append(sortedFiles, filePath)
	}

	for _, filePath := range sortedFiles {
		locations := fileGroups[filePath]
		
		response.WriteString(fmt.Sprintf("#### `%s` (%d reference(s))\n\n", filePath, len(locations)))
		
		for _, location := range locations {
			response.WriteString(fmt.Sprintf("- **Line %d, Column %d**", 
				location.Range.Start.Line+1, // Convert back to 1-based
				location.Range.Start.Character))
			
			// If there's a range, show it
			if location.Range.Start.Line != location.Range.End.Line || 
			   location.Range.Start.Character != location.Range.End.Character {
				response.WriteString(fmt.Sprintf(" - %d:%d",
					location.Range.End.Line+1, location.Range.End.Character))
			}
			
			response.WriteString("\n")
		}
		
		response.WriteString("\n")
	}

	// Add summary information
	response.WriteString("### Summary:\n\n")
	response.WriteString(fmt.Sprintf("- **Total References:** %d\n", len(result)))
	response.WriteString(fmt.Sprintf("- **Files Affected:** %d\n", len(fileGroups)))
	response.WriteString(fmt.Sprintf("- **Include Declaration:** %t\n", includeDeclaration))

	return response.String()
}
