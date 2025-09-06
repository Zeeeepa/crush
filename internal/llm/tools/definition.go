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

type DefinitionTool struct {
	lspClients map[string]*lsp.Client
}

type DefinitionParams struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

func NewDefinitionTool(lspClients map[string]*lsp.Client) BaseTool {
	return &DefinitionTool{
		lspClients: lspClients,
	}
}

func (d *DefinitionTool) Name() string {
	return "definition"
}

func (d *DefinitionTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "definition",
		Description: "Go to definition of a symbol at a specific position in a file using LSP. Provides the location where a symbol (function, variable, type, etc.) is defined.",
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
			},
			"required": []string{"file_path", "line", "column"},
		},
		Required: []string{"file_path", "line", "column"},
	}
}

func (d *DefinitionTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params DefinitionParams
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

	// Check if we have any LSP clients
	if len(d.lspClients) == 0 {
		return NewTextResponse("No LSP clients available for go-to-definition"), nil
	}

	// Find appropriate LSP client for this file
	client := d.findLSPClientForFile(params.FilePath)
	if client == nil {
		return NewTextResponse(fmt.Sprintf("No LSP client available for file type: %s", filepath.Ext(params.FilePath))), nil
	}

	// Convert to absolute path and URI
	absPath, err := filepath.Abs(params.FilePath)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to get absolute path: %v", err)), nil
	}
	
	uri := protocol.DocumentURI("file://" + absPath)

	// Create LSP definition request
	definitionParams := protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: uri,
			},
			Position: protocol.Position{
				Line:      uint32(params.Line - 1), // LSP uses 0-based line numbers
				Character: uint32(params.Column),
			},
		},
	}

	// Call LSP server
	result, err := client.Definition(ctx, definitionParams)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("LSP definition request failed: %v", err)), nil
	}

	// Format response
	response := d.formatDefinitionResponse(result, params.FilePath, params.Line, params.Column)
	return NewTextResponse(response), nil
}

func (d *DefinitionTool) findLSPClientForFile(filePath string) *lsp.Client {
	ext := filepath.Ext(filePath)
	
	// Try to find a client that handles this file extension
	for _, client := range d.lspClients {
		if d.clientHandlesFileType(client, ext) {
			return client
		}
	}
	
	// If no specific client found, return the first available client
	// This allows for fallback behavior
	for _, client := range d.lspClients {
		return client
	}
	
	return nil
}

// clientHandlesFileType checks if an LSP client handles a specific file type
// This is a temporary helper until we add this method to the LSP client
func (d *DefinitionTool) clientHandlesFileType(client *lsp.Client, fileExt string) bool {
	// For now, we'll use a simple mapping based on client names
	// This should be replaced with proper file type checking from the client
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

func (d *DefinitionTool) formatDefinitionResponse(result protocol.Or_Result_textDocument_definition, originalFile string, line, column int) string {
	var response strings.Builder
	
	response.WriteString(fmt.Sprintf("## Definition for symbol at %s:%d:%d\n\n", originalFile, line, column))

	// Handle different result types
	switch {
	case result.Value == nil:
		response.WriteString("No definition found for this symbol.\n")
		return response.String()
	}

	// Extract locations from the result
	locations := d.extractLocations(result)
	
	if len(locations) == 0 {
		response.WriteString("No definition found for this symbol.\n")
		return response.String()
	}

	if len(locations) == 1 {
		response.WriteString("### Definition Location:\n\n")
	} else {
		response.WriteString(fmt.Sprintf("### Definition Locations (%d found):\n\n", len(locations)))
	}

	for i, location := range locations {
		if len(locations) > 1 {
			response.WriteString(fmt.Sprintf("**%d.** ", i+1))
		}
		
		// Convert URI back to file path
		filePath := strings.TrimPrefix(string(location.URI), "file://")
		
		response.WriteString(fmt.Sprintf("**File:** `%s`\n", filePath))
		response.WriteString(fmt.Sprintf("**Position:** Line %d, Column %d\n", 
			location.Range.Start.Line+1, // Convert back to 1-based
			location.Range.Start.Character))
		
		// If there's a range, show it
		if location.Range.Start.Line != location.Range.End.Line || 
		   location.Range.Start.Character != location.Range.End.Character {
			response.WriteString(fmt.Sprintf("**Range:** Line %d:%d - %d:%d\n",
				location.Range.Start.Line+1, location.Range.Start.Character,
				location.Range.End.Line+1, location.Range.End.Character))
		}
		
		response.WriteString("\n")
	}

	return response.String()
}

func (d *DefinitionTool) extractLocations(result protocol.Or_Result_textDocument_definition) []protocol.Location {
	var locations []protocol.Location

	if result.Value == nil {
		return locations
	}

	// Handle the different possible result types
	// The result can be Location, []Location, or LocationLink[]
	switch v := result.Value.(type) {
	case protocol.Location:
		locations = append(locations, v)
	case []protocol.Location:
		locations = append(locations, v...)
	case []protocol.LocationLink:
		// Convert LocationLink to Location
		for _, link := range v {
			location := protocol.Location{
				URI:   link.TargetURI,
				Range: link.TargetRange,
			}
			locations = append(locations, location)
		}
	case []interface{}:
		// Handle generic slice - try to convert each element
		for _, item := range v {
			if loc, ok := item.(protocol.Location); ok {
				locations = append(locations, loc)
			} else if link, ok := item.(protocol.LocationLink); ok {
				location := protocol.Location{
					URI:   link.TargetURI,
					Range: link.TargetRange,
				}
				locations = append(locations, location)
			}
		}
	}

	return locations
}
