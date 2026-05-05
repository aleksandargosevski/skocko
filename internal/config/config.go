package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Keybindings struct {
	Zoxide      string `mapstructure:"zoxide"`
	KillSession string `mapstructure:"kill_session"`
	GitStatus   string `mapstructure:"git_status"`
	Preview     string `mapstructure:"preview"`
	SaveSession string `mapstructure:"save_session"`
	CopyPath    string `mapstructure:"copy_path"`
	DeleteSaved string `mapstructure:"delete_saved"`
	ToggleHelp  string `mapstructure:"toggle_help"`
	Refresh     string `mapstructure:"refresh"`
}

type GitStatusConfig struct {
	ShowOnStart bool   `mapstructure:"show_on_start"`
	Scope       string `mapstructure:"scope"`
	Detail      string `mapstructure:"detail"`
}

// WindowConfig defines a reusable window template.
type WindowConfig struct {
	Name    string `mapstructure:"name"`
	Command string `mapstructure:"command"`
}

// SessionConfig defines a custom session entry shown in the picker.
type SessionConfig struct {
	Name    string   `mapstructure:"name"`
	Path    string   `mapstructure:"path"`
	Windows []string `mapstructure:"windows"` // references WindowConfig.Name
}

// ProjectDefault defines default windows for projects matching a path or glob.
type ProjectDefault struct {
	Path    string   `mapstructure:"path"`
	Windows []string `mapstructure:"windows"` // references WindowConfig.Name
}

type AIStatusConfig struct {
	Enabled          bool `mapstructure:"enabled"`
	PollInterval     int  `mapstructure:"poll_interval"`      // seconds between checks
	NotifyOnComplete bool `mapstructure:"notify_on_complete"` // send notification when AI finishes
}

type Config struct {
	ProjectPaths    []string         `mapstructure:"project_paths"`
	Theme           string           `mapstructure:"theme"`
	ShowBorder      bool             `mapstructure:"show_border"`
	ShowHotkeys     bool             `mapstructure:"show_hotkeys"`
	Keybindings     Keybindings      `mapstructure:"keybindings"`
	GitStatus       GitStatusConfig  `mapstructure:"git_status"`
	AIStatus        AIStatusConfig   `mapstructure:"ai_status"`
	Windows         []WindowConfig   `mapstructure:"windows"`
	Sessions        []SessionConfig  `mapstructure:"sessions"`
	ProjectDefaults []ProjectDefault `mapstructure:"project_defaults"`
}

// ResolveWindows looks up window names against the Windows definitions
// and returns the matching WindowConfig entries.
func (c *Config) ResolveWindows(names []string) []WindowConfig {
	if len(names) == 0 {
		return nil
	}
	lookup := make(map[string]WindowConfig, len(c.Windows))
	for _, w := range c.Windows {
		lookup[w.Name] = w
	}
	var result []WindowConfig
	for _, name := range names {
		if w, ok := lookup[name]; ok {
			result = append(result, w)
		}
	}
	return result
}

// FindProjectWindows returns the window configs for a project path.
// Exact path matches take priority over glob patterns.
// Returns nil if no match.
func (c *Config) FindProjectWindows(projectPath string) []WindowConfig {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		abs = projectPath
	}

	// Try exact match first
	for _, pd := range c.ProjectDefaults {
		pdAbs, err := filepath.Abs(expandHome(pd.Path))
		if err != nil {
			continue
		}
		if pdAbs == abs {
			return c.ResolveWindows(pd.Windows)
		}
	}

	// Try glob match
	for _, pd := range c.ProjectDefaults {
		pattern := expandHome(pd.Path)
		matched, err := filepath.Match(pattern, abs)
		if err != nil {
			continue
		}
		if matched {
			return c.ResolveWindows(pd.Windows)
		}
	}

	return nil
}

