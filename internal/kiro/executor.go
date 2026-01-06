package kiro

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"budgie/internal/health"

	"github.com/google/uuid"
)

type Executor struct {
	binary         string
	timeout        time.Duration
	monitor        *health.Monitor
	sandboxEnabled bool
	sandboxImage   string
	kiroConfigDir  string
	authSourceDir  string
	verbose        bool
}

type Result struct {
	Output    string
	SessionID string
	Error     error
	Duration  time.Duration
	Retried   bool
}

func NewExecutor(binary string, timeout time.Duration, monitor *health.Monitor, sandboxEnabled bool, sandboxImage string, verbose bool) *Executor {
	homeDir, _ := os.UserHomeDir()

	var authSourceDir string
	if runtime.GOOS == "darwin" {
		authSourceDir = filepath.Join(homeDir, "Library", "Application Support", "kiro-cli")
	} else {
		authSourceDir = filepath.Join(homeDir, ".local", "share", "kiro-cli")
	}

	return &Executor{
		binary:         binary,
		timeout:        timeout,
		monitor:        monitor,
		sandboxEnabled: sandboxEnabled,
		sandboxImage:   sandboxImage,
		kiroConfigDir:  filepath.Join(homeDir, ".kiro"),
		authSourceDir:  authSourceDir,
		verbose:        verbose,
	}
}

// GetUniqueResponseFile generates a unique response filename for a workspace
func GetUniqueResponseFile(sessionDir string) string {
	return fmt.Sprintf("response-%s.txt", uuid.New().String()[:8])
}

// GetAuthSourceDir returns the auth source directory
func (e *Executor) GetAuthSourceDir() string {
	return e.authSourceDir
}

// BuildDirectCommand builds a direct kiro-cli command (for testing)
func (e *Executor) BuildDirectCommand(ctx context.Context, agentName, prompt, sessionDir, sessionID, model string) *exec.Cmd {
	return e.buildDirectCommand(ctx, agentName, prompt, sessionDir, sessionID, model)
}

// BuildDockerCommand builds a Docker command (for testing)
func (e *Executor) BuildDockerCommand(ctx context.Context, agentName, prompt, sessionDir, sessionID, model, workDir string) *exec.Cmd {
	return e.buildDockerCommand(ctx, agentName, prompt, sessionDir, sessionID, model, workDir)
}

func (e *Executor) Execute(ctx context.Context, agentName, prompt, sessionDir, sessionID, model string) Result {
	return e.ExecuteWithWorkDir(ctx, agentName, prompt, sessionDir, sessionID, model, "", "")
}

func (e *Executor) ExecuteWithWorkDir(ctx context.Context, agentName, prompt, sessionDir, sessionID, model, workDir, responseFile string) Result {
	start := time.Now()

	result := e.executeOnce(ctx, agentName, prompt, sessionDir, sessionID, model, workDir, responseFile)

	if result.Error != nil && shouldRetry(result.Error) {
		time.Sleep(2 * time.Second)
		retryResult := e.executeOnce(ctx, agentName, prompt, sessionDir, sessionID, model, workDir, responseFile)
		retryResult.Retried = true

		if retryResult.Error == nil {
			if e.monitor != nil {
				e.monitor.RecordSuccess(agentName, time.Since(start))
			}
			return retryResult
		}

		result = retryResult
	}

	if e.monitor != nil {
		duration := time.Since(start)
		if result.Error != nil {
			isTimeout := strings.Contains(result.Error.Error(), "timeout") ||
				strings.Contains(result.Error.Error(), "deadline exceeded")
			e.monitor.RecordFailure(agentName, duration, result.Error.Error(), isTimeout)
		} else {
			e.monitor.RecordSuccess(agentName, duration)
		}
	}

	return result
}

