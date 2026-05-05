package session

import "strings"

// DetectStrategy determines the restore strategy for a running process.
func DetectStrategy(command string) RestoreStrategy {
	cmd := strings.ToLower(command)

	switch {
	case cmd == "nvim" || cmd == "vim":
		return StrategyNvimSession
	case cmd == "opencode":
		return StrategyOpencodeContinue
	case cmd == "claude":
		return StrategyClaudeContinue
	case cmd == "pi":
		return StrategyRerun
	case cmd == "zsh" || cmd == "bash" || cmd == "fish" || cmd == "sh":
		return StrategyShell
	default:
		// Everything else (lazygit, node, npm, python, ssh, etc.) gets re-run
		return StrategyRerun
	}
}

// BuildRestoreCommand returns the command string to use when restoring a pane.
func BuildRestoreCommand(command, path string, strategy RestoreStrategy, nvimSessionFile string) string {
	switch strategy {
	case StrategyNvimSession:
		if nvimSessionFile != "" {
			return "nvim -S " + nvimSessionFile
		}
		// Fallback: just open nvim in the directory
		return "nvim"
	case StrategyOpencodeContinue:
		return "opencode --continue"
	case StrategyClaudeContinue:
		return "claude --continue"
	case StrategyShell:
		return "" // empty = just use the default shell
	case StrategyRerun:
		return command
	default:
		return command
	}
}
