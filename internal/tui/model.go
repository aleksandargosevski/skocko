package tui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"skocko/internal/config"
	"skocko/internal/git"
	"skocko/internal/session"
	"skocko/internal/tmux"
	"github.com/sahilm/fuzzy"
)

// --- Messages ---

type sessionKilledMsg struct{ name string }
type sessionKillFailedMsg struct {
	name string
	err  error
}

type gitStatusLoadedMsg struct {
	statuses map[string]git.Status
}

type sessionSavedMsg struct{ name string }
type sessionSaveFailedMsg struct {
	name string
	err  error
}

type aiStatusLoadedMsg struct {
	statuses map[string]AIStatus
}

type savedDeletedMsg struct{ name string }
type savedDeleteFailedMsg struct {
	name string
	err  error
}



// --- State enums ---

type gitFetchState int

const (
	gitNotFetched gitFetchState = iota
	gitFetching
	gitFetched
)

type restorePromptState int

const (
	restoreNone    restorePromptState = iota // no prompt shown
	restoreAsking                            // showing "Restore? y/n" prompt
)

// --- Model ---

type Model struct {
	projectItems     []Item
	zoxideItems      []Item
	filtered         []Item
	input            textinput.Model
	cursor           int
	selected         *Item
	quitting         bool
	width            int
	height           int
	mode             ViewMode
	keys             config.Keybindings
	cfg              *config.Config
	gitStatusVisible bool
	gitStatusDetail  string
	gitStatusScope   string
	gitState         gitFetchState
	previewVisible   bool
	statusMessage    string             // transient message shown in title bar
	restoreState     restorePromptState // restore prompt state
	restoreItem      *Item              // item pending restore decision
	theme            *Theme
	hotkeysVisible   bool
	showBorder       bool
	aiStatuses       map[string]AIStatus // current AI statuses: "session:win.pane" -> status
}

