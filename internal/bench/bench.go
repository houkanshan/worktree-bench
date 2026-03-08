package bench

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"worktree-bench/internal/config"
	"worktree-bench/internal/gitutil"
)

var prPattern = regexp.MustCompile(`^(#|pr:)?(\d+)$`)

func DetectSetupCmd(repoRoot string) string {
	if fileExists(filepath.Join(repoRoot, "pnpm-lock.yaml")) {
		return "pnpm install"
	}
	if fileExists(filepath.Join(repoRoot, "yarn.lock")) {
		return "yarn install"
	}
	if fileExists(filepath.Join(repoRoot, "package-lock.json")) {
		return "npm install"
	}
	return ""
}

func DetectDevCmd(repoRoot string) string {
	pkgPath := filepath.Join(repoRoot, "package.json")
	payload, err := os.ReadFile(pkgPath)
	if err != nil {
		return ""
	}
	if strings.Contains(string(payload), "\"dev\"") {
		if fileExists(filepath.Join(repoRoot, "pnpm-lock.yaml")) {
			return "pnpm dev"
		}
		if fileExists(filepath.Join(repoRoot, "yarn.lock")) {
			return "yarn dev"
		}
		return "npm run dev"
	}
	return ""
}

func EnsureSettings(repoRoot string) (config.Settings, error) {
	settings, err := config.LoadSettings(repoRoot)
	if err != nil {
		return config.Settings{}, err
	}
	updated := false
	if settings.SetupCmd == "" {
		settings.SetupCmd = DetectSetupCmd(repoRoot)
		updated = true
	}
	if settings.DevCmd == "" {
		settings.DevCmd = DetectDevCmd(repoRoot)
		updated = true
	}
	if updated {
		if err := config.SaveSettings(repoRoot, settings); err != nil {
			return config.Settings{}, err
		}
	}
	return settings, nil
}

func CreateWorkbench(repoRoot string, settings config.Settings, pool config.Pool, input CreateInput) (config.Pool, *config.Workbench, error) {
	if input.Type == "" {
		return pool, nil, errors.New("missing workbench type")
	}

	if input.UseExisting {
		bench := findBench(pool, input.BenchID)
		if bench == nil {
			return pool, nil, fmt.Errorf("workbench not found")
		}
		if err := gitutil.Fetch(bench.Path); err != nil {
			return pool, nil, err
		}
		if err := checkoutRef(bench.Path, input.Ref); err != nil {
			return pool, nil, err
		}
		return pool, bench, nil
	}

	if input.Name == "" {
		input.Name = nextName(pool, input.Type)
	}
	benchPath := filepath.Join(settings.WorktreesDir, input.Name)
	if err := os.MkdirAll(settings.WorktreesDir, 0o755); err != nil {
		return pool, nil, err
	}
	if err := gitutil.WorktreeAdd(repoRoot, benchPath, ""); err != nil {
		return pool, nil, err
	}
	if err := checkoutRef(benchPath, input.Ref); err != nil {
		return pool, nil, err
	}

	newBench := config.Workbench{
		ID:        config.NewWorkbenchID(),
		Name:      input.Name,
		Type:      input.Type,
		Path:      benchPath,
		CreatedAt: config.NowString(),
	}

	if input.Type == config.TypeFull || input.Type == config.TypeLight {
		if settings.SetupCmd != "" {
			if err := runCommand(benchPath, settings.SetupCmd); err != nil {
				return pool, nil, err
			}
			newBench.LastSetup = config.NowString()
		}
	}

	if input.Type == config.TypeFull && settings.DevCmd != "" {
		pid, err := startCommand(benchPath, settings.DevCmd)
		if err != nil {
			return pool, nil, err
		}
		newBench.DevServer = &config.DevServer{Cmd: settings.DevCmd, PID: pid, StartedAt: config.NowString()}
	}

	pool.Benches = append(pool.Benches, newBench)
	return pool, &newBench, nil
}

func checkoutRef(path, ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	if prPattern.MatchString(ref) {
		matches := prPattern.FindStringSubmatch(ref)
		num, _ := strconv.Atoi(matches[2])
		return ghCheckoutPR(path, num)
	}
	return gitutil.CheckoutBranch(path, ref)
}

