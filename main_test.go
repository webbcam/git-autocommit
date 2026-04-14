package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/webbcam/git-autocommit/internal/agent"
	"github.com/webbcam/git-autocommit/internal/config"
)

// ---- parseArgs tests ----

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    options
		wantErr bool
	}{
		{
			name: "default (no args)",
			args: []string{},
			want: options{},
		},
		{
			name: "--short",
			args: []string{"--short"},
			want: options{short: true},
		},
		{
			name: "--template explicit",
			args: []string{"--template", "conventional"},
			want: options{templateName: "conventional"},
		},
		{
			name: "--template with path",
			args: []string{"--template", "./my.tmpl"},
			want: options{templateName: "./my.tmpl"},
		},
		{
			name:    "--template missing value",
			args:    []string{"--template"},
			wantErr: true,
		},
		{
			name: "--skip",
			args: []string{"--skip"},
			want: options{skip: true},
		},
		{
			name: "-v",
			args: []string{"-v"},
			want: options{verbose: true},
		},
		{
			name: "--verbose",
			args: []string{"--verbose"},
			want: options{verbose: true},
		},
		{
			name: "--squash integer",
			args: []string{"--squash", "3"},
			want: options{squash: true, squashValue: "3"},
		},
		{
			name: "--squash range",
			args: []string{"--squash", "abc..HEAD"},
			want: options{squash: true, squashValue: "abc..HEAD"},
		},
		{
			name:    "--squash missing value",
			args:    []string{"--squash"},
			wantErr: true,
		},
		{
			name: "--rewrite no ref",
			args: []string{"--rewrite"},
			want: options{rewrite: true},
		},
		{
			name: "--rewrite with ref",
			args: []string{"--rewrite", "abc123"},
			want: options{rewrite: true, rewriteRef: "abc123"},
		},
		{
			name: "--rewrite does not consume following flag as ref",
			args: []string{"--rewrite", "--skip"},
			want: options{rewrite: true, skip: true},
		},
		{
			name: "--context with value",
			args: []string{"--context", "some plain text"},
			want: options{context: true, contextValue: "some plain text"},
		},
		{
			name:    "--context missing value",
			args:    []string{"--context"},
			wantErr: true,
		},
		{
			name: "-h",
			args: []string{"-h"},
			want: options{help: true},
		},
		{
			name: "--help",
			args: []string{"--help"},
			want: options{help: true},
		},
		{
			name:    "unknown flag",
			args:    []string{"--unknown"},
			wantErr: true,
		},
		{
			name: "--short --skip combined",
			args: []string{"--short", "--skip"},
			want: options{short: true, skip: true},
		},
		{
			name: "--squash --short --skip combined",
			args: []string{"--squash", "2", "--short", "--skip"},
			want: options{short: true, skip: true, squash: true, squashValue: "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs(%v) err = %v, wantErr = %v", tt.args, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if *got != tt.want {
				t.Errorf("parseArgs(%v)\n  got  %+v\n  want %+v", tt.args, *got, tt.want)
			}
		})
	}
}

func TestParseArgs_TemplateAndShortMutuallyExclusive(t *testing.T) {
	// Mutual exclusion is validated in runWithDeps, not parseArgs.
	// parseArgs itself should succeed for both flags.
	got, err := parseArgs([]string{"--template", "full", "--short"})
	if err != nil {
		t.Fatalf("parseArgs should not error: %v", err)
	}
	if got.templateName != "full" || !got.short {
		t.Error("expected both flags to be set")
	}
}

// ---- Orchestration tests ----

// stubAgent is a fake Agent implementation for orchestration tests.
type stubAgent struct {
	msg            string
	receivedPrompt string
}

func (s *stubAgent) Generate(p string) (string, error) {
	s.receivedPrompt = p
	return "===COMMIT_MSG_START===\n" + s.msg + "\n===COMMIT_MSG_END===", nil
}

// testAgentFn returns a runDeps.agentFn that always returns stub.
func testAgentFn(stub agent.Agent) func(*config.Config) (agent.Agent, error) {
	return func(*config.Config) (agent.Agent, error) { return stub, nil }
}

// testCfg returns a minimal config that satisfies validation.
func testCfg() *config.Config {
	return &config.Config{Agent: config.AgentConfig{Type: "claude"}}
}

