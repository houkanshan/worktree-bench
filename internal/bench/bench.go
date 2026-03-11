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

const tempCommitMessage = "__WTSWAP_TEMP__"

func tracef(format string, args ...any) {
	if os.Getenv("WTB_TRACE") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "[wtb] "+format+"\n", args...)
}

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
	detectRoot := repoRoot
	if mainRoot, err := gitutil.MainWorktreeRoot(repoRoot); err == nil && mainRoot != "" {
		detectRoot = mainRoot
	}
	updated := false
	if settings.SetupCmd == "" {
		settings.SetupCmd = DetectSetupCmd(detectRoot)
		updated = true
	}
	if settings.DevCmd == "" {
		settings.DevCmd = DetectDevCmd(detectRoot)
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
	benchType := config.NormalizeWorkbenchType(input.Type)
	if !config.IsWorkbenchType(benchType) {
		return pool, nil, fmt.Errorf("unsupported workbench type %q", input.Type)
	}
	input.Type = benchType
	baseBranch := strings.TrimSpace(input.BaseBranch)
	if baseBranch == "" {
		return pool, nil, errors.New("missing base branch")
	}
	branchName := config.NewBranchName(settings.BranchPrefix)

	if input.UseExisting {
		bench := findBench(pool, input.BenchID)
		if bench == nil {
			return pool, nil, fmt.Errorf("workbench not found")
		}
		if err := gitutil.Fetch(bench.Path); err != nil {
			return pool, nil, err
		}
		if err := gitutil.CheckoutNewBranch(bench.Path, branchName, baseBranch); err != nil {
			return pool, nil, err
		}
		return pool, bench, nil
	}

	if input.Name == "" {
		input.Name = nextName(pool, benchType, settings.WorktreeNamePrefix)
	}
	benchPath := filepath.Join(settings.WorktreesDir, input.Name)
	if err := os.MkdirAll(settings.WorktreesDir, 0o755); err != nil {
		return pool, nil, err
	}
	if err := gitutil.WorktreeAdd(repoRoot, benchPath, branchName, baseBranch); err != nil {
		return pool, nil, err
	}

	newBench := config.Workbench{
		ID:        config.NewWorkbenchID(),
		Name:      input.Name,
		Type:      benchType,
		Path:      benchPath,
		CreatedAt: config.NowString(),
	}

	if benchType == config.TypeLarge || benchType == config.TypeMedium {
		if settings.SetupCmd != "" {
			if err := runCommand(benchPath, settings.SetupCmd); err != nil {
				return pool, nil, err
			}
			newBench.LastSetup = config.NowString()
		}
	}

	if benchType == config.TypeLarge && settings.DevCmd != "" {
		pid, err := startCommand(benchPath, settings.DevCmd)
		if err != nil {
			return pool, nil, err
		}
		newBench.DevServer = &config.DevServer{Cmd: settings.DevCmd, PID: pid, StartedAt: config.NowString()}
	}

	if input.RunInit && settings.InitCmd != "" {
		if err := runCommand(benchPath, settings.InitCmd); err != nil {
			if newBench.DevServer != nil {
				killProcess(newBench.DevServer.PID)
			}
			return pool, nil, err
		}
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("gh pr checkout %d: %s", number, detail)
		}
		return fmt.Errorf("gh pr checkout %d: %w", number, err)
	}
	return nil
}

func runCommand(dir, command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("%s: %s", command, detail)
		}
		return fmt.Errorf("%s: %w", command, err)
	}
	return nil
}

func startCommand(dir, command string) (int, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("%s: %w", command, err)
	}
	return cmd.Process.Pid, nil
}