func (e *Executor) executeOnce(ctx context.Context, agentName, prompt, sessionDir, sessionID, model, workDir, responseFile string) Result {
	timeoutCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	var cmd *exec.Cmd

	if e.sandboxEnabled {
		cmd = e.buildDockerCommand(timeoutCtx, agentName, prompt, sessionDir, sessionID, model, workDir)
	} else {
		cmd = e.buildDirectCommand(timeoutCtx, agentName, prompt, sessionDir, sessionID, model)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Save chat output for debugging
	if e.verbose {
		e.saveChatDebug(sessionDir, stdout.String(), stderr.String(), prompt, agentName, workDir, responseFile, err)
	}

	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			return Result{Error: fmt.Errorf("agent timeout after %v", e.timeout)}
		}

		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return Result{Error: fmt.Errorf("kiro-cli failed: %s", errMsg)}
	}

	return Result{
		Output: strings.TrimSpace(stdout.String()),
		Error:  nil,
	}
}

func (e *Executor) saveChatDebug(sessionDir, stdout, stderr, prompt, agentName, workDir, responseFile string, execErr error) {
	// Extract ID from responseFile (format: response-XXXXXXXX.txt) to match chat with response
	chatID := strings.TrimSuffix(strings.TrimPrefix(responseFile, "response-"), ".txt")
	chatFileName := fmt.Sprintf("chat-%s.txt", chatID)

	var content strings.Builder
	content.WriteString(fmt.Sprintf("=== Chat Debug: %s ===\n", time.Now().Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("Agent: %s\n", agentName))
	content.WriteString(fmt.Sprintf("Working Directory: %s\n", workDir))
	content.WriteString(fmt.Sprintf("Response File: %s\n", responseFile))
	content.WriteString(fmt.Sprintf("Prompt: %s\n", prompt))
	content.WriteString("\n=== STDOUT ===\n")
	content.WriteString(stdout)
	content.WriteString("\n=== STDERR ===\n")
	content.WriteString(stderr)
	if execErr != nil {
		content.WriteString(fmt.Sprintf("\n=== ERROR ===\n%v\n", execErr))
	}

	if e.sandboxEnabled {
		// Append to volume using docker
		volumeName := "budgie-session-" + sessionDir
		cmd := exec.Command("docker", "run", "--rm", "-i",
			"-v", volumeName+":/data:rw",
			"alpine:latest",
			"sh", "-c", fmt.Sprintf("cat >> /data/%s", chatFileName))
		cmd.Stdin = strings.NewReader(content.String())
		cmd.Run()
	} else {
		f, err := os.OpenFile(filepath.Join(sessionDir, chatFileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString(content.String())
			f.Close()
		}
	}
}

func (e *Executor) buildDirectCommand(ctx context.Context, agentName, prompt, sessionDir, sessionID, model string) *exec.Cmd {
	args := []string{"chat", "--agent", agentName, "--no-interactive"}

	if model != "" {
		args = append(args, "--model", model)
	}

	if sessionID != "" {
		args = append(args, "--resume")
	}

	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, e.binary, args...)
	cmd.Dir = sessionDir
	return cmd
}

func (e *Executor) buildDockerCommand(ctx context.Context, agentName, prompt, sessionDir, sessionID, model, workDir string) *exec.Cmd {
	volumeName := "budgie-session-" + sessionDir

	args := []string{
		"run", "--rm",
		"-v", volumeName + ":/root/.local/share/kiro-cli:rw",
		"-v", e.authSourceDir + ":/auth:ro",
		"-v", e.kiroConfigDir + ":/root/.kiro:ro",
	}

	if workDir != "" {
		args = append(args, "-v", workDir+":/workspace:rw")
	}

	args = append(args, e.sandboxImage)

	kiroArgs := []string{"kiro-cli", "chat", "--agent", agentName, "--no-interactive"}

	if model != "" {
		kiroArgs = append(kiroArgs, "--model", model)
	}

	if sessionID != "" {
		kiroArgs = append(kiroArgs, "--resume")
	}

	kiroArgs = append(kiroArgs, prompt)

	args = append(args, kiroArgs...)

	return exec.CommandContext(ctx, "docker", args...)
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return true
	}

	if strings.Contains(errStr, "signal") || strings.Contains(errStr, "killed") {
		return true
	}

	if strings.Contains(errStr, "exit status") && !strings.Contains(errStr, "required") {
		return true
	}

	return false
}