func ghCheckoutPR(path string, number int) error {
	cmd := exec.Command("gh", "pr", "checkout", fmt.Sprintf("%d", number))
	cmd.Dir = path
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCommand(dir, command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startCommand(dir, command string) (int, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func findBench(pool config.Pool, id string) *config.Workbench {
	for i := range pool.Benches {
		if pool.Benches[i].ID == id {
			return &pool.Benches[i]
		}
	}
	return nil
}

func nextName(pool config.Pool, benchType string) string {
	count := 0
	for _, bench := range pool.Benches {
		if bench.Type == benchType {
			count++
		}
	}
	return fmt.Sprintf("%s-%d", benchType, count+1)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Switch handles swapping worktrees if requested and returns the target path.
func Switch(repoRoot string, targetPath string, swap bool) (string, error) {
	if !swap {
		return targetPath, nil
	}
	currentRoot, err := gitutil.RepoRoot()
	if err != nil {
		return "", err
	}
	currentCommon, err := gitutil.GitCommonDir(currentRoot)
	if err != nil {
		return "", err
	}
	targetCommon, err := gitutil.GitCommonDir(targetPath)
	if err != nil {
		return "", err
	}
	if currentCommon != targetCommon {
		return "", errors.New("current directory is not in the same repository")
	}
	if filepath.Clean(currentRoot) == filepath.Clean(targetPath) {
		return currentRoot, nil
	}
	parent := filepath.Dir(targetPath)
	tmpPath := filepath.Join(parent, fmt.Sprintf(".swap-%d", os.Getpid()))
	if err := gitutil.WorktreeMove(repoRoot, targetPath, tmpPath); err != nil {
		return "", err
	}
	if err := gitutil.WorktreeMove(repoRoot, currentRoot, targetPath); err != nil {
		return "", err
	}
	if err := gitutil.WorktreeMove(repoRoot, tmpPath, currentRoot); err != nil {
		return "", err
	}
	return currentRoot, nil
}

// CreateInput describes a create action.
type CreateInput struct {
	Type        string
	UseExisting bool
	BenchID     string
	Name        string
	Ref         string
}

// AdoptInput describes adopting the current worktree into the pool.
type AdoptInput struct {
	Type     string
	Name     string
	RunSetup bool
	RunDev   bool
}

// DeleteInput describes deleting a workbench.
type DeleteInput struct {
	BenchID string
	Force   bool
}

// AdoptWorkbench registers the current worktree as a workbench.
func AdoptWorkbench(repoRoot string, settings config.Settings, pool config.Pool, input AdoptInput) (config.Pool, *config.Workbench, error) {
	if input.Type == "" {
		return pool, nil, errors.New("missing workbench type")
	}
	if findBenchByPath(pool, repoRoot) != nil {
		return pool, nil, errors.New("current worktree is already registered")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = filepath.Base(repoRoot)
	}

	newBench := config.Workbench{
		ID:        config.NewWorkbenchID(),
		Name:      name,
		Type:      input.Type,
		Path:      repoRoot,
		CreatedAt: config.NowString(),
	}

	if input.RunSetup && settings.SetupCmd != "" && input.Type != config.TypeMinimal {
		if err := runCommand(repoRoot, settings.SetupCmd); err != nil {
			return pool, nil, err
		}
		newBench.LastSetup = config.NowString()
	}

	if input.RunDev && input.Type == config.TypeFull && settings.DevCmd != "" {
		pid, err := startCommand(repoRoot, settings.DevCmd)
		if err != nil {
			return pool, nil, err
		}
		newBench.DevServer = &config.DevServer{Cmd: settings.DevCmd, PID: pid, StartedAt: config.NowString()}
	}

	pool.Benches = append(pool.Benches, newBench)
	return pool, &newBench, nil
}

// DeleteWorkbench removes a workbench from the pool and removes its worktree directory.
func DeleteWorkbench(repoRoot string, pool config.Pool, input DeleteInput) (config.Pool, *config.Workbench, error) {
	benchIndex := -1
	for i, bench := range pool.Benches {
		if bench.ID == input.BenchID {
			benchIndex = i
			break
		}
	}
	if benchIndex == -1 {
		return pool, nil, errors.New("workbench not found")
	}
	bench := pool.Benches[benchIndex]

	currentRoot, err := gitutil.RepoRoot()
	if err == nil && filepath.Clean(currentRoot) == filepath.Clean(bench.Path) {
		return pool, nil, errors.New("cannot delete the current worktree")
	}

	if fileExists(bench.Path) {
		if err := gitutil.WorktreeRemove(repoRoot, bench.Path, input.Force); err != nil {
			return pool, nil, err
		}
	}

	pool.Benches = append(pool.Benches[:benchIndex], pool.Benches[benchIndex+1:]...)
	return pool, &bench, nil
}

func findBenchByPath(pool config.Pool, path string) *config.Workbench {
	for i := range pool.Benches {
		if filepath.Clean(pool.Benches[i].Path) == filepath.Clean(path) {
			return &pool.Benches[i]
		}
	}
	return nil
}

// FindWorkbenchByPath exposes lookup by path.
func FindWorkbenchByPath(pool config.Pool, path string) *config.Workbench {
	return findBenchByPath(pool, path)
}
