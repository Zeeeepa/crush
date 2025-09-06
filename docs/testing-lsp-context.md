# Testing LSP Context Integration

This document describes the comprehensive testing strategy for Crush's LSP context integration features, following the validation-gates agent pattern.

## Overview

The LSP context integration testing framework ensures that all new LSP-powered features meet quality standards through:

- **Comprehensive Test Coverage**: Unit, integration, and performance tests
- **Automated Validation**: Continuous validation gates that must pass
- **Mock Testing Infrastructure**: Realistic LSP server simulation
- **Performance Benchmarking**: Ensuring LSP features don't degrade performance
- **Error Handling Validation**: Graceful degradation when LSP services fail

## Testing Architecture

### 1. Mock LSP Server (`internal/lsp/testing/mock_server.go`)

The mock LSP server provides realistic LSP responses for testing without requiring actual LSP servers:

```go
// Create mock server with test data
mockServer := lsptesting.NewMockLSPServer()

// Add test definitions
testLocation := lsptesting.CreateTestLocation(uri, 10, 5)
mockServer.AddDefinition("file:///test.go:5:10", []protocol.Location{testLocation})

// Add hover information
testHover := lsptesting.CreateTestHover("Function documentation")
mockServer.AddHover("file:///test.go:10:5", testHover)
```

**Features:**
- Full LSP protocol support (definitions, references, hover, symbols, etc.)
- Request tracking for verification
- Configurable capabilities
- Thread-safe operations
- Helper functions for creating test data

### 2. Tool Testing Pattern

Each LSP tool follows a consistent testing pattern:

```go
func TestDefinitionTool_Run_Success(t *testing.T) {
    // 1. Setup mock LSP server
    mockServer := lsptesting.NewMockLSPServer()
    mockServer.AddDefinition("key", testData)
    
    // 2. Create tool with mock clients
    tool := NewDefinitionTool(mockLSPClients)
    
    // 3. Execute tool
    response, err := tool.Run(context.Background(), call)
    
    // 4. Verify results
    require.NoError(t, err)
    assert.Contains(t, response.Content, "expected content")
}
```

**Test Categories:**
- **Success Cases**: Normal operation with valid data
- **Edge Cases**: Empty results, multiple results, large datasets
- **Error Handling**: Invalid parameters, LSP failures, timeouts
- **Performance**: Benchmarks and resource usage
- **Integration**: Real LSP server testing (when available)

### 3. Context Enhancement Testing

The context enhancement framework has specialized tests:

```go
func TestContextEnhancer_EnhanceContext_MultipleContextTypes(t *testing.T) {
    // Test combining hover, definitions, and references
    request := ContextRequest{
        Options: ContextOptions{
            IncludeHover:      true,
            IncludeDefinition: true,
            IncludeReferences: true,
        },
    }
    
    enhanced, err := enhancer.EnhanceContext(ctx, request)
    
    // Verify all context types are included
    assert.Contains(t, enhanced.LSPContext, "hover info")
    assert.Contains(t, enhanced.LSPContext, "definition")
    assert.Contains(t, enhanced.LSPContext, "references")
}
```

## Validation Gates

### Automated Validation Script (`scripts/validate.sh`)

The validation script implements comprehensive quality gates:

```bash
# Run full validation suite
./scripts/validate.sh

# Run specific validation
./scripts/validate.sh lsp      # LSP-specific tests
./scripts/validate.sh test     # All tests
./scripts/validate.sh security # Security scan
```

### Validation Checklist

Before any LSP feature is considered complete:

- [ ] **Unit Tests Pass**: All individual component tests pass
- [ ] **Integration Tests Pass**: Tests with mock LSP servers pass
- [ ] **Performance Benchmarks**: No significant performance degradation
- [ ] **Error Handling**: Graceful failure when LSP unavailable
- [ ] **Code Coverage**: Minimum 80% coverage for new code
- [ ] **Linting**: No linting errors or warnings
- [ ] **Security Scan**: No security vulnerabilities detected
- [ ] **Documentation**: Tests and features documented

### Quality Metrics

- **Test Coverage**: Target ≥80% for LSP-related code
- **Performance**: LSP operations should complete within reasonable time limits
- **Error Rate**: Graceful handling of LSP failures (no crashes)
- **Memory Usage**: No memory leaks in long-running operations

## Test Categories

### 1. Unit Tests

**Location**: `internal/llm/tools/*_test.go`, `internal/llm/context/*_test.go`

Test individual components in isolation:

```go
// Test tool parameter validation
func TestDefinitionTool_Run_InvalidParams(t *testing.T)

// Test LSP response parsing
func TestDefinitionTool_ParseResponse(t *testing.T)

// Test error handling
func TestDefinitionTool_Run_LSPError(t *testing.T)
```

### 2. Integration Tests

Test components working together with mock LSP servers:

```go
// Test multiple LSP clients
func TestDefinitionTool_Run_MultipleLSPClients(t *testing.T)

// Test context enhancement integration
func TestViewTool_WithContextEnhancement(t *testing.T)
```

### 3. Performance Tests

Benchmark critical paths to ensure acceptable performance:

```go
func BenchmarkDefinitionTool_Run(b *testing.B)
func BenchmarkContextEnhancer_EnhanceContext(b *testing.B)
func BenchmarkContextEnhancer_WithCaching(b *testing.B)
```

