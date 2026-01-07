# Technology Stack & Conventions

## Language & Runtime

- Go 1.23+
- Module: `budgie`

## Dependencies

- `github.com/modelcontextprotocol/go-sdk/mcp` - MCP protocol implementation
- `github.com/google/uuid` - Session ID generation
- `gopkg.in/yaml.v3` - Frontmatter parsing
- Docker (optional, for sandbox mode)

## Go Conventions

### Error Handling

- Return errors, don't panic
- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`
- Check errors immediately after function calls

### Naming

- Packages: lowercase, single word (`agents`, `health`, `kiro`)
- Interfaces: verb-er suffix when appropriate (`Manager`, `Executor`)
- Exported functions: PascalCase
- Unexported: camelCase

### Package Organization

- One responsibility per package
- Keep `internal/` packages focused and small
- Public API in package root, helpers unexported

### Structs

- Group related fields
- Use struct tags for JSON/YAML: `json:"field,omitempty"`
- Prefer composition over inheritance

### Concurrency

- Use `sync.Mutex` or `sync.RWMutex` for shared state
- Lock/unlock in same function when possible
- Use `defer` for unlock

## CLI Flags

Use `flag` package with sensible defaults:
```go
flag.String("name", defaultValue, "description")
```

## Model Configuration

- Default model: `claude-sonnet-4.5`
- Override per-agent via `model` field in frontmatter

## Docker

- Image: `budgie-sandbox:latest`
- Base: `buildpack-deps:bookworm`
- Entrypoint handles auth token sync
