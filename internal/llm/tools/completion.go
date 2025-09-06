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

type CompletionTool struct {
	lspClients map[string]*lsp.Client
}

type CompletionParams struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Limit    int    `json:"limit,omitempty"`
}

func NewCompletionTool(lspClients map[string]*lsp.Client) BaseTool {
	return &CompletionTool{
		lspClients: lspClients,
	}
}

func (c *CompletionTool) Name() string {
	return "completion"
}

func (c *CompletionTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "completion",
		Description: "Get code completion suggestions at a specific position in a file using LSP. Provides intelligent autocomplete suggestions including functions, variables, types, and more based on context.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "Path to the file where completion is requested",
				},
				"line": map[string]any{
					"type":        "integer",
					"description": "Line number (1-based) where completion is requested",
				},
				"column": map[string]any{
					"type":        "integer",
					"description": "Column number (0-based) where completion is requested",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of completion items to return (default: 20)",
				},
			},
			"required": []string{"file_path", "line", "column"},
		},
		Required: []string{"file_path", "line", "column"},
	}
}

func (c *CompletionTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params CompletionParams
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

	// Set default limit
	if params.Limit <= 0 {
		params.Limit = 20
	}

	// Check if we have any LSP clients
	if len(c.lspClients) == 0 {
		return NewTextResponse("No LSP clients available for code completion"), nil
	}

	// Find appropriate LSP client for this file
	client := c.findLSPClientForFile(params.FilePath)
	if client == nil {
		return NewTextResponse(fmt.Sprintf("No LSP client available for file type: %s", filepath.Ext(params.FilePath))), nil
	}

	// Convert to absolute path and URI
	absPath, err := filepath.Abs(params.FilePath)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to get absolute path: %v", err)), nil
	}
	
	uri := protocol.DocumentURI("file://" + absPath)

	// Create LSP completion request
	completionParams := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: uri,
			},
			Position: protocol.Position{
				Line:      uint32(params.Line - 1), // LSP uses 0-based line numbers
				Character: uint32(params.Column),
			},
		},
		Context: &protocol.CompletionContext{
			TriggerKind: protocol.CompletionTriggerKindInvoked,
		},
	}

	// Call LSP server
	result, err := client.Completion(ctx, completionParams)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("LSP completion request failed: %v", err)), nil
	}

	// Format response
	response := c.formatCompletionResponse(result, params.FilePath, params.Line, params.Column, params.Limit)
	return NewTextResponse(response), nil
}

func (c *CompletionTool) findLSPClientForFile(filePath string) *lsp.Client {
	ext := filepath.Ext(filePath)
	
	// Try to find a client that handles this file extension
	for _, client := range c.lspClients {
		if c.clientHandlesFileType(client, ext) {
			return client
		}
	}
	
	// If no specific client found, return the first available client
	// This allows for fallback behavior
	for _, client := range c.lspClients {
		return client
	}
	
	return nil
}

