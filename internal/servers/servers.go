// Package servers detects node/bun/deno processes (dev servers, watchers) and
// attributes them to tmux sessions or projects, including processes that were
// started in the background by AI agents and later detached from tmux.
package servers

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"skocko/internal/tmux"
)

// Owner is something servers can belong to: an active session or a project.
type Owner struct {
	Key      string
	Path     string
	PanePIDs []int
}

// Server is a group of runtime processes sharing one root process.
type Server struct {
	PID      int
	PGID     int
	Args     string
	Cwd      string
	Ports    []int
	Detached bool // not running under any tmux pane (e.g. left behind by an agent)
}

// DefaultIgnore lists editor tooling and agent helpers that are not servers.
var DefaultIgnore = []string{
	"prettierd",
	"eslint_d",
	"language-server",
	"tsserver",
	"typingsinstaller",
	"vtsls",
	"modelcontextprotocol",
	"mcp-server",
	"mcp-remote",
	"-mcp",
}

const stopTimeout = 2 * time.Second

// Detect finds runtime processes and groups them per owner key.
func Detect(owners []Owner, extraIgnore []string) (map[string][]Server, error) {
	procs, err := listProcesses()
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}

	ignore := append(append([]string{}, DefaultIgnore...), extraIgnore...)
	panes := panePIDOwners(owners)
	groups := groupServers(procs, listListeningPorts(), panes, ignore)

	var detachedRoots []int
	for _, g := range groups {
		if g.owner == "" {
			detachedRoots = append(detachedRoots, g.server.PID)
		}
	}
	cwds := listCwds(detachedRoots)
	home, _ := os.UserHomeDir()

	return attributeToOwners(groups, cwds, owners, home), nil
}

// Stop terminates servers gracefully: SIGTERM, then SIGKILL for anything still
// alive after a short timeout. Servers whose root PID was reused are skipped.
func Stop(servers []Server) (int, error) {
	if len(servers) == 0 {
		return 0, nil
	}
	procs, err := listProcesses()
	if err != nil {
		return 0, fmt.Errorf("list processes: %w", err)
	}
	byPID := indexByPID(procs)

	var errs []error
	var targets []int
	stopped := 0
	for _, s := range servers {
		current, ok := byPID[s.PID]
		if !ok || current.Args != s.Args {
			continue
		}
		pids := signalTargets(s, procs)
		if err := signalAll(pids, syscall.SIGTERM); err != nil {
			errs = append(errs, fmt.Errorf("pid %d: %w", s.PID, err))
			continue
		}
		targets = append(targets, pids...)
		stopped++
	}

	waitForExit(targets, stopTimeout)
	_ = signalAll(aliveOnly(targets), syscall.SIGKILL)

	return stopped, errors.Join(errs...)
}

// --- grouping ---

type process struct {
	PID  int
	PPID int
	PGID int
	Comm string
	Args string
}

type group struct {
	server Server
	owner  string
}

var runtimeNames = map[string]bool{"node": true, "bun": true, "deno": true}

var agentNames = map[string]bool{
	"opencode": true, "claude": true, "aider": true, "pi": true,
	"codex": true, "gemini": true, "cursor": true, "copilot": true,
}

func panePIDOwners(owners []Owner) map[int]string {
	panes := make(map[int]string)
	for _, o := range owners {
		for _, pid := range o.PanePIDs {
			panes[pid] = o.Key
		}
	}
	return panes
}

// groupServers resolves each runtime process to its topmost runtime ancestor
// (stopping at agents and tmux panes) and merges processes sharing a root.
func groupServers(procs []process, ports map[int][]int, panes map[int]string, ignore []string) []group {
	byPID := indexByPID(procs)
	rootOf := make(map[int]int)
	groupsByRoot := make(map[int]*group)
	var order []int

	for _, p := range procs {
		if !isRuntime(p) {
			continue
		}
		root, owner, hidden := resolveRoot(p, byPID, panes, ignore)
		if hidden {
			continue
		}
		rootOf[p.PID] = root.PID
		if _, ok := groupsByRoot[root.PID]; !ok {
			groupsByRoot[root.PID] = &group{
				server: Server{PID: root.PID, PGID: root.PGID, Args: root.Args, Detached: owner == ""},
				owner:  owner,
			}
			order = append(order, root.PID)
		}
	}

	for pid, list := range ports {
		root, ok := findMemberRoot(pid, byPID, rootOf)
		if !ok {
			continue
		}
		g := groupsByRoot[root]
		g.server.Ports = mergePorts(g.server.Ports, list)
	}

	groups := make([]group, 0, len(order))
	for _, pid := range order {
		groups = append(groups, *groupsByRoot[pid])
	}
	return groups
}

