package tmux

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// PaneProcess represents a single process running in a tmux pane.
type PaneProcess struct {
	Command     string
	WindowIndex string
	PaneIndex   string
	PID         int
}

type Session struct {
	Name      string
	Path      string
	Processes []PaneProcess // all pane processes (not deduplicated)
	Activity  int64         // unix timestamp of last activity in session
}

// ListSessions returns all active tmux sessions with their running processes.
// Uses two tmux calls total (list-sessions + list-panes -a) instead of N+1.
// Returns an empty slice if tmux server is not running.
func ListSessions() []Session {
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}\t#{session_path}\t#{session_activity}").Output()
	if err != nil {
		return nil
	}

	// Build session map
	sessionMap := make(map[string]*Session)
	var order []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		name := parts[0]
		path := ""
		var activity int64
		if len(parts) > 1 {
			path = parts[1]
		}
		if len(parts) > 2 {
			activity, _ = strconv.ParseInt(parts[2], 10, 64)
		}
		sessionMap[name] = &Session{Name: name, Path: path, Activity: activity}
		order = append(order, name)
	}

	// Single call to get all pane processes (no dedup - every instance tracked)
	paneOut, err := exec.Command("tmux", "list-panes", "-a",
		"-F", "#{session_name}\t#{pane_current_command}\t#{window_index}\t#{pane_index}\t#{pane_pid}").Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(paneOut)), "\n") {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 5)
			if len(parts) < 5 {
				continue
			}
			pid, _ := strconv.Atoi(parts[4])
			cmd := ResolveCommand(parts[1], pid)
			if s, ok := sessionMap[parts[0]]; ok {
				s.Processes = append(s.Processes, PaneProcess{
					Command:     cmd,
					WindowIndex: parts[2],
					PaneIndex:   parts[3],
					PID:         pid,
				})
			}
		}
	}

	// Preserve original order
	sessions := make([]Session, 0, len(order))
	for _, name := range order {
		if s, ok := sessionMap[name]; ok {
			sessions = append(sessions, *s)
		}
	}

	return sessions
}

// SessionExists checks if a tmux session with the given name exists.
func SessionExists(name string) bool {
	err := exec.Command("tmux", "has-session", "-t", name).Run()
	return err == nil
}

// CreateSession creates a new detached tmux session with the given name and working directory.
func CreateSession(name, path string) error {
	return exec.Command("tmux", "new-session", "-d", "-s", name, "-c", path).Run()
}

// SwitchTo switches the current tmux client to the given session.
func SwitchTo(name string) error {
	return exec.Command("tmux", "switch-client", "-t", name).Run()
}

