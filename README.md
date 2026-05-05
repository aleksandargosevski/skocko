# skocko

Smart tmux session manager with a built-in TUI picker.

> **Note:** This project is entirely vibe-coded for personal use. It works for my workflow but may have rough edges. Use at your own risk, PRs welcome.

## Features

- **Fuzzy picker** - search and connect to sessions without leaving tmux
- **Project scanning** - auto-discovers projects from configured directories
- **Session sections** - Active Sessions, Configured, and Projects separated in the picker
- **Process detection** - shows running processes per session (node, nvim, opencode, lazygit, docker, etc.)
- **Git status** - async branch status with dirty/ahead/behind/untracked indicators
- **Zoxide integration** - toggle to browse zoxide entries
- **Session save/restore** - persist sessions across tmux restarts including nvim editor state
- **Custom sessions** - define sessions for directories outside project paths
- **Default windows** - auto-create windows with startup commands per project or glob pattern
- **Preview panel** - inline split view with git details, window layout, and running commands
- **Graceful kill** - SIGTERM to process groups before killing sessions
- **AI status** - detects when AI tools are streaming, yellow icon + desktop notifications when done
- **Copy path** - copy any item's path to clipboard
- **Theming** - Catppuccin (Mocha, Latte, Frappe, Macchiato) + 256-color fallback
- **Configurable** - every keybinding, theme, and behavior is configurable via YAML

## Requirements

