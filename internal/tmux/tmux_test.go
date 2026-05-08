package tmux

import "testing"

func TestCanonicalCommandNameResolvesNodeScriptAliases(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    string
		want    string
	}{
		{
			name:    "pi global bin",
			command: "node",
			args:    "node /opt/homebrew/bin/pi",
			want:    "pi",
		},
		{
			name:    "pi argv0",
			command: "node",
			args:    "pi",
			want:    "pi",
		},
		{
			name:    "opencode global bin",
			command: "/opt/homebrew/opt/node/bin/node",
			args:    "/opt/homebrew/opt/node/bin/node /opt/homebrew/bin/opencode",
			want:    "opencode",
		},
		{
			name:    "regular node app",
			command: "node",
			args:    "node /Users/me/app/server.js",
			want:    "node",
		},
		{
			name:    "typescript path does not match pi substring",
			command: "node",
			args:    "node /Users/me/node_modules/typescript/lib/tsserver.js",
			want:    "node",
		},
		{
			name:    "non node stays basename",
			command: "/bin/zsh",
			args:    "zsh",
			want:    "zsh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanonicalCommandName(tt.command, tt.args); got != tt.want {
				t.Fatalf("CanonicalCommandName(%q, %q) = %q, want %q", tt.command, tt.args, got, tt.want)
			}
		})
	}
}

func TestResolveNodeAliasForPID(t *testing.T) {
	processes := []processInfo{
		{PID: 10, PPID: 1, Command: "node", Args: "node /opt/homebrew/bin/pi"},
		{PID: 20, PPID: 1, Command: "zsh", Args: "zsh"},
		{PID: 21, PPID: 20, Command: "node", Args: "pi"},
		{PID: 30, PPID: 1, Command: "zsh", Args: "zsh"},
		{PID: 31, PPID: 30, Command: "sh", Args: "sh wrapper"},
		{PID: 32, PPID: 31, Command: "node", Args: "node /opt/homebrew/bin/claude"},
	}

	tests := []struct {
		name    string
		panePID int
		want    string
	}{
		{name: "pane pid is node process", panePID: 10, want: "pi"},
		{name: "direct child", panePID: 20, want: "pi"},
		{name: "descendant", panePID: 30, want: "claude"},
		{name: "missing", panePID: 99, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveNodeAliasForPID(processes, tt.panePID); got != tt.want {
				t.Fatalf("resolveNodeAliasForPID(..., %d) = %q, want %q", tt.panePID, got, tt.want)
			}
		})
	}
}
