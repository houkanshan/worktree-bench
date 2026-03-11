package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"worktree-bench/internal/gitutil"
)

const DefaultBranchPrefix = "bench/"

const (
	PoolFileName     = "pool.json"
	SettingsFileName = "config.json"
)

const (
	TypeLarge  = "large"
	TypeMedium = "medium"
	TypeSmall  = "small"
)

const (
	legacyTypeFull    = "full"
	legacyTypeLight   = "light"
	legacyTypeMinimal = "minimal"
	legacyTypeMinimum = "minimum"
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
	WorktreesDir       string `json:"worktrees_dir"`
	SetupCmd           string `json:"setup_cmd"`
	DevCmd             string `json:"dev_cmd"`
	InitCmd            string `json:"init_cmd"`
	BranchPrefix       string `json:"branch_prefix"`
	WorktreeNamePrefix string `json:"worktree_name_prefix"`
}

func LoadSettings(repoRoot string) (Settings, error) {
	configDir, defaultsRoot, err := resolveConfigDir(repoRoot)
	if err != nil {
		return Settings{}, err
	}
	settingsPath := filepath.Join(configDir, SettingsFileName)
	if _, err := os.Stat(settingsPath); errors.Is(err, os.ErrNotExist) {
		defaults := Settings{WorktreesDir: defaultWorktreesDir(defaultsRoot)}
		if err := os.MkdirAll(configDir, 0o755); err != nil {
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
		settings.WorktreesDir = defaultWorktreesDir(defaultsRoot)
	}

	return settings, nil
}

func SaveSettings(repoRoot string, settings Settings) error {
	configDir, _, err := resolveConfigDir(repoRoot)
	if err != nil {
		return err
	}
	settingsPath := filepath.Join(configDir, SettingsFileName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	return writeJSON(settingsPath, settings)
}

func LoadPool(repoRoot string, settings Settings) (Pool, error) {
	configDir, defaultsRoot, err := resolveConfigDir(repoRoot)
	if err != nil {
		return Pool{}, err
	}
	poolPath := filepath.Join(configDir, PoolFileName)
	if _, err := os.Stat(poolPath); errors.Is(err, os.ErrNotExist) {
		pool := Pool{Version: 1, RepoRoot: defaultsRoot, WorktreesDir: settings.WorktreesDir, Benches: []Workbench{}}
		if err := os.MkdirAll(configDir, 0o755); err != nil {
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
	if pool.RepoRoot == "" {
		pool.RepoRoot = defaultsRoot
	}
	for i := range pool.Benches {
		pool.Benches[i].Type = NormalizeWorkbenchType(pool.Benches[i].Type)
	}

	return pool, nil
}

func SavePool(repoRoot string, pool Pool) error {
	configDir, defaultsRoot, err := resolveConfigDir(repoRoot)
	if err != nil {
		return err
	}
	poolPath := filepath.Join(configDir, PoolFileName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	pool.RepoRoot = defaultsRoot
	return writeJSON(poolPath, pool)
}

func resolveConfigDir(repoRoot string) (string, string, error) {
	mainRoot, err := gitutil.MainWorktreeRoot(repoRoot)
	if err == nil && mainRoot != "" {
		if _, statErr := os.Stat(mainRoot); statErr == nil {
			return filepath.Join(mainRoot, ".worktree-bench"), mainRoot, nil
		}
	}
	globalDir, globalErr := globalConfigDir(repoRoot)
	if globalErr != nil {
		if err != nil {
			return "", "", err
		}
		return "", "", globalErr
	}
	return globalDir, repoRoot, nil
}

func defaultWorktreesDir(repoRoot string) string {
	return filepath.Join(repoRoot, "..", ".worktrees")
}

func globalConfigDir(repoRoot string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	key := repoKey(repoRoot)
	return filepath.Join(home, ".worktree-bench", "repos", key), nil
}

func repoKey(repoRoot string) string {
	clean := filepath.Clean(repoRoot)
	sum := sha256.Sum256([]byte(clean))
	base := filepath.Base(clean)
	return fmt.Sprintf("%s-%x", base, sum[:8])
}

func NewWorkbenchID() string {
	return fmt.Sprintf("wb-%d", time.Now().UnixNano())
}

func WorkbenchTypes() []string {
	return []string{TypeLarge, TypeMedium, TypeSmall}
}

func IsWorkbenchType(value string) bool {
	switch NormalizeWorkbenchType(value) {
	case TypeLarge, TypeMedium, TypeSmall:
		return true
	default:
		return false
	}
}

func NormalizeWorkbenchType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case TypeLarge, legacyTypeFull:
		return TypeLarge
	case TypeMedium, legacyTypeLight:
		return TypeMedium
	case TypeSmall, legacyTypeMinimal, legacyTypeMinimum:
		return TypeSmall
	default:
		return normalized
	}
}

func WorkbenchTypeShortName(benchType string) string {
	switch NormalizeWorkbenchType(benchType) {
	case TypeLarge:
		return "l"
	case TypeMedium:
		return "m"
	case TypeSmall:
		return "s"
	default:
		return ""
	}
}

// NewBranchName generates a branch name as "{prefix}{4-digit-hex}".
// If prefix is empty, DefaultBranchPrefix is used.
func NewBranchName(prefix string) string {
	if prefix == "" {
		prefix = DefaultBranchPrefix
	}
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based hex if crypto/rand fails.
		b[0] = byte(time.Now().UnixNano() >> 8)
		b[1] = byte(time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(b)
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
