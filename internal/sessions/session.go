package sessions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

type Manager struct {
	baseDir     string
	sessions    map[string]bool
	mutex       sync.Mutex
	sandboxMode bool
}

func NewManager(baseDir string, sandboxMode bool) *Manager {
	return &Manager{
		baseDir:     baseDir,
		sessions:    make(map[string]bool),
		sandboxMode: sandboxMode,
	}
}

func (m *Manager) GetWorkspaceDir(sessionID string) (string, error) {
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	m.mutex.Lock()
	m.sessions[sessionID] = true
	m.mutex.Unlock()

	if m.sandboxMode {
		volumeName := "budgie-session-" + sessionID
		cmd := exec.Command("docker", "volume", "create", volumeName)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to create docker volume: %w", err)
		}
		return sessionID, nil
	}

	var sessionDir string
	if m.baseDir != "" {
		sessionDir = filepath.Join(m.baseDir, sessionID)
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		sessionDir = filepath.Join(homeDir, ".kiro", "sub-agents", "sessions", sessionID)
	}

	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", err
	}

	return sessionDir, nil
}

func (m *Manager) GetSessionID(sessionDir string) string {
	if m.sandboxMode {
		return sessionDir
	}
	return filepath.Base(sessionDir)
}

func (m *Manager) Cleanup() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for sessionID := range m.sessions {
		if m.sandboxMode {
			volumeName := "budgie-session-" + sessionID
			exec.Command("docker", "volume", "rm", volumeName).Run()
		} else {
			var sessionDir string
			if m.baseDir != "" {
				sessionDir = filepath.Join(m.baseDir, sessionID)
			}
			if sessionDir != "" {
				os.RemoveAll(sessionDir)
			}
		}
	}
	m.sessions = make(map[string]bool)
}
