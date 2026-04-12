package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/webbcam/git-autocommit/internal/agent"
	"github.com/webbcam/git-autocommit/internal/config"
	"github.com/webbcam/git-autocommit/internal/git"
	"github.com/webbcam/git-autocommit/internal/parser"
	"github.com/webbcam/git-autocommit/internal/prompt"
)

const usageText = `git-autocommit — Generate git commit messages using AI

Usage:
  git-autocommit [options]

Options:
  --formal          Use formal multi-section commit message template (default)
  --informal        Use single-line commit message (max 72 chars)
  --skip            Skip confirmation prompt and commit immediately
  --squash VALUE    Squash commits. VALUE is an integer N (≥2), a single ref, or REF1..REF2
  --rewrite [REF]   Rewrite an existing commit's message. Optional REF targets a specific commit
  --context VALUE   Additional context for the AI (file path, URL, or plain string)
  -h, --help        Print this usage information

Examples:
  git-autocommit                      # Commit staged changes with AI-generated message
  git-autocommit --informal           # Single-line commit message
  git-autocommit --squash 3           # Squash last 3 commits
  git-autocommit --squash abc123..HEAD # Squash range via interactive rebase
  git-autocommit --rewrite            # Amend last commit message
  git-autocommit --rewrite abc123     # Rewrite a specific commit's message
  git-autocommit --context ./ticket.md # Use file as additional context
`

// options holds parsed CLI arguments.
type options struct {
	formal       bool
	informal     bool
	skip         bool
	squash       bool
	squashValue  string
	rewrite      bool
	rewriteRef   string // empty means "rewrite HEAD"
	context      bool
	contextValue string
	help         bool
}

// runDeps holds injectable dependencies for runWithDeps.
// In production, run() provides real OS values. Tests inject stubs.
type runDeps struct {
	args    []string
	stdin   io.Reader
	cfg     *config.Config // if nil, config.Load() is called
	agentFn func(*config.Config) (agent.Agent, error)
}

func printUsage() {
	fmt.Print(usageText)
}

func parseArgs(args []string) (*options, error) {
	opts := &options{
		formal: true, // default
	}

	i := 0
	for i < len(args) {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			opts.help = true
			return opts, nil
		case "--formal":
			opts.formal = true
			opts.informal = false
		case "--informal":
			opts.informal = true
			opts.formal = false
		case "--skip":
			opts.skip = true
		case "--squash":
			opts.squash = true
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("--squash requires an argument.")
			}
			opts.squashValue = args[i]
		case "--rewrite":
			opts.rewrite = true
			// REF is optional: if next arg exists and doesn't start with '-', treat as REF
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				opts.rewriteRef = args[i]
			}
		case "--context":
			opts.context = true
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("--context requires a value.")
			}
			opts.contextValue = args[i]
		default:
			return nil, fmt.Errorf("unknown option: %s", arg)
		}
		i++
	}

	return opts, nil
}

// run is the real entry point, wiring real OS dependencies.
func run() error {
	return runWithDeps(runDeps{
		args:    os.Args[1:],
		stdin:   os.Stdin,
		agentFn: buildAgent,
	})
}

