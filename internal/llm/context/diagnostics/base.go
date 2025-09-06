package diagnostics

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DiagnosticSource represents a source of diagnostic information (ruff, mypy, biome, etc.)
type DiagnosticSource interface {
	// Name returns the name of the diagnostic source
	Name() string
	
	// IsAvailable checks if the diagnostic tool is available in the system
	IsAvailable(ctx context.Context) bool
	
	// GetDiagnostics retrieves diagnostic information for a file or directory
	GetDiagnostics(ctx context.Context, path string) (*DiagnosticResult, error)
	
	// GetErrorList retrieves a formatted error list for a file or directory
	GetErrorList(ctx context.Context, path string) (string, error)
	
	// SupportsFileType checks if this diagnostic source supports the given file type
	SupportsFileType(fileExt string) bool
}

// DiagnosticResult contains the results from a diagnostic source
type DiagnosticResult struct {
	Source      string                 `json:"source"`
	FilePath    string                 `json:"file_path"`
	Diagnostics []Diagnostic           `json:"diagnostics"`
	Summary     DiagnosticSummary      `json:"summary"`
	Metadata    map[string]interface{} `json:"metadata"`
	GeneratedAt time.Time              `json:"generated_at"`
}

// Diagnostic represents a single diagnostic issue
type Diagnostic struct {
	File        string            `json:"file"`
	Line        int               `json:"line"`
	Column      int               `json:"column"`
	EndLine     int               `json:"end_line,omitempty"`
	EndColumn   int               `json:"end_column,omitempty"`
	Severity    DiagnosticSeverity `json:"severity"`
	Code        string            `json:"code,omitempty"`
	Message     string            `json:"message"`
	Rule        string            `json:"rule,omitempty"`
	Category    string            `json:"category,omitempty"`
	Fixable     bool              `json:"fixable,omitempty"`
	Suggestion  string            `json:"suggestion,omitempty"`
}

// DiagnosticSeverity represents the severity level of a diagnostic
type DiagnosticSeverity string

const (
	SeverityError   DiagnosticSeverity = "error"
	SeverityWarning DiagnosticSeverity = "warning"
	SeverityInfo    DiagnosticSeverity = "info"
	SeverityHint    DiagnosticSeverity = "hint"
)

// DiagnosticSummary provides a summary of diagnostic results
type DiagnosticSummary struct {
	TotalIssues int `json:"total_issues"`
	Errors      int `json:"errors"`
	Warnings    int `json:"warnings"`
	Info        int `json:"info"`
	Hints       int `json:"hints"`
	Fixable     int `json:"fixable"`
}

// DiagnosticManager manages multiple diagnostic sources
type DiagnosticManager struct {
	sources []DiagnosticSource
}

// NewDiagnosticManager creates a new diagnostic manager
func NewDiagnosticManager() *DiagnosticManager {
	return &DiagnosticManager{
		sources: make([]DiagnosticSource, 0),
	}
}

// RegisterSource registers a new diagnostic source
func (dm *DiagnosticManager) RegisterSource(source DiagnosticSource) {
	dm.sources = append(dm.sources, source)
}

// GetAvailableSources returns all available diagnostic sources
func (dm *DiagnosticManager) GetAvailableSources(ctx context.Context) []DiagnosticSource {
	var available []DiagnosticSource
	for _, source := range dm.sources {
		if source.IsAvailable(ctx) {
			available = append(available, source)
		}
	}
	return available
}

// GetDiagnosticsForFile retrieves diagnostics from all applicable sources for a file
func (dm *DiagnosticManager) GetDiagnosticsForFile(ctx context.Context, filePath string) (map[string]*DiagnosticResult, error) {
	results := make(map[string]*DiagnosticResult)
	
	// Determine file extension
	fileExt := getFileExtension(filePath)
	
	// Get diagnostics from all applicable sources
	for _, source := range dm.sources {
		if source.IsAvailable(ctx) && source.SupportsFileType(fileExt) {
			result, err := source.GetDiagnostics(ctx, filePath)
			if err != nil {
				// Log error but continue with other sources
				continue
			}
			results[source.Name()] = result
		}
	}
	
	return results, nil
}

