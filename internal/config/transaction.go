package config

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// UpdatePool serializes repository mutations across processes. The callback must
// use its fresh pool, not a snapshot captured before entering the transaction.
// Call only after interactive prompts; hold the lock through side effects and save.
func UpdatePool(repoRoot string, settings Settings, mutate func(Pool) (Pool, error)) (Pool, error) {
	dir, err := ConfigDir(repoRoot)
	if err != nil {
		return Pool{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Pool{}, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "pool.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return Pool{}, err
	}
	// Keep the file: unlinking a flock file can create independent lock inodes.
	defer lock.Close()
	for {
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX)
		if err != syscall.EINTR {
			break
		}
	}
	if err != nil {
		return Pool{}, fmt.Errorf("lock workbench pool: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck
	pool, err := LoadPool(repoRoot, settings)
	if err != nil {
		return Pool{}, err
	}
	updated, err := mutate(pool)
	if err != nil {
		return pool, err
	}
	if err := savePool(repoRoot, updated); err != nil {
		return pool, err
	}
	return updated, nil
}
