package cmd

import (
	"path/filepath"

	"worktree-bench/internal/config"
	"worktree-bench/internal/statuscache"
)

func invalidateStatusCache(repoRoot string) {
	_ = statuscache.Remove(repoRoot)
}

func invalidateStatusCacheEntries(repoRoot string, ids ...string) {
	_ = statuscache.RemoveEntries(repoRoot, ids...)
}

func invalidateStatusCachePaths(repoRoot string, paths ...string) {
	_ = statuscache.RemovePaths(repoRoot, paths...)
}

func invalidateStatusCacheForPaths(repoRoot string, pool config.Pool, paths ...string) {
	ids := []string{}
	cleanPaths := []string{}
	for _, path := range paths {
		if path == "" {
			continue
		}
		cleanPaths = append(cleanPaths, path)
		for _, wb := range pool.Benches {
			if filepath.Clean(wb.Path) == filepath.Clean(path) {
				ids = append(ids, wb.ID)
				break
			}
		}
	}
	invalidateStatusCacheEntries(repoRoot, ids...)
	invalidateStatusCachePaths(repoRoot, cleanPaths...)
}
