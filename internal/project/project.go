package project

import (
	"os"
	"path/filepath"
	"sort"
)

type Project struct {
	Name    string
	Path    string
	IsGit   bool
	ModTime int64 // unix timestamp of directory modification time
}

// Scan reads all immediate subdirectories from each project path
// and detects whether they are git repos.
// Results are sorted by modification time (most recent first).
func Scan(projectPaths []string) []Project {
	var projects []Project
	seen := make(map[string]bool)

	for _, base := range projectPaths {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			// Skip hidden directories
			if entry.Name()[0] == '.' {
				continue
			}

			fullPath := filepath.Join(base, entry.Name())

			// Deduplicate by absolute path
			abs, err := filepath.Abs(fullPath)
			if err != nil {
				abs = fullPath
			}
			if seen[abs] {
				continue
			}
			seen[abs] = true

			var modTime int64
			if info, err := os.Stat(fullPath); err == nil {
				modTime = info.ModTime().Unix()
			}

			projects = append(projects, Project{
				Name:    entry.Name(),
				Path:    fullPath,
				IsGit:   isGitRepo(fullPath),
				ModTime: modTime,
			})
		}
	}

	// Sort by modification time, most recent first
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ModTime > projects[j].ModTime
	})

	return projects
}

func isGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	// .git can be a directory (normal repo) or a file (worktree/submodule)
	return info.IsDir() || info.Mode().IsRegular()
}
