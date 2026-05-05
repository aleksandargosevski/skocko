package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"skocko/internal/config"
	"skocko/internal/notify"
	"skocko/internal/tui"

	"github.com/spf13/cobra"
)

var (
	watchInterval int
	watchDaemon   bool
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Monitor AI sessions and notify when they finish",
	Long:  "Monitors tmux sessions for AI tools (opencode, claude, aider, pi).\nSends a desktop notification when an AI tool finishes streaming.\n\nUse --daemon to run in the background.",
	RunE:  runWatch,
}

func init() {
	watchCmd.Flags().IntVar(&watchInterval, "interval", 0, "poll interval in seconds (0 = use config value)")
	watchCmd.Flags().BoolVarP(&watchDaemon, "daemon", "d", false, "run in background (detach from terminal)")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	// If --daemon, re-exec ourselves detached from the terminal
	if watchDaemon {
		return daemonize()
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	interval := cfg.AIStatus.PollInterval
	if watchInterval > 0 {
		interval = watchInterval
	}
	if interval < 1 {
		interval = 3
	}

	fmt.Printf("Watching AI sessions (poll every %ds). Press Ctrl+C to stop.\n", interval)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	prevStatuses := make(map[string]tui.AIStatus)

	for {
		select {
		case <-sigCh:
			fmt.Println("\nStopped watching.")
			return nil
		case <-ticker.C:
			// Use non-tmux detection (only ps, no tmux commands)
			// This avoids interfering with tmux popups
			statuses := tui.DetectAllAIStatusesNonTmux()
			if statuses == nil {
				statuses = make(map[string]tui.AIStatus)
			}

			prevWorking := make(map[string]bool)
			for key, status := range prevStatuses {
				if status == tui.AIWorking {
					prevWorking[key] = true
				}
			}

			nowWorking := make(map[string]bool)
			for key, status := range statuses {
				if status == tui.AIWorking {
					nowWorking[key] = true
				}
			}

			// Detect any transition from working to idle
			for key := range prevWorking {
				if !nowWorking[key] {
					notify.Send("skocko", "AI finished")
				}
			}

			prevStatuses = statuses
		}
	}
}

// daemonize re-execs the watch command as a detached background process.
func daemonize() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not find executable: %w", err)
	}

	args := []string{"watch"}
	if watchInterval > 0 {
		args = append(args, "--interval", fmt.Sprintf("%d", watchInterval))
	}
	if cfgFile != "" {
		args = append(args, "--config", cfgFile)
	}

	cmd := exec.Command(exe, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // create new session, fully detach
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Printf("Watching AI sessions in background (pid %d)\n", cmd.Process.Pid)
	return nil
}
