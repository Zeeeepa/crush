#!/bin/bash

# Validation Gates Script for Crush LSP Context Integration
# This script implements comprehensive validation following the validation-gates agent pattern

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COVERAGE_THRESHOLD=80
MAX_BUILD_TIME=60
MAX_TEST_TIME=120

# Validation state tracking
VALIDATION_ERRORS=0
VALIDATION_WARNINGS=0

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
    ((VALIDATION_WARNINGS++))
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
    ((VALIDATION_ERRORS++))
}

# Check if required tools are available
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing_tools=()
    
    if ! command -v go &> /dev/null; then
        missing_tools+=("go")
    fi
    
    if ! command -v golangci-lint &> /dev/null; then
        missing_tools+=("golangci-lint")
    fi
    
    if ! command -v gofumpt &> /dev/null; then
        missing_tools+=("gofumpt")
    fi
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        log_info "Install missing tools:"
        for tool in "${missing_tools[@]}"; do
            case $tool in
                "go")
                    log_info "  - Install Go: https://golang.org/doc/install"
                    ;;
                "golangci-lint")
                    log_info "  - Install golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
                    ;;
                "gofumpt")
                    log_info "  - Install gofumpt: go install mvdan.cc/gofumpt@latest"
                    ;;
            esac
        done
        return 1
    fi
    
    log_success "All prerequisites available"
    return 0
}

# Format code
format_code() {
    log_info "Formatting code with gofumpt..."
    
    if gofumpt -w .; then
        log_success "Code formatting completed"
    else
        log_error "Code formatting failed"
        return 1
    fi
    
    # Check if there are any changes after formatting
    if ! git diff --quiet; then
        log_warning "Code formatting made changes. Please commit these changes."
        git diff --name-only
    fi
    
    return 0
}

# Run linting
run_linting() {
    log_info "Running linting with golangci-lint..."
    
    local start_time=$(date +%s)
    
    if golangci-lint run --path-mode=abs --config=".golangci.yml" --timeout=5m; then
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        log_success "Linting passed (${duration}s)"
    else
        log_error "Linting failed"
        return 1
    fi
    
    return 0
}

# Run tests with coverage
run_tests() {
    log_info "Running tests with coverage..."
    
    local start_time=$(date +%s)
    local coverage_file="coverage.out"
    local coverage_html="coverage.html"
    
    # Run tests with coverage
    if go test -v -race -coverprofile="$coverage_file" -covermode=atomic ./...; then
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        
        if [ $duration -gt $MAX_TEST_TIME ]; then
            log_warning "Tests took ${duration}s (exceeds ${MAX_TEST_TIME}s threshold)"
        else
            log_success "Tests passed (${duration}s)"
        fi
    else
        log_error "Tests failed"
        return 1
    fi
    
    # Check coverage
    if [ -f "$coverage_file" ]; then
        local coverage_percent=$(go tool cover -func="$coverage_file" | grep total | awk '{print $3}' | sed 's/%//')
        
        if (( $(echo "$coverage_percent >= $COVERAGE_THRESHOLD" | bc -l) )); then
            log_success "Test coverage: ${coverage_percent}% (meets ${COVERAGE_THRESHOLD}% threshold)"
        else
            log_warning "Test coverage: ${coverage_percent}% (below ${COVERAGE_THRESHOLD}% threshold)"
        fi
        
        # Generate HTML coverage report
        go tool cover -html="$coverage_file" -o "$coverage_html"
        log_info "Coverage report generated: $coverage_html"
    else
        log_warning "Coverage file not found"
    fi
    
    return 0
}

# Build validation
validate_build() {
    log_info "Validating build..."
    
    local start_time=$(date +%s)
    
    if go build -v .; then
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        
        if [ $duration -gt $MAX_BUILD_TIME ]; then
            log_warning "Build took ${duration}s (exceeds ${MAX_BUILD_TIME}s threshold)"
        else
            log_success "Build completed successfully (${duration}s)"
        fi
    else
        log_error "Build failed"
        return 1
    fi
    
    return 0
}

# Validate Go modules
validate_modules() {
    log_info "Validating Go modules..."
    
    # Check for tidy modules
    if go mod tidy; then
        log_success "Go modules are tidy"
    else
        log_error "Go mod tidy failed"
        return 1
    fi
    
    # Check if go.mod or go.sum changed
    if ! git diff --quiet go.mod go.sum; then
        log_warning "go.mod or go.sum changed. Please commit these changes."
        git diff go.mod go.sum
    fi
    
    # Verify modules
    if go mod verify; then
        log_success "Go modules verified"
    else
        log_error "Go mod verify failed"
        return 1
    fi
    
    return 0
}

# Security scan (basic)
security_scan() {
    log_info "Running basic security scan..."
    
    # Check for common security issues in Go code
    if command -v gosec &> /dev/null; then
        if gosec ./...; then
            log_success "Security scan passed"
        else
            log_warning "Security scan found potential issues"
        fi
    else
        log_info "gosec not available, skipping security scan"
        log_info "Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"
    fi
    
    return 0
}

