package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"budgie/internal/agents"
	"budgie/internal/config"
	"budgie/internal/frontmatter"
	"budgie/internal/health"
	"budgie/internal/kiro"
	"budgie/internal/sessions"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolInput struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"sessionId,omitempty"`
	Directory string `json:"directory,omitempty"`
}

type ToolOutput struct {
	Response  string `json:"response"`
	SessionID string `json:"sessionId"`
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %v", err)
	}

	agentsDir := flag.String("agents-dir", filepath.Join(homeDir, ".kiro", "agents"), "Directory containing agent JSON files")
	sessionsDir := flag.String("sessions-dir", filepath.Join(homeDir, ".kiro", "sub-agents", "sessions"), "Base directory for session workspaces")
	promptsDir := flag.String("prompts-dir", filepath.Join(homeDir, ".kiro", "sub-agents", "prompts"), "Directory containing agent prompt files")
	systemPromptPath := flag.String("system-prompt", filepath.Join(homeDir, ".kiro", "sub-agents", "prompts", "_system.md"), "Path to system prompt template file")
	contextSummaryPath := flag.String("context-summary-prompt", filepath.Join(homeDir, ".kiro", "sub-agents", "prompts", "_context-summary.md"), "Path to context summary prompt template file")
	kiroBinary := flag.String("kiro-binary", "kiro-cli", "Path to kiro-cli binary")
	toolPrefix := flag.String("tool-prefix", "kiro-subagents.", "Prefix for registered tool names")
	agentTimeout := flag.Duration("agent-timeout", 10*time.Minute, "Timeout for agent execution")
	sandboxEnabled := flag.Bool("sandbox", false, "Enable sandbox mode (run agents in Docker containers)")
	sandboxImage := flag.String("sandbox-image", "budgie-sandbox:latest", "Docker image for sandbox mode")
	verbose := flag.Bool("verbose", false, "Enable verbose output including chat debug logs")
	listTools := flag.Bool("list-tools", false, "Print tool information and exit")
	flag.Parse()

	agentList, err := agents.Load(*agentsDir)
	if err != nil {
		log.Fatalf("Failed to load agents: %v", err)
	}

	if len(agentList) == 0 {
		log.Fatalf("No agents found in %s", *agentsDir)
	}

	// Initialize config
	cfg := &config.Config{
		AgentsDir:          *agentsDir,
		SessionsDir:        *sessionsDir,
		PromptsDir:         *promptsDir,
		SystemPromptPath:   *systemPromptPath,
		ContextSummaryPath: *contextSummaryPath,
		KiroBinary:         *kiroBinary,
		ToolPrefix:         *toolPrefix,
		AgentTimeout:       *agentTimeout,
		SandboxEnabled:     *sandboxEnabled,
		SandboxImage:       *sandboxImage,
		Verbose:            *verbose,
	}

	// Create dependencies
	healthMonitor := health.NewMonitor()
	sessionMgr := sessions.NewManager(cfg.SessionsDir, cfg.SandboxEnabled)
	executor := kiro.NewExecutor(cfg.KiroBinary, cfg.AgentTimeout, healthMonitor, cfg.SandboxEnabled, cfg.SandboxImage, cfg.Verbose)

	// List tools mode: print tool information and exit
	if *listTools {
		fmt.Println("=== Debug Mode: Tool Information ===")
		fmt.Println()
		for _, agent := range agentList {
			agentName := agent.Name
			
			if agentName == "orchestrator" {
				continue
			}
			
			if !agents.IsSubAgent(agent.Description) {
				continue
			}

			toolName := agents.NormalizeToolName(agentName, cfg.ToolPrefix)
			description := agents.FilterDescription(agent.Description)
			
			fmt.Printf("Tool: %s\n", toolName)
			fmt.Printf("Agent: %s\n", agentName)
			fmt.Printf("Base Description: %s\n", description)
			
			if metadata, err := frontmatter.LoadFromPrompt(cfg.PromptsDir, agentName); err == nil && metadata != nil {
				fmt.Printf("Frontmatter: LOADED\n")
				fmt.Printf("Enhanced Description:\n%s\n", metadata.EnhancedDescription())
			} else if err != nil {
				fmt.Printf("Frontmatter: ERROR - %v\n", err)
			} else {
				fmt.Printf("Frontmatter: NOT FOUND\n")
			}
			fmt.Println("\n" + strings.Repeat("-", 80) + "\n")
		}
		return
	}

	// Setup cleanup on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down, cleaning up sessions...")
		sessionMgr.Cleanup()
		cancel()
	}()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "kiro-subagents",
		Version: "1.0.0",
	}, nil)

	for _, agent := range agentList {
		agentName := agent.Name
		
		// Skip orchestrator to prevent circular dependency
		if agentName == "orchestrator" {
			continue
		}
		
		// Only register agents with "sub-agent:" prefix
		if !agents.IsSubAgent(agent.Description) {
			continue
		}

		toolName := agents.NormalizeToolName(agentName, cfg.ToolPrefix)
		description := agents.FilterDescription(agent.Description)
		model := "claude-sonnet-4.5"
		
		// Try to load frontmatter from prompt file
		if metadata, err := frontmatter.LoadFromPrompt(cfg.PromptsDir, agentName); err == nil && metadata != nil {
			description = metadata.EnhancedDescription()
			if metadata.Model != "" {
				model = metadata.Model
			}
			log.Printf("Loaded frontmatter for %s (model: %s)", agentName, model)
		} else if err != nil {
			log.Printf("Failed to load frontmatter for %s: %v", agentName, err)
		}
		
		handler := createHandler(agentName, model, sessionMgr, executor, cfg)
		tool := &mcp.Tool{
			Name:        toolName,
			Description: description,
		}

		mcp.AddTool(server, tool, handler)
		log.Printf("Registered tool: %s (agent: %s)", toolName, agentName)
	}

	// Register health-check tool
	healthTool := &mcp.Tool{
		Name:        cfg.ToolPrefix + "health-check",
		Description: "Get health metrics for all sub-agents including success rates, average duration, and failure counts",
	}

	healthHandler := func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, map[string]interface{}, error) {
		allMetrics := healthMonitor.GetAllMetrics()

		result := make(map[string]interface{})
		agentStats := make([]map[string]interface{}, 0)

		totalCalls := 0
		totalSuccess := 0

		for agent, metrics := range allMetrics {
			totalCalls += metrics.TotalCalls
			totalSuccess += metrics.SuccessCalls

			agentStats = append(agentStats, map[string]interface{}{
				"agent":        agent,
				"totalCalls":   metrics.TotalCalls,
				"successCalls": metrics.SuccessCalls,
				"failedCalls":  metrics.FailedCalls,
				"timeoutCalls": metrics.TimeoutCalls,
				"successRate":  fmt.Sprintf("%.1f%%", metrics.SuccessRate()*100),
				"avgDuration":  metrics.AvgDuration().String(),
				"lastSuccess":  metrics.LastSuccess.Format(time.RFC3339),
				"lastFailure":  metrics.LastFailure.Format(time.RFC3339),
				"lastError":    metrics.LastError,
			})
		}

		overallRate := 0.0
		if totalCalls > 0 {
			overallRate = float64(totalSuccess) / float64(totalCalls)
		}

		result["overall"] = map[string]interface{}{
			"totalCalls":   totalCalls,
			"successCalls": totalSuccess,
			"successRate":  fmt.Sprintf("%.1f%%", overallRate*100),
		}
		result["agents"] = agentStats

		return nil, result, nil
	}

	mcp.AddTool(server, healthTool, healthHandler)
	log.Printf("Registered health-check tool")

	log.Printf("Starting Kiro sub-agents MCP server with %d agents", len(agentList))
	if cfg.SandboxEnabled {
		log.Printf("Sandbox mode enabled with image: %s", cfg.SandboxImage)
	}
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	sessionMgr.Cleanup()
}

func createHandler(agentName, model string, sessionMgr *sessions.Manager, executor *kiro.Executor, cfg *config.Config) func(context.Context, *mcp.CallToolRequest, ToolInput) (*mcp.CallToolResult, ToolOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ToolInput) (*mcp.CallToolResult, ToolOutput, error) {
		if input.Prompt == "" {
			return nil, ToolOutput{}, fmt.Errorf("prompt is required")
		}

		if input.Directory == "" {
			return nil, ToolOutput{}, fmt.Errorf("directory is required")
		}

		sessionDir, err := sessionMgr.GetWorkspaceDir(input.SessionID)
		if err != nil {
			return nil, ToolOutput{}, fmt.Errorf("failed to create workspace: %w", err)
		}

		sessionID := sessionMgr.GetSessionID(sessionDir)

		// Generate unique response file name
		responseFile := kiro.GetUniqueResponseFile(sessionDir)

		// Determine working directory path for prompt
		workingDir := input.Directory
		if cfg.SandboxEnabled {
			workingDir = "/workspace"
		}

		// Augment prompt with directory instruction
		enhancedPrompt := fmt.Sprintf("In directory %s, %s", workingDir, input.Prompt)

		// Load and inject system prompt with response file placeholder
		if systemPromptTemplate, err := os.ReadFile(cfg.SystemPromptPath); err == nil {
			// Response file path must be absolute so agent knows where to write
			responseFilePath := filepath.Join(sessionDir, responseFile)
			if cfg.SandboxEnabled {
				responseFilePath = "/root/.local/share/kiro-cli/" + responseFile
			}
			systemPrompt := strings.ReplaceAll(string(systemPromptTemplate), "{{RESPONSE_FILE}}", responseFilePath)
			systemPrompt = strings.ReplaceAll(systemPrompt, "{{WORKING_DIRECTORY}}", workingDir)
			enhancedPrompt = enhancedPrompt + "\n\n" + systemPrompt
		}

		// Pass working directory for sandbox mount
		result := executor.ExecuteWithWorkDir(ctx, agentName, enhancedPrompt, sessionDir, input.SessionID, model, input.Directory, responseFile)
		if result.Error != nil {
			// Return error in response body with sessionID so orchestrator can retry
			return nil, ToolOutput{
				Response:  fmt.Sprintf("ERROR: %v", result.Error),
				SessionID: sessionID,
			}, nil
		}

		// Determine response file path
		var responsePath string
		if cfg.SandboxEnabled {
			responsePath = "/root/.local/share/kiro-cli/" + responseFile
		} else {
			responsePath = filepath.Join(sessionDir, responseFile)
		}

		// Try to read response file first
		responseOutput := result.Output
		var responseFound bool
		if cfg.SandboxEnabled {
			if content := readResponseFromVolume(sessionID, responseFile); content != "" {
				responseOutput = content
				responseFound = true
			}
		} else if content, err := os.ReadFile(responsePath); err == nil {
			responseOutput = strings.TrimSpace(string(content))
			responseFound = true
		}

		// Fallback: Request file creation using template
		if !responseFound {
			if contextSummaryTemplate, err := os.ReadFile(cfg.ContextSummaryPath); err == nil {
				fallbackPrompt := strings.ReplaceAll(string(contextSummaryTemplate), "{{RESPONSE_FILE}}", responsePath)

				fallbackResult := executor.ExecuteWithWorkDir(ctx, agentName, fallbackPrompt, sessionDir, sessionID, model, input.Directory, responseFile)
				if fallbackResult.Error == nil {
					if cfg.SandboxEnabled {
						if content := readResponseFromVolume(sessionID, responseFile); content != "" {
							responseOutput = content
						}
					} else if content, err := os.ReadFile(responsePath); err == nil {
						responseOutput = strings.TrimSpace(string(content))
					}
				}
			}
		}

		return nil, ToolOutput{
			Response:  responseOutput,
			SessionID: sessionID,
		}, nil
	}
}

func readResponseFromVolume(sessionID, responseFile string) string {
	volumeName := "budgie-session-" + sessionID
	cmd := exec.Command("docker", "run", "--rm",
		"-v", volumeName+":/data:ro",
		"alpine:latest",
		"cat", "/data/"+responseFile)

	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
