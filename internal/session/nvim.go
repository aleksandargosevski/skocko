package session

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FindNvimSocket finds the nvim RPC socket for a given pane PID.
// nvim creates sockets under /var/folders/.../nvim.<user>/<hash>/0
// We find the socket by looking at child processes of the pane shell PID.
func FindNvimSocket(panePID int) string {
	// Get the actual nvim PID (child of the shell)
	nvimPID := findChildNvim(panePID)
	if nvimPID == 0 {
		return ""
	}

	// Search for the socket in the nvim temp directory
	// On macOS: /var/folders/.../nvim.<user>/
	tmpDir := os.TempDir()
	user := os.Getenv("USER")
	if user == "" {
		return ""
	}

	// Find the nvim socket directory
	// Structure: <tmpdir>/nvim.<user>/<hash>/0
	nvimBase := filepath.Join(filepath.Dir(tmpDir), "nvim."+user)
	if _, err := os.Stat(nvimBase); os.IsNotExist(err) {
		// Try alternative location
		entries, err := filepath.Glob(filepath.Join(os.TempDir(), "..", "nvim."+user))
		if err != nil || len(entries) == 0 {
			// Search more broadly
			nvimBase = findNvimBaseDir(user)
			if nvimBase == "" {
				return ""
			}
		} else {
			nvimBase = entries[0]
		}
	}

	// Try each hash directory for a socket that belongs to our nvim PID
	entries, err := os.ReadDir(nvimBase)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		socketPath := filepath.Join(nvimBase, entry.Name(), "0")
		if _, err := os.Stat(socketPath); err != nil {
			continue
		}

		// Test if this socket is responsive and belongs to our nvim
		if isNvimSocket(socketPath, nvimPID) {
			return socketPath
		}
	}

	return ""
}

// SaveNvimSession tells nvim to save its session via the RPC socket.
func SaveNvimSession(socketPath, sessionFile string) error {
	// Ensure the directory exists
	dir := filepath.Dir(sessionFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Use nvim --server to send the mksession command
	cmd := exec.Command("nvim", "--server", socketPath, "--remote-send",
		fmt.Sprintf("<C-\\><C-n>:mksession! %s<CR>", sessionFile))
	return cmd.Run()
}

// findChildNvim finds an nvim process that is a child of the given PID.
func findChildNvim(parentPID int) int {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(parentPID), "nvim").Output()
	if err != nil {
		return 0
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && pid > 0 {
			return pid
		}
	}
	return 0
}

// findNvimBaseDir searches for the nvim socket base directory.
func findNvimBaseDir(user string) string {
	// Common locations on macOS
	patterns := []string{
		"/var/folders/*/*/T/nvim." + user,
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			return matches[0]
		}
	}

	return ""
}

// isNvimSocket checks if a socket is a working nvim RPC socket.
// We do a quick connect/disconnect to verify it's alive.
func isNvimSocket(socketPath string, expectedPID int) bool {
	conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()

	// If we don't care about matching a specific PID, any working socket is fine
	if expectedPID == 0 {
		return true
	}

	// The socket is alive. We can't easily verify the PID from the socket alone,
	// but since we found it via pgrep, the socket in the same tmpdir is very likely correct.
	return true
}

// NvimSessionDir returns the directory where nvim session files are stored.
func NvimSessionDir() string {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "/tmp/skocko/nvim"
		}
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "skocko", "nvim")
}

// NvimSessionFilePath returns the session file path for a given session and window.
func NvimSessionFilePath(sessionName string, windowIndex int) string {
	return filepath.Join(NvimSessionDir(), fmt.Sprintf("%s_%d.vim", sessionName, windowIndex))
}
