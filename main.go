package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/webbcam/git-autocommit/internal/agent"
	"github.com/webbcam/git-autocommit/internal/config"
	"github.com/webbcam/git-autocommit/internal/git"
	"github.com/webbcam/git-autocommit/internal/parser"
	"github.com/webbcam/git-autocommit/internal/prompt"
	tmpl "github.com/webbcam/git-autocommit/internal/template"
)

const usageText = `git-autocommit — Generate git commit messages using AI

Usage:
  git-autocommit [options]
  git-autocommit config
  git-autocommit templates list
  git-autocommit templates show <name>

Commands:
  config                      Interactive setup wizard to configure your AI agent
  templates list              List all available templates
  templates show <name>       Print a template's contents

Options:
  --template <name-or-path>   Use the named template or path to a .tmpl file
  --short                     Shorthand for --template short (single-line message)
  --skip                      Skip confirmation prompt and commit immediately
  --squash VALUE              Squash commits. VALUE is an integer N (≥2), a single ref, or REF1..REF2
  --rewrite [REF]             Rewrite an existing commit's message. Optional REF targets a specific commit
  --context VALUE             Additional context for the AI (file path, URL, or plain string)
  -v, --verbose               Log the resolved template and its source before generating
  -h, --help                  Print this usage information

Environment variables:
  GIT_AUTOCOMMIT_TEMPLATE     Template name or path (overridden by --template / --short)

Examples:
  git-autocommit config                    # Set up your AI agent
  git-autocommit                           # Commit staged changes (uses full template)
  git-autocommit --short                   # Single-line commit message
  git-autocommit --template conventional  # Use a named template
  git-autocommit --template ./my.tmpl     # Use a template file
  git-autocommit --squash 3               # Squash last 3 commits
  git-autocommit --squash abc123..HEAD    # Squash range via interactive rebase
  git-autocommit --rewrite                # Amend last commit message
  git-autocommit --rewrite abc123         # Rewrite a specific commit's message
  git-autocommit --context ./ticket.md    # Use file as additional context
  git-autocommit templates list           # Show available templates
  git-autocommit templates show full      # Print the full template
`

// options holds parsed CLI arguments.
type options struct {
	templateName string // --template flag value (empty = not set)
	short        bool   // --short flag
	verbose      bool   // -v / --verbose
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
	opts := &options{}

	i := 0
	for i < len(args) {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			opts.help = true
			return opts, nil
		case "--template":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("--template requires an argument.")
			}
			opts.templateName = args[i]
		case "--short":
			opts.short = true
		case "-v", "--verbose":
			opts.verbose = true
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
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "config":
			return runConfig(os.Stdin, os.Stdout)
		case "templates":
			return runTemplates(args[1:], os.Stdout)
		}
	}
	return runWithDeps(runDeps{
		args:    args,
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

	// Validate mutually exclusive flags
	if opts.templateName != "" && opts.short {
		return fmt.Errorf("--short and --template are mutually exclusive")
	}
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
		gitCmd, commitFunc, err = handleStandard()
		if err != nil {
			return err
		}
	}

	// Resolve which template to use
	repoRoot, _ := git.RunCommand([]string{"rev-parse", "--show-toplevel"})
	repoRoot = strings.TrimSpace(repoRoot)
	cwd, _ := os.Getwd()
	remoteURL, _ := git.RunCommand([]string{"remote", "get-url", "origin"})
	remoteURL = strings.TrimSpace(remoteURL)

	ref, err := config.ResolveTemplate(config.ResolveInput{
		FlagTemplate: opts.templateName,
		FlagShort:    opts.short,
		GlobalCfg:    cfg,
		RepoRoot:     repoRoot,
		Cwd:          cwd,
		RemoteURL:    remoteURL,
	})
	if err != nil {
		return err
	}

	// Load and render the template
	t, err := tmpl.Load(ref.Ref)
	if err != nil {
		return fmt.Errorf("loading template: %w", err)
	}
	templateBody, err := t.Render()
	if err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}

	if opts.verbose {
		fmt.Fprintf(os.Stderr, "template: %s (source: %s)\n", ref.Ref, ref.Source)
	}

	// Run the git command to get the diff
	parts := strings.Fields(gitCmd)
	diff, err := git.RunCommand(parts[1:])
	if err != nil {
		return err
	}

	// Determine context type
	var ctx prompt.Context
	if opts.context {
		ctx = prompt.DetectContextType(opts.contextValue)
	}

	// Resolve context content before building the prompt
	if err := resolveContext(&ctx); err != nil {
		return err
	}

	// Build the prompt
	p := prompt.Build(diff, templateBody, ctx)

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

