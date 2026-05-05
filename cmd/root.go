package cmd

import (
	"fmt"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"skocko/internal/config"
	"skocko/internal/project"
	"skocko/internal/session"
	"skocko/internal/tmux"
	"skocko/internal/tui"
	"skocko/internal/zoxide"
)

var (
	version = "dev"
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:     "skocko",
	Short:   "Smart tmux session manager",
	Long:    "skocko is a TUI for managing tmux sessions based on your project directories.",
	Version: version,
	RunE:    run,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/skocko/skocko.yaml)")
}

func run(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.ProjectPaths) == 0 && len(cfg.Sessions) == 0 {
		return fmt.Errorf("no project_paths or sessions configured\n\nCreate a config file at ~/.config/skocko/skocko.yaml:\n\n  project_paths:\n    - ~/Sites\n    - ~/projects\n")
	}

	// Fetch data sources in parallel (AI statuses fetched async after TUI starts)
	var (
		projects      []project.Project
		sessions      []tmux.Session
		zoxideEntries []zoxide.Entry
		wg            sync.WaitGroup
	)

	wg.Add(3)
	go func() { defer wg.Done(); projects = project.Scan(cfg.ProjectPaths) }()
	go func() { defer wg.Done(); sessions = tmux.ListSessions() }()
	go func() { defer wg.Done(); zoxideEntries = zoxide.Query() }()
	wg.Wait()

	projectItems := tui.BuildItems(projects, sessions, cfg)

	// Mark items that have saved state on disk
	for i := range projectItems {
		if session.HasSavedState(projectItems[i].Name) {
			projectItems[i].HasSavedState = true
		}
	}

	zoxideItems := tui.BuildZoxideItems(zoxideEntries)

	if len(projectItems) == 0 && len(zoxideItems) == 0 {
		return fmt.Errorf("no projects found in configured paths and no zoxide entries")
	}

	// AI statuses loaded async after TUI renders (saves ~60ms startup)
	model := tui.NewModel(projectItems, zoxideItems, cfg, nil)
	p := tea.NewProgram(model)

	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	m, ok := result.(tui.Model)
	if !ok {
		return nil
	}

	selected := m.Selected()
	if selected == nil {
		return nil
	}

	// If user chose to restore saved state
	if !selected.IsActive && selected.HasSavedState {
		snapshot, err := session.LoadSnapshot(selected.Name)
		if err == nil {
			if err := session.RestoreSession(snapshot); err == nil {
				// Restore succeeded - clean up saved data
				_ = session.DeleteSavedState(selected.Name)

				if tmux.IsInsideTmux() {
					return tmux.SwitchTo(selected.Name)
				}
				return tmux.Attach(selected.Name)
			}
			// Restore failed, fall through to normal connect
		}
	}

	// Normal connect: resolve window definitions from config
	var windowDefs []tmux.WindowDef
	if len(selected.WindowNames) > 0 {
		resolved := cfg.ResolveWindows(selected.WindowNames)
		for _, w := range resolved {
			windowDefs = append(windowDefs, tmux.WindowDef{
				Name:    w.Name,
				Command: w.Command,
			})
		}
	}

	return tmux.ConnectToSession(selected.Name, selected.Path, windowDefs)
}