// ---- test repo helpers ----
// Tests that use initTestRepo must NOT call t.Parallel() — they change the
// process working directory.

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init")
	mustGit(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	mustGit(t, dir, "config", "user.email", "test@example.com")
	mustGit(t, dir, "config", "user.name", "Test")
	mustGit(t, dir, "config", "commit.gpgsign", "false")

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return dir
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s", strings.Join(args, " "), out)
	}
}

func stageFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", name)
}

// makeCommit appends to a shared file to ensure a non-empty diff per commit.
func makeCommit(t *testing.T, dir, msg string) string {
	t.Helper()
	path := filepath.Join(dir, "changes.txt")
	content, _ := os.ReadFile(path)
	content = append(content, []byte(msg+"\n")...)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "changes.txt")
	mustGit(t, dir, "commit", "-m", msg)
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func headSubject(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "log", "-1", "--format=%s", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func commitCount(t *testing.T, dir string) int {
	t.Helper()
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// ---- orchestration tests ----

func TestRun_StandardCommit(t *testing.T) {
	dir := initTestRepo(t)
	makeCommit(t, dir, "initial")
	stageFile(t, dir, "new.txt", "hello")

	stub := &stubAgent{msg: "Add greeting file"}
	if err := runWithDeps(runDeps{
		args:    []string{"--skip"},
		stdin:   strings.NewReader(""),
		cfg:     testCfg(),
		agentFn: testAgentFn(stub),
	}); err != nil {
		t.Fatalf("runWithDeps: %v", err)
	}

	if headSubject(t, dir) != "Add greeting file" {
		t.Errorf("head subject = %q", headSubject(t, dir))
	}
	if commitCount(t, dir) != 2 {
		t.Errorf("expected 2 commits, got %d", commitCount(t, dir))
	}
}

func TestRun_NoStagedChanges_Error(t *testing.T) {
	dir := initTestRepo(t)
	makeCommit(t, dir, "initial")

	err := runWithDeps(runDeps{
		args:    []string{"--skip"},
		stdin:   strings.NewReader(""),
		cfg:     testCfg(),
		agentFn: testAgentFn(&stubAgent{msg: "anything"}),
	})
	if err == nil {
		t.Fatal("expected error for no staged changes")
	}
	if !strings.Contains(err.Error(), "No staged changes") {
		t.Errorf("unexpected error message: %v", err)
	}
	_ = dir
}

func TestRun_ShortStyle_PromptMention(t *testing.T) {
	dir := initTestRepo(t)
	makeCommit(t, dir, "initial")
	stageFile(t, dir, "new.txt", "hello")

	stub := &stubAgent{msg: "Add hello"}
	if err := runWithDeps(runDeps{
		args:    []string{"--short", "--skip"},
		stdin:   strings.NewReader(""),
		cfg:     testCfg(),
		agentFn: testAgentFn(stub),
	}); err != nil {
		t.Fatalf("runWithDeps: %v", err)
	}

	if !strings.Contains(stub.receivedPrompt, "72") {
		t.Error("short template prompt should mention 72 character limit")
	}
	_ = dir
}

func TestRun_TemplateAndShort_MutuallyExclusive(t *testing.T) {
	dir := initTestRepo(t)
	makeCommit(t, dir, "initial")

	err := runWithDeps(runDeps{
		args:    []string{"--template", "full", "--short"},
		stdin:   strings.NewReader(""),
		cfg:     testCfg(),
		agentFn: testAgentFn(&stubAgent{msg: "msg"}),
	})
	if err == nil {
		t.Error("expected error for --template and --short together")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want it to mention 'mutually exclusive'", err.Error())
	}
	_ = dir
}

func TestRun_SquashN(t *testing.T) {
	dir := initTestRepo(t)
	makeCommit(t, dir, "base")
	makeCommit(t, dir, "first")
	makeCommit(t, dir, "second")
	makeCommit(t, dir, "third")

	stub := &stubAgent{msg: "Squash three into one"}
	if err := runWithDeps(runDeps{
		args:    []string{"--squash", "3", "--skip"},
		stdin:   strings.NewReader(""),
		cfg:     testCfg(),
		agentFn: testAgentFn(stub),
	}); err != nil {
		t.Fatalf("runWithDeps: %v", err)
	}

	if commitCount(t, dir) != 2 {
		t.Errorf("expected 2 commits after squash, got %d", commitCount(t, dir))
	}
	if headSubject(t, dir) != "Squash three into one" {
		t.Errorf("head subject = %q", headSubject(t, dir))
	}
}

func TestRun_SquashN_All(t *testing.T) {
	dir := initTestRepo(t)
	makeCommit(t, dir, "first")
	makeCommit(t, dir, "second")
	makeCommit(t, dir, "third")

	stub := &stubAgent{msg: "Squash all three"}
	if err := runWithDeps(runDeps{
		args:    []string{"--squash", "3", "--skip"},
		stdin:   strings.NewReader(""),
		cfg:     testCfg(),
		agentFn: testAgentFn(stub),
	}); err != nil {
		t.Fatalf("runWithDeps: %v", err)
	}

	if commitCount(t, dir) != 1 {
		t.Errorf("expected 1 commit after squash-all, got %d", commitCount(t, dir))
	}
	if headSubject(t, dir) != "Squash all three" {
		t.Errorf("head subject = %q", headSubject(t, dir))
	}
}

func TestRun_SquashN_NotEnoughCommits(t *testing.T) {
	dir := initTestRepo(t)
	makeCommit(t, dir, "only commit")

	err := runWithDeps(runDeps{
		args:    []string{"--squash", "3", "--skip"},
		stdin:   strings.NewReader(""),
		cfg:     testCfg(),
		agentFn: testAgentFn(&stubAgent{msg: "unused"}),
	})
	if err == nil {
		t.Fatal("expected error for squash N with only 1 commit, got nil")
	}
	if !strings.Contains(err.Error(), "not enough commits") {
		t.Errorf("error = %q, want it to mention 'not enough commits'", err.Error())
	}
}

func TestRun_Rewrite(t *testing.T) {
	dir := initTestRepo(t)
	makeCommit(t, dir, "base")
	makeCommit(t, dir, "original message")

	stub := &stubAgent{msg: "Rewritten message"}
	if err := runWithDeps(runDeps{
		args:    []string{"--rewrite", "--skip"},
		stdin:   strings.NewReader(""),
		cfg:     testCfg(),
		agentFn: testAgentFn(stub),
	}); err != nil {
		t.Fatalf("runWithDeps: %v", err)
	}

	if headSubject(t, dir) != "Rewritten message" {
		t.Errorf("head subject = %q", headSubject(t, dir))
	}
	if commitCount(t, dir) != 2 {
		t.Errorf("expected 2 commits after rewrite, got %d", commitCount(t, dir))
	}
}

func TestRun_ConfirmationAccepted(t *testing.T) {
	dir := initTestRepo(t)
	makeCommit(t, dir, "initial")
	stageFile(t, dir, "new.txt", "hello")

	stub := &stubAgent{msg: "Add file"}
	if err := runWithDeps(runDeps{
		args:    []string{},
		stdin:   strings.NewReader("y\n"),
		cfg:     testCfg(),
		agentFn: testAgentFn(stub),
	}); err != nil {
		t.Fatalf("runWithDeps: %v", err)
	}

	if headSubject(t, dir) != "Add file" {
		t.Errorf("head subject = %q", headSubject(t, dir))
	}
}

func TestRun_ConfirmationRejected(t *testing.T) {
	dir := initTestRepo(t)
	makeCommit(t, dir, "initial")
	stageFile(t, dir, "new.txt", "hello")
	before := commitCount(t, dir)

	_ = runWithDeps(runDeps{
		args:    []string{},
		stdin:   strings.NewReader("n\n"),
		cfg:     testCfg(),
		agentFn: testAgentFn(&stubAgent{msg: "Add file"}),
	})

	if commitCount(t, dir) != before {
		t.Errorf("expected %d commits (no new commit), got %d", before, commitCount(t, dir))
	}
}

func TestRun_SquashAndRewrite_MutuallyExclusive(t *testing.T) {
	dir := initTestRepo(t)
	makeCommit(t, dir, "initial")

	err := runWithDeps(runDeps{
		args:    []string{"--squash", "2", "--rewrite"},
		stdin:   strings.NewReader(""),
		cfg:     testCfg(),
		agentFn: testAgentFn(&stubAgent{msg: "msg"}),
	})
	if err == nil {
		t.Error("expected error for --squash and --rewrite together")
	}
	_ = dir
}