// Attach attaches to an existing tmux session.
func Attach(name string) error {
	cmd := exec.Command("tmux", "attach-session", "-t", name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// IsInsideTmux returns true if we're running inside a tmux session.
func IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// KillSession gracefully terminates all processes in a session, then kills the session.
// It sends SIGTERM to each pane's process group, waits briefly for cleanup, then kills the session.
func KillSession(name string) error {
	// Get all pane PIDs
	pids := listPanePIDs(name)

	// Send SIGTERM to each process group so child processes (servers etc.) also get the signal
	for _, pid := range pids {
		// Negative PID sends signal to entire process group
		_ = syscall.Kill(-pid, syscall.SIGTERM)
	}

	// Give processes a moment to clean up
	if len(pids) > 0 {
		time.Sleep(500 * time.Millisecond)
	}

	// Now kill the tmux session (cleans up any remaining processes)
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

// listPanePIDs returns all pane PIDs for a session.
func listPanePIDs(sessionName string) []int {
	out, err := exec.Command("tmux", "list-panes", "-s", "-t", sessionName, "-F", "#{pane_pid}").Output()
	if err != nil {
		return nil
	}

	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}

var nodeScriptAliases = map[string]string{
	"aider":    "aider",
	"claude":   "claude",
	"opencode": "opencode",
	"pi":       "pi",
}

// ResolveCommand checks if a generic node process is actually a known tool.
func ResolveCommand(command string, panePID int) string {
	if panePID <= 0 || baseCommandName(command) != "node" {
		return command
	}
	processes, err := listProcesses()
	if err != nil {
		return command
	}
	if alias := resolveNodeAliasForPID(processes, panePID); alias != "" {
		return alias
	}
	return command
}

// CanonicalCommandName returns a known tool name when a process command is a generic runtime.
func CanonicalCommandName(command, args string) string {
	cmd := baseCommandName(command)
	if cmd != "node" {
		return cmd
	}
	if alias := resolveNodeAlias(command, args); alias != "" {
		return alias
	}
	return cmd
}

type processInfo struct {
	PID     int
	PPID    int
	Command string
	Args    string
}

func listProcesses() ([]processInfo, error) {
	out, err := exec.Command("ps", "-eo", "pid,ppid,comm,args").Output()
	if err != nil {
		return nil, err
	}

	var processes []processInfo
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		processes = append(processes, processInfo{
			PID:     pid,
			PPID:    ppid,
			Command: fields[2],
			Args:    strings.Join(fields[3:], " "),
		})
	}
	return processes, nil
}

func resolveNodeAliasForPID(processes []processInfo, panePID int) string {
	childrenByParent := make(map[int][]processInfo)
	for _, process := range processes {
		if process.PID == panePID {
			if alias := resolveNodeAlias(process.Command, process.Args); alias != "" {
				return alias
			}
		}
		childrenByParent[process.PPID] = append(childrenByParent[process.PPID], process)
	}

	queue := append([]processInfo(nil), childrenByParent[panePID]...)
	for len(queue) > 0 {
		process := queue[0]
		queue = queue[1:]
		if alias := resolveNodeAlias(process.Command, process.Args); alias != "" {
			return alias
		}
		queue = append(queue, childrenByParent[process.PID]...)
	}
	return ""
}

func resolveNodeAlias(command, args string) string {
	if alias, ok := nodeScriptAliases[baseCommandName(command)]; ok {
		return alias
	}
	return nodeScriptAliases[nodeScriptName(args)]
}

func nodeScriptName(args string) string {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return ""
	}
	if alias, ok := nodeScriptAliases[baseCommandName(fields[0])]; ok {
		return alias
	}
	for _, field := range fields[1:] {
		field = strings.Trim(field, "'\"")
		if field == "" || strings.HasPrefix(field, "-") {
			continue
		}
		return baseCommandName(field)
	}
	return ""
}

func baseCommandName(command string) string {
	command = strings.Trim(strings.ToLower(command), "'\"")
	if idx := strings.LastIndex(command, "/"); idx >= 0 {
		command = command[idx+1:]
	}
	return command
}

// WindowDef describes a window to create within a tmux session.
type WindowDef struct {
	Name    string
	Command string
}

// CreateSessionWithWindows creates a new detached tmux session with configured windows.
// The first window definition replaces the default window; additional ones are added.
// Commands are sent via send-keys so they run inside the shell.
func CreateSessionWithWindows(name, path string, windows []WindowDef) error {
	if len(windows) == 0 {
		return CreateSession(name, path)
	}

	// Create session - the first window gets created automatically
	if err := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", path, "-n", windows[0].Name).Run(); err != nil {
		return err
	}

	// Run command in first window
	if windows[0].Command != "" {
		_ = exec.Command("tmux", "send-keys", "-t", name+":"+windows[0].Name, windows[0].Command, "Enter").Run()
	}

	// Create additional windows
	for _, w := range windows[1:] {
		if err := exec.Command("tmux", "new-window", "-t", name, "-n", w.Name, "-c", path).Run(); err != nil {
			continue
		}
		if w.Command != "" {
			_ = exec.Command("tmux", "send-keys", "-t", name+":"+w.Name, w.Command, "Enter").Run()
		}
	}

	// Select the first window
	_ = exec.Command("tmux", "select-window", "-t", name+":"+windows[0].Name).Run()

	return nil
}

// ConnectToSession creates a session if it doesn't exist, then attaches or switches to it.
// If windows are provided and the session is new, they are created with their startup commands.
func ConnectToSession(name, path string, windows []WindowDef) error {
	if !SessionExists(name) {
		if err := CreateSessionWithWindows(name, path, windows); err != nil {
			return err
		}
	}

	if IsInsideTmux() {
		return SwitchTo(name)
	}
	return Attach(name)
}
