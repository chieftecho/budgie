package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"budgie/internal/config"
	"budgie/internal/health"
	"budgie/internal/kiro"
	"budgie/internal/sessions"
)

// TestSandboxVolumeCreation verifies that Docker volumes are created for sandbox sessions
func TestSandboxVolumeCreation(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker not available")
	}

	tmpDir := t.TempDir()
	mgr := sessions.NewManager(tmpDir, true)

	sessionID := "test-sandbox-" + fmt.Sprintf("%d", time.Now().Unix())
	dir, err := mgr.GetWorkspaceDir(sessionID)
	if err != nil {
		t.Fatalf("GetWorkspaceDir failed: %v", err)
	}

	if dir != sessionID {
		t.Errorf("Expected session ID as return value in sandbox mode, got %s", dir)
	}

	volumeName := "budgie-session-" + sessionID
	if !volumeExists(volumeName) {
		t.Errorf("Docker volume %s was not created", volumeName)
	}

	mgr.Cleanup()

	if volumeExists(volumeName) {
		t.Errorf("Docker volume %s should be removed after cleanup", volumeName)
	}
}

// TestSandboxSessionIDRetrieval verifies GetSessionID works correctly in sandbox mode
func TestSandboxSessionIDRetrieval(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := sessions.NewManager(tmpDir, true)

	sessionID := "test-session-id"
	dir, err := mgr.GetWorkspaceDir(sessionID)
	if err != nil {
		t.Fatalf("GetWorkspaceDir failed: %v", err)
	}

	retrievedID := mgr.GetSessionID(dir)
	if retrievedID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, retrievedID)
	}
}

// TestSandboxExecutorDockerCommand verifies Docker command construction
func TestSandboxExecutorDockerCommand(t *testing.T) {
	monitor := health.NewMonitor()
	executor := kiro.NewExecutor("kiro-cli", 5*time.Minute, monitor, true, "budgie-sandbox:latest", false)

	sessionDir := "test-session-123"
	agentName := "test-agent"
	prompt := "test prompt"
	model := "claude-sonnet-4.5"
	workDir := "/home/user/project"

	cmd := executor.BuildDockerCommand(context.Background(), agentName, prompt, sessionDir, "session-123", model, workDir)

	cmdStr := strings.Join(cmd.Args, " ")

	if !strings.Contains(cmdStr, "docker") {
		t.Error("Command should use docker")
	}

	if !strings.Contains(cmdStr, "budgie-session-test-session-123") {
		t.Error("Command should include volume mount for session")
	}

	if !strings.Contains(cmdStr, "/root/.local/share/kiro-cli") {
		t.Error("Command should mount kiro-cli data directory")
	}

	if !strings.Contains(cmdStr, "/auth") {
		t.Error("Command should mount auth directory")
	}

	if !strings.Contains(cmdStr, "/root/.kiro") {
		t.Error("Command should mount kiro config directory")
	}

	if !strings.Contains(cmdStr, workDir+":/workspace") {
		t.Error("Command should mount working directory to /workspace")
	}

	if !strings.Contains(cmdStr, "budgie-sandbox:latest") {
		t.Error("Command should use specified sandbox image")
	}

	if !strings.Contains(cmdStr, "--no-interactive") {
		t.Error("Command should include --no-interactive flag")
	}

	if !strings.Contains(cmdStr, "--model") || !strings.Contains(cmdStr, model) {
		t.Error("Command should include model specification")
	}

	if !strings.Contains(cmdStr, "--resume") {
		t.Error("Command should include --resume flag for session")
	}
}