// runWithDeps is the testable core of the program.
func runWithDeps(deps runDeps) error {
	opts, err := parseArgs(deps.args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err)
		printUsage()
		return fmt.Errorf("")
	}

	if opts.help {
		printUsage()
		return nil
	}

	// Validate mutually exclusive modes
	if opts.squash && opts.rewrite {
		return fmt.Errorf("--squash and --rewrite cannot be used together")
	}

	// Early validation for squash integer value (before config load)
	if opts.squash {
		isInt, intVal, _, _, _, _ := git.ParseSquashValue(opts.squashValue)
		if isInt && intVal < 2 {
			return fmt.Errorf(
				"--squash N requires N ≥ 2. To rewrite a single commit's message, use --rewrite instead.",
			)
		}
	}

	// Load configuration
	cfg := deps.cfg
	if cfg == nil {
		cfg, err = config.Load()
		if err != nil {
			return err
		}
	}

	// Determine message style
	style := prompt.StyleFormal
	if opts.informal {
		style = prompt.StyleInformal
	}

	// Determine context type
	var ctx prompt.Context
	if opts.context {
		ctx = prompt.DetectContextType(opts.contextValue)
	}

	// Determine the git command to use for the diff
	var gitCmd string
	var commitFunc func(msg string) error

	switch {
	case opts.squash:
		gitCmd, commitFunc, err = handleSquash(opts)
		if err != nil {
			return err
		}
	case opts.rewrite:
		gitCmd, commitFunc, err = handleRewrite(opts)
		if err != nil {
			return err
		}
	default:
		// Standard commit
		gitCmd, commitFunc, err = handleStandard()
		if err != nil {
			return err
		}
	}

	// Run the git command to get the diff
	parts := strings.Fields(gitCmd) // e.g. ["git", "diff", "--cached"]
	diff, err := git.RunCommand(parts[1:])
	if err != nil {
		return err
	}

	// Resolve context content before building the prompt
	if err := resolveContext(&ctx); err != nil {
		return err
	}

	// Build the prompt
	p := prompt.Build(diff, style, ctx)

	// Create the agent
	ag, err := deps.agentFn(cfg)
	if err != nil {
		return err
	}

	// Generate the commit message
	fmt.Fprintln(os.Stderr, "Generating commit message...")
	raw, err := ag.Generate(p)
	if err != nil {
		return err
	}

	// Parse the commit message from agent output
	msg, err := parser.ExtractCommitMessage(raw)
	if err != nil {
		return err
	}

	// Confirmation
	if !opts.skip {
		fmt.Println("\n--- Proposed commit message ---")
		fmt.Println(msg)
		fmt.Println("-------------------------------")
		fmt.Print("Commit with this message? [y/N] ")

		reader := bufio.NewReader(deps.stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(answer)
		if answer != "y" && answer != "Y" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return fmt.Errorf("")
		}
	}

	// Perform the commit
	if err := commitFunc(msg); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "Committed successfully.")
	return nil
}

// buildAgent constructs the Agent implementation from config.
func buildAgent(cfg *config.Config) (agent.Agent, error) {
	switch strings.ToLower(cfg.Agent.Type) {
	case "claude":
		return agent.NewClaudeAgent(cfg.Agent.Binary, cfg.Agent.Model), nil
	case "anthropic":
		if cfg.Agent.APIKey == "" {
			return nil, fmt.Errorf("anthropic agent requires an API key (set ANTHROPIC_API_KEY or api_key in config)")
		}
		return agent.NewAnthropicAgent(cfg.Agent.APIKey, cfg.Agent.Model), nil
	case "openai":
		if cfg.Agent.APIKey == "" {
			return nil, fmt.Errorf("openai agent requires an API key (set OPENAI_API_KEY or api_key in config)")
		}
		return agent.NewOpenAIAgent(cfg.Agent.APIKey, cfg.Agent.Model, cfg.Agent.BaseURL), nil
	case "ollama":
		return agent.NewOllamaAgent(cfg.Agent.Model, cfg.Agent.BaseURL), nil
	case "opencode":
		return agent.NewOpenCodeAgent(cfg.Agent.Binary, cfg.Agent.Model), nil
	case "kiro":
		return agent.NewKiroAgent(cfg.Agent.Binary), nil
	default:
		return nil, fmt.Errorf("unsupported agent type: %s", cfg.Agent.Type)
	}
}

// handleStandard handles the default commit mode.
func handleStandard() (string, func(string) error, error) {
	hasStagged, err := git.HasStagedChanges()
	if err != nil {
		return "", nil, err
	}
	if !hasStagged {
		return "", nil, fmt.Errorf("No staged changes found. Stage changes with 'git add' first.")
	}

	gitCmd := "git diff --cached"
	commitFn := func(msg string) error {
		return git.Commit(msg)
	}
	return gitCmd, commitFn, nil
}

