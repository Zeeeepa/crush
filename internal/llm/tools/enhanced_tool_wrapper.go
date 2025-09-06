package tools

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/llm/context"
)

// EnhancedToolWrapper wraps existing tools to automatically provide LSP context
// This is the middleware that makes ALL tools Ferrari-level smart
type EnhancedToolWrapper struct {
	BaseTool
	autoEnhancer *context.AutoEnhancer
}

// NewEnhancedToolWrapper creates a wrapper that automatically enhances tool responses
func NewEnhancedToolWrapper(tool BaseTool, autoEnhancer *context.AutoEnhancer) BaseTool {
	return &EnhancedToolWrapper{
		BaseTool:     tool,
		autoEnhancer: autoEnhancer,
	}
}

// Run executes the wrapped tool and automatically enhances the response with LSP context
func (etw *EnhancedToolWrapper) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	// Execute the original tool
	response, err := etw.BaseTool.Run(ctx, call)
	if err != nil {
		return response, err
	}

	// Only enhance tools that work with code files
	if !etw.shouldEnhance(etw.BaseTool.Name()) {
		return response, nil
	}

	// Extract file path from the tool call
	filePath := etw.extractFilePath(call)
	if filePath == "" {
		return response, nil
	}

	// Only enhance for code files
	if !etw.isCodeFile(filePath) {
		return response, nil
	}

	// Enhance the response with automatic LSP context
	if etw.autoEnhancer != nil {
		enhanced := etw.autoEnhancer.EnhanceToolContent(ctx, etw.BaseTool.Name(), response.Content, filePath)
		response.Content = enhanced
	}

	return response, nil
}

// shouldEnhance determines if a tool should be enhanced with LSP context
func (etw *EnhancedToolWrapper) shouldEnhance(toolName string) bool {
	enhanceableTools := map[string]bool{
		"view":       true,
		"edit":       true,
		"multi_edit": true,
		"write":      true,
		"grep":       true,
		"bash":       true, // When working with code files
	}

	return enhanceableTools[toolName]
}

// extractFilePath extracts the file path from a tool call
func (etw *EnhancedToolWrapper) extractFilePath(call ToolCall) string {
	// This is a simplified extraction - in practice, you'd parse the JSON
	// to get the file_path parameter for each tool type
	input := string(call.Input)
	
	// Look for common file path patterns in JSON
	patterns := []string{
		`"file_path":"([^"]+)"`,
		`"path":"([^"]+)"`,
		`"filepath":"([^"]+)"`,
	}

	for _, pattern := range patterns {
		if matches := extractFromPattern(input, pattern); matches != "" {
			return matches
		}
	}

	return ""
}

// isCodeFile checks if a file is a code file that would benefit from LSP context
func (etw *EnhancedToolWrapper) isCodeFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	codeExtensions := map[string]bool{
		".go":   true,
		".ts":   true,
		".js":   true,
		".tsx":  true,
		".jsx":  true,
		".py":   true,
		".rs":   true,
		".c":    true,
		".cpp":  true,
		".h":    true,
		".hpp":  true,
		".java": true,
		".cs":   true,
		".php":  true,
		".rb":   true,
		".swift": true,
		".kt":   true,
		".scala": true,
		".clj":  true,
		".hs":   true,
		".ml":   true,
		".fs":   true,
		".elm":  true,
		".dart": true,
		".lua":  true,
		".r":    true,
		".jl":   true,
		".nim":  true,
		".zig":  true,
		".v":    true,
	}

	return codeExtensions[ext]
}

// extractFromPattern is a helper to extract strings using regex-like patterns
func extractFromPattern(input, pattern string) string {
	// This is a simplified implementation
	// In practice, you'd use proper JSON parsing or regex
	
	// Look for the pattern and extract the value
	if strings.Contains(input, `"file_path":`) {
		start := strings.Index(input, `"file_path":"`)
		if start == -1 {
			return ""
		}
		start += len(`"file_path":"`)
		end := strings.Index(input[start:], `"`)
		if end == -1 {
			return ""
		}
		return input[start : start+end]
	}

	if strings.Contains(input, `"path":`) {
		start := strings.Index(input, `"path":"`)
		if start == -1 {
			return ""
		}
		start += len(`"path":"`)
		end := strings.Index(input[start:], `"`)
		if end == -1 {
			return ""
		}
		return input[start : start+end]
	}

	return ""
}
