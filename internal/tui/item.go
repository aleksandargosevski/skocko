package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"skocko/internal/git"
	"skocko/internal/tmux"
)

// SectionKind distinguishes between active sessions, configured, projects, and zoxide entries.
type SectionKind int

const (
	SectionActiveSessions SectionKind = iota
	SectionConfigured
	SectionProjects
	SectionZoxide
)

const (
	gitIcon   = "󰊢" // Nerd Font git icon
	noIcon    = " "  // blank space to keep alignment
	savedIcon = "󰆓" // floppy disk / save icon
)

// ViewMode represents which list the TUI is currently showing.
type ViewMode int

const (
	ModeProjects ViewMode = iota
	ModeZoxide
)

// ProcessIcon maps a detected process to a nerd font icon.
type ProcessIcon struct {
	Icon      string
	ColorRole string // semantic color role: "green", "blue", "peach", "red", "yellow", "teal", "lavender", "subtext"
	PaneKey   string // "session:window.pane" for per-instance AI status lookup
}

// Known process icons (order matters for display).
var processIcons = []struct {
	Names     []string
	Icon      string
	ColorRole string
}{
	{Names: []string{"node", "npm", "npx", "bun", "deno"}, Icon: "󰎙", ColorRole: "green"},
	{Names: []string{"nvim"}, Icon: "", ColorRole: "green"},
	{Names: []string{"opencode", "claude", "aider", "pi", "cursor", "copilot"}, Icon: "󰚩", ColorRole: "lavender"},
	{Names: []string{"lazygit"}, Icon: "󰊢", ColorRole: "peach"},
	{Names: []string{"docker", "docker-compose"}, Icon: "󰡨", ColorRole: "blue"},
	{Names: []string{"python", "python3", "pip"}, Icon: "󰌠", ColorRole: "yellow"},
	{Names: []string{"go"}, Icon: "󰟓", ColorRole: "blue"},
	{Names: []string{"ruby", "rails", "bundle"}, Icon: "", ColorRole: "red"},
	{Names: []string{"cargo", "rustc"}, Icon: "󱘗", ColorRole: "peach"},
	{Names: []string{"java", "mvn", "gradle"}, Icon: "", ColorRole: "red"},
	{Names: []string{"ssh"}, Icon: "󰣀", ColorRole: "subtext"},
	{Names: []string{"postgres", "psql", "mysql", "redis-server"}, Icon: "󰆼", ColorRole: "teal"},
}

// DetectProcessIcons returns one icon per running process instance.
// sessionName is used to build pane keys for AI status lookups.
func DetectProcessIcons(processes []tmux.PaneProcess, sessionName string) []ProcessIcon {
	var icons []ProcessIcon
	for _, pi := range processIcons {
		for _, proc := range processes {
			for _, name := range pi.Names {
				if strings.EqualFold(proc.Command, name) {
					paneKey := ""
					if pi.ColorRole == "lavender" {
						paneKey = sessionName + ":" + proc.WindowIndex + "." + proc.PaneIndex
					}
					icons = append(icons, ProcessIcon{
						Icon:      pi.Icon,
						ColorRole: pi.ColorRole,
						PaneKey:   paneKey,
					})
					break
				}
			}
		}
	}
	return icons
}

// ResolveIconColor resolves a color role to a lipgloss.Color using the theme.
func ResolveIconColor(role string, t *Theme) lipgloss.Color {
	switch role {
	case "green":
		return t.Green
	case "blue":
		return t.Blue
	case "peach":
		return t.Peach
	case "red":
		return t.Red
	case "yellow":
		return t.Yellow
	case "teal":
		return t.Teal
	case "pink":
		return t.Pink
	case "lavender":
		return t.Lavender
	case "subtext":
		return t.Subtext
	default:
		return t.Text
	}
}

// RenderGitStatus renders git status badges based on the detail level.
func RenderGitStatus(s git.Status, detail string, t *Theme) string {
	if s.IsClean() {
		return ""
	}

	dirtyStyle := lipgloss.NewStyle().Foreground(t.Warning)
	aheadStyle := lipgloss.NewStyle().Foreground(t.Success)
	behindStyle := lipgloss.NewStyle().Foreground(t.Error)
	untrackedStyle := lipgloss.NewStyle().Foreground(t.Subtext)

	var parts []string

	if s.Dirty {
		parts = append(parts, dirtyStyle.Render("*"))
	}

	if detail == "ahead_behind" || detail == "full" {
		if s.Ahead > 0 {
			parts = append(parts, aheadStyle.Render(fmt.Sprintf("↑%d", s.Ahead)))
		}
		if s.Behind > 0 {
			parts = append(parts, behindStyle.Render(fmt.Sprintf("↓%d", s.Behind)))
		}
	}

	if detail == "full" {
		if s.Untracked > 0 {
			parts = append(parts, untrackedStyle.Render(fmt.Sprintf("?%d", s.Untracked)))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "")
}

// Item represents a single entry in the TUI list.
type Item struct {
	Name            string
	Path            string
	IsGit           bool
	IsActive        bool          // true if there's an active tmux session
	Section         SectionKind
	OriginalSection SectionKind   // section before promotion to Active (for kill->restore)
	ProcessIcons    []ProcessIcon // detected running process icons
	SortTime        int64         // unix timestamp for sorting (activity or mtime)
	GitStatus       *git.Status   // nil if git status not available/loaded
	WindowNames     []string      // window names to create on session start (references config)
	HasSavedState   bool          // true if there's a saved snapshot on disk
}
