package cmd

import (
	"fmt"

	"skocko/internal/session"
	"skocko/internal/tmux"

	"github.com/spf13/cobra"
)

var deleteFlag bool

var restoreCmd = &cobra.Command{
	Use:   "restore [session-name]",
	Short: "Restore a saved tmux session",
	Long:  "Restores a previously saved session, recreating windows and restarting processes.\nIf no session name is given, lists all saved sessions.\nUse --delete to remove saved state without restoring.",
	RunE:  runRestore,
}

func init() {
	restoreCmd.Flags().BoolVar(&deleteFlag, "delete", false, "delete saved state instead of restoring")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// List saved sessions
		names := session.ListSavedSessions()
		if len(names) == 0 {
			fmt.Println("No saved sessions found.")
			return nil
		}
		fmt.Println("Saved sessions:")
		for _, name := range names {
			snapshot, err := session.LoadSnapshot(name)
			status := ""
			if err == nil {
				status = fmt.Sprintf("(%d windows, saved %s)", len(snapshot.Windows), snapshot.SavedAt.Format("Jan 2 15:04"))
			}
			active := ""
			if tmux.SessionExists(name) {
				active = " [active]"
			}
			fmt.Printf("  %s %s%s\n", name, status, active)
		}
		return nil
	}

	name := args[0]

	// Delete mode
	if deleteFlag {
		if !session.HasSavedState(name) {
			return fmt.Errorf("no saved state for session %q", name)
		}
		if err := session.DeleteSavedState(name); err != nil {
			return fmt.Errorf("failed to delete saved state: %w", err)
		}
		fmt.Printf("Deleted saved state for %q\n", name)
		return nil
	}

	// Check if session is already running
	if tmux.SessionExists(name) {
		return fmt.Errorf("session %q is already running. Kill it first or use a different name", name)
	}

	// Load snapshot
	snapshot, err := session.LoadSnapshot(name)
	if err != nil {
		return fmt.Errorf("failed to load saved session %q: %w", name, err)
	}

	fmt.Printf("Restoring session %q (%d windows)...\n", snapshot.Name, len(snapshot.Windows))

	// Restore the session
	if err := session.RestoreSession(snapshot); err != nil {
		return fmt.Errorf("failed to restore session: %w", err)
	}

	// Clean up saved state after successful restore
	_ = session.DeleteSavedState(name)

	fmt.Printf("Session %q restored.\n", name)

	// Attach or switch to the restored session
	if tmux.IsInsideTmux() {
		return tmux.SwitchTo(name)
	}
	return tmux.Attach(name)
}
