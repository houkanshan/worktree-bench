package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	PoolFileName    = "pool.json"
	SettingsFileName = "config.json"
)

const (
	TypeFull    = "full"
	TypeLight   = "light"
	TypeMinimal = "minimal"
)

type Pool struct {
	Version      int         `json:"version"`
	RepoRoot     string      `json:"repo_root"`
	WorktreesDir string      `json:"worktrees_dir"`
	Benches      []Workbench `json:"benches"`
}

type Workbench struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Path      string     `json:"path"`
	CreatedAt string     `json:"created_at"`
	LastSetup string     `json:"last_setup"`
	DevServer *DevServer `json:"dev_server,omitempty"`
}

type DevServer struct {
	Cmd       string `json:"cmd"`
	PID       int    `json:"pid"`
	StartedAt string `json:"started_at"`
}

type Settings struct {
	WorktreesDir string `json:"worktrees_dir"`
	SetupCmd     string `json:"setup_cmd"`
	DevCmd       string `json:"dev_cmd"`
}

func LoadSettings(repoRoot string) (Settings, error) {
	settingsPath := filepath.Join(repoRoot, ".worktree-bench", SettingsFileName)
	if _, err := os.Stat(settingsPath); errors.Is(err, os.ErrNotExist) {
		defaults := Settings{WorktreesDir: filepath.Join(repoRoot, "..", ".worktrees")}
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
			return Settings{}, err
		}
		if err := writeJSON(settingsPath, defaults); err != nil {
			return Settings{}, err
		}
		return defaults, nil
	}

	var settings Settings
	if err := readJSON(settingsPath, &settings); err != nil {
		return Settings{}, err
	}

	if settings.WorktreesDir == "" {
		settings.WorktreesDir = filepath.Join(repoRoot, "..", ".worktrees")
	}

	return settings, nil
}

func SaveSettings(repoRoot string, settings Settings) error {
	settingsPath := filepath.Join(repoRoot, ".worktree-bench", SettingsFileName)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}
	return writeJSON(settingsPath, settings)
}

func LoadPool(repoRoot string, settings Settings) (Pool, error) {
	poolPath := filepath.Join(repoRoot, ".worktree-bench", PoolFileName)
	if _, err := os.Stat(poolPath); errors.Is(err, os.ErrNotExist) {
		pool := Pool{Version: 1, RepoRoot: repoRoot, WorktreesDir: settings.WorktreesDir, Benches: []Workbench{}}
		if err := os.MkdirAll(filepath.Dir(poolPath), 0o755); err != nil {
			return Pool{}, err
		}
		if err := writeJSON(poolPath, pool); err != nil {
			return Pool{}, err
		}
		return pool, nil
	}

	var pool Pool
	if err := readJSON(poolPath, &pool); err != nil {
		return Pool{}, err
	}

	if pool.WorktreesDir == "" {
		pool.WorktreesDir = settings.WorktreesDir
	}

	return pool, nil
}

func SavePool(repoRoot string, pool Pool) error {
	poolPath := filepath.Join(repoRoot, ".worktree-bench", PoolFileName)
	if err := os.MkdirAll(filepath.Dir(poolPath), 0o755); err != nil {
		return err
	}
	pool.RepoRoot = repoRoot
	return writeJSON(poolPath, pool)
}

func NewWorkbenchID() string {
	return fmt.Sprintf("wb-%d", time.Now().UnixNano())
}

func NowString() string {
	return time.Now().Format(time.RFC3339)
}

func writeJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func readJSON(path string, out any) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, out)
}
