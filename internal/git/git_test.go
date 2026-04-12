package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ---- helpers ----

// testRepo is a temporary git repository for use in tests.
// Tests that use testRepo must NOT call t.Parallel() — they change the
// process working directory via os.Chdir, which affects the whole process.
type testRepo struct {
	dir string
	t   *testing.T
}

// newTestRepo creates a fresh initialised git repository in a temp directory
// and changes the process working directory to it. The original working
// directory is restored automatically via t.Cleanup.
func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	dir := t.TempDir()
	r := &testRepo{dir: dir, t: t}

	r.mustExec("git", "init")
	r.mustExec("git", "symbolic-ref", "HEAD", "refs/heads/main")
	r.mustExec("git", "config", "user.email", "test@example.com")
	r.mustExec("git", "config", "user.name", "Test User")
	r.mustExec("git", "config", "commit.gpgsign", "false")

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	return r
}

// mustExec runs a command with cmd.Dir set to the repo directory and fails
// the test if it errors. Returns trimmed stdout.
func (r *testRepo) mustExec(name string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = r.dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("%s %s: %s", name, strings.Join(args, " "), out)
	}
	return strings.TrimSpace(string(out))
}

// makeCommit appends to a single tracking file and commits it, returning the
// new HEAD SHA. Using a single file ensures every commit has a non-empty diff.
func (r *testRepo) makeCommit(msg string) string {
	r.t.Helper()
	path := filepath.Join(r.dir, "changes.txt")
	content, _ := os.ReadFile(path)
	content = append(content, []byte(msg+"\n")...)
	if err := os.WriteFile(path, content, 0644); err != nil {
		r.t.Fatal(err)
	}
	r.mustExec("git", "add", "changes.txt")
	r.mustExec("git", "commit", "-m", msg)
	return r.mustExec("git", "rev-parse", "HEAD")
}

// stageFile writes a file and stages it.
func (r *testRepo) stageFile(name, content string) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.dir, name), []byte(content), 0644); err != nil {
		r.t.Fatal(err)
	}
	r.mustExec("git", "add", name)
}

// headSubject returns the subject line of the current HEAD commit.
func (r *testRepo) headSubject() string {
	r.t.Helper()
	return r.mustExec("git", "log", "-1", "--format=%s", "HEAD")
}

// commitCount returns the total number of commits reachable from HEAD.
func (r *testRepo) commitCount() int {
	r.t.Helper()
	s := r.mustExec("git", "rev-list", "--count", "HEAD")
	n, err := strconv.Atoi(s)
	if err != nil {
		r.t.Fatalf("invalid commit count %q: %v", s, err)
	}
	return n
}

// subjectAt returns the subject line of the commit at the given ref.
func (r *testRepo) subjectAt(ref string) string {
	r.t.Helper()
	return r.mustExec("git", "log", "-1", "--format=%s", ref)
}

// ---- ParseSquashValue (pure) ----

func TestParseSquashValue_Integer(t *testing.T) {
	tests := []struct {
		val    string
		intVal int
	}{
		{"2", 2},
		{"5", 5},
		{"10", 10},
	}
	for _, tt := range tests {
		isInt, n, _, _, isSingle, err := ParseSquashValue(tt.val)
		if err != nil {
			t.Errorf("ParseSquashValue(%q) err = %v", tt.val, err)
		}
		if !isInt {
			t.Errorf("ParseSquashValue(%q) isInt = false", tt.val)
		}
		if n != tt.intVal {
			t.Errorf("ParseSquashValue(%q) intVal = %d, want %d", tt.val, n, tt.intVal)
		}
		if isSingle {
			t.Errorf("ParseSquashValue(%q) isSingle = true, want false", tt.val)
		}
	}
}

func TestParseSquashValue_Range(t *testing.T) {
	isInt, _, ref1, ref2, isSingle, err := ParseSquashValue("abc123..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if isInt {
		t.Error("isInt should be false for a range")
	}
	if isSingle {
		t.Error("isSingle should be false for a range")
	}
	if ref1 != "abc123" {
		t.Errorf("ref1 = %q, want abc123", ref1)
	}
	if ref2 != "HEAD" {
		t.Errorf("ref2 = %q, want HEAD", ref2)
	}
}

func TestParseSquashValue_SingleRef(t *testing.T) {
	isInt, _, ref1, ref2, isSingle, err := ParseSquashValue("deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if isInt {
		t.Error("isInt should be false for a single ref")
	}
	if !isSingle {
		t.Error("isSingle should be true for a single ref")
	}
	if ref1 != "deadbeef" {
		t.Errorf("ref1 = %q, want deadbeef", ref1)
	}
	if ref2 != "" {
		t.Errorf("ref2 = %q, want empty", ref2)
	}
}

// ---- Git integration tests ----

func TestHasStagedChanges(t *testing.T) {
	r := newTestRepo(t)
	r.makeCommit("initial")

	has, err := HasStagedChanges()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("expected no staged changes after clean commit")
	}

	r.stageFile("new.txt", "hello")
	has, err = HasStagedChanges()
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("expected staged changes after git add")
	}
}

func TestHasParent(t *testing.T) {
	r := newTestRepo(t)
	r.makeCommit("root")

	has, err := HasParent()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("root commit should have no parent")
	}

	r.makeCommit("second")
	has, err = HasParent()
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("non-root commit should have a parent")
	}
}