func Load(cfgFile string) (*Config, error) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		configDir := os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("could not find home directory: %w", err)
			}
			configDir = filepath.Join(home, ".config")
		}

		viper.AddConfigPath(filepath.Join(configDir, "skocko"))
		viper.SetConfigName("skocko")
		viper.SetConfigType("yaml")
	}

	// Defaults
	viper.SetDefault("project_paths", []string{})
	viper.SetDefault("theme", "catppuccin-mocha")
	viper.SetDefault("show_border", false)
	viper.SetDefault("show_hotkeys", false)
	viper.SetDefault("keybindings.zoxide", "ctrl+s")
	viper.SetDefault("keybindings.kill_session", "ctrl+x")
	viper.SetDefault("keybindings.git_status", "ctrl+g")
	viper.SetDefault("keybindings.preview", "ctrl+o")
	viper.SetDefault("keybindings.save_session", "ctrl+w")
	viper.SetDefault("keybindings.copy_path", "ctrl+y")
	viper.SetDefault("keybindings.delete_saved", "alt+d")
	viper.SetDefault("keybindings.toggle_help", "ctrl+_")
	viper.SetDefault("keybindings.refresh", "ctrl+r")
	viper.SetDefault("ai_status.enabled", false)
	viper.SetDefault("ai_status.poll_interval", 3)
	viper.SetDefault("ai_status.notify_on_complete", true)
	viper.SetDefault("git_status.show_on_start", false)
	viper.SetDefault("git_status.scope", "all")
	viper.SetDefault("git_status.detail", "full")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return defaultConfig(), nil
		}
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}

	// Expand ~ in all paths
	for i, p := range cfg.ProjectPaths {
		cfg.ProjectPaths[i] = expandHome(p)
	}
	for i := range cfg.Sessions {
		cfg.Sessions[i].Path = expandHome(cfg.Sessions[i].Path)
	}
	for i := range cfg.ProjectDefaults {
		cfg.ProjectDefaults[i].Path = expandHome(cfg.ProjectDefaults[i].Path)
	}

	applyDefaults(&cfg)

	return &cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		ProjectPaths: []string{},
		Theme:        "catppuccin-mocha",
		Keybindings: Keybindings{
			Zoxide:      "ctrl+s",
			KillSession: "ctrl+x",
			GitStatus:   "ctrl+g",
			Preview:     "ctrl+o",
			SaveSession: "ctrl+w",
			CopyPath:    "ctrl+y",
			DeleteSaved: "alt+d",
			ToggleHelp:  "ctrl+_",
			Refresh:     "ctrl+r",
		},
		AIStatus: AIStatusConfig{
			Enabled:          false,
			PollInterval:     3,
			NotifyOnComplete: true,
		},
		GitStatus: GitStatusConfig{
			ShowOnStart: false,
			Scope:       "all",
			Detail:      "full",
		},
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Keybindings.Zoxide == "" {
		cfg.Keybindings.Zoxide = "ctrl+s"
	}
	if cfg.Keybindings.KillSession == "" {
		cfg.Keybindings.KillSession = "ctrl+x"
	}
	if cfg.Keybindings.GitStatus == "" {
		cfg.Keybindings.GitStatus = "ctrl+g"
	}
	if cfg.Keybindings.Preview == "" {
		cfg.Keybindings.Preview = "ctrl+o"
	}
	if cfg.Keybindings.SaveSession == "" {
		cfg.Keybindings.SaveSession = "ctrl+w"
	}
	if cfg.Keybindings.CopyPath == "" {
		cfg.Keybindings.CopyPath = "ctrl+y"
	}
	if cfg.Keybindings.DeleteSaved == "" {
		cfg.Keybindings.DeleteSaved = "alt+d"
	}
	if cfg.Keybindings.ToggleHelp == "" {
		cfg.Keybindings.ToggleHelp = "ctrl+_"
	}
	if cfg.Keybindings.Refresh == "" {
		cfg.Keybindings.Refresh = "ctrl+r"
	}
	if cfg.GitStatus.Scope == "" {
		cfg.GitStatus.Scope = "all"
	}
	if cfg.GitStatus.Detail == "" {
		cfg.GitStatus.Detail = "full"
	}
}

func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
