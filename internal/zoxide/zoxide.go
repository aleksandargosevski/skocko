package zoxide

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Entry struct {
	Name  string
	Path  string
	IsGit bool
}

// Query returns all zoxide entries sorted by frecency (highest first).
// Returns an empty slice if zoxide is not installed or has no entries.
func Query() []Entry {
	out, err := exec.Command("zoxide", "query", "-l").Output()
	if err != nil {
		return nil
	}

	var entries []Entry
	seen := make(map[string]bool)

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Deduplicate
		if seen[line] {
			continue
		}
		seen[line] = true

		name := filepath.Base(line)

		entries = append(entries, Entry{
			Name:  name,
			Path:  line,
			IsGit: isGitRepo(line),
		})
	}

	return entries
}

func isGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir() || info.Mode().IsRegular()
}
