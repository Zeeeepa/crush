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
	Direction string `json:"direction,omitempty"`
	Depth     int    `json:"depth,omitempty"`
}

func NewCallHierarchyTool(lspClients map[string]*lsp.Client) BaseTool {
	return &CallHierarchyTool{
		lspClients: lspClients,
	}
}

func (ch *CallHierarchyTool) Name() string {
	return "call_hierarchy"
}

func (ch *CallHierarchyTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "call_hierarchy",
		Description: "Analyze call hierarchy for a function/method at a specific position using LSP. Shows incoming calls (who calls this function) and outgoing calls (what this function calls).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "Path to the file containing the function/method",
				},
				"line": map[string]any{
					"type":        "integer",
					"description": "Line number (1-based) where the function/method is located",
				},
				"column": map[string]any{
					"type":        "integer",
					"description": "Column number (0-based) where the function/method is located",
				},
				"direction": map[string]any{
					"type":        "string",
					"description": "Direction of call hierarchy: 'incoming' (who calls this), 'outgoing' (what this calls), or 'both' (default: 'both')",
					"enum":        []string{"incoming", "outgoing", "both"},
				},
				"depth": map[string]any{
					"type":        "integer",
					"description": "Maximum depth to traverse in the call hierarchy (default: 2)",
				},
			},
			"required": []string{"file_path", "line", "column"},
		},
		Required: []string{"file_path", "line", "column"},
	}
}

func (ch *CallHierarchyTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params CallHierarchyParams
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

	// Set defaults
	if params.Direction == "" {
		params.Direction = "both"
	}
	if params.Depth <= 0 {
		params.Depth = 2
	}

	// Validate direction
	if params.Direction != "incoming" && params.Direction != "outgoing" && params.Direction != "both" {
		return NewTextErrorResponse("direction must be 'incoming', 'outgoing', or 'both'"), nil
	}

	// Check if we have any LSP clients
	if len(ch.lspClients) == 0 {
		return NewTextResponse("No LSP clients available for call hierarchy analysis"), nil
	}

	// Find appropriate LSP client for this file
	client := ch.findLSPClientForFile(params.FilePath)
	if client == nil {
		return NewTextResponse(fmt.Sprintf("No LSP client available for file type: %s", filepath.Ext(params.FilePath))), nil
	}

	// Convert to absolute path and URI
	absPath, err := filepath.Abs(params.FilePath)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to get absolute path: %v", err)), nil
	}
	
	uri := protocol.DocumentURI("file://" + absPath)

	// First, prepare call hierarchy items
	prepareParams := protocol.CallHierarchyPrepareParams{
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

	items, err := client.PrepareCallHierarchy(ctx, prepareParams)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("LSP prepare call hierarchy request failed: %v", err)), nil
	}

	if len(items) == 0 {
		return NewTextResponse("No call hierarchy information available for this position (not a function/method)"), nil
	}

	// Analyze call hierarchy for each item
	var results []CallHierarchyResult
	for _, item := range items {
		result := CallHierarchyResult{
			Item:      item,
			Incoming:  []CallHierarchyCall{},
			Outgoing:  []CallHierarchyCall{},
		}

		// Get incoming calls if requested
		if params.Direction == "incoming" || params.Direction == "both" {
			incoming, err := ch.getIncomingCalls(ctx, client, item, params.Depth)
			if err == nil {
				result.Incoming = incoming
			}
		}

		// Get outgoing calls if requested
		if params.Direction == "outgoing" || params.Direction == "both" {
			outgoing, err := ch.getOutgoingCalls(ctx, client, item, params.Depth)
			if err == nil {
				result.Outgoing = outgoing
			}
		}

		results = append(results, result)
	}

	// Format response
	response := ch.formatCallHierarchyResponse(results, params.FilePath, params.Line, params.Column, params.Direction, params.Depth)
	return NewTextResponse(response), nil
}

type CallHierarchyResult struct {
	Item     protocol.CallHierarchyItem
	Incoming []CallHierarchyCall
	Outgoing []CallHierarchyCall
}

type CallHierarchyCall struct {
	Item  protocol.CallHierarchyItem
	Calls []protocol.Range
	Depth int
}

func (ch *CallHierarchyTool) getIncomingCalls(ctx context.Context, client *lsp.Client, item protocol.CallHierarchyItem, maxDepth int) ([]CallHierarchyCall, error) {
	var allCalls []CallHierarchyCall
	
	// Get direct incoming calls
	incomingParams := protocol.CallHierarchyIncomingCallsParams{
		Item: item,
	}
	
	incoming, err := client.IncomingCalls(ctx, incomingParams)
	if err != nil {
		return nil, err
	}
	
	for _, call := range incoming {
		hierarchyCall := CallHierarchyCall{
			Item:  call.From,
			Calls: call.FromRanges,
			Depth: 1,
		}
		allCalls = append(allCalls, hierarchyCall)
		
		// Recursively get calls if we haven't reached max depth
		if maxDepth > 1 {
			nestedCalls, err := ch.getIncomingCalls(ctx, client, call.From, maxDepth-1)
			if err == nil {
				// Increase depth for nested calls
				for i := range nestedCalls {
					nestedCalls[i].Depth++
				}
				allCalls = append(allCalls, nestedCalls...)
			}
		}
	}
	
	return allCalls, nil
}