// handleSquash handles the --squash mode.
func handleSquash(opts *options) (string, func(string) error, error) {
	val := opts.squashValue

	isInt, intVal, ref1, ref2, isSingleRef, err := git.ParseSquashValue(val)
	if err != nil {
		return "", nil, err
	}

	switch {
	case isInt:
		// Integer N
		if intVal < 2 {
			return "", nil, fmt.Errorf(
				"--squash N requires N ≥ 2. To rewrite a single commit's message, use --rewrite instead.",
			)
		}
		// Capture the diff BEFORE reset
		headRef := fmt.Sprintf("HEAD~%d", intVal)
		gitCmd := fmt.Sprintf("git diff %s HEAD", headRef)

		commitFn := func(msg string) error {
			if err := git.SoftReset(headRef); err != nil {
				return err
			}
			return git.Commit(msg)
		}
		return gitCmd, commitFn, nil

	case isSingleRef:
		// Single ref
		if err := git.ValidateRef(ref1); err != nil {
			return "", nil, err
		}
		gitCmd := fmt.Sprintf("git diff %s~1 HEAD", ref1)
		commitFn := func(msg string) error {
			if err := git.SoftReset(ref1 + "~1"); err != nil {
				return err
			}
			return git.Commit(msg)
		}
		return gitCmd, commitFn, nil

	default:
		// Range REF1..REF2
		if err := git.ValidateRef(ref1); err != nil {
			return "", nil, err
		}
		if err := git.ValidateRef(ref2); err != nil {
			return "", nil, err
		}
		// Verify ancestry
		isAnc, err := git.IsAncestor(ref1, ref2)
		if err != nil {
			return "", nil, err
		}
		if !isAnc {
			return "", nil, fmt.Errorf("'%s' is not an ancestor of '%s'", ref1, ref2)
		}

		gitCmd := fmt.Sprintf("git diff %s %s", ref1, ref2)
		commitFn := func(msg string) error {
			return git.RebaseSquashRange(ref1, ref2, msg)
		}
		return gitCmd, commitFn, nil
	}
}

// handleRewrite handles the --rewrite mode.
func handleRewrite(opts *options) (string, func(string) error, error) {
	if opts.rewriteRef == "" {
		// Rewrite HEAD: amend
		hasParent, err := git.HasParent()
		if err != nil {
			return "", nil, err
		}

		var gitCmd string
		if hasParent {
			gitCmd = "git diff HEAD~1 HEAD"
		} else {
			// Root commit
			gitCmd = "git show HEAD --format= -p"
		}

		commitFn := func(msg string) error {
			return git.AmendCommit(msg)
		}
		return gitCmd, commitFn, nil
	}

	// Rewrite a specific ref via interactive rebase
	ref := opts.rewriteRef
	if err := git.ValidateRef(ref); err != nil {
		return "", nil, err
	}

	sha, err := git.ResolveRef(ref)
	if err != nil {
		return "", nil, err
	}

	gitCmd := fmt.Sprintf("git show %s --format= -p", sha)

	commitFn := func(msg string) error {
		// Stash uncommitted changes before rebase
		hasChanges, err := git.HasUncommittedChanges()
		if err != nil {
			return err
		}
		if hasChanges {
			if err := git.Stash(); err != nil {
				return err
			}
			defer func() {
				if popErr := git.StashPop(); popErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to pop stash: %v\n", popErr)
				}
			}()
		}
		return git.RebaseRewordCommit(sha, msg)
	}
	return gitCmd, commitFn, nil
}

// resolveContext populates ctx.Content by reading files or fetching URLs.
// For string contexts, Content is already set by DetectContextType.
func resolveContext(ctx *prompt.Context) error {
	switch ctx.Type {
	case prompt.ContextFile:
		b, err := os.ReadFile(ctx.Value)
		if err != nil {
			return fmt.Errorf("failed to read context file %s: %w", ctx.Value, err)
		}
		ctx.Content = string(b)
	case prompt.ContextURL:
		body, err := fetchURL(ctx.Value)
		if err != nil {
			return err
		}
		ctx.Content = body
	}
	return nil
}

// fetchURL fetches the body of a URL and returns it as a string.
func fetchURL(url string) (string, error) {
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response from %s: %w", url, err)
	}
	return string(b), nil
}

func main() {
	if err := run(); err != nil {
		msg := err.Error()
		if msg != "" {
			fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
		}
		os.Exit(1)
	}
}
