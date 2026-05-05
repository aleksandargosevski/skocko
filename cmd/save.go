package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"skocko/internal/session"
	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save [session-name]",
	Short: "Save tmux session state to disk",
	Long:  "Saves the current state of a tmux session (windows, processes, editor state) for later restoration.\nIf no session name is given, saves all active sessions.",
	RunE:  runSave,
}

func init() {
	rootCmd.AddCommand(saveCmd)
}

func runSave(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		// Save specific session
		name := args[0]
		snapshot, err := session.SaveSession(name)
		if err != nil {
			return fmt.Errorf("failed to save session %q: %w", name, err)
		}
		fmt.Printf("Saved session %q (%d windows)\n", snapshot.Name, len(snapshot.Windows))
		return nil
	}

	// Save all active sessions
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return fmt.Errorf("no tmux sessions found")
	}

	var saved, failed int
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		snapshot, err := session.SaveSession(name)
		if err != nil {
			fmt.Printf("  Failed to save %q: %v\n", name, err)
			failed++
			continue
		}
		fmt.Printf("  Saved %q (%d windows)\n", snapshot.Name, len(snapshot.Windows))
		saved++
	}

	fmt.Printf("\nSaved %d sessions", saved)
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	fmt.Println()

	return nil
}