func TestValidateRef(t *testing.T) {
	r := newTestRepo(t)
	sha := r.makeCommit("first")

	if err := ValidateRef(sha); err != nil {
		t.Errorf("valid SHA should not error: %v", err)
	}
	if err := ValidateRef("HEAD"); err != nil {
		t.Errorf("HEAD should be valid: %v", err)
	}
	if err := ValidateRef("nonexistentref12345"); err == nil {
		t.Error("expected error for invalid ref")
	}
}

func TestIsAncestor(t *testing.T) {
	r := newTestRepo(t)
	sha1 := r.makeCommit("first")
	sha2 := r.makeCommit("second")

	anc, err := IsAncestor(sha1, sha2)
	if err != nil {
		t.Fatal(err)
	}
	if !anc {
		t.Error("sha1 should be an ancestor of sha2")
	}

	anc, err = IsAncestor(sha2, sha1)
	if err != nil {
		t.Fatal(err)
	}
	if anc {
		t.Error("sha2 should not be an ancestor of sha1")
	}
}

func TestCommit(t *testing.T) {
	r := newTestRepo(t)
	r.makeCommit("initial")

	r.stageFile("new.txt", "content")
	if err := Commit("my new commit"); err != nil {
		t.Fatal(err)
	}

	if r.headSubject() != "my new commit" {
		t.Errorf("head subject = %q", r.headSubject())
	}
	if r.commitCount() != 2 {
		t.Errorf("expected 2 commits, got %d", r.commitCount())
	}
}

func TestAmendCommit(t *testing.T) {
	r := newTestRepo(t)
	r.makeCommit("initial")
	r.makeCommit("original message")

	before := r.commitCount()
	if err := AmendCommit("amended message"); err != nil {
		t.Fatal(err)
	}

	if r.commitCount() != before {
		t.Errorf("amend should not change commit count: got %d, want %d", r.commitCount(), before)
	}
	if r.headSubject() != "amended message" {
		t.Errorf("head subject = %q, want %q", r.headSubject(), "amended message")
	}
}

func TestSoftReset(t *testing.T) {
	r := newTestRepo(t)
	r.makeCommit("first")
	r.makeCommit("second")
	r.makeCommit("third")

	if err := SoftReset("HEAD~2"); err != nil {
		t.Fatal(err)
	}

	// Only "first" commit should remain
	if r.commitCount() != 1 {
		t.Errorf("expected 1 commit after reset, got %d", r.commitCount())
	}

	// The changes from second and third should now be staged
	has, err := HasStagedChanges()
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("expected staged changes after soft reset")
	}
}

func TestListCommitsInRange(t *testing.T) {
	r := newTestRepo(t)
	sha1 := r.makeCommit("first")
	sha2 := r.makeCommit("second")
	sha3 := r.makeCommit("third")

	// sha1..HEAD should return sha2 and sha3 (oldest first)
	shas, err := ListCommitsInRange(sha1, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(shas) != 2 {
		t.Fatalf("expected 2 SHAs, got %d: %v", len(shas), shas)
	}
	if shas[0] != sha2 {
		t.Errorf("shas[0] = %q, want sha2 (%q)", shas[0], sha2)
	}
	if shas[1] != sha3 {
		t.Errorf("shas[1] = %q, want sha3 (%q)", shas[1], sha3)
	}
}

func TestRebaseRewordCommit(t *testing.T) {
	r := newTestRepo(t)
	r.makeCommit("base")
	sha := r.makeCommit("original message")
	r.makeCommit("after")
	// Commits: base → original message → after (HEAD)

	before := r.commitCount()
	if err := RebaseRewordCommit(sha, "reworded message"); err != nil {
		t.Fatal(err)
	}

	if r.commitCount() != before {
		t.Errorf("reword should not change commit count: got %d, want %d", r.commitCount(), before)
	}
	// HEAD is still "after"
	if r.headSubject() != "after" {
		t.Errorf("HEAD subject = %q, want %q", r.headSubject(), "after")
	}
	// HEAD~1 should now have the new message
	if r.subjectAt("HEAD~1") != "reworded message" {
		t.Errorf("HEAD~1 subject = %q, want %q", r.subjectAt("HEAD~1"), "reworded message")
	}
}

func TestRebaseSquashRange(t *testing.T) {
	r := newTestRepo(t)
	r.makeCommit("base")
	refA := r.makeCommit("commit-A") // boundary — not squashed
	r.makeCommit("commit-B")         // first in squash range
	refC := r.makeCommit("commit-C") // last in squash range
	r.makeCommit("commit-D")         // preserved after squash
	// Commits: base → A → B → C → D (HEAD)
	// Squash range: refA..refC (B and C are squashed into one)

	if err := RebaseSquashRange(refA, refC, "squashed B+C"); err != nil {
		t.Fatal(err)
	}

	// base → A → squashed(B+C) → D = 4 commits
	if r.commitCount() != 4 {
		t.Errorf("expected 4 commits after squash, got %d", r.commitCount())
	}
	// HEAD is still commit-D (preserved)
	if r.headSubject() != "commit-D" {
		t.Errorf("HEAD subject = %q, want commit-D", r.headSubject())
	}
	// HEAD~1 is the squashed commit
	if r.subjectAt("HEAD~1") != "squashed B+C" {
		t.Errorf("HEAD~1 subject = %q, want %q", r.subjectAt("HEAD~1"), "squashed B+C")
	}
	// HEAD~2 is commit-A (unchanged)
	if r.subjectAt("HEAD~2") != "commit-A" {
		t.Errorf("HEAD~2 subject = %q, want commit-A", r.subjectAt("HEAD~2"))
	}
}
