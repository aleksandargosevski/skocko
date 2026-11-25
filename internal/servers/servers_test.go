package servers

import (
	"reflect"
	"testing"
)

const home = "/Users/me"

func fixtureProcesses() []process {
	return []process{
		// krojna session: pane shell running npm run dev -> nuxt with workers
		{PID: 100, PPID: 1, PGID: 100, Comm: "zsh", Args: "-zsh"},
		{PID: 101, PPID: 100, PGID: 101, Comm: "node", Args: "npm run dev"},
		{PID: 102, PPID: 101, PGID: 101, Comm: "sh", Args: "sh -c nuxt dev"},
		{PID: 103, PPID: 102, PGID: 101, Comm: "node", Args: "node /Users/me/Sites/krojna/node_modules/.bin/nuxt dev"},
		{PID: 104, PPID: 103, PGID: 101, Comm: "node", Args: "/usr/bin/node --enable-source-maps worker.js"},
		{PID: 105, PPID: 103, PGID: 101, Comm: "esbuild", Args: "esbuild --service"},

		// nvim with language servers in the same pane
		{PID: 110, PPID: 100, PGID: 110, Comm: "nvim", Args: "nvim"},
		{PID: 111, PPID: 110, PGID: 111, Comm: "node", Args: "node /mason/bin/typescript-language-server --stdio"},
		{PID: 112, PPID: 111, PGID: 111, Comm: "node", Args: "node /krojna/node_modules/typescript/lib/tsserver.js"},

		// agent pane: claude (node) started a vite server via a shell
		{PID: 200, PPID: 1, PGID: 200, Comm: "zsh", Args: "-zsh"},
		{PID: 201, PPID: 200, PGID: 201, Comm: "node", Args: "node /opt/homebrew/bin/claude"},
		{PID: 202, PPID: 201, PGID: 201, Comm: "zsh", Args: "/bin/zsh -c npm run dev"},
		{PID: 203, PPID: 202, PGID: 201, Comm: "node", Args: "node /Users/me/Sites/app/node_modules/.bin/vite"},

		// orphans left behind by agents (reparented to launchd)
		{PID: 300, PPID: 1, PGID: 300, Comm: "node", Args: "npm run dev"},
		{PID: 301, PPID: 300, PGID: 300, Comm: "node", Args: "node /Users/me/Sites/shop/node_modules/.bin/next dev"},
		{PID: 310, PPID: 1, PGID: 310, Comm: "node", Args: "node server.js"},

		// noise
		{PID: 400, PPID: 1, PGID: 400, Comm: "node", Args: "prettierd"},
		{PID: 401, PPID: 1, PGID: 401, Comm: "node", Args: "node /Users/me/Sites/shop/scratch.js"},
	}
}

func TestGroupServers(t *testing.T) {
	ports := map[int][]int{103: {3100, 24678}, 105: {9999}, 203: {5173}, 301: {3000}, 400: {4000}, 201: {7000}}
	panes := map[int]string{100: "krojna", 200: "app"}

	groups := groupServers(fixtureProcesses(), ports, panes, DefaultIgnore)

	type summary struct {
		PID      int
		Owner    string
		Ports    []int
		Detached bool
	}
	var got []summary
	for _, g := range groups {
		got = append(got, summary{g.server.PID, g.owner, g.server.Ports, g.server.Detached})
	}
	want := []summary{
		{101, "krojna", []int{3100, 9999, 24678}, false},
		{203, "app", []int{5173}, false},
		{300, "", []int{3000}, true},
		{310, "", nil, true},
		{401, "", nil, true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groupServers()\n got: %+v\nwant: %+v", got, want)
	}
}

func TestAttributeToOwners(t *testing.T) {
	owners := []Owner{
		{Key: "Sites", Path: "/Users/me/Sites"},
		{Key: "shop", Path: "/Users/me/Sites/shop"},
		{Key: "home", Path: home},
	}
	groups := []group{
		{server: Server{PID: 1, Ports: []int{3000}}, owner: "krojna"},
		{server: Server{PID: 2, Ports: []int{3001}, Detached: true}},
		{server: Server{PID: 3, Detached: true}},
		{server: Server{PID: 4, Detached: true}},
		{server: Server{PID: 5, Detached: true}},
		{server: Server{PID: 6, Ports: []int{80}, Detached: true}},
	}
	cwds := map[int]string{
		2: "/Users/me/Sites/shop",
		3: "/Users/me/Sites/shop/packages/api",
		4: "/Users/me/Sites/other",
		5: "/Users/me/Documents",
		6: "/Users/me/Sites/shopping",
	}

	result := attributeToOwners(groups, cwds, owners, home)

	assertPIDs(t, result, "krojna", 1)
	assertPIDs(t, result, "shop", 2, 3)
	assertPIDs(t, result, "Sites", 6, 4)
	assertPIDs(t, result, "home")
}

func assertPIDs(t *testing.T, result map[string][]Server, key string, want ...int) {
	t.Helper()
	var got []int
	for _, s := range result[key] {
		got = append(got, s.PID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("servers for %q = %v, want %v", key, got, want)
	}
}

func TestSignalTargets(t *testing.T) {
	procs := fixtureProcesses()

	if got := signalTargets(Server{PID: 300, PGID: 300}, procs); !reflect.DeepEqual(got, []int{-300}) {
		t.Errorf("group leader targets = %v, want [-300]", got)
	}
	// vite shares the claude process group, so only its own tree is signalled
	if got := signalTargets(Server{PID: 203, PGID: 201}, procs); !reflect.DeepEqual(got, []int{203}) {
		t.Errorf("shared group targets = %v, want [203]", got)
	}
}

func TestParseLsof(t *testing.T) {
	ports := parseLsofPorts("p50622\nf14\nn[::1]:3100\nf213\nn*:24679\nf215\nn127.0.0.1:52012\np70\nf5\nn127.0.0.1:3100\np80\nf6\nn127.0.0.1:65466\n")
	want := map[int][]int{50622: {3100, 24679}, 70: {3100}}
	if !reflect.DeepEqual(ports, want) {
		t.Errorf("parseLsofPorts() = %v, want %v", ports, want)
	}

	cwds := parseLsofCwds("p1\nfcwd\nn/Users/me/Sites/krojna\np2\nfcwd\nn/tmp/with space\n")
	wantCwds := map[int]string{1: "/Users/me/Sites/krojna", 2: "/tmp/with space"}
	if !reflect.DeepEqual(cwds, wantCwds) {
		t.Errorf("parseLsofCwds() = %v, want %v", cwds, wantCwds)
	}
}

func TestDisplayCommand(t *testing.T) {
	tests := map[string]string{
		"node /Users/me/krojna/node_modules/.bin/nuxt dev --port 3100": "nuxt dev --port 3100",
		"npm run dev": "npm run dev",
		"/usr/local/bin/node --inspect server.js": "node --inspect server.js",
		"/opt/bin/bun run dev":                    "bun run dev",
		"node":                                    "node",
	}
	for args, want := range tests {
		if got := DisplayCommand(args); got != want {
			t.Errorf("DisplayCommand(%q) = %q, want %q", args, got, want)
		}
	}
}