const templatesUsageText = `Usage:
  git-autocommit templates list
  git-autocommit templates show <name>

Commands:
  list           List all available templates (built-in and user-defined)
  show <name>    Print the contents of a named template
`

// runTemplates handles the "templates" subcommand.
func runTemplates(args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(out, templatesUsageText)
		return nil
	}
	switch args[0] {
	case "list":
		return runTemplatesList(out)
	case "show":
		if len(args) < 2 {
			return fmt.Errorf("usage: git-autocommit templates show <name>")
		}
		return runTemplatesShow(args[1], out)
	default:
		return fmt.Errorf("unknown templates subcommand: %s", args[0])
	}
}

// runTemplatesList prints all available templates grouped by source.
func runTemplatesList(out io.Writer) error {
	builtins, err := tmpl.AllEmbedded()
	if err != nil {
		return err
	}
	userTmpls, err := tmpl.UserTemplates()
	if err != nil {
		return err
	}

	// Build a set of user template names for override detection.
	userNames := make(map[string]bool, len(userTmpls))
	for _, t := range userTmpls {
		userNames[t.Name] = true
	}

	fmt.Fprintln(out, "Built-in:")
	for _, t := range builtins {
		fmt.Fprintf(out, "  %-14s %s\n", t.Name, t.Description)
	}

	if len(userTmpls) > 0 {
		userDir, _ := tmpl.UserTemplateDir()
		fmt.Fprintf(out, "\nUser (%s):\n", userDir)
		for _, t := range userTmpls {
			desc := t.Description
			if _, isBuiltin := embeddedNameSet(builtins)[t.Name]; isBuiltin {
				if desc != "" {
					desc = "(overrides built-in)  " + desc
				} else {
					desc = "(overrides built-in)"
				}
			}
			fmt.Fprintf(out, "  %-14s %s\n", t.Name, desc)
		}
	}

	return nil
}

// embeddedNameSet returns a set of names from a slice of templates.
func embeddedNameSet(ts []*tmpl.Template) map[string]struct{} {
	m := make(map[string]struct{}, len(ts))
	for _, t := range ts {
		m[t.Name] = struct{}{}
	}
	return m
}

// runTemplatesShow prints the raw contents of a named template.
func runTemplatesShow(name string, out io.Writer) error {
	raw, err := tmpl.LoadRaw(name)
	if err != nil {
		return err
	}
	fmt.Fprint(out, raw)
	return nil
}

// fieldDef describes a single config field that the wizard should prompt for.
type fieldDef struct {
	label      string                            // prompt text
	required   bool                              // re-prompt until non-empty when true
	defaultVal string                            // shown in brackets; used when input is empty
	set        func(*config.AgentConfig, string) // writes the value into the config
}

// agentDef is a registry entry: wizard metadata + how to build the agent.
type agentDef struct {
	name   string
	desc   string
	fields []fieldDef
	build  func(*config.Config) (agent.Agent, error)
}