func NewModel(projectItems []Item, zoxideItems []Item, cfg *config.Config, aiStatuses map[string]AIStatus) Model {
	t := GetTheme(cfg.Theme)

	if aiStatuses == nil {
		aiStatuses = make(map[string]AIStatus)
	}

	ti := textinput.New()
	ti.Placeholder = "Search projects..."
	ti.Focus()
	ti.CharLimit = 100
	ti.PromptStyle = lipgloss.NewStyle().Foreground(t.Accent)
	ti.Prompt = "  "
	ti.TextStyle = lipgloss.NewStyle().Foreground(t.Text)

	return Model{
		projectItems:     projectItems,
		zoxideItems:      zoxideItems,
		filtered:         projectItems,
		input:            ti,
		cursor:           0,
		width:            80,
		height:           24,
		theme:            t,
		hotkeysVisible:   cfg.ShowHotkeys,
		showBorder:       cfg.ShowBorder,
		aiStatuses:       aiStatuses,
		mode:             ModeProjects,
		keys:             cfg.Keybindings,
		cfg:              cfg,
		gitStatusVisible: cfg.GitStatus.ShowOnStart,
		gitStatusDetail:  cfg.GitStatus.Detail,
		gitStatusScope:   cfg.GitStatus.Scope,
		gitState:         gitNotFetched,
		previewVisible:   false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		// Fetch AI statuses async (doesn't block initial render)
		func() tea.Msg {
			statuses := DetectAllAIStatuses()
			if statuses == nil {
				statuses = make(map[string]AIStatus)
			}
			return aiStatusLoadedMsg{statuses: statuses}
		},
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sessionKilledMsg:
		m.onSessionKilled(msg.name)
		m.statusMessage = "Session killed: " + msg.name
		return m, nil

	case sessionKillFailedMsg:
		m.statusMessage = "Failed to kill: " + msg.name
		return m, nil

	case gitStatusLoadedMsg:
		m.applyGitStatuses(msg.statuses)
		m.gitState = gitFetched
		m.applyFilter()
		return m, nil

	case sessionSavedMsg:
		m.statusMessage = "Saved: " + msg.name
		// Update the saved state indicator on the item
		for i := range m.projectItems {
			if m.projectItems[i].Name == msg.name {
				m.projectItems[i].HasSavedState = true
			}
		}
		m.applyFilter()
		return m, nil

	case sessionSaveFailedMsg:
		m.statusMessage = "Save failed: " + msg.name
		return m, nil

	case savedDeletedMsg:
		m.statusMessage = "Deleted saved: " + msg.name
		for i := range m.projectItems {
			if m.projectItems[i].Name == msg.name {
				m.projectItems[i].HasSavedState = false
			}
		}
		m.applyFilter()
		return m, nil

	case savedDeleteFailedMsg:
		m.statusMessage = "Delete failed: " + msg.name
		return m, nil

	case aiStatusLoadedMsg:
		m.aiStatuses = msg.statuses
		if m.statusMessage == "Refreshing..." {
			m.statusMessage = "AI status refreshed"
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// Clear status message on any key press
		m.statusMessage = ""

		// If we're in restore prompt mode, handle y/n
		if m.restoreState == restoreAsking {
			switch key {
			case "y", "Y":
				m.restoreState = restoreNone
				item := *m.restoreItem
				item.HasSavedState = true // signal to cmd/root.go to restore
				m.selected = &item
				m.quitting = true
				return m, tea.Quit
			case "n", "N", "esc":
				// Skip restore, connect normally
				m.restoreState = restoreNone
				item := *m.restoreItem
				item.HasSavedState = false // signal: don't restore
				m.selected = &item
				m.quitting = true
				return m, tea.Quit
			}
			// Ignore other keys while prompting
			return m, nil
		}

		if key == m.keys.Zoxide {
			m.toggleMode()
			return m, nil
		}
		if key == m.keys.KillSession {
			return m.handleKillSession()
		}
		if key == m.keys.GitStatus {
			return m.handleGitToggle()
		}
		if key == m.keys.Preview {
			m.previewVisible = !m.previewVisible
			return m, nil
		}
		if key == m.keys.SaveSession {
			return m.handleSaveSession()
		}
		if key == m.keys.CopyPath {
			m.handleCopyPath()
			return m, nil
		}
		if key == m.keys.DeleteSaved {
			return m.handleDeleteSaved()
		}
		if key == m.keys.ToggleHelp {
			m.hotkeysVisible = !m.hotkeysVisible
			return m, nil
		}
		if key == m.keys.Refresh {
			m.statusMessage = "Refreshing..."
			return m, func() tea.Msg {
				statuses := DetectAllAIStatuses()
				if statuses == nil {
					statuses = make(map[string]AIStatus)
				}
				return aiStatusLoadedMsg{statuses: statuses}
			}
		}

		switch key {
		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				item := m.filtered[m.cursor]

				// If item is not active but has saved state, prompt for restore
				if !item.IsActive && item.HasSavedState {
					m.restoreState = restoreAsking
					m.restoreItem = &item
					return m, nil
				}

				m.selected = &item
				m.quitting = true
				return m, tea.Quit
			}
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.applyFilter()

	return m, cmd
}

// handleSaveSession saves the currently selected active session.
func (m Model) handleSaveSession() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return m, nil
	}

	item := m.filtered[m.cursor]
	if !item.IsActive {
		m.statusMessage = "Can only save active sessions"
		return m, nil
	}

	name := item.Name
	return m, func() tea.Msg {
		_, err := session.SaveSession(name)
		if err != nil {
			return sessionSaveFailedMsg{name: name, err: err}
		}
		return sessionSavedMsg{name: name}
	}
}

// handleCopyPath copies the selected item's path to the system clipboard.
func (m *Model) handleCopyPath() {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return
	}

	item := m.filtered[m.cursor]
	if item.Path == "" {
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, fall back to xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		}
	default:
		m.statusMessage = "Clipboard not supported on this OS"
		return
	}

	cmd.Stdin = strings.NewReader(item.Path)
	if err := cmd.Run(); err != nil {
		m.statusMessage = "Copy failed"
		return
	}

	m.statusMessage = "Copied: " + item.Path
}

// handleDeleteSaved deletes the saved state for the selected item.
func (m Model) handleDeleteSaved() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return m, nil
	}

	item := m.filtered[m.cursor]
	if !item.HasSavedState {
		m.statusMessage = "No saved state to delete"
		return m, nil
	}

	name := item.Name
	return m, func() tea.Msg {
		err := session.DeleteSavedState(name)
		if err != nil {
			return savedDeleteFailedMsg{name: name, err: err}
		}
		return savedDeletedMsg{name: name}
	}
}

