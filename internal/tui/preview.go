package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"skocko/internal/config"
)

// PreviewData holds all the info displayed in the preview panel.
type PreviewData struct {
	Name string
	Path string

	// Git info (empty if not a git repo)
	GitBranch string
	GitCommit string // short hash + message + relative time
	GitRemote string

	// tmux windows (only for active sessions)
	Windows []PreviewWindow

	// Configured windows (for non-active items with window config)
	ConfiguredWindows []config.WindowConfig

	IsActive bool
}

// PreviewWindow represents a tmux window in the preview panel.
type PreviewWindow struct {
	Index   string
	Name    string
	Panes   []PreviewPane
	Active  bool
}

// PreviewPane represents a pane within a window.
type PreviewPane struct {
	Command string
	Path    string
}

// FetchPreviewData gathers preview information for a single item.
// This is called synchronously per cursor move - total ~20ms for a single item.
func FetchPreviewData(item Item, cfg *config.Config) PreviewData {
	data := PreviewData{
		Name:     item.Name,
		Path:     item.Path,
		IsActive: item.IsActive,
	}

	// Git info
	if item.IsGit && item.Path != "" {
		data.GitBranch = gitBranch(item.Path)
		data.GitCommit = gitLastCommit(item.Path)
		data.GitRemote = gitRemote(item.Path)
	}

	// tmux windows for active sessions
	if item.IsActive {
		data.Windows = fetchTmuxWindows(item.Name)
	}

	// Configured windows for non-active items
	if !item.IsActive && len(item.WindowNames) > 0 && cfg != nil {
		data.ConfiguredWindows = cfg.ResolveWindows(item.WindowNames)
	}

	return data
}

// RenderPreview renders the preview panel content.
func RenderPreview(data PreviewData, width, height int, t *Theme) string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent)
	labelStyle := lipgloss.NewStyle().
		Foreground(t.Subtext)
	valueStyle := lipgloss.NewStyle().
		Foreground(t.Text)
	dimStyle := lipgloss.NewStyle().
		Foreground(t.Dim)
	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Subtext).
		MarginTop(1)
	activeStyle := lipgloss.NewStyle().
		Foreground(t.Success)
	windowStyle := lipgloss.NewStyle().
		Foreground(t.Text)
	paneStyle := lipgloss.NewStyle().
		Foreground(t.Dim)

	var lines []string

	// Name header
	lines = append(lines, headerStyle.Render(data.Name))
	lines = append(lines, dimStyle.Render(data.Path))
	lines = append(lines, "")

	// Git info
	if data.GitBranch != "" {
		lines = append(lines, sectionStyle.Render("Git"))
		lines = append(lines, labelStyle.Render("  branch ")+valueStyle.Render(data.GitBranch))
		if data.GitCommit != "" {
			lines = append(lines, labelStyle.Render("  commit ")+valueStyle.Render(data.GitCommit))
		}
		if data.GitRemote != "" {
			// Truncate long remote URLs
			remote := data.GitRemote
			maxRemote := width - 12
			if maxRemote > 0 && len(remote) > maxRemote {
				remote = remote[:maxRemote-3] + "..."
			}
			lines = append(lines, labelStyle.Render("  remote ")+dimStyle.Render(remote))
		}
		lines = append(lines, "")
	}

	// tmux windows for active sessions
	if data.IsActive && len(data.Windows) > 0 {
		lines = append(lines, sectionStyle.Render("Windows"))
		for _, w := range data.Windows {
			marker := "  "
			nameStr := windowStyle.Render(w.Name)
			if w.Active {
				marker = activeStyle.Render("* ")
				nameStr = activeStyle.Render(w.Name)
			}
			lines = append(lines, fmt.Sprintf("  %s%s %s", marker, dimStyle.Render(w.Index+":"), nameStr))
			for _, p := range w.Panes {
				lines = append(lines, paneStyle.Render(fmt.Sprintf("      %s", p.Command)))
			}
		}
	}

	// Configured windows for non-active items
	if !data.IsActive && len(data.ConfiguredWindows) > 0 {
		lines = append(lines, sectionStyle.Render("Windows (on create)"))
		for _, w := range data.ConfiguredWindows {
			lines = append(lines, fmt.Sprintf("  %s  %s", windowStyle.Render(w.Name), dimStyle.Render(w.Command)))
		}
	}

	// Status line
	if !data.IsActive {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("  session not running"))
	}

	// Truncate to available height
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// --- git helpers (single-item, fast) ---

func gitBranch(path string) string {
	out, err := exec.Command("git", "-C", path, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitLastCommit(path string) string {
	out, err := exec.Command("git", "-C", path, "log", "-1", "--format=%h %s (%cr)").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitRemote(path string) string {
	out, err := exec.Command("git", "-C", path, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// --- tmux window/pane helpers ---

func fetchTmuxWindows(sessionName string) []PreviewWindow {
	// Get window list
	out, err := exec.Command("tmux", "list-windows", "-t", sessionName,
		"-F", "#{window_index}\t#{window_name}\t#{window_active}").Output()
	if err != nil {
		return nil
	}

	var windows []PreviewWindow
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		w := PreviewWindow{
			Index:  parts[0],
			Name:   parts[1],
			Active: parts[2] == "1",
		}

		// Get panes for this window
		paneOut, err := exec.Command("tmux", "list-panes",
			"-t", sessionName+":"+parts[0],
			"-F", "#{pane_current_command}\t#{pane_current_path}").Output()
		if err == nil {
			for _, paneLine := range strings.Split(strings.TrimSpace(string(paneOut)), "\n") {
				if paneLine == "" {
					continue
				}
				paneParts := strings.SplitN(paneLine, "\t", 2)
				pane := PreviewPane{Command: paneParts[0]}
				if len(paneParts) > 1 {
					pane.Path = paneParts[1]
				}
				w.Panes = append(w.Panes, pane)
			}
		}

		windows = append(windows, w)
	}

	return windows
}
