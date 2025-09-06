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

type HoverTool struct {
	lspClients map[string]*lsp.Client
}

type HoverParams struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

func NewHoverTool(lspClients map[string]*lsp.Client) BaseTool {
	return &HoverTool{
		lspClients: lspClients,
	}
}

func (h *HoverTool) Name() string {
	return "hover"
}

func (h *HoverTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "hover",
		Description: "Get hover information (documentation, type info, signatures) for a symbol at a specific position in a file using LSP. Provides rich context about symbols including documentation, type information, and function signatures.",
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

func (h *HoverTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params HoverParams
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
	if len(h.lspClients) == 0 {
		return NewTextResponse("No LSP clients available for hover information"), nil
	}

	// Find appropriate LSP client for this file
	client := h.findLSPClientForFile(params.FilePath)
	if client == nil {
		return NewTextResponse(fmt.Sprintf("No LSP client available for file type: %s", filepath.Ext(params.FilePath))), nil
	}

	// Convert to absolute path and URI
	absPath, err := filepath.Abs(params.FilePath)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to get absolute path: %v", err)), nil
	}
	
	uri := protocol.DocumentURI("file://" + absPath)

	// Create LSP hover request
	hoverParams := protocol.HoverParams{
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
	result, err := client.Hover(ctx, hoverParams)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("LSP hover request failed: %v", err)), nil
	}

	// Format response
	response := h.formatHoverResponse(result, params.FilePath, params.Line, params.Column)
	return NewTextResponse(response), nil
}

func (h *HoverTool) findLSPClientForFile(filePath string) *lsp.Client {
	ext := filepath.Ext(filePath)
	
	// Try to find a client that handles this file extension
	for _, client := range h.lspClients {
		if h.clientHandlesFileType(client, ext) {
			return client
		}
	}
	
	// If no specific client found, return the first available client
	// This allows for fallback behavior
	for _, client := range h.lspClients {
		return client
	}
	
	return nil
}

// clientHandlesFileType checks if an LSP client handles a specific file type
func (h *HoverTool) clientHandlesFileType(client *lsp.Client, fileExt string) bool {
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

func (h *HoverTool) formatHoverResponse(result protocol.Hover, originalFile string, line, column int) string {
	var response strings.Builder
	
	response.WriteString(fmt.Sprintf("## Hover Information for symbol at %s:%d:%d\n\n", originalFile, line, column))

	// Check if we have any hover content
	if len(result.Contents.Value) == 0 {
		response.WriteString("No hover information available for this symbol.\n")
		return response.String()
	}

	// Format the hover contents
	response.WriteString("### Symbol Information:\n\n")
	
	for i, content := range result.Contents.Value {
		if i > 0 {
			response.WriteString("\n---\n\n")
		}
		
		// Handle different content types
		switch c := content.(type) {
		case protocol.MarkedString:
			if c.Language != "" {
				// Code block with language
				response.WriteString(fmt.Sprintf("```%s\n%s\n```\n", c.Language, c.Value))
			} else {
				// Plain text
				response.WriteString(c.Value)
				response.WriteString("\n")
			}
		case protocol.MarkupContent:
			if c.Kind == protocol.MarkupKindMarkdown {
				response.WriteString(c.Value)
				response.WriteString("\n")
			} else {
				// Plain text
				response.WriteString(c.Value)
				response.WriteString("\n")
			}
		case string:
			// Plain string content
			response.WriteString(c)
			response.WriteString("\n")
		default:
			// Try to convert to string
			if str, ok := content.(string); ok {
				response.WriteString(str)
				response.WriteString("\n")
			} else {
				response.WriteString(fmt.Sprintf("Unknown content type: %T\n", content))
			}
		}
	}

	// Add range information if available
	if result.Range != nil {
		response.WriteString("\n### Range Information:\n\n")
		response.WriteString(fmt.Sprintf("**Start:** Line %d, Column %d\n", 
			result.Range.Start.Line+1, result.Range.Start.Character))
		response.WriteString(fmt.Sprintf("**End:** Line %d, Column %d\n", 
			result.Range.End.Line+1, result.Range.End.Character))
	}

	return response.String()
}