// --- Git toggle ---

func (m Model) handleGitToggle() (tea.Model, tea.Cmd) {
	m.gitStatusVisible = !m.gitStatusVisible
	if m.gitStatusVisible && m.gitState == gitNotFetched {
		m.gitState = gitFetching
		return m, m.fetchGitStatus()
	}
	return m, nil
}

func (m Model) fetchGitStatus() tea.Cmd {
	var paths []string
	scope := m.gitStatusScope

	if scope == "active" {
		for _, item := range m.projectItems {
			if item.IsGit && item.IsActive {
				paths = append(paths, item.Path)
			}
		}
	} else {
		for _, item := range m.projectItems {
			if item.IsGit {
				paths = append(paths, item.Path)
			}
		}
	}

	return func() tea.Msg {
		if len(paths) == 0 {
			return gitStatusLoadedMsg{statuses: nil}
		}
		statuses := git.GetStatusParallel(paths)
		return gitStatusLoadedMsg{statuses: statuses}
	}
}

func (m *Model) applyGitStatuses(statuses map[string]git.Status) {
	if statuses == nil {
		return
	}
	for i := range m.projectItems {
		if !m.projectItems[i].IsGit {
			continue
		}
		absPath, err := filepath.Abs(m.projectItems[i].Path)
		if err != nil {
			continue
		}
		if s, ok := statuses[absPath]; ok {
			sCopy := s
			m.projectItems[i].GitStatus = &sCopy
		}
	}
}

// --- Mode toggle ---

func (m *Model) toggleMode() {
	if m.mode == ModeProjects {
		m.mode = ModeZoxide
		m.input.Placeholder = "Search zoxide..."
	} else {
		m.mode = ModeProjects
		m.input.Placeholder = "Search projects..."
	}
	m.input.SetValue("")
	m.cursor = 0
	m.applyFilter()
}

// --- Kill session ---

func (m Model) handleKillSession() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return m, nil
	}

	item := m.filtered[m.cursor]
	if !item.IsActive {
		return m, nil
	}

	name := item.Name
	return m, func() tea.Msg {
		err := tmux.KillSession(name)
		if err != nil {
			return sessionKillFailedMsg{name: name, err: err}
		}
		return sessionKilledMsg{name: name}
	}
}

