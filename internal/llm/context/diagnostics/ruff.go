package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// RuffSource implements DiagnosticSource for Ruff Python linter
type RuffSource struct{}

// NewRuffSource creates a new Ruff diagnostic source
func NewRuffSource() DiagnosticSource {
	return &RuffSource{}
}

func (r *RuffSource) Name() string {
	return "ruff"
}

func (r *RuffSource) IsAvailable(ctx context.Context) bool {
	_, err := exec.LookPath("ruff")
	return err == nil
}

func (r *RuffSource) SupportsFileType(fileExt string) bool {
	return fileExt == ".py" || fileExt == ".pyi"
}

func (r *RuffSource) GetDiagnostics(ctx context.Context, path string) (*DiagnosticResult, error) {
	if !r.IsAvailable(ctx) {
		return nil, fmt.Errorf("ruff is not available")
	}

	// Run ruff check with JSON output
	cmd := exec.CommandContext(ctx, "ruff", "check", "--output-format=json", path)
	output, err := cmd.Output()
	if err != nil {
		// Ruff returns non-zero exit code when issues are found, which is expected
		if exitError, ok := err.(*exec.ExitError); ok {
			output = exitError.Stderr
			if len(output) == 0 {
				// Try to get stdout if stderr is empty
				output, _ = cmd.Output()
			}
		} else {
			return nil, fmt.Errorf("failed to run ruff: %v", err)
		}
	}

	// Parse JSON output
	var ruffIssues []RuffIssue
	if len(output) > 0 {
		if err := json.Unmarshal(output, &ruffIssues); err != nil {
			return nil, fmt.Errorf("failed to parse ruff output: %v", err)
		}
	}

	// Convert to our diagnostic format
	diagnostics := make([]Diagnostic, 0, len(ruffIssues))
	summary := DiagnosticSummary{}

	for _, issue := range ruffIssues {
		severity := r.mapSeverity(issue.Type)
		
		diagnostic := Diagnostic{
			File:      issue.Filename,
			Line:      issue.Location.Row,
			Column:    issue.Location.Column,
			EndLine:   issue.EndLocation.Row,
			EndColumn: issue.EndLocation.Column,
			Severity:  severity,
			Code:      issue.Code,
			Message:   issue.Message,
			Rule:      issue.Code,
			Category:  "style",
			Fixable:   issue.Fix != nil,
		}

		if issue.Fix != nil && issue.Fix.Message != "" {
			diagnostic.Suggestion = issue.Fix.Message
		}

		diagnostics = append(diagnostics, diagnostic)

		// Update summary
		summary.TotalIssues++
		switch severity {
		case SeverityError:
			summary.Errors++
		case SeverityWarning:
			summary.Warnings++
		case SeverityInfo:
			summary.Info++
		case SeverityHint:
			summary.Hints++
		}
		
		if diagnostic.Fixable {
			summary.Fixable++
		}
	}

	result := &DiagnosticResult{
		Source:      r.Name(),
		FilePath:    path,
		Diagnostics: diagnostics,
		Summary:     summary,
		Metadata: map[string]interface{}{
			"ruff_version": r.getRuffVersion(ctx),
			"total_files":  1,
		},
		GeneratedAt: time.Now(),
	}

	return result, nil
}

func (r *RuffSource) GetErrorList(ctx context.Context, path string) (string, error) {
	result, err := r.GetDiagnostics(ctx, path)
	if err != nil {
		return "", err
	}

	return FormatDiagnosticResult(result), nil
}

// mapSeverity maps Ruff issue types to our severity levels
func (r *RuffSource) mapSeverity(ruffType string) DiagnosticSeverity {
	switch strings.ToLower(ruffType) {
	case "error", "e":
		return SeverityError
	case "warning", "w":
		return SeverityWarning
	case "info", "i":
		return SeverityInfo
	default:
		return SeverityWarning // Default to warning for unknown types
	}
}

// getRuffVersion gets the version of ruff for metadata
func (r *RuffSource) getRuffVersion(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "ruff", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// RuffIssue represents a single issue from Ruff JSON output
type RuffIssue struct {
	Code        string      `json:"code"`
	Message     string      `json:"message"`
	Type        string      `json:"type"`
	Filename    string      `json:"filename"`
	Location    RuffLocation `json:"location"`
	EndLocation RuffLocation `json:"end_location"`
	Fix         *RuffFix    `json:"fix,omitempty"`
	URL         string      `json:"url,omitempty"`
}

// RuffLocation represents a location in Ruff output
type RuffLocation struct {
	Row    int `json:"row"`
	Column int `json:"column"`
}

// RuffFix represents a fix suggestion from Ruff
type RuffFix struct {
	Message string `json:"message"`
	Edits   []RuffEdit `json:"edits"`
}

// RuffEdit represents an edit in a Ruff fix
type RuffEdit struct {
	Content   string       `json:"content"`
	Location  RuffLocation `json:"location"`
	EndLocation RuffLocation `json:"end_location"`
}
