package session

import "time"

// RestoreStrategy defines how a process should be restored.
type RestoreStrategy string

const (
	StrategyRerun            RestoreStrategy = "rerun"             // re-run the same command
	StrategyNvimSession      RestoreStrategy = "nvim_session"      // restore via nvim -S <file>
	StrategyOpencodeContinue RestoreStrategy = "opencode_continue" // restore via opencode --continue
	StrategyClaudeContinue   RestoreStrategy = "claude_continue"   // restore via claude --continue
	StrategyShell            RestoreStrategy = "shell"             // just open a shell
)

// PaneSnapshot holds the state of a single tmux pane.
type PaneSnapshot struct {
	Index          int             `json:"index"`
	Command        string          `json:"command"`          // current running command
	Path           string          `json:"path"`             // pane working directory
	Strategy       RestoreStrategy `json:"strategy"`         // how to restore this pane
	RestoreCommand string          `json:"restore_command"`  // exact command to run on restore
	NvimSessionFile string         `json:"nvim_session_file,omitempty"` // path to nvim session file
}

// WindowSnapshot holds the state of a single tmux window.
type WindowSnapshot struct {
	Index  int            `json:"index"`
	Name   string         `json:"name"`
	Layout string         `json:"layout"` // tmux layout string for pane arrangement
	Path   string         `json:"path"`   // window working directory
	Active bool           `json:"active"` // was this the active window
	Panes  []PaneSnapshot `json:"panes"`
}

// SessionSnapshot holds the complete state of a tmux session.
type SessionSnapshot struct {
	Name    string           `json:"name"`
	Path    string           `json:"path"`     // session root path
	SavedAt time.Time        `json:"saved_at"`
	Windows []WindowSnapshot `json:"windows"`
}