### 4. End-to-End Tests

Test complete workflows (marked as integration tests):

```go
func TestLSPWorkflow_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    // Test complete LSP workflow with real servers
}
```

## Running Tests

### Quick Test Run

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run LSP-specific tests only
go test ./internal/llm/tools/... -run "Test.*LSP"
```

### Using Task Runner

```bash
# Run tests with coverage report
task test:coverage

# Run LSP-specific tests
task test:lsp

# Run full validation suite
task validate
```

### Continuous Integration

The validation script integrates with CI/CD:

```yaml
# Example CI configuration
- name: Validate LSP Features
  run: |
    ./scripts/validate.sh
    # Uploads coverage reports and validation results
```

## Mock Data Creation

### Creating Test Symbols

```go
symbol := lsptesting.CreateTestSymbol(
    "TestFunction",           // name
    "function",              // kind
    protocol.DocumentURI("file:///test.go"), // uri
    10,                      // line
    5                        // character
)
```

### Creating Test Locations

```go
location := lsptesting.CreateTestLocation(
    protocol.DocumentURI("file:///test.go"),
    10,  // line
    5    // character
)
```

### Creating Test Hover Information

```go
hover := lsptesting.CreateTestHover(
    "## Function: testFunction\n\nReturns: void\n\nDocumentation here..."
)
```

## Error Handling Testing

### Testing LSP Failures

```go
func TestDefinitionTool_Run_LSPUnavailable(t *testing.T) {
    // Test with no LSP clients
    tool := NewDefinitionTool(nil)
    
    response, err := tool.Run(ctx, call)
    
    // Should handle gracefully
    require.NoError(t, err)
    assert.Contains(t, response.Content, "No LSP clients available")
}
```

### Testing Timeouts

```go
func TestDefinitionTool_Run_Timeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
    defer cancel()
    
    response, err := tool.Run(ctx, call)
    
    // Should handle timeout gracefully
    require.NoError(t, err)
}
```

## Performance Testing

### Benchmark Guidelines

- **Baseline Performance**: Establish performance baselines for all LSP operations
- **Regression Testing**: Ensure new features don't degrade existing performance
- **Memory Profiling**: Monitor memory usage, especially for caching
- **Concurrent Operations**: Test performance under concurrent LSP requests

### Example Benchmarks

```go
func BenchmarkDefinitionTool_ConcurrentRequests(b *testing.B) {
    tool := setupBenchmarkTool()
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, err := tool.Run(context.Background(), call)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}
```

## Test Data Management

### Consistent Test Data

Use helper functions to create consistent test data:

```go
// Standard test file
const testGoFile = "/test.go"
const testGoContent = `package main

func testFunction() {
    // Test function
}

func anotherFunction() {
    testFunction() // Reference
}`

// Standard test positions
var (
    testFunctionPos = Position{Line: 2, Character: 5}
    referencePos    = Position{Line: 6, Character: 4}
)
```

### Test File Organization

```
internal/
├── lsp/
│   ├── testing/
│   │   ├── mock_server.go      # Mock LSP server
│   │   ├── test_data.go        # Common test data
│   │   └── helpers.go          # Test helper functions
│   └── client_test.go          # LSP client tests
├── llm/
│   ├── tools/
│   │   ├── definition_test.go  # Definition tool tests
│   │   ├── references_test.go  # References tool tests
│   │   └── ...
│   └── context/
│       ├── enhancer_test.go    # Context enhancer tests
│       └── cache_test.go       # Cache tests
```

## Debugging Tests

### Verbose Test Output

```bash
# Run tests with verbose output
go test -v ./internal/llm/tools/...

# Run specific test with debugging
go test -v -run TestDefinitionTool_Run_Success ./internal/llm/tools/
```

### Test Debugging Tips

1. **Use `t.Logf()`** for debugging output in tests
2. **Check mock server requests** to verify LSP calls
3. **Use `testing.Short()`** to skip slow tests during development
4. **Enable debug logging** in LSP clients for integration tests

## Contributing Test Guidelines

### Writing New Tests

1. **Follow Naming Convention**: `TestComponentName_MethodName_Scenario`
2. **Use Table-Driven Tests** for multiple scenarios
3. **Include Benchmarks** for performance-critical code
4. **Test Error Conditions** as thoroughly as success conditions
5. **Use Mock Servers** instead of real LSP servers for unit tests

### Test Review Checklist

- [ ] Tests cover happy path, edge cases, and error conditions
- [ ] Mock data is realistic and comprehensive
- [ ] Performance benchmarks are included for new features
- [ ] Tests are deterministic (no flaky tests)
- [ ] Integration tests are properly marked and skippable
- [ ] Test names clearly describe what is being tested

## Troubleshooting

### Common Test Issues

1. **Flaky Tests**: Usually caused by timing issues or shared state
2. **Mock Setup**: Ensure mock data matches expected LSP responses
3. **Context Timeouts**: Use appropriate timeouts for test scenarios
4. **Resource Cleanup**: Ensure tests clean up resources properly

### Getting Help

- Check existing test patterns in similar tools
- Review mock server documentation
- Run validation script to identify specific issues
- Use benchmark comparisons to identify performance regressions

This testing framework ensures that Crush's LSP context integration is robust, performant, and reliable across all supported use cases.