- [tmux](https://github.com/tmux/tmux)
- [zoxide](https://github.com/ajeetdsouza/zoxide) (optional, for zoxide mode)
- A [Nerd Font](https://www.nerdfonts.com/) (for icons)

## Installation

### Homebrew

```sh
brew install aleksandargosevski/tap/skocko
```

### Shell script

```sh
curl -fsSL https://raw.githubusercontent.com/aleksandargosevski/skocko/main/install-remote.sh | bash
```

Installs to `~/.local/bin/` by default. Override with `INSTALL_DIR`:

```sh
curl -fsSL ... | INSTALL_DIR=/usr/local/bin bash
```

### Build from source

Requires [Go 1.21+](https://go.dev/dl/) and `make`.

```sh
git clone https://github.com/aleksandargosevski/skocko.git
cd skocko
make install
```

Installs to `$(go env GOPATH)/bin`.

To update: `git pull && make install`

## Setup

```sh
mkdir -p ~/.config/skocko
```

Minimal `~/.config/skocko/skocko.yaml`:

```yaml
project_paths:
  - ~/Sites
```

### tmux popup

Add to your `tmux.conf`:

```tmux
bind-key "T" display-popup -E -w 80% -h 70% "skocko"
```

## Usage

```sh
skocko                        # open the picker
skocko save                   # save all active sessions
skocko save tfl               # save a specific session
skocko restore                # list saved sessions
skocko restore tfl            # restore a saved session
skocko restore --delete tfl   # delete saved state without restoring
skocko watch                  # monitor AI sessions (foreground)
skocko watch -d               # monitor AI sessions (background daemon)
```

## Keybindings

All keybindings are configurable. Press `ctrl+/` to toggle the hotkey bar in the picker.

| Key | Default | Action |
|-----|---------|--------|
| Type | - | Fuzzy search (always active) |
| `up` / `ctrl+p` | - | Move up |
| `down` / `ctrl+n` | - | Move down |
| `enter` | - | Connect to session (create if needed) |
| `esc` / `ctrl+c` | - | Quit |
| Toggle zoxide | `ctrl+s` | Switch between Projects and Zoxide views |
| Kill session | `ctrl+x` | Kill selected session (graceful SIGTERM) |
| Git status | `ctrl+g` | Toggle git status badges |
| Preview | `ctrl+o` | Toggle info preview panel |
| Save session | `ctrl+w` | Save selected session to disk |
| Copy path | `ctrl+y` | Copy selected item's path to clipboard |
| Delete saved | `alt+d` | Delete saved state for selected item |
| Refresh | `ctrl+r` | Refresh AI statuses and process info |
| Toggle help | `ctrl+/` | Show/hide hotkeys bar |

## Configuration

Full reference (`~/.config/skocko/skocko.yaml`):

```yaml
# Theme: catppuccin-mocha (default), catppuccin-latte, catppuccin-frappe, catppuccin-macchiato, default
theme: catppuccin-mocha

# UI
show_border: false     # draw border around picker (default: false, tmux popup provides one)
show_hotkeys: false    # show hotkeys bar on startup (default: false, toggle with ctrl+/)

# Directories to scan for projects
project_paths:
  - ~/Sites
  - ~/projects

# Reusable window definitions
windows:
  - name: editor
    command: nvim
  - name: server
    command: npm run dev
  - name: git
    command: lazygit

# Custom sessions (shown in "Configured" section)
sessions:
  - name: dotfiles
    path: ~/dotfiles
    windows: [editor, git]
  - name: notes
    path: ~/Documents/notes
    windows: [editor]

# Default windows for scanned projects (on first session creation only)
# Exact paths take priority over globs.
project_defaults:
  - path: ~/Sites/my-app
    windows: [editor, server, git]
  - path: "~/Sites/*"
    windows: [editor]

# Git status indicators
git_status:
  show_on_start: false   # show on launch (default: false, toggle with ctrl+g)
  scope: all             # "all" or "active"
  detail: full           # "dirty", "ahead_behind", or "full"

# Keybindings
keybindings:
  zoxide: ctrl+s
  kill_session: ctrl+x
  git_status: ctrl+g
  preview: ctrl+o
  save_session: ctrl+w
  copy_path: ctrl+y
  delete_saved: alt+d
  refresh: ctrl+r
  toggle_help: ctrl+_    # ctrl+/ on keyboard

# AI status detection (used by skocko watch and picker)
ai_status:
  poll_interval: 3            # seconds between checks (watch command)
  notify_on_complete: true    # desktop notification when AI finishes
```

### Windows

Define reusable window templates under `windows:`. Reference by name from `sessions` and `project_defaults`. Only created on new session creation, not when attaching to existing sessions.

### Custom Sessions

Entries under `sessions:` appear in a "Configured" section. For directories outside `project_paths`. Active configured sessions get promoted to "Active Sessions".

### Project Defaults

Match by exact path or glob. First session creation auto-creates configured windows with their commands.

### Git Status

Fetched asynchronously on first `ctrl+g` press. Subsequent toggles are instant. Three detail levels:

| Level | Shows |
|-------|-------|
| `dirty` | `*` uncommitted changes |
| `ahead_behind` | + `↑N` ahead, `↓N` behind |
| `full` | + `?N` untracked files |

### Themes

| Theme | Style |
|-------|-------|
| `catppuccin-mocha` | Dark, warm (default) |
| `catppuccin-macchiato` | Dark, slightly lighter |
| `catppuccin-frappe` | Medium dark |
| `catppuccin-latte` | Light |
| `default` | 256-color ANSI (no true color needed) |

All colors including process icons adapt to the active theme.

## Process Icons

| Process | Icon |
|---------|------|
| node / npm / bun / deno | 󰎙 |
| nvim |  |
| opencode / claude / aider / pi | 󰚩 |
| lazygit | 󰊢 |
| docker | 󰡨 |
| python | 󰌠 |
| go | 󰟓 |
| ruby / rails |  |
| cargo / rustc | 󱘗 |
| java / mvn / gradle |  |
| ssh | 󰣀 |
| postgres / mysql / redis | 󰆼 |

## Session Save/Restore

```sh
skocko save tfl     # saves windows, processes, nvim editor state
skocko restore tfl  # recreates everything, auto-deletes saved data
```

Or `ctrl+w` in the picker. Items with saved state show 󰆓. Selecting a non-active saved item prompts to restore.

Saved state is auto-deleted after successful restore. Manual delete: `alt+d` in picker or `skocko restore --delete <name>`.

| Process | Restore method |
|---------|---------------|
| nvim | `:mksession` on save, `nvim -S` on restore |
| opencode | `opencode --continue` |
| claude | `claude --continue` |
| lazygit, node, etc. | Re-runs the command |
| Shell | Opens in correct directory |

Data stored in `~/.local/share/skocko/`.

## AI Watch

Monitors AI tools (opencode, claude, aider, pi) and sends a desktop notification when they finish streaming. Uses CPU-based detection (no tmux pane capture).

The picker shows AI status: the `󰚩` icon turns yellow when an AI tool is actively working. Press `ctrl+r` to refresh statuses while the picker is open.

For background notifications, run the watch daemon in a **separate terminal** (not from tmux.conf - the daemon process conflicts with tmux popups):

```sh
skocko watch              # foreground
skocko watch -d           # daemon (detaches, runs in background)
skocko watch --interval 5 # custom poll interval
```

> **Warning**: Do not auto-start `skocko watch` from `tmux.conf`. The daemon process interferes with `display-popup` and will cause the picker to flash/close immediately.

**Stop**: `pkill -f "skocko watch"`

**Config**:

```yaml
ai_status:
  poll_interval: 3            # seconds between checks
  notify_on_complete: true    # desktop notification when AI finishes
```

## Preview Panel

`ctrl+o` toggles a split panel:

- **Git**: branch, last commit, remote
- **Windows**: tmux window list with pane commands
- **Configured windows**: what would be created on connect

Auto-hides below 60 columns width.

## Project Structure

```
skocko/
├── main.go
├── install.sh
├── cmd/
│   ├── root.go          # picker
│   ├── save.go          # skocko save
│   ├── restore.go       # skocko restore
│   └── watch.go         # skocko watch
└── internal/
    ├── config/          # YAML config (Viper)
    ├── git/             # git status (parallel)
    ├── notify/          # desktop notifications
    ├── project/         # directory scanning
    ├── session/         # save/restore
    ├── tmux/            # tmux interaction
    ├── tui/             # Bubble Tea TUI + theming
    └── zoxide/          # zoxide integration
```

## License

MIT
