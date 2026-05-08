package tui

import (
	"os/exec"
	"strconv"
	"strings"

	"skocko/internal/tmux"
)

// AIStatus represents the current state of an AI tool in a tmux pane.
type AIStatus int

const (
	AIUnknown AIStatus = iota
	AIIdle             // waiting for user input
	AIWorking          // streaming / processing
)

// cpuThreshold: if an AI process uses more than this % CPU, it's considered working.
const cpuThreshold = 5.0

// AI process names that we should monitor.
var aiProcessNames = map[string]bool{
	"opencode": true,
	"claude":   true,
	"aider":    true,
	"pi":       true,
}

// IsAIProcess returns true if the command name is a known AI tool.
func IsAIProcess(command string) bool {
	return aiProcessNames[strings.ToLower(command)]
}

// DetectAllAIStatuses detects AI process statuses using only ps (no tmux commands).
// Safe to call from anywhere including tmux popups.
// Returns a map from "session:window.pane" to AIStatus.
func DetectAllAIStatuses() map[string]AIStatus {
	// Step 1: Get pane info from tmux (just metadata, no capture-pane)
	panes := getTmuxAIPanes()
	if len(panes) == 0 {
		return nil
	}

	// Step 2: Get CPU of all AI processes via ps only
	cpuMap := getAIProcessCPU()

	// Step 3: Match panes to CPU usage
	statuses := make(map[string]AIStatus, len(panes))
	for _, p := range panes {
		cpu := cpuMap[p.shellPID]
		if cpu > cpuThreshold {
			statuses[p.key] = AIWorking
		} else {
			statuses[p.key] = AIIdle
		}
	}

	return statuses
}

// DetectAllAIStatusesNonTmux detects AI statuses using ONLY ps commands.
// No tmux interaction at all - safe to run from daemons that might conflict with popups.
// Returns a map keyed by PID string instead of tmux pane coordinates.
func DetectAllAIStatusesNonTmux() map[string]AIStatus {
	out, err := exec.Command("ps", "-eo", "pid,pcpu,comm,args").Output()
	if err != nil {
		return nil
	}

	statuses := make(map[string]AIStatus)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid := fields[0]
		cpu, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}

		cmd := tmux.CanonicalCommandName(fields[2], strings.Join(fields[3:], " "))
		if !IsAIProcess(cmd) {
			continue
		}

		if cpu > cpuThreshold {
			statuses[pid] = AIWorking
		} else {
			statuses[pid] = AIIdle
		}
	}

	if len(statuses) == 0 {
		return nil
	}
	return statuses
}

// --- internal helpers ---

type aiPaneInfo struct {
	key      string // "session:window.pane"
	shellPID int
}

func getTmuxAIPanes() []aiPaneInfo {
	out, err := exec.Command("tmux", "list-panes", "-a",
		"-F", "#{session_name}\t#{window_index}\t#{pane_index}\t#{pane_current_command}\t#{pane_pid}").Output()
	if err != nil {
		return nil
	}

	var panes []aiPaneInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 5 {
			continue
		}
		pid, err := strconv.Atoi(parts[4])
		if err != nil || pid <= 0 {
			continue
		}
		cmd := tmux.ResolveCommand(parts[3], pid)
		if !IsAIProcess(cmd) {
			continue
		}
		key := parts[0] + ":" + parts[1] + "." + parts[2]
		panes = append(panes, aiPaneInfo{key: key, shellPID: pid})
	}
	return panes
}

// getAIProcessCPU returns max AI CPU% by process PID and ancestor PID.
func getAIProcessCPU() map[int]float64 {
	out, err := exec.Command("ps", "-eo", "pid,ppid,pcpu,comm,args").Output()
	if err != nil {
		return nil
	}

	type processCPU struct {
		pid     int
		ppid    int
		cpu     float64
		command string
		args    string
	}

	var processes []processCPU
	parentByPID := make(map[int]int)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
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
		cpu, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			continue
		}

		processes = append(processes, processCPU{
			pid:     pid,
			ppid:    ppid,
			cpu:     cpu,
			command: fields[3],
			args:    strings.Join(fields[4:], " "),
		})
		parentByPID[pid] = ppid
	}

	result := make(map[int]float64)
	for _, process := range processes {
		cmd := tmux.CanonicalCommandName(process.command, process.args)
		if !IsAIProcess(cmd) {
			continue
		}

		for current, seen := process.pid, map[int]bool{}; current > 0 && !seen[current]; current = parentByPID[current] {
			seen[current] = true
			if process.cpu > result[current] {
				result[current] = process.cpu
			}
		}
	}
	return result
}

// SessionHasWorkingAI returns true if any AI pane in the given session is working.
func SessionHasWorkingAI(sessionName string, statuses map[string]AIStatus) bool {
	for key, status := range statuses {
		if strings.HasPrefix(key, sessionName+":") && status == AIWorking {
			return true
		}
	}
	return false
}

// SessionAIStatus returns the "most active" AI status for a session.
func SessionAIStatus(sessionName string, statuses map[string]AIStatus) AIStatus {
	hasAI := false
	for key, status := range statuses {
		if strings.HasPrefix(key, sessionName+":") {
			hasAI = true
			if status == AIWorking {
				return AIWorking
			}
		}
	}
	if hasAI {
		return AIIdle
	}
	return AIUnknown
}