func resolveRoot(p process, byPID map[int]process, panes map[int]string, ignore []string) (root process, owner string, hidden bool) {
	root = p
	climbingRoot := true
	cur := p
	for depth := 0; depth < 64; depth++ {
		if matchesIgnore(cur.Args, ignore) {
			return root, "", true
		}
		if climbingRoot {
			if isAgent(cur) {
				if cur.PID == p.PID {
					return root, "", true
				}
				climbingRoot = false
			} else if isRuntime(cur) {
				root = cur
			}
		}
		if key, ok := panes[cur.PID]; ok {
			return root, key, false
		}
		parent, ok := byPID[cur.PPID]
		if !ok || cur.PPID <= 1 {
			break
		}
		cur = parent
	}
	return root, "", false
}

// findMemberRoot attributes a listening process to the group of its nearest
// runtime ancestor, so ports opened by helper binaries are not lost.
func findMemberRoot(pid int, byPID map[int]process, rootOf map[int]int) (int, bool) {
	cur := pid
	for depth := 0; depth < 64 && cur > 1; depth++ {
		if root, ok := rootOf[cur]; ok {
			return root, true
		}
		p, ok := byPID[cur]
		if !ok {
			return 0, false
		}
		cur = p.PPID
	}
	return 0, false
}

func attributeToOwners(groups []group, cwds map[int]string, owners []Owner, home string) map[string][]Server {
	result := make(map[string][]Server)
	for _, g := range groups {
		key := g.owner
		if key == "" {
			cwd := cwds[g.server.PID]
			g.server.Cwd = cwd
			key = ownerForPath(cwd, owners, home)
		}
		if key == "" {
			continue
		}
		result[key] = append(result[key], g.server)
	}
	for key := range result {
		sortServers(result[key])
	}
	return result
}

// ownerForPath picks the owner with the longest path containing dir.
// Home and root only match exactly, so they don't swallow unrelated processes.
func ownerForPath(dir string, owners []Owner, home string) string {
	if dir == "" {
		return ""
	}
	bestKey, bestLen := "", -1
	for _, o := range owners {
		if o.Path == "" {
			continue
		}
		path := filepath.Clean(o.Path)
		broad := path == "/" || path == home
		matches := dir == path || (!broad && strings.HasPrefix(dir, path+"/"))
		if matches && len(path) > bestLen {
			bestKey, bestLen = o.Key, len(path)
		}
	}
	return bestKey
}

// --- process classification ---

func isRuntime(p process) bool {
	if runtimeNames[baseName(p.Comm)] {
		return true
	}
	fields := strings.Fields(p.Args)
	return len(fields) > 0 && runtimeNames[baseName(fields[0])]
}

func isAgent(p process) bool {
	return agentNames[tmux.CanonicalCommandName(p.Comm, p.Args)]
}

