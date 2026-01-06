package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetWorkspaceDir_NewSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir, false)

	dir, err := mgr.GetWorkspaceDir("")
	if err != nil {
		t.Fatalf("GetWorkspaceDir failed: %v", err)
	}

	if !filepath.IsAbs(dir) {
		t.Errorf("Expected absolute path, got: %s", dir)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("Directory was not created: %s", dir)
	}

	if len(mgr.sessions) != 1 {
		t.Errorf("Expected 1 tracked session, got %d", len(mgr.sessions))
	}
}

func TestGetWorkspaceDir_ExistingSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir, false)

	sessionID := "test-session-123"
	dir, err := mgr.GetWorkspaceDir(sessionID)
	if err != nil {
		t.Fatalf("GetWorkspaceDir failed: %v", err)
	}

	if !filepath.IsAbs(dir) {
		t.Errorf("Expected absolute path, got: %s", dir)
	}

	if mgr.GetSessionID(dir) != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, mgr.GetSessionID(dir))
	}

	if len(mgr.sessions) != 1 {
		t.Errorf("Expected 1 tracked session, got %d", len(mgr.sessions))
	}
}

func TestGetWorkspaceDir_Reuse(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir, false)

	sessionID := "reuse-session"
	dir1, _ := mgr.GetWorkspaceDir(sessionID)
	dir2, _ := mgr.GetWorkspaceDir(sessionID)

	if dir1 != dir2 {
		t.Errorf("Expected same directory, got %s and %s", dir1, dir2)
	}

	if len(mgr.sessions) != 1 {
		t.Errorf("Expected 1 tracked session, got %d", len(mgr.sessions))
	}
}

func TestCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir, false)

	dir1, _ := mgr.GetWorkspaceDir("")
	dir2, _ := mgr.GetWorkspaceDir("")

	if _, err := os.Stat(dir1); os.IsNotExist(err) {
		t.Fatalf("Directory 1 should exist before cleanup")
	}
	if _, err := os.Stat(dir2); os.IsNotExist(err) {
		t.Fatalf("Directory 2 should exist before cleanup")
	}

	mgr.Cleanup()

	if _, err := os.Stat(dir1); !os.IsNotExist(err) {
		t.Errorf("Directory 1 should be removed after cleanup")
	}
	if _, err := os.Stat(dir2); !os.IsNotExist(err) {
		t.Errorf("Directory 2 should be removed after cleanup")
	}

	if len(mgr.sessions) != 0 {
		t.Errorf("Expected 0 tracked sessions after cleanup, got %d", len(mgr.sessions))
	}
}