// agentRegistry is the single source of truth for all supported agents.
var agentRegistry = []agentDef{
	{
		name: "claude",
		desc: "Claude CLI",
		fields: []fieldDef{
			{label: "Binary path", defaultVal: "claude", set: func(a *config.AgentConfig, v string) { a.Binary = v }},
			{label: "Model", defaultVal: "sonnet", set: func(a *config.AgentConfig, v string) { a.Model = v }},
		},
		build: func(cfg *config.Config) (agent.Agent, error) {
			return agent.NewClaudeAgent(cfg.Agent.Binary, cfg.Agent.Model), nil
		},
	},
	{
		name: "anthropic",
		desc: "Anthropic API",
		fields: []fieldDef{
			{label: "API key", required: true, set: func(a *config.AgentConfig, v string) { a.APIKey = v }},
			{label: "Model", defaultVal: "claude-sonnet-4-6", set: func(a *config.AgentConfig, v string) { a.Model = v }},
		},
		build: func(cfg *config.Config) (agent.Agent, error) {
			if cfg.Agent.APIKey == "" {
				return nil, fmt.Errorf("anthropic agent requires an API key (set ANTHROPIC_API_KEY or api_key in config)")
			}
			return agent.NewAnthropicAgent(cfg.Agent.APIKey, cfg.Agent.Model), nil
		},
	},
	{
		name: "openai",
		desc: "OpenAI API (or compatible)",
		fields: []fieldDef{
			{label: "API key", required: true, set: func(a *config.AgentConfig, v string) { a.APIKey = v }},
			{label: "Model", defaultVal: "gpt-4o", set: func(a *config.AgentConfig, v string) { a.Model = v }},
			{label: "Base URL (optional, press Enter for OpenAI default)", set: func(a *config.AgentConfig, v string) { a.BaseURL = v }},
		},
		build: func(cfg *config.Config) (agent.Agent, error) {
			if cfg.Agent.APIKey == "" {
				return nil, fmt.Errorf("openai agent requires an API key (set OPENAI_API_KEY or api_key in config)")
			}
			return agent.NewOpenAIAgent(cfg.Agent.APIKey, cfg.Agent.Model, cfg.Agent.BaseURL), nil
		},
	},
	{
		name: "ollama",
		desc: "Ollama (local)",
		fields: []fieldDef{
			{label: "Model", required: true, set: func(a *config.AgentConfig, v string) { a.Model = v }},
			{label: "Base URL", defaultVal: "http://localhost:11434/v1/chat/completions", set: func(a *config.AgentConfig, v string) { a.BaseURL = v }},
		},
		build: func(cfg *config.Config) (agent.Agent, error) {
			return agent.NewOllamaAgent(cfg.Agent.Model, cfg.Agent.BaseURL), nil
		},
	},
	{
		name: "opencode",
		desc: "OpenCode CLI",
		fields: []fieldDef{
			{label: "Binary path", defaultVal: "opencode", set: func(a *config.AgentConfig, v string) { a.Binary = v }},
			{label: "Model (optional, press Enter to skip)", set: func(a *config.AgentConfig, v string) { a.Model = v }},
		},
		build: func(cfg *config.Config) (agent.Agent, error) {
			return agent.NewOpenCodeAgent(cfg.Agent.Binary, cfg.Agent.Model), nil
		},
	},
	{
		name: "opencode-go",
		desc: "OpenCode Go (direct API)",
		fields: []fieldDef{
			{label: "API key", required: true, set: func(a *config.AgentConfig, v string) { a.APIKey = v }},
			{label: "Model (e.g. kimi-k2.5, glm-5.1, minimax-m2.7)", defaultVal: "kimi-k2.5", set: func(a *config.AgentConfig, v string) { a.Model = v }},
			{label: "Endpoint type — openai (default) or anthropic (for minimax models)", defaultVal: "openai", set: func(a *config.AgentConfig, v string) { a.EndpointType = v }},
		},
		build: func(cfg *config.Config) (agent.Agent, error) {
			if cfg.Agent.APIKey == "" {
				return nil, fmt.Errorf("opencode-go agent requires an API key (set OPENCODE_GO_API_KEY or api_key in config)")
			}
			return agent.NewOpenCodeGoAgent(cfg.Agent.APIKey, cfg.Agent.Model, cfg.Agent.EndpointType), nil
		},
	},
	{
		name: "kiro",
		desc: "Kiro CLI",
		fields: []fieldDef{
			{label: "Binary path", defaultVal: "kiro", set: func(a *config.AgentConfig, v string) { a.Binary = v }},
		},
		build: func(cfg *config.Config) (agent.Agent, error) {
			return agent.NewKiroAgent(cfg.Agent.Binary), nil
		},
	},
}

