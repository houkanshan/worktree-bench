package statuscache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"worktree-bench/internal/config"
)

const (
	FileName = "status-cache.json"
	Version  = 1
)

type Cache struct {
	Version int              `json:"version"`
	Entries map[string]Entry `json:"entries"`
}

type Entry struct {
	Path        string      `json:"path"`
	Fingerprint string      `json:"fingerprint"`
	Status      BenchStatus `json:"status"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type BenchStatus struct {
	DirMissing   bool      `json:"dir_missing"`
	Description  string    `json:"description"`
	Branch       string    `json:"branch"`
	LastCommit   time.Time `json:"last_commit"`
	ChangesLines int       `json:"changes_lines"`
	ChangesErr   string    `json:"changes_err"`
}

func Load(repoRoot string) (Cache, error) {
	path, err := Path(repoRoot)
	if err != nil {
		return newCache(), err
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return newCache(), err
	}
	var cache Cache
	if err := json.Unmarshal(payload, &cache); err != nil || cache.Version != Version {
		return newCache(), nil
	}
	if cache.Entries == nil {
		cache.Entries = map[string]Entry{}
	}
	return cache, nil
}

func Save(repoRoot string, cache Cache) error {
	path, err := Path(repoRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	cache.Version = Version
	if cache.Entries == nil {
		cache.Entries = map[string]Entry{}
	}
	payload, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func Remove(repoRoot string) error {
	path, err := Path(repoRoot)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func RemoveEntries(repoRoot string, ids ...string) error {
	cache, err := Load(repoRoot)
	if err != nil {
		return err
	}
	changed := false
	for _, id := range ids {
		if _, ok := cache.Entries[id]; ok {
			delete(cache.Entries, id)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return Save(repoRoot, cache)
}

func RemovePaths(repoRoot string, paths ...string) error {
	cache, err := Load(repoRoot)
	if err != nil {
		return err
	}
	want := map[string]bool{}
	for _, path := range paths {
		want[filepath.Clean(path)] = true
	}
	changed := false
	for id, entry := range cache.Entries {
		if want[filepath.Clean(entry.Path)] {
			delete(cache.Entries, id)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return Save(repoRoot, cache)
}

func Path(repoRoot string) (string, error) {
	configDir, err := config.ConfigDir(repoRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, FileName), nil
}

func Get(cache Cache, bench config.Workbench, fingerprint string) (BenchStatus, bool) {
	entry, ok := cache.Entries[bench.ID]
	if !ok || entry.Path != bench.Path || entry.Fingerprint != fingerprint {
		return BenchStatus{}, false
	}
	return entry.Status, true
}

func Put(cache *Cache, bench config.Workbench, fingerprint string, status BenchStatus) {
	if cache.Entries == nil {
		cache.Entries = map[string]Entry{}
	}
	cache.Version = Version
	cache.Entries[bench.ID] = Entry{Path: bench.Path, Fingerprint: fingerprint, Status: status, UpdatedAt: time.Now()}
}

func newCache() Cache {
	return Cache{Version: Version, Entries: map[string]Entry{}}
}
