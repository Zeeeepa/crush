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

type CallHierarchyTool struct {
	lspClients map[string]*lsp.Client
}

type CallHierarchyParams struct {
	FilePath  string `json:"file_path"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Direction string `json:"direction"` // "incoming" or "outgoing"
}

func NewCallHierarchyTool(lspClients map[string]*lsp.Client) BaseTool {
	return &CallHierarchyTool{
		lspClients: lspClients,
	}
}

func (c *CallHierarchyTool) Name() string {
	return "call_hierarchy"
}

func (c *CallHierarchyTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "call_hierarchy",
		Description: "Show call hierarchy (incoming/outgoing calls) for a symbol at a specific position using LSP. Helps understand how functions are called and what they call.",
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
					"description": "Column number (1-based) where the symbol is located",
				},
				"direction": map[string]any{
					"type":        "string",
					"description": "Direction of call hierarchy: 'incoming' (who calls this) or 'outgoing' (what this calls)",
					"enum":        []string{"incoming", "outgoing"},
					"default":     "incoming",
				},
			},
			"required": []string{"file_path", "line", "column"},
		},
	}
}

func (c *CallHierarchyTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params CallHierarchyParams
	if err := json.Unmarshal(call.Input, &params); err != nil {
		return ToolResponse{}, fmt.Errorf("invalid parameters: %w", err)
	}

	if params.FilePath == "" {
		return ToolResponse{}, fmt.Errorf("file_path is required")
	}

	if params.Line <= 0 {
		return ToolResponse{}, fmt.Errorf("line must be positive")
	}

	if params.Column <= 0 {
		return ToolResponse{}, fmt.Errorf("column must be positive")
	}

	if params.Direction == "" {
		params.Direction = "incoming"
	}

	if params.Direction != "incoming" && params.Direction != "outgoing" {
		return ToolResponse{}, fmt.Errorf("direction must be 'incoming' or 'outgoing'")
	}

	client := c.findLSPClientForFile(params.FilePath)
	if client == nil {
		return ToolResponse{}, fmt.Errorf("no LSP client available for file: %s", params.FilePath)
	}

	// First, prepare call hierarchy items
	prepareParams := protocol.CallHierarchyPrepareParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI("file://" + params.FilePath),
			},
			Position: protocol.Position{
				Line:      uint32(params.Line - 1), // LSP uses 0-based indexing
				Character: uint32(params.Column - 1),
			},
		},
	}

	items, err := client.PrepareCallHierarchy(ctx, prepareParams)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to prepare call hierarchy: %w", err)
	}

	if len(items) == 0 {
		return ToolResponse{
			Content: fmt.Sprintf("No call hierarchy information available for symbol at %s:%d:%d", 
				params.FilePath, params.Line, params.Column),
		}, nil
	}

	// Get call hierarchy based on direction
	var result string
	if params.Direction == "incoming" {
		result, err = c.getIncomingCalls(ctx, client, items[0], params)
	} else {
		result, err = c.getOutgoingCalls(ctx, client, items[0], params)
	}

	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to get %s calls: %w", params.Direction, err)
	}

	return ToolResponse{Content: result}, nil
}

func (c *CallHierarchyTool) getIncomingCalls(ctx context.Context, client *lsp.Client, item protocol.CallHierarchyItem, params CallHierarchyParams) (string, error) {
	incomingParams := protocol.CallHierarchyIncomingCallsParams{
		Item: item,
	}

	calls, err := client.IncomingCalls(ctx, incomingParams)
	if err != nil {
		return "", err
	}

	if len(calls) == 0 {
		return fmt.Sprintf("No incoming calls found for symbol '%s' at %s:%d:%d", 
			item.Name, params.FilePath, params.Line, params.Column), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("📞 Incoming calls to '%s' (%d found):\n\n", item.Name, len(calls)))

	for i, call := range calls {
		caller := call.From
		filePath := strings.TrimPrefix(string(caller.URI), "file://")
		
		result.WriteString(fmt.Sprintf("%d. **%s** (%s)\n", i+1, caller.Name, caller.Kind))
		result.WriteString(fmt.Sprintf("   📍 %s:%d:%d\n", 
			filepath.Base(filePath), 
			caller.Range.Start.Line+1, 
			caller.Range.Start.Character+1))
		
		// Show call ranges if available
		if len(call.FromRanges) > 0 {
			result.WriteString("   📞 Call sites:\n")
			for _, callRange := range call.FromRanges {
				result.WriteString(fmt.Sprintf("      - Line %d:%d\n", 
					callRange.Start.Line+1, callRange.Start.Character+1))
			}
		}
		result.WriteString("\n")
	}

	return result.String(), nil
}

func (c *CallHierarchyTool) getOutgoingCalls(ctx context.Context, client *lsp.Client, item protocol.CallHierarchyItem, params CallHierarchyParams) (string, error) {
	outgoingParams := protocol.CallHierarchyOutgoingCallsParams{
		Item: item,
	}

	calls, err := client.OutgoingCalls(ctx, outgoingParams)
	if err != nil {
		return "", err
	}

	if len(calls) == 0 {
		return fmt.Sprintf("No outgoing calls found for symbol '%s' at %s:%d:%d", 
			item.Name, params.FilePath, params.Line, params.Column), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("📱 Outgoing calls from '%s' (%d found):\n\n", item.Name, len(calls)))

	for i, call := range calls {
		callee := call.To
		filePath := strings.TrimPrefix(string(callee.URI), "file://")
		
		result.WriteString(fmt.Sprintf("%d. **%s** (%s)\n", i+1, callee.Name, callee.Kind))
		result.WriteString(fmt.Sprintf("   📍 %s:%d:%d\n", 
			filepath.Base(filePath), 
			callee.Range.Start.Line+1, 
			callee.Range.Start.Character+1))
		
		// Show call ranges if available
		if len(call.FromRanges) > 0 {
			result.WriteString("   📞 Call sites:\n")
			for _, callRange := range call.FromRanges {
				result.WriteString(fmt.Sprintf("      - Line %d:%d\n", 
					callRange.Start.Line+1, callRange.Start.Character+1))
			}
		}
		result.WriteString("\n")
	}

	return result.String(), nil
}

func (c *CallHierarchyTool) findLSPClientForFile(filePath string) *lsp.Client {
	if filePath == "" {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	// Try to find a client that handles this file extension
	for _, client := range c.lspClients {
		if c.clientHandlesFileType(client, ext) {
			return client
		}
	}

	return nil
}

func (c *CallHierarchyTool) clientHandlesFileType(client *lsp.Client, fileExt string) bool {
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
