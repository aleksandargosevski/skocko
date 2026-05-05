package session

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SaveSession snapshots an active tmux session and writes it to disk.
// For nvim panes, it also triggers :mksession to save editor state.
func SaveSession(sessionName string) (*SessionSnapshot, error) {
	snapshot, err := SnapshotSession(sessionName)
	if err != nil {
		return nil, err
	}

	// Save nvim sessions before writing the snapshot
	for i, win := range snapshot.Windows {
		for j, pane := range win.Panes {
			if pane.Strategy == StrategyNvimSession {
				sessionFile := NvimSessionFilePath(sessionName, win.Index)
				panePID := getPanePID(sessionName, win.Index, pane.Index)
				if panePID > 0 {
					socketPath := FindNvimSocket(panePID)
					if socketPath != "" {
						if err := SaveNvimSession(socketPath, sessionFile); err == nil {
							snapshot.Windows[i].Panes[j].NvimSessionFile = sessionFile
							snapshot.Windows[i].Panes[j].RestoreCommand = BuildRestoreCommand(
								pane.Command, pane.Path, pane.Strategy, sessionFile)
						}
					}
				}
			}
		}
	}

	// Write snapshot to disk
	if err := WriteSnapshot(snapshot); err != nil {
		return nil, err
	}

	return snapshot, nil
}

// SnapshotSession captures the current state of a tmux session.
func SnapshotSession(sessionName string) (*SessionSnapshot, error) {
	// Get session path
	out, err := exec.Command("tmux", "display-message", "-t", sessionName,
		"-p", "#{session_path}").Output()
	if err != nil {
		return nil, fmt.Errorf("session %q not found: %w", sessionName, err)
	}
	sessionPath := strings.TrimSpace(string(out))

	// Get windows
	winOut, err := exec.Command("tmux", "list-windows", "-t", sessionName,
		"-F", "#{window_index}\t#{window_name}\t#{window_layout}\t#{window_active}\t#{pane_current_path}").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list windows: %w", err)
	}

	var windows []WindowSnapshot
	for _, line := range strings.Split(strings.TrimSpace(string(winOut)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 5 {
			continue
		}

		idx, _ := strconv.Atoi(parts[0])
		active := parts[3] == "1"

		// Get panes for this window
		panes := snapshotPanes(sessionName, idx)

		windows = append(windows, WindowSnapshot{
			Index:  idx,
			Name:   parts[1],
			Layout: parts[2],
			Path:   parts[4],
			Active: active,
			Panes:  panes,
		})
	}

	return &SessionSnapshot{
		Name:    sessionName,
		Path:    sessionPath,
		SavedAt: time.Now(),
		Windows: windows,
	}, nil
}

// snapshotPanes captures all panes in a window.
func snapshotPanes(sessionName string, windowIndex int) []PaneSnapshot {
	target := fmt.Sprintf("%s:%d", sessionName, windowIndex)
	out, err := exec.Command("tmux", "list-panes", "-t", target,
		"-F", "#{pane_index}\t#{pane_current_command}\t#{pane_current_path}\t#{pane_pid}").Output()
	if err != nil {
		return nil
	}

	var panes []PaneSnapshot
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}

		idx, _ := strconv.Atoi(parts[0])
		command := parts[1]
		path := parts[2]

		strategy := DetectStrategy(command)
		restoreCmd := BuildRestoreCommand(command, path, strategy, "")

		panes = append(panes, PaneSnapshot{
			Index:          idx,
			Command:        command,
			Path:           path,
			Strategy:       strategy,
			RestoreCommand: restoreCmd,
		})
	}

	return panes
}

// getPanePID returns the PID of a specific pane.
func getPanePID(sessionName string, windowIndex, paneIndex int) int {
	target := fmt.Sprintf("%s:%d.%d", sessionName, windowIndex, paneIndex)
	out, err := exec.Command("tmux", "display-message", "-t", target, "-p", "#{pane_pid}").Output()
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return pid
}

// --- Persistence ---

// SnapshotDir returns the directory where session snapshots are stored.
func SnapshotDir() string {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "/tmp/skocko/sessions"
		}
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "skocko", "sessions")
}

// WriteSnapshot saves a session snapshot to disk as JSON.
func WriteSnapshot(snapshot *SessionSnapshot) error {
	dir := SnapshotDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	path := filepath.Join(dir, snapshot.Name+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	return nil
}

// LoadSnapshot reads a session snapshot from disk.
func LoadSnapshot(sessionName string) (*SessionSnapshot, error) {
	path := filepath.Join(SnapshotDir(), sessionName+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var snapshot SessionSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot: %w", err)
	}

	return &snapshot, nil
}

// HasSavedState checks if a session has a saved snapshot on disk.
func HasSavedState(sessionName string) bool {
	path := filepath.Join(SnapshotDir(), sessionName+".json")
	_, err := os.Stat(path)
	return err == nil
}

// DeleteSavedState removes the saved snapshot and any associated nvim session files.
func DeleteSavedState(sessionName string) error {
	// Remove the snapshot JSON
	snapshotPath := filepath.Join(SnapshotDir(), sessionName+".json")
	if err := os.Remove(snapshotPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	// Remove any nvim session files (glob: <name>_*.vim)
	pattern := filepath.Join(NvimSessionDir(), sessionName+"_*.vim")
	matches, err := filepath.Glob(pattern)
	if err == nil {
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}

	return nil
}

// ListSavedSessions returns the names of all saved session snapshots.
func ListSavedSessions() []string {
	dir := SnapshotDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names
}