func (m *Model) onSessionKilled(name string) {
	updated := make([]Item, 0, len(m.projectItems))
	var movedConfigured []Item
	var movedProjects []Item

	for _, item := range m.projectItems {
		if item.Name == name && item.IsActive {
			// Use OriginalSection to determine where the item goes back to
			targetSection := item.OriginalSection

			// Orphan sessions (OriginalSection == SectionActiveSessions) get dropped
			if targetSection == SectionActiveSessions {
				continue
			}

			if item.Path != "" {
				moved := Item{
					Name:            item.Name,
					Path:            item.Path,
					IsGit:           item.IsGit,
					IsActive:        false,
					Section:         targetSection,
					OriginalSection: targetSection,
					GitStatus:       item.GitStatus,
					WindowNames:     item.WindowNames,
					HasSavedState:   item.HasSavedState,
				}
				if targetSection == SectionConfigured {
					movedConfigured = append(movedConfigured, moved)
				} else {
					movedProjects = append(movedProjects, moved)
				}
			}
		} else {
			updated = append(updated, item)
		}
	}

	var result []Item
	configuredInserted := false
	projectsInserted := false

	for _, item := range updated {
		if item.Section == SectionConfigured && !configuredInserted {
			result = append(result, movedConfigured...)
			configuredInserted = true
		}
		if item.Section == SectionProjects && !projectsInserted {
			if !configuredInserted {
				result = append(result, movedConfigured...)
				configuredInserted = true
			}
			result = append(result, movedProjects...)
			projectsInserted = true
		}
		result = append(result, item)
	}
	if !configuredInserted {
		result = append(result, movedConfigured...)
	}
	if !projectsInserted {
		result = append(result, movedProjects...)
	}

	m.projectItems = result
	m.applyFilter()

	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// --- Filter ---

func (m *Model) activeSource() []Item {
	if m.mode == ModeZoxide {
		return m.zoxideItems
	}
	return m.projectItems
}

func (m *Model) applyFilter() {
	source := m.activeSource()
	query := strings.TrimSpace(m.input.Value())

	if query == "" {
		m.filtered = source
	} else {
		targets := make([]string, len(source))
		for i, item := range source {
			targets[i] = item.Name
		}

		matches := fuzzy.Find(query, targets)
		filtered := make([]Item, 0, len(matches))
		for _, match := range matches {
			filtered = append(filtered, source[match.Index])
		}
		m.filtered = filtered
	}

	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// --- View ---

func (m Model) View() string {
	if m.quitting && m.selected == nil {
		return ""
	}

	// Styles
	t := m.theme
	borderColor := t.Accent
	separatorStyle := lipgloss.NewStyle().Foreground(t.Separator)
	hintStyle := lipgloss.NewStyle().Foreground(t.Dim)
	loadingStyle := lipgloss.NewStyle().Foreground(t.Warning).Italic(true)

	borderPad := 0
	if m.showBorder {
		borderPad = 2 // left + right border chars
	}
	// dialogWidth is what lipgloss.Width is set to (total including padding)
	dialogWidth := m.width - borderPad
	// contentWidth is the usable area inside padding (what separators/text should fill)
	contentWidth := dialogWidth - 2 // -2 for Padding(0, 1) left + right
	if contentWidth < 40 {
		contentWidth = 40
	}

	// title(1) + input(1) + sep(1) + footer_sep(1) + footer(1-2) + padding(2)
	chrome := 7
	if m.hotkeysVisible {
		chrome += 2 // separator + full help line
	}
	if m.showBorder {
		chrome += 2 // top + bottom border
	}
	maxListHeight := m.height - chrome
	if maxListHeight < 5 {
		maxListHeight = 5
	}

	// Determine if we should show preview (need enough width)
	showPreview := m.previewVisible && contentWidth >= 60

	// Calculate widths for split layout
	listWidth := contentWidth
	previewWidth := 0
	if showPreview {
		previewWidth = contentWidth * 45 / 100
		listWidth = contentWidth - previewWidth - 1 // -1 for the vertical separator
	}

	var b strings.Builder

	// Title + mode tabs
	b.WriteString(m.renderTitleBar(loadingStyle))
	b.WriteString("\n")

	// Search input
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Separator
	b.WriteString(separatorStyle.Render(strings.Repeat("─", contentWidth)))
	b.WriteString("\n")

	// Main content area
	listContent := m.renderList(listWidth, maxListHeight)

	if showPreview {
		// Fetch preview data for the current item
		previewContent := ""
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			item := m.filtered[m.cursor]
			data := FetchPreviewData(item, m.cfg)
			previewContent = RenderPreview(data, previewWidth-2, maxListHeight, t)
		}

		// Vertical separator
		vertSep := separatorStyle.Render(
			strings.Repeat("│\n", maxListHeight),
		)
		vertSep = strings.TrimRight(vertSep, "\n")

		listBox := lipgloss.NewStyle().Width(listWidth).Height(maxListHeight).Render(listContent)
		sepBox := lipgloss.NewStyle().Width(1).Height(maxListHeight).Render(vertSep)
		previewBox := lipgloss.NewStyle().Width(previewWidth).Height(maxListHeight).PaddingLeft(1).Render(previewContent)

		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, listBox, sepBox, previewBox))
	} else {
		// Fixed height list area - prevents header from shifting when results change
		listBox := lipgloss.NewStyle().Height(maxListHeight).Render(listContent)
		b.WriteString(listBox)
	}

	// Help bar
	if m.hotkeysVisible {
		b.WriteString("\n")
		b.WriteString(separatorStyle.Render(strings.Repeat("─", contentWidth)))
		b.WriteString("\n")

		helpParts := []string{
			displayKey(m.keys.Zoxide) + " zoxide",
		}
		if m.mode == ModeProjects {
			helpParts = append(helpParts, displayKey(m.keys.KillSession)+" kill")
		}
		helpParts = append(helpParts, displayKey(m.keys.GitStatus)+" git")
		helpParts = append(helpParts, displayKey(m.keys.Preview)+" info")
		helpParts = append(helpParts, displayKey(m.keys.SaveSession)+" save")
		helpParts = append(helpParts, displayKey(m.keys.CopyPath)+" copy")
		helpParts = append(helpParts, displayKey(m.keys.DeleteSaved)+" del saved")
		helpParts = append(helpParts, displayKey(m.keys.Refresh)+" refresh")
		helpParts = append(helpParts, displayKey(m.keys.ToggleHelp)+" help")
		b.WriteString(hintStyle.Render("  " + strings.Join(helpParts, "  ")))
	} else {
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("  " + displayKey(m.keys.ToggleHelp) + " help"))
	}

	// Wrap content
	dialogStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(dialogWidth)

	if m.showBorder {
		dialogStyle = dialogStyle.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor)
	}

	dialog := dialogStyle.Render(b.String())

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}

