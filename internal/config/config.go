package config

import "time"

type Config struct {
	AgentsDir          string
	SessionsDir        string
	PromptsDir         string
	SystemPromptPath   string
	ContextSummaryPath string
	KiroBinary         string
	ToolPrefix         string
	AgentTimeout       time.Duration
	SandboxEnabled     bool
	SandboxImage       string
	Verbose            bool
}
