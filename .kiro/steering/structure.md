# Project Structure

```
budgie/
├── cmd/server/main.go      # Entry point, MCP server setup, tool registration
├── internal/
│   ├── agents/             # Agent loading from JSON files
│   │   ├── loader.go       # Load(), FilterDescription(), IsSubAgent(), NormalizeToolName(name, prefix)
│   │   └── loader_test.go
│   ├── config/             # Configuration struct
│   │   └── config.go       # Config{} with all CLI flag values
│   ├── frontmatter/        # YAML frontmatter parsing from prompt files
│   │   └── frontmatter.go  # LoadFromPrompt(), EnhancedDescription()
│   ├── health/             # Health metrics tracking
│   │   ├── metrics.go      # Monitor, AgentMetrics, RecordSuccess/Failure
│   │   └── metrics_test.go
│   ├── kiro/               # Kiro CLI executor
│   │   ├── executor.go     # Execute(), ExecuteWithWorkDir(), retry logic
│   │   └── executor_test.go
│   ├── sandbox/            # Sandbox mode integration tests
│   │   └── sandbox_test.go
│   └── sessions/           # Session management
│       ├── session.go      # Manager, GetWorkspaceDir(), Cleanup()
│       └── session_test.go
├── agents/                 # Source agent configs (copied to ~/.kiro/ on install)
│   ├── config/*.json       # Agent JSON definitions
│   └── prompts/*.md        # Agent prompt files with frontmatter
├── docker/
│   └── entrypoint.sh       # Container entrypoint for auth sync
├── Dockerfile              # Sandbox image definition
├── README.md
├── Makefile
└── go.mod
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `deps` | Download Go modules |
| `build` | Build `budgie` binary |
| `clean` | Remove built artifacts |
| `install` | Build and install to `~/.local/bin/`, copy agent configs and prompts to `~/.kiro/` |
| `build-all` | Cross-compile for darwin/linux/windows (amd64/arm64) |
| `test` | Run all tests |

### Install Process

`make install` does:
1. Builds binary to `~/.local/bin/budgie`
2. Copies `hook-notify.sh` to `~/.local/bin/`
3. Creates `~/.kiro/agents/`, `~/.kiro/sub-agents/prompts/`, `~/.kiro/prompts/`
4. Processes agent JSON templates (substitutes `{{KIRO_DIR}}`, `{{BUDGIE_BINARY}}`, `{{HOOK_NOTIFY}}`)
5. Copies prompt files

## Key Patterns

### Entry Point (`cmd/server/main.go`)

1. Parse CLI flags
2. Load agents from JSON files
3. Initialize config, health monitor, session manager, executor
4. Register MCP tools for each sub-agent
5. Register health-check tool
6. Start MCP server on stdio

### Tool Handler Flow

1. Validate input (prompt, directory required)
2. Get/create session workspace
3. Generate unique response file name
4. Enhance prompt with directory and system prompt
5. Execute via kiro.Executor
6. Read response from file or fallback to stdout
7. Return ToolOutput with response and sessionId

## File Locations

**Source (in repo):**
- Agent JSON files: `agents/config/*.json`
- Agent prompts: `agents/prompts/*.md`

**Installed (after `make install`):**
- Agent JSON files: `~/.kiro/agents/*.json`
- Agent prompts: `~/.kiro/sub-agents/prompts/{agent}.md`
- Session workspaces: `~/.kiro/sub-agents/sessions/{uuid}/`
- System prompt template: `~/.kiro/sub-agents/prompts/_system.md`
- Context summary template: `~/.kiro/sub-agents/prompts/_context-summary.md`
