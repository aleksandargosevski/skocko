package tui

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// OpenCode v2 runs only a TUI client inside the pane while a shared background
// service does the actual work, so the pane's CPU usage says nothing about
// whether it is busy. Instead we ask the service which sessions are running.

const opencodeAPITimeout = 2 * time.Second

var (
	opencodeDirCache   = make(map[string]string) // session ID -> directory
	opencodeDirCacheMu sync.Mutex
)

// opencodeRunningSessions returns IDs of sessions the opencode service is
// currently running. ok is false when the service API is unavailable
// (opencode v1, not installed, service down), so callers can fall back to CPU.
func opencodeRunningSessions() (ids []string, ok bool) {
	var response struct {
		Data map[string]struct {
			Type string `json:"type"`
		} `json:"data"`
	}
	if err := opencodeAPIGet("/api/session/active", &response); err != nil {
		return nil, false
	}

	for id, active := range response.Data {
		if active.Type == "running" {
			ids = append(ids, id)
		}
	}
	return ids, true
}

// opencodeWorkingDirectories returns the directories of running opencode sessions.
func opencodeWorkingDirectories() (dirs map[string]bool, ok bool) {
	ids, ok := opencodeRunningSessions()
	if !ok {
		return nil, false
	}

	dirs = make(map[string]bool, len(ids))
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			dir := opencodeSessionDirectory(id)
			if dir == "" {
				return
			}
			mu.Lock()
			dirs[dir] = true
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return dirs, true
}

func opencodeSessionDirectory(id string) string {
	opencodeDirCacheMu.Lock()
	dir, cached := opencodeDirCache[id]
	opencodeDirCacheMu.Unlock()
	if cached {
		return dir
	}

	var response struct {
		Data struct {
			Location struct {
				Directory string `json:"directory"`
			} `json:"location"`
		} `json:"data"`
	}
	if err := opencodeAPIGet("/api/session/"+id, &response); err != nil {
		return ""
	}

	dir = normalizePath(response.Data.Location.Directory)
	if dir == "" {
		return ""
	}
	opencodeDirCacheMu.Lock()
	opencodeDirCache[id] = dir
	opencodeDirCacheMu.Unlock()
	return dir
}

// opencodeAPIGet uses `opencode api`, which handles service discovery and auth.
func opencodeAPIGet(path string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), opencodeAPITimeout)
	defer cancel()

	body, err := exec.CommandContext(ctx, "opencode", "api", "get", path).Output()
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func normalizePath(path string) string {
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Clean(path)
}