func (ch *CallHierarchyTool) getOutgoingCalls(ctx context.Context, client *lsp.Client, item protocol.CallHierarchyItem, maxDepth int) ([]CallHierarchyCall, error) {
	var allCalls []CallHierarchyCall
	
	// Get direct outgoing calls
	outgoingParams := protocol.CallHierarchyOutgoingCallsParams{
		Item: item,
	}
	
	outgoing, err := client.OutgoingCalls(ctx, outgoingParams)
	if err != nil {
		return nil, err
	}
	
	for _, call := range outgoing {
		hierarchyCall := CallHierarchyCall{
			Item:  call.To,
			Calls: call.FromRanges,
			Depth: 1,
		}
		allCalls = append(allCalls, hierarchyCall)
		
		// Recursively get calls if we haven't reached max depth
		if maxDepth > 1 {
			nestedCalls, err := ch.getOutgoingCalls(ctx, client, call.To, maxDepth-1)
			if err == nil {
				// Increase depth for nested calls
				for i := range nestedCalls {
					nestedCalls[i].Depth++
				}
				allCalls = append(allCalls, nestedCalls...)
			}
		}
	}
	
	return allCalls, nil
}

func (ch *CallHierarchyTool) findLSPClientForFile(filePath string) *lsp.Client {
	ext := filepath.Ext(filePath)
	
	// Try to find a client that handles this file extension
	for _, client := range ch.lspClients {
		if ch.clientHandlesFileType(client, ext) {
			return client
		}
	}
	
	// If no specific client found, return the first available client
	// This allows for fallback behavior
	for _, client := range ch.lspClients {
		return client
	}
	
	return nil
}

// clientHandlesFileType checks if an LSP client handles a specific file type
func (ch *CallHierarchyTool) clientHandlesFileType(client *lsp.Client, fileExt string) bool {
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

func (ch *CallHierarchyTool) formatCallHierarchyResponse(results []CallHierarchyResult, originalFile string, line, column int, direction string, depth int) string {
	var response strings.Builder
	
	response.WriteString(fmt.Sprintf("## Call Hierarchy for %s:%d:%d\n\n", originalFile, line, column))
	response.WriteString(fmt.Sprintf("**Direction:** %s | **Max Depth:** %d\n\n", direction, depth))

	if len(results) == 0 {
		response.WriteString("No call hierarchy information available.\n")
		return response.String()
	}

	for i, result := range results {
		if len(results) > 1 {
			response.WriteString(fmt.Sprintf("### Function %d: %s\n\n", i+1, result.Item.Name))
		} else {
			response.WriteString(fmt.Sprintf("### Function: %s\n\n", result.Item.Name))
		}

		// Show function details
		filePath := strings.TrimPrefix(string(result.Item.URI), "file://")
		response.WriteString(fmt.Sprintf("**Location:** `%s` - Line %d:%d\n", 
			filePath, 
			result.Item.Range.Start.Line+1, 
			result.Item.Range.Start.Character))
		
		if result.Item.Detail != "" {
			response.WriteString(fmt.Sprintf("**Details:** %s\n", result.Item.Detail))
		}
		
		response.WriteString("\n")

		// Show incoming calls
		if direction == "incoming" || direction == "both" {
			response.WriteString("#### Incoming Calls (Who calls this function):\n\n")
			if len(result.Incoming) == 0 {
				response.WriteString("No incoming calls found.\n\n")
			} else {
				ch.formatCallList(&response, result.Incoming, "incoming")
			}
		}

		// Show outgoing calls
		if direction == "outgoing" || direction == "both" {
			response.WriteString("#### Outgoing Calls (What this function calls):\n\n")
			if len(result.Outgoing) == 0 {
				response.WriteString("No outgoing calls found.\n\n")
			} else {
				ch.formatCallList(&response, result.Outgoing, "outgoing")
			}
		}
	}

	// Add summary
	response.WriteString("### Summary:\n\n")
	totalIncoming := 0
	totalOutgoing := 0
	for _, result := range results {
		totalIncoming += len(result.Incoming)
		totalOutgoing += len(result.Outgoing)
	}
	
	if direction == "incoming" || direction == "both" {
		response.WriteString(fmt.Sprintf("- **Total Incoming Calls:** %d\n", totalIncoming))
	}
	if direction == "outgoing" || direction == "both" {
		response.WriteString(fmt.Sprintf("- **Total Outgoing Calls:** %d\n", totalOutgoing))
	}

	return response.String()
}

func (ch *CallHierarchyTool) formatCallList(response *strings.Builder, calls []CallHierarchyCall, direction string) {
	// Group by depth for better visualization
	depthGroups := make(map[int][]CallHierarchyCall)
	for _, call := range calls {
		depthGroups[call.Depth] = append(depthGroups[call.Depth], call)
	}

	// Display calls by depth
	for depth := 1; depth <= len(depthGroups); depth++ {
		if groupCalls, exists := depthGroups[depth]; exists {
			indent := strings.Repeat("  ", depth-1)
			
			for _, call := range groupCalls {
				filePath := strings.TrimPrefix(string(call.Item.URI), "file://")
				response.WriteString(fmt.Sprintf("%s- **%s** in `%s` - Line %d:%d\n",
					indent,
					call.Item.Name,
					filePath,
					call.Item.Range.Start.Line+1,
					call.Item.Range.Start.Character))
				
				// Show call locations if available
				if len(call.Calls) > 1 {
					response.WriteString(fmt.Sprintf("%s  *Called from %d location(s)*\n", indent, len(call.Calls)))
				}
			}
		}
	}
	
	response.WriteString("\n")
}