func killProcess(pid int) {
	if pid <= 0 {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Kill()
}

func findBench(pool config.Pool, id string) *config.Workbench {
	for i := range pool.Benches {
		if pool.Benches[i].ID == id {
			return &pool.Benches[i]
		}
	}
	return nil
}

func nextName(pool config.Pool, benchType string, prefix string) string {
	shortName := config.WorkbenchTypeShortName(benchType)
	if shortName == "" {
		shortName = benchType
	}
	namePrefix := fmt.Sprintf("%s%s-", prefix, shortName)
	next := 1
	for _, bench := range pool.Benches {
		if config.NormalizeWorkbenchType(bench.Type) != benchType {
			continue
		}
		if !strings.HasPrefix(bench.Name, namePrefix) {
			continue
		}
		suffix := strings.TrimPrefix(bench.Name, namePrefix)
		index, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		if index >= next {
			next = index + 1
		}
	}
	return fmt.Sprintf("%s%d", namePrefix, next)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Switch handles swapping worktrees if requested and returns the target path.
func Switch(repoRoot string, targetPath string, swap bool) (string, error) {
	tracef("switch: swap=%t target=%s", swap, targetPath)
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
	if err := swapBranches(currentRoot, targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

func swapBranches(currentPath, targetPath string) error {
	tracef("swap-branches: current=%s target=%s", currentPath, targetPath)
	currentBranch, err := branchOrError(currentPath, "current")
	if err != nil {
		return err
	}
	targetBranch, err := branchOrError(targetPath, "target")
	if err != nil {
		return err
	}

	currentHadChanges, err := createTempCommit(currentPath)
	if err != nil {
		return err
	}
	targetHadChanges, err := createTempCommit(targetPath)
	if err != nil {
		if currentHadChanges {
			_ = resetTempCommit(currentPath)
		}
		return err
	}

	rollback := func() {
		_ = checkoutDetach(targetPath)
		_ = checkoutBranch(currentPath, currentBranch)
		_ = checkoutBranch(targetPath, targetBranch)
		_ = resetTempCommit(currentPath)
		_ = resetTempCommit(targetPath)
	}

	if err := checkoutDetach(targetPath); err != nil {
		rollback()
		return err
	}
	if err := checkoutBranch(currentPath, targetBranch); err != nil {
		rollback()
		return err
	}
	if err := checkoutBranch(targetPath, currentBranch); err != nil {
		rollback()
		return err
	}

	if currentHadChanges {
		if err := resetTempCommit(targetPath); err != nil {
			return err
		}
	}
	if targetHadChanges {
		if err := resetTempCommit(currentPath); err != nil {
			return err
		}
	}

	return nil
}

func branchOrError(path, label string) (string, error) {
	branch, err := gitutil.Branch(path)
	if err != nil {
		return "", err
	}
	if branch == "HEAD" {
		return "", fmt.Errorf("%s worktree is in detached HEAD state", label)
	}
	return branch, nil
}

func hasChanges(path string) (bool, error) {
	out, err := exec.Command("git", "-C", path, "status", "--porcelain").Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func createTempCommit(path string) (bool, error) {
	changed, err := hasChanges(path)
	if err != nil || !changed {
		return false, err
	}
	if err := runGit(path, "add", "-A"); err != nil {
		return false, err
	}
	if err := runGit(path, "commit", "-m", tempCommitMessage, "--no-verify"); err != nil {
		return false, err
	}
	return true, nil
}

func resetTempCommit(path string) error {
	isTemp, err := isTempCommit(path)
	if err != nil || !isTemp {
		return err
	}
	if err := runGit(path, "reset", "--soft", "HEAD~1"); err != nil {
		return err
	}
	return runGit(path, "reset", "HEAD")
}

func isTempCommit(path string) (bool, error) {
	out, err := exec.Command("git", "-C", path, "log", "-1", "--format=%s").Output()
	if err != nil {
		return false, fmt.Errorf("git log: %w", err)
	}
	return strings.TrimSpace(string(out)) == tempCommitMessage, nil
}

func checkoutDetach(path string) error {
	return runGit(path, "checkout", "--detach")
}

func checkoutBranch(path, branch string) error {
	return runGit(path, "checkout", branch)
}

func runGit(path string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// CreateInput describes a create action.
type CreateInput struct {
	Type        string
	UseExisting bool
	BenchID     string
	Name        string
	BaseBranch  string
	RunInit     bool
}

// AdoptInput describes adopting the current worktree into the pool.
type AdoptInput struct {
	Type     string
	Name     string
	RunSetup bool
	RunDev   bool
	RunInit  bool
}

// DeleteInput describes deleting a workbench.
type DeleteInput struct {
	BenchID string
	Force   bool
}

// AdoptWorkbench registers the current worktree as a workbench.
func AdoptWorkbench(repoRoot string, settings config.Settings, pool config.Pool, input AdoptInput) (config.Pool, *config.Workbench, error) {
	benchType := config.NormalizeWorkbenchType(input.Type)
	if !config.IsWorkbenchType(benchType) {
		return pool, nil, fmt.Errorf("unsupported workbench type %q", input.Type)
	}
	input.Type = benchType
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
		Type:      benchType,
		Path:      repoRoot,
		CreatedAt: config.NowString(),
	}

	if input.RunSetup && settings.SetupCmd != "" && benchType != config.TypeSmall {
		if err := runCommand(repoRoot, settings.SetupCmd); err != nil {
			return pool, nil, err
		}
		newBench.LastSetup = config.NowString()
	}

	if input.RunDev && benchType == config.TypeLarge && settings.DevCmd != "" {
		pid, err := startCommand(repoRoot, settings.DevCmd)
		if err != nil {
			return pool, nil, err
		}
		newBench.DevServer = &config.DevServer{Cmd: settings.DevCmd, PID: pid, StartedAt: config.NowString()}
	}

	if input.RunInit && settings.InitCmd != "" {
		if err := runCommand(repoRoot, settings.InitCmd); err != nil {
			if newBench.DevServer != nil {
				killProcess(newBench.DevServer.PID)
			}
			return pool, nil, err
		}
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
