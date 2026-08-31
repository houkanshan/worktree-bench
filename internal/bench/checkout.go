package bench

import (
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"worktree-bench/internal/gitutil"
)

var prPattern = regexp.MustCompile(`^(#|pr:)?(\d+)$`)

func CheckoutTarget(path, ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	if number, ok := parsePRNumber(ref); ok {
		return ghCheckoutPR(path, number)
	}
	return gitutil.CheckoutBranch(path, ref)
}

func parsePRNumber(ref string) (int, bool) {
	if prPattern.MatchString(ref) {
		matches := prPattern.FindStringSubmatch(ref)
		number, err := strconv.Atoi(matches[2])
		return number, err == nil && number > 0
	}
	parsed, err := url.Parse(ref)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return 0, false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 4 || parts[0] == "" || parts[1] == "" || parts[2] != "pull" {
		return 0, false
	}
	number, err := strconv.Atoi(parts[3])
	return number, err == nil && number > 0
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
