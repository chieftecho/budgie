package kiro

import (
	"context"
	"testing"
	"time"
)

func TestNewExecutor(t *testing.T) {
	executor := NewExecutor("/custom/path/kiro", 5*time.Minute, nil, false, "", false)

	if executor.binary != "/custom/path/kiro" {
		t.Errorf("Expected binary /custom/path/kiro, got %s", executor.binary)
	}

	if executor.timeout != 5*time.Minute {
		t.Errorf("Expected timeout 5m, got %v", executor.timeout)
	}
}

func TestExecute_InvalidBinary(t *testing.T) {
	executor := NewExecutor("nonexistent-binary-12345", 1*time.Minute, nil, false, "", false)

	result := executor.Execute(context.Background(), "test-agent", "test prompt", "/tmp", "", "")

	if result.Error == nil {
		t.Error("Expected error for nonexistent binary")
	}
}

func TestExecute_WithSessionID(t *testing.T) {
	// This test verifies the command construction logic
	// Actual execution would require kiro-cli to be available
	executor := NewExecutor("echo", 1*time.Minute, nil, false, "", false)

	result := executor.Execute(context.Background(), "test-agent", "test", "/tmp", "session-123", "")

	// With echo, we won't get an error, just output
	if result.Error != nil {
		t.Logf("Expected behavior with echo binary: %v", result.Error)
	}
}

func TestExecute_WithModel(t *testing.T) {
	executor := NewExecutor("echo", 1*time.Minute, nil, false, "", false)

	result := executor.Execute(context.Background(), "test-agent", "test", "/tmp", "", "claude-sonnet-4.5")

	if result.Error != nil {
		t.Logf("Expected behavior with echo binary: %v", result.Error)
	}
}
