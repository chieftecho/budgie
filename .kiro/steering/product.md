# Budgie - Product Overview

## Purpose

Budgie is an MCP server that exposes Kiro agents as MCP tools for orchestration. It enables sub-agent delegation from an orchestrator agent until kiro-cli natively supports sub-agents.

## Target Users

- Developers using kiro-cli who need multi-agent orchestration
- Teams building complex AI workflows with specialized agents

## Key Features

- Auto-discovers agents from `~/.kiro/agents/*.json` with `sub-agent:` prefix
- Session persistence via sessionId for multi-turn conversations
- Health monitoring with success rates, durations, and automatic retries
- Mandatory directory parameter for explicit working directory control
- Response file decoupling (responses written to session dir, not working dir)
- Per-agent model selection via frontmatter (default: `claude-sonnet-4.5`)

## Sandbox Mode

Runs sub-agents in isolated Docker containers for security:

- Filesystem isolation: agents only access mounted working directory
- Credential protection: no access to `~/.aws/`, `~/.ssh/`, etc.
- Per-session Docker volumes (`budgie-session-<uuid>`)
- Auth tokens copied read-only from host

### Container Mounts

| Source | Container Path | Mode | Purpose |
|--------|----------------|------|---------|
| Working directory | `/workspace` | RW | User's project files |
| Docker volume | `/root/.local/share/kiro-cli` | RW | Session state |
| Host kiro auth | `/auth` | RO | Auth tokens |
| `~/.kiro/` | `/root/.kiro/` | RO | Agent configs |

## Architecture

```
Orchestrator → MCP Client → Budgie → kiro-cli → Sub-Agent
                              ↓
                        Health Monitor
```

## Known Limitations

- Every sub-agent requires `fs_read`/`fs_write` for response files
- MCP servers in container may need path translation
- Experimental feature until kiro-cli adds native sub-agent support