func (m Model) renderTitleBar(loadingStyle lipgloss.Style) string {
	t := m.theme
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Accent).PaddingLeft(1)
	modeActiveStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	modeInactiveStyle := lipgloss.NewStyle().Foreground(t.Dim)
	statusStyle := lipgloss.NewStyle().Foreground(t.Success).Italic(true)
	promptStyle := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)

	title := titleStyle.Render("skocko")
	var projectsTab, zoxideTab string
	if m.mode == ModeProjects {
		projectsTab = modeActiveStyle.Render("[Projects]")
		zoxideTab = modeInactiveStyle.Render(" Zoxide")
	} else {
		projectsTab = modeInactiveStyle.Render(" Projects")
		zoxideTab = modeActiveStyle.Render("[Zoxide]")
	}
	line := title + "  " + projectsTab + "  " + zoxideTab

	if m.restoreState == restoreAsking {
		line += "  " + promptStyle.Render("Restore saved state? (y/n)")
	} else if m.gitState == gitFetching {
		line += "  " + loadingStyle.Render("fetching git...")
	} else if m.statusMessage != "" {
		line += "  " + statusStyle.Render(m.statusMessage)
	}

	return line
}

func (m Model) renderList(width, maxHeight int) string {
	t := m.theme
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Dim)
	separatorStyle := lipgloss.NewStyle().Foreground(t.Separator)
	selectedStyle := lipgloss.NewStyle().Foreground(t.Selected).Bold(true)
	selectedIconStyle := lipgloss.NewStyle().Foreground(t.SelectedIcon)
	normalStyle := lipgloss.NewStyle().Foreground(t.Text)
	normalIconStyle := lipgloss.NewStyle().Foreground(t.Subtext)
	pathStyle := lipgloss.NewStyle().Foreground(t.Dim)
	savedStyle := lipgloss.NewStyle().Foreground(t.Success)
	cursorStr := lipgloss.NewStyle().Foreground(t.Accent).Render("> ")

	var b strings.Builder

	if len(m.filtered) == 0 {
		b.WriteString(pathStyle.Render("  No matches found"))
		b.WriteString("\n")
		return b.String()
	}

	visibleStart, visibleEnd := scrollWindow(m.cursor, len(m.filtered), maxHeight)

	lastSection := SectionKind(-1)
	linesRendered := 0
	isFiltering := strings.TrimSpace(m.input.Value()) != ""

	for idx := visibleStart; idx < visibleEnd && linesRendered < maxHeight; idx++ {
		item := m.filtered[idx]

		// Section headers (only when not filtering - filtering mixes sections by relevance)
		if m.mode == ModeProjects && !isFiltering && item.Section != lastSection {
			if lastSection != -1 {
				sepWidth := width - 4
				if sepWidth < 10 {
					sepWidth = 10
				}
				b.WriteString(separatorStyle.Render("  " + strings.Repeat("─", sepWidth)))
				b.WriteString("\n")
				linesRendered++
				if linesRendered >= maxHeight {
					break
				}
			}

			header := "  Active Sessions"
			if item.Section == SectionConfigured {
				header = "  Configured"
			} else if item.Section == SectionProjects {
				header = "  Projects"
			}
			b.WriteString(sectionStyle.Render(header))
			b.WriteString("\n")
			linesRendered++
			lastSection = item.Section
			if linesRendered >= maxHeight {
				break
			}
		}

		// Item row
		icon := noIcon
		isSelected := idx == m.cursor

		badges := renderProcessBadges(item.ProcessIcons, t, m.aiStatuses)
		gitBadge := ""
		if m.gitStatusVisible && item.IsGit && item.GitStatus != nil {
			gitBadge = RenderGitStatus(*item.GitStatus, m.gitStatusDetail, t)
		}
		savedBadge := ""
		if item.HasSavedState {
			savedBadge = savedStyle.Render(savedIcon)
		}

		if isSelected {
			if item.IsGit {
				icon = selectedIconStyle.Render(gitIcon)
			}
			line := cursorStr + icon + " " + selectedStyle.Render(item.Name)
			if savedBadge != "" {
				line += " " + savedBadge
			}
			if gitBadge != "" {
				line += " " + gitBadge
			}
			if badges != "" {
				line += " " + badges
			}
			line += "  " + pathStyle.Render(item.Path)
			b.WriteString(line)
		} else {
			if item.IsGit {
				icon = normalIconStyle.Render(gitIcon)
			}
			line := "  " + icon + " " + normalStyle.Render(item.Name)
			if savedBadge != "" {
				line += " " + savedBadge
			}
			if gitBadge != "" {
				line += " " + gitBadge
			}
			if badges != "" {
				line += " " + badges
			}
			b.WriteString(line)
		}
		b.WriteString("\n")
		linesRendered++
	}

	return b.String()
}