// clientHandlesFileType checks if an LSP client handles a specific file type
func (c *CompletionTool) clientHandlesFileType(client *lsp.Client, fileExt string) bool {
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

func (c *CompletionTool) formatCompletionResponse(result protocol.Or_Result_textDocument_completion, originalFile string, line, column, limit int) string {
	var response strings.Builder
	
	response.WriteString(fmt.Sprintf("## Code Completion at %s:%d:%d\n\n", originalFile, line, column))

	// Extract completion items
	items := c.extractCompletionItems(result)
	
	if len(items) == 0 {
		response.WriteString("No completion suggestions available at this position.\n")
		return response.String()
	}

	// Limit results
	if len(items) > limit {
		items = items[:limit]
		response.WriteString(fmt.Sprintf("### Top %d completion suggestions (of %d total):\n\n", limit, len(items)))
	} else {
		response.WriteString(fmt.Sprintf("### %d completion suggestion(s):\n\n", len(items)))
	}

	// Group by completion kind for better organization
	kindGroups := make(map[string][]protocol.CompletionItem)
	for _, item := range items {
		kind := c.completionKindToString(item.Kind)
		kindGroups[kind] = append(kindGroups[kind], item)
	}

	// Display results grouped by kind
	for kind, groupItems := range kindGroups {
		if len(kindGroups) > 1 {
			response.WriteString(fmt.Sprintf("#### %s (%d)\n\n", kind, len(groupItems)))
		}
		
		for _, item := range groupItems {
			response.WriteString(fmt.Sprintf("- **%s**", item.Label))
			
			// Add kind if not already grouped
			if len(kindGroups) == 1 {
				response.WriteString(fmt.Sprintf(" `%s`", kind))
			}
			
			// Add detail if available
			if item.Detail != "" {
				response.WriteString(fmt.Sprintf(" - %s", item.Detail))
			}
			
			response.WriteString("\n")
			
			// Add documentation if available
			if item.Documentation != nil {
				doc := c.extractDocumentation(item.Documentation)
				if doc != "" {
					// Truncate long documentation
					if len(doc) > 100 {
						doc = doc[:97] + "..."
					}
					response.WriteString(fmt.Sprintf("  *%s*\n", doc))
				}
			}
		}
		
		response.WriteString("\n")
	}

	// Add summary
	response.WriteString("### Summary:\n\n")
	response.WriteString(fmt.Sprintf("- **Total Suggestions:** %d\n", len(items)))
	response.WriteString("- **By Type:**\n")
	for kind, groupItems := range kindGroups {
		response.WriteString(fmt.Sprintf("  - %s: %d\n", kind, len(groupItems)))
	}

	return response.String()
}

func (c *CompletionTool) extractCompletionItems(result protocol.Or_Result_textDocument_completion) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	if result.Value == nil {
		return items
	}

	switch v := result.Value.(type) {
	case []protocol.CompletionItem:
		items = v
	case protocol.CompletionList:
		items = v.Items
	case map[string]interface{}:
		// Handle generic map - try to extract items
		if itemsInterface, ok := v["items"]; ok {
			if itemsSlice, ok := itemsInterface.([]interface{}); ok {
				for _, item := range itemsSlice {
					if completionItem, ok := item.(protocol.CompletionItem); ok {
						items = append(items, completionItem)
					}
				}
			}
		}
	}

	return items
}

func (c *CompletionTool) completionKindToString(kind protocol.CompletionItemKind) string {
	switch kind {
	case protocol.CompletionItemKindText:
		return "Text"
	case protocol.CompletionItemKindMethod:
		return "Method"
	case protocol.CompletionItemKindFunction:
		return "Function"
	case protocol.CompletionItemKindConstructor:
		return "Constructor"
	case protocol.CompletionItemKindField:
		return "Field"
	case protocol.CompletionItemKindVariable:
		return "Variable"
	case protocol.CompletionItemKindClass:
		return "Class"
	case protocol.CompletionItemKindInterface:
		return "Interface"
	case protocol.CompletionItemKindModule:
		return "Module"
	case protocol.CompletionItemKindProperty:
		return "Property"
	case protocol.CompletionItemKindUnit:
		return "Unit"
	case protocol.CompletionItemKindValue:
		return "Value"
	case protocol.CompletionItemKindEnum:
		return "Enum"
	case protocol.CompletionItemKindKeyword:
		return "Keyword"
	case protocol.CompletionItemKindSnippet:
		return "Snippet"
	case protocol.CompletionItemKindColor:
		return "Color"
	case protocol.CompletionItemKindFile:
		return "File"
	case protocol.CompletionItemKindReference:
		return "Reference"
	case protocol.CompletionItemKindFolder:
		return "Folder"
	case protocol.CompletionItemKindEnumMember:
		return "EnumMember"
	case protocol.CompletionItemKindConstant:
		return "Constant"
	case protocol.CompletionItemKindStruct:
		return "Struct"
	case protocol.CompletionItemKindEvent:
		return "Event"
	case protocol.CompletionItemKindOperator:
		return "Operator"
	case protocol.CompletionItemKindTypeParameter:
		return "TypeParameter"
	default:
		return fmt.Sprintf("Unknown(%d)", kind)
	}
}

func (c *CompletionTool) extractDocumentation(doc interface{}) string {
	switch d := doc.(type) {
	case string:
		return d
	case protocol.MarkupContent:
		return d.Value
	case protocol.MarkedString:
		return d.Value
	default:
		return ""
	}
}
