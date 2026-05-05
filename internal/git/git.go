package git

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Status holds the git status for a repository.
type Status struct {
	Dirty     bool // has staged or unstaged changes
	Ahead     int  // commits ahead of upstream
	Behind    int  // commits behind upstream
	Untracked int  // number of untracked files
}

// IsClean returns true if the repo has no changes and is in sync with upstream.
func (s Status) IsClean() bool {
	return !s.Dirty && s.Ahead == 0 && s.Behind == 0 && s.Untracked == 0
}

// GetStatus returns the git status for a single repository path.
// Returns a zero Status if the path is not a git repo or git fails.
func GetStatus(path string) Status {
	var s Status

	// Check if it's actually a git repo
	gitDir := filepath.Join(path, ".git")
	if _, err := exec.Command("git", "-C", path, "rev-parse", "--git-dir").Output(); err != nil {
		_ = gitDir
		return s
	}

	// git status --porcelain for dirty + untracked
	out, err := exec.Command("git", "-C", path, "status", "--porcelain").Output()
	if err != nil {
		return s
	}

	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 2 {
			continue
		}
		if line[0] == '?' && line[1] == '?' {
			s.Untracked++
		} else {
			s.Dirty = true
		}
	}

	// git rev-list --left-right --count HEAD...@{upstream} for ahead/behind
	// This will fail if there's no upstream set, which is fine (ahead/behind stay 0)
	out, err = exec.Command("git", "-C", path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}").Output()
	if err == nil {
		parts := strings.Fields(strings.TrimSpace(string(out)))
		if len(parts) == 2 {
			s.Ahead, _ = strconv.Atoi(parts[0])
			s.Behind, _ = strconv.Atoi(parts[1])
		}
	}

	return s
}

// GetStatusParallel runs GetStatus on multiple paths concurrently.
// Returns a map from absolute path to Status.
// Uses bounded concurrency to avoid fd exhaustion.
func GetStatusParallel(paths []string) map[string]Status {
	const maxConcurrency = 8

	results := make(map[string]Status, len(paths))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)

	for _, p := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			status := GetStatus(path)
			abs, err := filepath.Abs(path)
			if err != nil {
				abs = path
			}

			mu.Lock()
			results[abs] = status
			mu.Unlock()
		}(p)
	}

	wg.Wait()
	return results
}
