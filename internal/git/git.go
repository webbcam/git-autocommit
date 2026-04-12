package git

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// RunCommand executes a git subcommand with the given arguments and returns stdout.
func RunCommand(args []string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// HasStagedChanges returns true if there are staged changes in the current repository.
func HasStagedChanges() (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// exit code 1 means there are differences (staged changes exist)
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, fmt.Errorf("failed to check staged changes: %w", err)
	}
	// exit code 0 means no differences
	return false, nil
}

// ValidateRef checks that the given git ref is valid and resolvable.
func ValidateRef(ref string) error {
	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("invalid ref '%s'", ref)
	}
	return nil
}

// ResolveRef resolves a ref to its full SHA.
func ResolveRef(ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("invalid ref '%s'", ref)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsAncestor returns true if ref1 is an ancestor of ref2.
func IsAncestor(ref1, ref2 string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ref1, ref2)
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check ancestry: %w", err)
	}
	return true, nil
}

// HasParent returns true if HEAD has a parent commit (i.e., is not the root commit).
func HasParent() (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", "HEAD~1")
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, fmt.Errorf("failed to check parent: %w", err)
	}
	return true, nil
}

// SoftReset performs a git reset --soft to the given ref.
func SoftReset(ref string) error {
	cmd := exec.Command("git", "reset", "--soft", ref)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset --soft %s failed: %s", ref, string(out))
	}
	return nil
}

// Commit creates a new commit with the given message.
func Commit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %s", string(out))
	}
	return nil
}

// AmendCommit amends the last commit with the given message.
func AmendCommit(message string) error {
	cmd := exec.Command("git", "commit", "--amend", "-m", message)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit --amend failed: %s", string(out))
	}
	return nil
}

// Stash stashes uncommitted changes.
func Stash() error {
	cmd := exec.Command("git", "stash")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git stash failed: %s", string(out))
	}
	return nil
}

// StashPop pops the most recent stash.
func StashPop() error {
	cmd := exec.Command("git", "stash", "pop")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git stash pop failed: %s", string(out))
	}
	return nil
}

// HasUncommittedChanges returns true if there are uncommitted changes (staged or unstaged).
func HasUncommittedChanges() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// ListCommitsInRange returns the list of commit SHAs in REF1..REF2 (oldest first).
func ListCommitsInRange(ref1, ref2 string) ([]string, error) {
	rangeArg := ref1 + ".." + ref2
	cmd := exec.Command("git", "log", "--reverse", "--format=%H", rangeArg)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list commits in range %s: %w", rangeArg, err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var shas []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			shas = append(shas, l)
		}
	}
	return shas, nil
}

// RebaseRewordCommit runs interactive rebase to reword a single commit identified by sha.
// newMessage is the new commit message.
func RebaseRewordCommit(sha, newMessage string) error {
	// Commits after sha up to HEAD must be preserved with pick.
	afterSHAs, err := ListCommitsInRange(sha, "HEAD")
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("reword %s placeholder\n", sha))
	for _, afterSHA := range afterSHAs {
		sb.WriteString(fmt.Sprintf("pick %s placeholder\n", afterSHA))
	}
	scriptContent := sb.String()

	msgFile, err := writeTempFile(newMessage)
	if err != nil {
		return fmt.Errorf("failed to write commit message: %w", err)
	}
	defer func() { _ = os.Remove(msgFile) }()

	todoFile, err := writeTempFile(scriptContent)
	if err != nil {
		return fmt.Errorf("failed to write rebase todo: %w", err)
	}
	defer func() { _ = os.Remove(todoFile) }()

	sequenceEditor := fmt.Sprintf("cp %s", shellEscape(todoFile))
	commitEditor := fmt.Sprintf("cp %s", shellEscape(msgFile))

	baseRef := sha + "~1"
	cmd := exec.Command("git", "rebase", "-i", baseRef)
	cmd.Env = append(os.Environ(),
		"GIT_SEQUENCE_EDITOR="+sequenceEditor,
		"GIT_EDITOR="+commitEditor,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git rebase failed: %s", string(out))
	}
	return nil
}

// RebaseSquashRange squashes commits in REF1..REF2 via interactive rebase.
// The first commit in the range is reworded, the rest are fixup'd.
// Any commits after REF2 up to HEAD are preserved with pick.
func RebaseSquashRange(ref1, ref2, newMessage string) error {
	// Commits to squash: ref1..ref2
	rangeSHAs, err := ListCommitsInRange(ref1, ref2)
	if err != nil {
		return err
	}
	if len(rangeSHAs) == 0 {
		return fmt.Errorf("no commits found in range %s..%s", ref1, ref2)
	}

	// Commits after ref2 up to HEAD that must be preserved
	afterSHAs, err := ListCommitsInRange(ref2, "HEAD")
	if err != nil {
		return err
	}

	var sb strings.Builder
	for i, sha := range rangeSHAs {
		if i == 0 {
			sb.WriteString(fmt.Sprintf("reword %s placeholder\n", sha))
		} else {
			sb.WriteString(fmt.Sprintf("fixup %s placeholder\n", sha))
		}
	}
	for _, sha := range afterSHAs {
		sb.WriteString(fmt.Sprintf("pick %s placeholder\n", sha))
	}
	scriptContent := sb.String()

	msgFile, err := writeTempFile(newMessage)
	if err != nil {
		return fmt.Errorf("failed to write commit message: %w", err)
	}
	defer func() { _ = os.Remove(msgFile) }()

	todoFile, err := writeTempFile(scriptContent)
	if err != nil {
		return fmt.Errorf("failed to write rebase todo: %w", err)
	}
	defer func() { _ = os.Remove(todoFile) }()

	sequenceEditor := fmt.Sprintf("cp %s", shellEscape(todoFile))
	commitEditor := fmt.Sprintf("cp %s", shellEscape(msgFile))

	cmd := exec.Command("git", "rebase", "-i", ref1)
	cmd.Env = append(os.Environ(),
		"GIT_SEQUENCE_EDITOR="+sequenceEditor,
		"GIT_EDITOR="+commitEditor,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git rebase failed: %s", string(out))
	}
	return nil
}

// ParseSquashValue parses the value given to --squash.
// Returns (isInt, intVal, ref1, ref2, isSingleRef, err).
// If isSingleRef, ref1 is the single ref and ref2 is empty.
// If !isInt && !isSingleRef, it's a range (ref1..ref2).
func ParseSquashValue(val string) (isInt bool, intVal int, ref1, ref2 string, isSingleRef bool, err error) {
	// Check if it's a range REF1..REF2
	if idx := strings.Index(val, ".."); idx >= 0 {
		r1 := val[:idx]
		r2 := val[idx+2:]
		return false, 0, r1, r2, false, nil
	}

	// Check if it's an integer
	if n, parseErr := strconv.Atoi(val); parseErr == nil {
		return true, n, "", "", false, nil
	}

	// It's a single ref
	return false, 0, val, "", true, nil
}

// writeTempFile writes content to a temporary file and returns its path.
func writeTempFile(content string) (string, error) {
	f, err := os.CreateTemp("", "git-autocommit-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	return f.Name(), nil
}

// shellEscape quotes a string for safe use in shell single-quoted strings.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