// TestSandboxExecutorNormalMode verifies normal (non-sandbox) mode still works
func TestSandboxExecutorNormalMode(t *testing.T) {
	monitor := health.NewMonitor()
	executor := kiro.NewExecutor("kiro-cli", 5*time.Minute, monitor, false, "", false)

	sessionDir := "/tmp/test-session"
	agentName := "test-agent"
	prompt := "test prompt"
	model := "claude-sonnet-4.5"

	cmd := executor.BuildDirectCommand(context.Background(), agentName, prompt, sessionDir, "session-123", model)

	cmdStr := strings.Join(cmd.Args, " ")

	if strings.Contains(cmdStr, "docker") {
		t.Error("Normal mode should not use docker")
	}

	if !strings.Contains(cmdStr, "kiro-cli") {
		t.Error("Command should use kiro-cli binary")
	}

	if !strings.Contains(cmdStr, "--no-interactive") {
		t.Error("Command should include --no-interactive flag")
	}

	if !strings.Contains(cmdStr, "--resume") {
		t.Error("Command should include --resume flag")
	}

	if cmd.Dir != sessionDir {
		t.Errorf("Expected working directory %s, got %s", sessionDir, cmd.Dir)
	}
}

// TestSandboxConfigPropagation verifies sandbox config is properly set
func TestSandboxConfigPropagation(t *testing.T) {
	cfg := &config.Config{
		SandboxEnabled: true,
		SandboxImage:   "custom-sandbox:v1.0",
	}

	if !cfg.SandboxEnabled {
		t.Error("SandboxEnabled should be true")
	}

	if cfg.SandboxImage != "custom-sandbox:v1.0" {
		t.Error("SandboxImage should be set to custom value")
	}
}

// TestSandboxMultipleSessions verifies multiple sandbox sessions can coexist
func TestSandboxMultipleSessions(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker not available")
	}

	tmpDir := t.TempDir()
	mgr := sessions.NewManager(tmpDir, true)

	sessionID1 := "test-session-1-" + fmt.Sprintf("%d", time.Now().Unix())
	sessionID2 := "test-session-2-" + fmt.Sprintf("%d", time.Now().Unix())

	dir1, err1 := mgr.GetWorkspaceDir(sessionID1)
	dir2, err2 := mgr.GetWorkspaceDir(sessionID2)

	if err1 != nil || err2 != nil {
		t.Fatalf("GetWorkspaceDir failed: %v, %v", err1, err2)
	}

	if dir1 == dir2 {
		t.Error("Different sessions should have different IDs")
	}

	volume1 := "budgie-session-" + sessionID1
	volume2 := "budgie-session-" + sessionID2

	if !volumeExists(volume1) || !volumeExists(volume2) {
		t.Error("Both volumes should exist")
	}

	mgr.Cleanup()

	if volumeExists(volume1) || volumeExists(volume2) {
		t.Error("Both volumes should be removed after cleanup")
	}
}

// TestSandboxAuthPathDetection verifies correct auth path for different OS
func TestSandboxAuthPathDetection(t *testing.T) {
	monitor := health.NewMonitor()
	executor := kiro.NewExecutor("kiro-cli", 5*time.Minute, monitor, true, "budgie-sandbox:latest", false)

	if executor.GetAuthSourceDir() == "" {
		t.Error("Auth source directory should not be empty")
	}

	if !strings.Contains(executor.GetAuthSourceDir(), "kiro-cli") {
		t.Error("Auth source directory should contain kiro-cli")
	}
}

// TestSandboxResponseFileHandling verifies response file paths in sandbox mode
func TestSandboxResponseFileHandling(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "test-session")
	os.MkdirAll(sessionDir, 0755)

	responseFile := kiro.GetUniqueResponseFile(sessionDir)

	if !strings.HasPrefix(responseFile, "response-") {
		t.Error("Response file should start with 'response-'")
	}

	if !strings.HasSuffix(responseFile, ".txt") {
		t.Error("Response file should end with .txt")
	}

	if len(responseFile) < 20 {
		t.Error("Response file name should include UUID")
	}
}

// Helper functions

func isDockerAvailable() bool {
	cmd := exec.Command("docker", "version")
	return cmd.Run() == nil
}

func volumeExists(volumeName string) bool {
	cmd := exec.Command("docker", "volume", "inspect", volumeName)
	return cmd.Run() == nil
}