// runConfig runs the interactive configuration wizard.
func runConfig(stdin io.Reader, stdout io.Writer) error {
	reader := bufio.NewReader(stdin)

	fmt.Fprintln(stdout, "Select agent type:")
	for i, def := range agentRegistry {
		fmt.Fprintf(stdout, "  %d. %-12s %s\n", i+1, def.name, def.desc)
	}
	fmt.Fprintf(stdout, "Enter choice [1-%d]: ", len(agentRegistry))
	line, _ := reader.ReadString('\n')
	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 1 || choice > len(agentRegistry) {
		return fmt.Errorf("invalid choice")
	}

	def := agentRegistry[choice-1]
	cfg := &config.Config{}
	cfg.Agent.Type = def.name

	fmt.Fprintln(stdout)
	for _, field := range def.fields {
		var val string
		for {
			if field.defaultVal != "" {
				fmt.Fprintf(stdout, "%s [%s]: ", field.label, field.defaultVal)
			} else {
				fmt.Fprintf(stdout, "%s: ", field.label)
			}
			line, _ := reader.ReadString('\n')
			val = strings.TrimSpace(line)
			if val == "" {
				if field.required {
					fmt.Fprintln(stdout, "  This field is required.")
					continue
				}
				val = field.defaultVal
			}
			break
		}
		field.set(&cfg.Agent, val)
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	configPath, _ := config.DefaultConfigPath()
	fmt.Fprintf(stdout, "\nConfiguration saved to %s\n", configPath)
	return nil
}

// buildAgent constructs the Agent from config using the registry.
func buildAgent(cfg *config.Config) (agent.Agent, error) {
	for _, def := range agentRegistry {
		if strings.EqualFold(def.name, cfg.Agent.Type) {
			return def.build(cfg)
		}
	}
	return nil, fmt.Errorf("unsupported agent type: %s", cfg.Agent.Type)
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
		if intVal < 2 {
			return "", nil, fmt.Errorf(
				"--squash N requires N ≥ 2. To rewrite a single commit's message, use --rewrite instead.",
			)
		}
		headRef := fmt.Sprintf("HEAD~%d", intVal)
		if err := git.ValidateRef(headRef); err != nil {
			hasParent, parentErr := git.HasParent()
			if parentErr != nil {
				return "", nil, parentErr
			}
			if !hasParent {
				return "", nil, fmt.Errorf("not enough commits to squash %d: repository has only 1 commit", intVal)
			}
			const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
			gitCmd := fmt.Sprintf("git diff %s HEAD", emptyTree)
			commitFn := func(msg string) error {
				root, err := git.RootCommit()
				if err != nil {
					return err
				}
				if err := git.SoftReset(root); err != nil {
					return err
				}
				return git.AmendCommit(msg)
			}
			return gitCmd, commitFn, nil
		}
		gitCmd := fmt.Sprintf("git diff %s HEAD", headRef)
		commitFn := func(msg string) error {
			if err := git.SoftReset(headRef); err != nil {
				return err
			}
			return git.Commit(msg)
		}
		return gitCmd, commitFn, nil

	case isSingleRef:
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
		if err := git.ValidateRef(ref1); err != nil {
			return "", nil, err
		}
		if err := git.ValidateRef(ref2); err != nil {
			return "", nil, err
		}
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
		hasParent, err := git.HasParent()
		if err != nil {
			return "", nil, err
		}

		var gitCmd string
		if hasParent {
			gitCmd = "git diff HEAD~1 HEAD"
		} else {
			gitCmd = "git show HEAD --format= -p"
		}

		commitFn := func(msg string) error {
			return git.AmendCommit(msg)
		}
		return gitCmd, commitFn, nil
	}

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
