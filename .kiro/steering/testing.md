# Testing Standards

## Test File Location

Tests live alongside source files: `*_test.go`

## Patterns

### Table-Driven Tests

```go
func TestFunction(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"input1", "expected1"},
        {"input2", "expected2"},
    }

    for _, tt := range tests {
        result := Function(tt.input)
        if result != tt.expected {
            t.Errorf("Function(%q) = %q, want %q", tt.input, result, tt.expected)
        }
    }
}
```

### Temporary Directories

Use `t.TempDir()` for test isolation:
```go
func TestWithFiles(t *testing.T) {
    tmpDir := t.TempDir()
    // tmpDir is automatically cleaned up
}
```

### Docker Availability

Skip tests requiring Docker when unavailable:
```go
func TestSandbox(t *testing.T) {
    if !isDockerAvailable() {
        t.Skip("Docker not available")
    }
    // ...
}

func isDockerAvailable() bool {
    cmd := exec.Command("docker", "version")
    return cmd.Run() == nil
}
```

## Test Categories

- **Unit tests**: Mock dependencies, test single functions
- **Integration tests**: Test component interactions (e.g., sandbox tests)

## Running Tests

```bash
go test ./...                    # All tests
go test ./internal/health/...    # Specific package
go test -v ./...                 # Verbose output
go test -run TestName ./...      # Specific test
```

## Assertions

Use standard `t.Errorf` with descriptive messages:
```go
if got != want {
    t.Errorf("Function() = %v, want %v", got, want)
}
```

For fatal errors that should stop the test:
```go
if err != nil {
    t.Fatalf("Setup failed: %v", err)
}
```
