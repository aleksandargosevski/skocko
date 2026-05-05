package tui

import (
	"os"
	"path/filepath"
	"sort"

	"skocko/internal/config"
	"skocko/internal/project"
	"skocko/internal/tmux"
	"skocko/internal/zoxide"
)

// BuildItems constructs the item list from active tmux sessions, configured sessions, and scanned projects.
// Order: Active Sessions > Configured (non-active) > Projects (non-active).
func BuildItems(projects []project.Project, sessions []tmux.Session, cfg *config.Config) []Item {
	sessionByName := make(map[string]tmux.Session)
	sessionByPath := make(map[string]tmux.Session)
	matched := make(map[string]bool) // tracks which tmux sessions have been matched

	for _, s := range sessions {
		sessionByName[s.Name] = s
		if s.Path != "" {
			abs, err := filepath.Abs(s.Path)
			if err == nil {
				sessionByPath[abs] = s
			}
		}
	}

	// Track which configured session paths are already handled
	configuredPaths := make(map[string]bool)
	for _, sc := range cfg.Sessions {
		abs, err := filepath.Abs(sc.Path)
		if err == nil {
			configuredPaths[abs] = true
		}
	}

	var activeItems []Item
	var configuredItems []Item
	var projectItems []Item

	// --- Process configured sessions ---
	for _, sc := range cfg.Sessions {
		absPath, _ := filepath.Abs(sc.Path)
		isGit := isGitRepo(absPath)
		windowNames := sc.Windows

		sess, byName := sessionByName[sc.Name]
		sessByPath, byPath := sessionByPath[absPath]

		if byName || byPath {
			// Configured session is active -> goes to Active Sessions
			s := sess
			if !byName && byPath {
				s = sessByPath
			}
			activeItems = append(activeItems, Item{
				Name:            sc.Name,
				Path:            sc.Path,
				IsGit:           isGit,
				IsActive:        true,
				Section:         SectionActiveSessions,
				OriginalSection: SectionConfigured,
				ProcessIcons:    DetectProcessIcons(s.Processes, sc.Name),
				SortTime:        s.Activity,
				WindowNames:     windowNames,
			})
			matched[sc.Name] = true
			if byPath {
				matched[sessByPath.Name] = true
			}
		} else {
			// Not active -> Configured section
			configuredItems = append(configuredItems, Item{
				Name:            sc.Name,
				Path:            sc.Path,
				IsGit:           isGit,
				IsActive:        false,
				Section:         SectionConfigured,
				OriginalSection: SectionConfigured,
				WindowNames:     windowNames,
			})
		}
	}

	// --- Process scanned projects ---
	for _, p := range projects {
		absPath, _ := filepath.Abs(p.Path)

		// Skip if this path is a configured session (already handled above)
		if configuredPaths[absPath] {
			continue
		}

		sess, byName := sessionByName[p.Name]
		sessByPath, byPath := sessionByPath[absPath]

		// Resolve window names from project_defaults
		var windowNames []string
		if wins := cfg.FindProjectWindows(p.Path); len(wins) > 0 {
			for _, w := range wins {
				windowNames = append(windowNames, w.Name)
			}
		}

		if byName || byPath {
			s := sess
			if !byName && byPath {
				s = sessByPath
			}
			activeItems = append(activeItems, Item{
				Name:            p.Name,
				Path:            p.Path,
				IsGit:           p.IsGit,
				IsActive:        true,
				Section:         SectionActiveSessions,
				OriginalSection: SectionProjects,
				ProcessIcons:    DetectProcessIcons(s.Processes, p.Name),
				SortTime:        s.Activity,
				WindowNames:     windowNames,
			})
			matched[p.Name] = true
			if byPath {
				matched[sessByPath.Name] = true
			}
		} else {
			projectItems = append(projectItems, Item{
				Name:            p.Name,
				Path:            p.Path,
				IsGit:           p.IsGit,
				IsActive:        false,
				Section:         SectionProjects,
				OriginalSection: SectionProjects,
				SortTime:        p.ModTime,
				WindowNames:     windowNames,
			})
		}
	}

	// --- Add remaining tmux sessions (not matched to any project or configured session) ---
	for _, s := range sessions {
		if !matched[s.Name] {
			activeItems = append(activeItems, Item{
				Name:         s.Name,
				Path:         s.Path,
				IsGit:        false,
				IsActive:     true,
				Section:      SectionActiveSessions,
				ProcessIcons: DetectProcessIcons(s.Processes, s.Name),
				SortTime:     s.Activity,
			})
		}
	}

	// Sort each section
	sort.Slice(activeItems, func(i, j int) bool {
		return activeItems[i].SortTime > activeItems[j].SortTime
	})
	// Configured items keep config order (intentional)
	sort.Slice(projectItems, func(i, j int) bool {
		return projectItems[i].SortTime > projectItems[j].SortTime
	})

	var items []Item
	items = append(items, activeItems...)
	items = append(items, configuredItems...)
	items = append(items, projectItems...)

	return items
}

// BuildZoxideItems converts zoxide entries into TUI items.
func BuildZoxideItems(entries []zoxide.Entry) []Item {
	items := make([]Item, 0, len(entries))
	for _, e := range entries {
		items = append(items, Item{
			Name:     e.Name,
			Path:     e.Path,
			IsGit:    e.IsGit,
			IsActive: false,
			Section:  SectionZoxide,
		})
	}
	return items
}

func isGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir() || info.Mode().IsRegular()
}
