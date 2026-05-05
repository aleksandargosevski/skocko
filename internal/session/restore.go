package session

import (
	"fmt"
	"os/exec"
	"time"
)

// RestoreSession recreates a tmux session from a saved snapshot.
// It creates the session, windows, and runs restore commands for each pane.
func RestoreSession(snapshot *SessionSnapshot) error {
	if len(snapshot.Windows) == 0 {
		// No windows to restore, just create a plain session
		return exec.Command("tmux", "new-session", "-d", "-s", snapshot.Name, "-c", snapshot.Path).Run()
	}

	// Create session with the first window
	firstWin := snapshot.Windows[0]
	firstPanePath := firstWin.Path
	if len(firstWin.Panes) > 0 && firstWin.Panes[0].Path != "" {
		firstPanePath = firstWin.Panes[0].Path
	}

	if err := exec.Command("tmux", "new-session", "-d",
		"-s", snapshot.Name,
		"-n", firstWin.Name,
		"-c", firstPanePath,
	).Run(); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Restore panes in first window
	restoreWindowPanes(snapshot.Name, firstWin)

	// Create remaining windows
	for _, win := range snapshot.Windows[1:] {
		winPath := win.Path
		if len(win.Panes) > 0 && win.Panes[0].Path != "" {
			winPath = win.Panes[0].Path
		}

		if err := exec.Command("tmux", "new-window",
			"-t", snapshot.Name,
			"-n", win.Name,
			"-c", winPath,
		).Run(); err != nil {
			continue
		}

		restoreWindowPanes(snapshot.Name, win)
	}

	// Apply layouts after all panes are created
	for _, win := range snapshot.Windows {
		if win.Layout != "" {
			target := fmt.Sprintf("%s:%s", snapshot.Name, win.Name)
			_ = exec.Command("tmux", "select-layout", "-t", target, win.Layout).Run()
		}
	}

	// Select the originally active window
	for _, win := range snapshot.Windows {
		if win.Active {
			target := fmt.Sprintf("%s:%s", snapshot.Name, win.Name)
			_ = exec.Command("tmux", "select-window", "-t", target).Run()
			break
		}
	}

	return nil
}

// restoreWindowPanes restores the panes in a window by running their restore commands.
func restoreWindowPanes(sessionName string, win WindowSnapshot) {
	if len(win.Panes) == 0 {
		return
	}

	// The first pane is already created with the window.
	// Run its restore command.
	firstPane := win.Panes[0]
	if firstPane.RestoreCommand != "" {
		target := fmt.Sprintf("%s:%s", sessionName, win.Name)
		// Small delay to let the shell initialize
		time.Sleep(50 * time.Millisecond)
		_ = exec.Command("tmux", "send-keys", "-t", target, firstPane.RestoreCommand, "Enter").Run()
	}

	// Create additional panes (if the window had splits)
	for _, pane := range win.Panes[1:] {
		panePath := pane.Path
		if panePath == "" {
			panePath = win.Path
		}

		target := fmt.Sprintf("%s:%s", sessionName, win.Name)
		if err := exec.Command("tmux", "split-window", "-t", target, "-c", panePath).Run(); err != nil {
			continue
		}

		if pane.RestoreCommand != "" {
			time.Sleep(50 * time.Millisecond)
			_ = exec.Command("tmux", "send-keys", "-t", target, pane.RestoreCommand, "Enter").Run()
		}
	}
}