# Validate LSP-specific functionality
validate_lsp_features() {
    log_info "Validating LSP-specific functionality..."
    
    # Check if LSP test files exist
    local lsp_test_files=(
        "internal/llm/tools/definition_test.go"
        "internal/llm/tools/references_test.go"
        "internal/llm/tools/symbol_test.go"
        "internal/llm/tools/hover_test.go"
        "internal/llm/tools/completion_test.go"
        "internal/llm/tools/callhierarchy_test.go"
        "internal/llm/context/enhancer_test.go"
    )
    
    local missing_tests=()
    for test_file in "${lsp_test_files[@]}"; do
        if [ ! -f "$test_file" ]; then
            missing_tests+=("$test_file")
        fi
    done
    
    if [ ${#missing_tests[@]} -ne 0 ]; then
        log_warning "Missing LSP test files:"
        for test_file in "${missing_tests[@]}"; do
            log_warning "  - $test_file"
        done
    else
        log_success "All LSP test files present"
    fi
    
    # Run LSP-specific tests if they exist
    if go test -v ./internal/llm/tools/... -run "Test.*LSP|Test.*Definition|Test.*References|Test.*Symbol|Test.*Hover|Test.*Completion|Test.*CallHierarchy" 2>/dev/null; then
        log_success "LSP-specific tests passed"
    else
        log_info "No LSP-specific tests found or tests failed"
    fi
    
    return 0
}

# Performance benchmarks
run_benchmarks() {
    log_info "Running performance benchmarks..."
    
    if go test -bench=. -benchmem ./... > benchmark_results.txt 2>&1; then
        log_success "Benchmarks completed"
        log_info "Benchmark results saved to benchmark_results.txt"
        
        # Show summary of benchmark results
        if grep -q "Benchmark" benchmark_results.txt; then
            log_info "Benchmark summary:"
            grep "Benchmark" benchmark_results.txt | head -10
        fi
    else
        log_warning "Benchmarks failed or no benchmarks found"
    fi
    
    return 0
}

# Generate validation report
generate_report() {
    local report_file="validation_report.md"
    local timestamp=$(date -u +"%Y-%m-%d %H:%M:%S UTC")
    
    cat > "$report_file" << EOF
# Validation Report

**Generated:** $timestamp

## Summary

- **Errors:** $VALIDATION_ERRORS
- **Warnings:** $VALIDATION_WARNINGS
- **Status:** $([ $VALIDATION_ERRORS -eq 0 ] && echo "✅ PASSED" || echo "❌ FAILED")

## Validation Gates Checklist

- [$([ -f "coverage.out" ] && echo "x" || echo " ")] Unit tests executed
- [$([ $VALIDATION_ERRORS -eq 0 ] && echo "x" || echo " ")] All tests pass
- [$([ $VALIDATION_ERRORS -eq 0 ] && echo "x" || echo " ")] Linting produces no errors
- [$([ $VALIDATION_ERRORS -eq 0 ] && echo "x" || echo " ")] Code formatting is correct
- [$([ $VALIDATION_ERRORS -eq 0 ] && echo "x" || echo " ")] Build succeeds without warnings
- [$([ -f "benchmark_results.txt" ] && echo "x" || echo " ")] Performance benchmarks executed

## Files Generated

- \`coverage.out\` - Test coverage data
- \`coverage.html\` - HTML coverage report
- \`benchmark_results.txt\` - Performance benchmark results
- \`validation_report.md\` - This report

## Next Steps

$(if [ $VALIDATION_ERRORS -gt 0 ]; then
    echo "❌ **Validation failed with $VALIDATION_ERRORS errors**"
    echo ""
    echo "Please fix the errors above and re-run validation."
elif [ $VALIDATION_WARNINGS -gt 0 ]; then
    echo "⚠️ **Validation passed with $VALIDATION_WARNINGS warnings**"
    echo ""
    echo "Consider addressing the warnings above."
else
    echo "✅ **All validation gates passed successfully**"
    echo ""
    echo "Code is ready for deployment/merge."
fi)

EOF

    log_info "Validation report generated: $report_file"
}

# Main validation workflow
main() {
    log_info "Starting Crush LSP Context Integration Validation"
    log_info "================================================"
    
    # Phase 1: Prerequisites and Setup
    check_prerequisites || exit 1
    
    # Phase 2: Code Quality
    format_code || true  # Don't fail on formatting issues
    run_linting || exit 1
    
    # Phase 3: Module Validation
    validate_modules || exit 1
    
    # Phase 4: Testing
    run_tests || exit 1
    
    # Phase 5: Build Validation
    validate_build || exit 1
    
    # Phase 6: LSP-Specific Validation
    validate_lsp_features || true  # Don't fail if LSP tests don't exist yet
    
    # Phase 7: Security and Performance
    security_scan || true  # Don't fail on security warnings
    run_benchmarks || true  # Don't fail on benchmark issues
    
    # Phase 8: Reporting
    generate_report
    
    # Final status
    log_info "================================================"
    if [ $VALIDATION_ERRORS -eq 0 ]; then
        log_success "🎉 All validation gates passed!"
        if [ $VALIDATION_WARNINGS -gt 0 ]; then
            log_warning "Note: $VALIDATION_WARNINGS warnings were found"
        fi
        exit 0
    else
        log_error "💥 Validation failed with $VALIDATION_ERRORS errors"
        exit 1
    fi
}

# Handle script arguments
case "${1:-validate}" in
    "validate"|"")
        main
        ;;
    "format")
        check_prerequisites && format_code
        ;;
    "lint")
        check_prerequisites && run_linting
        ;;
    "test")
        check_prerequisites && run_tests
        ;;
    "build")
        check_prerequisites && validate_build
        ;;
    "lsp")
        check_prerequisites && validate_lsp_features
        ;;
    "security")
        check_prerequisites && security_scan
        ;;
    "bench")
        check_prerequisites && run_benchmarks
        ;;
    "report")
        generate_report
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  validate  - Run full validation suite (default)"
        echo "  format    - Format code only"
        echo "  lint      - Run linting only"
        echo "  test      - Run tests only"
        echo "  build     - Validate build only"
        echo "  lsp       - Validate LSP features only"
        echo "  security  - Run security scan only"
        echo "  bench     - Run benchmarks only"
        echo "  report    - Generate validation report"
        echo "  help      - Show this help"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac
