package tui

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette for the TUI.
type Theme struct {
	Name string

	// UI chrome
	Accent       lipgloss.Color // title, border, cursor, selected items
	Dim          lipgloss.Color // section headers, paths, help text
	Separator    lipgloss.Color // horizontal lines
	Text         lipgloss.Color // normal item names
	Subtext      lipgloss.Color // secondary text, unselected icons

	// Semantic
	Selected     lipgloss.Color // selected item name
	SelectedIcon lipgloss.Color // selected git/process icon
	Success      lipgloss.Color // git ahead, saved state, status messages
	Warning      lipgloss.Color // git dirty, loading, prompts
	Error        lipgloss.Color // git behind
	Info         lipgloss.Color // inactive tabs, subtle info
	Lavender     lipgloss.Color // AI icons, special highlights

	// Process icon colors (themed for harmony)
	Green     lipgloss.Color // node, nvim
	Blue      lipgloss.Color // docker, go
	Peach     lipgloss.Color // lazygit, rust
	Red       lipgloss.Color // ruby, java
	Yellow    lipgloss.Color // python
	Teal      lipgloss.Color // databases
	Pink      lipgloss.Color // misc
	AIWorking lipgloss.Color // AI tool actively streaming
}

// GetTheme returns a theme by name. Falls back to catppuccin-mocha if not found.
func GetTheme(name string) *Theme {
	switch name {
	case "catppuccin-mocha":
		return catppuccinMocha()
	case "catppuccin-macchiato":
		return catppuccinMacchiato()
	case "catppuccin-frappe":
		return catppuccinFrappe()
	case "catppuccin-latte":
		return catppuccinLatte()
	case "default":
		return defaultTheme()
	default:
		return catppuccinMocha()
	}
}

func catppuccinMocha() *Theme {
	return &Theme{
		Name:         "catppuccin-mocha",
		Accent:       lipgloss.Color("#cba6f7"), // mauve
		Dim:          lipgloss.Color("#7f849c"), // overlay1
		Separator:    lipgloss.Color("#313244"), // surface0
		Text:         lipgloss.Color("#cdd6f4"), // text
		Subtext:      lipgloss.Color("#a6adc8"), // subtext0
		Selected:     lipgloss.Color("#cba6f7"), // mauve
		SelectedIcon: lipgloss.Color("#fab387"), // peach
		Success:      lipgloss.Color("#a6e3a1"), // green
		Warning:      lipgloss.Color("#f9e2af"), // yellow
		Error:        lipgloss.Color("#f38ba8"), // red
		Info:         lipgloss.Color("#7f849c"), // overlay1
		Lavender:     lipgloss.Color("#b4befe"), // lavender
		Green:        lipgloss.Color("#a6e3a1"), // green
		Blue:         lipgloss.Color("#89b4fa"), // blue
		Peach:        lipgloss.Color("#fab387"), // peach
		Red:          lipgloss.Color("#f38ba8"), // red
		Yellow:       lipgloss.Color("#f9e2af"), // yellow
		Teal:         lipgloss.Color("#94e2d5"), // teal
		Pink:         lipgloss.Color("#f5c2e7"), // pink
		AIWorking:    lipgloss.Color("#f9e2af"), // yellow - stands out as "busy"
	}
}

func catppuccinMacchiato() *Theme {
	return &Theme{
		Name:         "catppuccin-macchiato",
		Accent:       lipgloss.Color("#c6a0f6"), // mauve
		Dim:          lipgloss.Color("#8087a2"), // overlay1
		Separator:    lipgloss.Color("#363a4f"), // surface0
		Text:         lipgloss.Color("#cad3f5"), // text
		Subtext:      lipgloss.Color("#a5adcb"), // subtext0
		Selected:     lipgloss.Color("#c6a0f6"), // mauve
		SelectedIcon: lipgloss.Color("#f5a97f"), // peach
		Success:      lipgloss.Color("#a6da95"), // green
		Warning:      lipgloss.Color("#eed49f"), // yellow
		Error:        lipgloss.Color("#ed8796"), // red
		Info:         lipgloss.Color("#8087a2"), // overlay1
		Lavender:     lipgloss.Color("#b7bdf8"), // lavender
		Green:        lipgloss.Color("#a6da95"),
		Blue:         lipgloss.Color("#8aadf4"),
		Peach:        lipgloss.Color("#f5a97f"),
		Red:          lipgloss.Color("#ed8796"),
		Yellow:       lipgloss.Color("#eed49f"),
		Teal:         lipgloss.Color("#8bd5ca"),
		Pink:         lipgloss.Color("#f5bde6"),
		AIWorking:    lipgloss.Color("#eed49f"), // yellow
	}
}

func catppuccinFrappe() *Theme {
	return &Theme{
		Name:         "catppuccin-frappe",
		Accent:       lipgloss.Color("#ca9ee6"), // mauve
		Dim:          lipgloss.Color("#838ba7"), // overlay1
		Separator:    lipgloss.Color("#414559"), // surface0
		Text:         lipgloss.Color("#c6d0f5"), // text
		Subtext:      lipgloss.Color("#a5adce"), // subtext0
		Selected:     lipgloss.Color("#ca9ee6"), // mauve
		SelectedIcon: lipgloss.Color("#ef9f76"), // peach
		Success:      lipgloss.Color("#a6d189"), // green
		Warning:      lipgloss.Color("#e5c890"), // yellow
		Error:        lipgloss.Color("#e78284"), // red
		Info:         lipgloss.Color("#838ba7"), // overlay1
		Lavender:     lipgloss.Color("#babbf1"), // lavender
		Green:        lipgloss.Color("#a6d189"),
		Blue:         lipgloss.Color("#8caaee"),
		Peach:        lipgloss.Color("#ef9f76"),
		Red:          lipgloss.Color("#e78284"),
		Yellow:       lipgloss.Color("#e5c890"),
		Teal:         lipgloss.Color("#81c8be"),
		Pink:         lipgloss.Color("#f4b8e4"),
		AIWorking:    lipgloss.Color("#e5c890"), // yellow
	}
}

func catppuccinLatte() *Theme {
	return &Theme{
		Name:         "catppuccin-latte",
		Accent:       lipgloss.Color("#8839ef"), // mauve
		Dim:          lipgloss.Color("#8c8fa1"), // overlay1
		Separator:    lipgloss.Color("#ccd0da"), // surface0
		Text:         lipgloss.Color("#4c4f69"), // text
		Subtext:      lipgloss.Color("#6c6f85"), // subtext0
		Selected:     lipgloss.Color("#8839ef"), // mauve
		SelectedIcon: lipgloss.Color("#fe640b"), // peach
		Success:      lipgloss.Color("#40a02b"), // green
		Warning:      lipgloss.Color("#df8e1d"), // yellow
		Error:        lipgloss.Color("#d20f39"), // red
		Info:         lipgloss.Color("#8c8fa1"), // overlay1
		Lavender:     lipgloss.Color("#7287fd"), // lavender
		Green:        lipgloss.Color("#40a02b"),
		Blue:         lipgloss.Color("#1e66f5"),
		Peach:        lipgloss.Color("#fe640b"),
		Red:          lipgloss.Color("#d20f39"),
		Yellow:       lipgloss.Color("#df8e1d"),
		Teal:         lipgloss.Color("#179299"),
		Pink:         lipgloss.Color("#ea76cb"),
		AIWorking:    lipgloss.Color("#df8e1d"), // yellow
	}
}

func defaultTheme() *Theme {
	return &Theme{
		Name:         "default",
		Accent:       lipgloss.Color("170"),
		Dim:          lipgloss.Color("240"),
		Separator:    lipgloss.Color("236"),
		Text:         lipgloss.Color("252"),
		Subtext:      lipgloss.Color("246"),
		Selected:     lipgloss.Color("170"),
		SelectedIcon: lipgloss.Color("208"),
		Success:      lipgloss.Color("114"),
		Warning:      lipgloss.Color("220"),
		Error:        lipgloss.Color("203"),
		Info:         lipgloss.Color("240"),
		Lavender:     lipgloss.Color("183"),
		Green:        lipgloss.Color("70"),
		Blue:         lipgloss.Color("75"),
		Peach:        lipgloss.Color("208"),
		Red:          lipgloss.Color("160"),
		Yellow:       lipgloss.Color("220"),
		Teal:         lipgloss.Color("39"),
		Pink:         lipgloss.Color("183"),
		AIWorking:    lipgloss.Color("220"),
	}
}