func matchesIgnore(args string, ignore []string) bool {
	lower := strings.ToLower(args)
	for _, pattern := range ignore {
		if pattern != "" && strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func baseName(command string) string {
	command = strings.ToLower(strings.TrimSpace(command))
	if idx := strings.LastIndex(command, "/"); idx >= 0 {
		command = command[idx+1:]
	}
	return command
}

// DisplayCommand shortens a process command line for display:
// "node /path/node_modules/.bin/nuxt dev" becomes "nuxt dev".
func DisplayCommand(args string) string {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return ""
	}
	if runtimeNames[baseName(fields[0])] {
		for i := 1; i < len(fields); i++ {
			if strings.HasPrefix(fields[i], "-") {
				continue
			}
			if strings.Contains(fields[i], "/") {
				return strings.Join(append([]string{filepath.Base(fields[i])}, fields[i+1:]...), " ")
			}
			break
		}
	}
	fields[0] = filepath.Base(fields[0])
	return strings.Join(fields, " ")
}

// --- signals ---

// signalTargets returns PIDs to signal. A negative PID addresses the whole
// process group, used only when the server leads its own group so we never
// signal a parent agent or shell sharing the group.
func signalTargets(s Server, procs []process) []int {
	if s.PGID == s.PID {
		return []int{-s.PGID}
	}
	children := make(map[int][]int)
	for _, p := range procs {
		children[p.PPID] = append(children[p.PPID], p.PID)
	}
	pids := []int{s.PID}
	for i := 0; i < len(pids); i++ {
		pids = append(pids, children[pids[i]]...)
	}
	return pids
}

func signalAll(pids []int, sig syscall.Signal) error {
	var errs []error
	for _, pid := range pids {
		if err := syscall.Kill(pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func waitForExit(pids []int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(aliveOnly(pids)) == 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func aliveOnly(pids []int) []int {
	var alive []int
	for _, pid := range pids {
		if err := syscall.Kill(pid, 0); err == nil || errors.Is(err, syscall.EPERM) {
			alive = append(alive, pid)
		}
	}
	return alive
}

// --- system queries ---

func listProcesses() ([]process, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,pgid=,args=").Output()
	if err != nil {
		return nil, err
	}
	comms := listCommNames()

	var procs []process
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		pgid, err3 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		procs = append(procs, process{
			PID:  pid,
			PPID: ppid,
			PGID: pgid,
			Comm: comms[pid],
			Args: strings.Join(fields[3:], " "),
		})
	}
	return procs, nil
}

// listCommNames returns the executable name per PID. Needed because tools like
// prettierd or Next.js rewrite their args (process title) and hide "node".
func listCommNames() map[int]string {
	comms := make(map[int]string)
	out, err := exec.Command("ps", "-axo", "pid=,ucomm=").Output()
	if err != nil {
		return comms
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		idx := strings.IndexAny(line, " \t")
		if idx < 0 {
			continue
		}
		pid, err := strconv.Atoi(line[:idx])
		if err != nil {
			continue
		}
		comms[pid] = strings.TrimSpace(line[idx:])
	}
	return comms
}

// listListeningPorts maps PIDs to TCP ports they listen on.
// Returns an empty map when lsof is unavailable; ports are optional info.
func listListeningPorts() map[int][]int {
	out, err := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-Fpn").Output()
	if err != nil && len(out) == 0 {
		return map[int][]int{}
	}
	return parseLsofPorts(string(out))
}

func parseLsofPorts(out string) map[int][]int {
	ports := make(map[int][]int)
	pid := 0
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'n':
			idx := strings.LastIndex(line, ":")
			if idx < 0 || pid == 0 {
				continue
			}
			if port, err := strconv.Atoi(line[idx+1:]); err == nil && !isEphemeralPort(port) {
				ports[pid] = mergePorts(ports[pid], []int{port})
			}
		}
	}
	return ports
}

// isEphemeralPort reports ports auto-assigned by the OS (listen on port 0),
// e.g. workerd/miniflare instances, inspector and HMR sockets. They are
// internal plumbing, not something a user opens in a browser.
func isEphemeralPort(port int) bool {
	return port >= ephemeralPortMin
}

const ephemeralPortMin = 49152

func listCwds(pids []int) map[int]string {
	if len(pids) == 0 {
		return map[int]string{}
	}
	list := make([]string, len(pids))
	for i, pid := range pids {
		list[i] = strconv.Itoa(pid)
	}
	out, err := exec.Command("lsof", "-a", "-d", "cwd", "-p", strings.Join(list, ","), "-Fpn").Output()
	if err != nil && len(out) == 0 {
		return map[int]string{}
	}
	return parseLsofCwds(string(out))
}

func parseLsofCwds(out string) map[int]string {
	cwds := make(map[int]string)
	pid := 0
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'n':
			if pid != 0 {
				cwds[pid] = line[1:]
			}
		}
	}
	return cwds
}

// --- helpers ---

func indexByPID(procs []process) map[int]process {
	byPID := make(map[int]process, len(procs))
	for _, p := range procs {
		byPID[p.PID] = p
	}
	return byPID
}

func mergePorts(existing, add []int) []int {
	for _, port := range add {
		found := false
		for _, e := range existing {
			if e == port {
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, port)
		}
	}
	sort.Ints(existing)
	return existing
}

func sortServers(list []Server) {
	sort.SliceStable(list, func(i, j int) bool {
		pi, pj := firstPort(list[i]), firstPort(list[j])
		if pi != pj {
			return pi < pj
		}
		return list[i].PID < list[j].PID
	})
}

// firstPort sorts servers without ports last.
func firstPort(s Server) int {
	if len(s.Ports) == 0 {
		return 1 << 30
	}
	return s.Ports[0]
}