func (m Model) Selected() *Item {
	return m.selected
}

// iconGroup groups consecutive icons of the same type for collapse rendering.
type iconGroup struct {
	icon      string
	colorRole string
	paneKeys  []string
	count     int
}

func renderProcessBadges(icons []ProcessIcon, t *Theme, aiStatuses map[string]AIStatus) string {
	if len(icons) == 0 {
		return ""
	}

	// Group consecutive icons by icon+colorRole
	var groups []iconGroup
	for _, pi := range icons {
		if len(groups) > 0 && groups[len(groups)-1].icon == pi.Icon && groups[len(groups)-1].colorRole == pi.ColorRole {
			groups[len(groups)-1].count++
			groups[len(groups)-1].paneKeys = append(groups[len(groups)-1].paneKeys, pi.PaneKey)
		} else {
			groups = append(groups, iconGroup{
				icon:      pi.Icon,
				colorRole: pi.ColorRole,
				paneKeys:  []string{pi.PaneKey},
				count:     1,
			})
		}
	}

	dimCount := lipgloss.NewStyle().Foreground(t.Dim)
	var parts []string

	for _, g := range groups {
		isAI := g.colorRole == "lavender"

		if isAI && g.count <= 2 {
			// Individual AI icons with per-instance status
			for _, key := range g.paneKeys {
				if key != "" && aiStatuses[key] == AIWorking {
					parts = append(parts, lipgloss.NewStyle().Foreground(t.AIWorking).Render(g.icon))
				} else {
					parts = append(parts, lipgloss.NewStyle().Foreground(ResolveIconColor(g.colorRole, t)).Render(g.icon))
				}
			}
		} else if isAI && g.count >= 3 {
			// Collapsed AI: yellow if any working
			anyWorking := false
			for _, key := range g.paneKeys {
				if key != "" && aiStatuses[key] == AIWorking {
					anyWorking = true
					break
				}
			}
			color := ResolveIconColor(g.colorRole, t)
			if anyWorking {
				color = t.AIWorking
			}
			parts = append(parts, lipgloss.NewStyle().Foreground(color).Render(g.icon)+dimCount.Render(fmt.Sprintf("×%d", g.count)))
		} else if g.count <= 2 {
			// Non-AI individual
			style := lipgloss.NewStyle().Foreground(ResolveIconColor(g.colorRole, t))
			for i := 0; i < g.count; i++ {
				parts = append(parts, style.Render(g.icon))
			}
		} else {
			// Non-AI collapsed
			style := lipgloss.NewStyle().Foreground(ResolveIconColor(g.colorRole, t))
			parts = append(parts, style.Render(g.icon)+dimCount.Render(fmt.Sprintf("×%d", g.count)))
		}
	}

	return strings.Join(parts, " ")
}

// displayKey returns a human-friendly label for a key binding.
func displayKey(key string) string {
	if key == "ctrl+_" {
		return "ctrl+/"
	}
	return key
}

func scrollWindow(cursor, total, maxVisible int) (start, end int) {
	if total <= maxVisible {
		return 0, total
	}

	half := maxVisible / 2
	start = cursor - half
	if start < 0 {
		start = 0
	}
	end = start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
	}
	return start, end
}