// GetErrorListsForFile retrieves formatted error lists from all applicable sources for a file
func (dm *DiagnosticManager) GetErrorListsForFile(ctx context.Context, filePath string) (map[string]string, error) {
	results := make(map[string]string)
	
	// Determine file extension
	fileExt := getFileExtension(filePath)
	
	// Get error lists from all applicable sources
	for _, source := range dm.sources {
		if source.IsAvailable(ctx) && source.SupportsFileType(fileExt) {
			errorList, err := source.GetErrorList(ctx, filePath)
			if err != nil {
				// Log error but continue with other sources
				results[source.Name()] = fmt.Sprintf("Error retrieving %s diagnostics: %v", source.Name(), err)
				continue
			}
			if errorList != "" {
				results[source.Name()] = errorList
			}
		}
	}
	
	return results, nil
}

// FormatDiagnosticResult formats a diagnostic result as a human-readable string
func FormatDiagnosticResult(result *DiagnosticResult) string {
	if result == nil || len(result.Diagnostics) == 0 {
		return fmt.Sprintf("No %s diagnostics found", result.Source)
	}
	
	var output strings.Builder
	
	// Header
	output.WriteString(fmt.Sprintf("## %s Diagnostics\n\n", strings.Title(result.Source)))
	
	// Summary
	summary := result.Summary
	output.WriteString(fmt.Sprintf("**Summary:** %d total issues", summary.TotalIssues))
	if summary.Errors > 0 {
		output.WriteString(fmt.Sprintf(" (%d errors", summary.Errors))
		if summary.Warnings > 0 || summary.Info > 0 || summary.Hints > 0 {
			output.WriteString(fmt.Sprintf(", %d warnings", summary.Warnings))
			if summary.Info > 0 || summary.Hints > 0 {
				output.WriteString(fmt.Sprintf(", %d info/hints", summary.Info+summary.Hints))
			}
		}
		output.WriteString(")")
	} else if summary.Warnings > 0 {
		output.WriteString(fmt.Sprintf(" (%d warnings", summary.Warnings))
		if summary.Info > 0 || summary.Hints > 0 {
			output.WriteString(fmt.Sprintf(", %d info/hints", summary.Info+summary.Hints))
		}
		output.WriteString(")")
	}
	
	if summary.Fixable > 0 {
		output.WriteString(fmt.Sprintf(" - %d fixable", summary.Fixable))
	}
	output.WriteString("\n\n")
	
	// Group diagnostics by file
	fileGroups := make(map[string][]Diagnostic)
	for _, diag := range result.Diagnostics {
		fileGroups[diag.File] = append(fileGroups[diag.File], diag)
	}
	
	// Display diagnostics by file
	for file, diagnostics := range fileGroups {
		output.WriteString(fmt.Sprintf("### `%s` (%d issue(s))\n\n", file, len(diagnostics)))
		
		for _, diag := range diagnostics {
			// Format severity
			severityIcon := getSeverityIcon(diag.Severity)
			
			output.WriteString(fmt.Sprintf("- %s **Line %d:%d** ", severityIcon, diag.Line, diag.Column))
			
			if diag.Code != "" {
				output.WriteString(fmt.Sprintf("`%s` ", diag.Code))
			}
			
			output.WriteString(diag.Message)
			
			if diag.Rule != "" {
				output.WriteString(fmt.Sprintf(" (%s)", diag.Rule))
			}
			
			if diag.Fixable {
				output.WriteString(" *[fixable]*")
			}
			
			output.WriteString("\n")
			
			if diag.Suggestion != "" {
				output.WriteString(fmt.Sprintf("  *Suggestion: %s*\n", diag.Suggestion))
			}
		}
		
		output.WriteString("\n")
	}
	
	return output.String()
}

// Helper functions
func getFileExtension(filePath string) string {
	parts := strings.Split(filePath, ".")
	if len(parts) > 1 {
		return "." + parts[len(parts)-1]
	}
	return ""
}

func getSeverityIcon(severity DiagnosticSeverity) string {
	switch severity {
	case SeverityError:
		return "❌"
	case SeverityWarning:
		return "⚠️"
	case SeverityInfo:
		return "ℹ️"
	case SeverityHint:
		return "💡"
	default:
		return "•"
	}
}


