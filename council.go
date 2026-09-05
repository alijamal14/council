package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

var Version = "1.6.0"

var (
	commit = "none"
	date   = "unknown"
)

type Config struct {
	AgentRunTimeout   int
	AgentCheckTimeout int
	PingTimeout       int
	AllowedAgents     []string
	Verbose           bool
	IsTerminal        bool
	ContinueDir       string
	ContinueFeedback  string
	Task              string
	RemoteHost        string
	Subcommand        string   // "install", "doctor", "setup", "login", "models", "update", "config", or "" (default)
	SubArgs           []string // raw args after the subcommand (config, login)
	DoctorJSON        bool     // doctor: emit machine-readable JSON
	DoctorPing        bool     // doctor: verify auth with a real (tiny) prompt per agent
	SetupApply        bool     // setup: actually run install commands
	SetupFree         bool     // setup: restrict to free-tier / open-source agents
	UpdateCheck       bool     // update: report versions only, change nothing
	UpdateQuiet       bool     // update: minimal output (auto-update mode)

	// Derived paths
	RepoRoot       string
	CouncilRunsDir string
	AuditLogMD     string
	AuditLogJSONL  string
	DomainsDir     string
	RegistryFile   string

	// Model Overrides (COUNCIL_<AGENT>_MODEL)
	GeminiModel      string
	CodexModel       string
	ClaudeModel      string
	CopilotModel     string
	CursorModel      string
	AntigravityModel string
	AiderModel       string
	OpenCodeModel    string
	QwenModel        string
	GooseModel       string
	DroidModel       string
	Unrestricted     bool
}

func main() {
	os.Exit(run(context.Background(), parseFlags()))
}

// flush forces stdout to flush immediately — required when stdout is piped
// (e.g. Bash tool) since Go buffers output in non-terminal mode.
func flush() {
	os.Stdout.Sync()
}

// detectCaller identifies which AI agent called the council binary
func detectCaller() string {
	// Claude Code — sets CLAUDECODE env var
	if os.Getenv("CLAUDECODE") == "1" {
		return "claude-code"
	}

	// Codex — sets CODEX_CI, CODEX_THREAD_ID, etc.
	if os.Getenv("CODEX_CI") == "1" || os.Getenv("CODEX_THREAD_ID") != "" {
		return "codex"
	}

	// Copilot — sets COPILOT_AGENT or GITHUB_COPILOT_TOKEN (not generic GH_TOKEN)
	if os.Getenv("COPILOT_AGENT") == "1" || os.Getenv("GITHUB_COPILOT_TOKEN") != "" {
		return "copilot"
	}

	// Gemini — sets GOOGLE_APPLICATION_CREDENTIALS or has ~/.gemini
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" {
		return "gemini"
	}
	homeDir, _ := os.UserHomeDir()
	if _, err := os.Stat(filepath.Join(homeDir, ".gemini")); err == nil {
		return "gemini"
	}

	// Terminal or unknown — assume unknown for safety
	return "unknown"
}

// printInvocationGuide prints caller-specific instructions to stderr
func printInvocationGuide(caller string) {
	var msg string
	switch caller {
	case "claude-code":
		msg = `╔══════════════════════════════════════════════════════════════╗
║  COUNCIL INVOCATION GUIDE (Claude Code)                      ║
║                                                              ║
║  ⚠️  Bash tool does NOT stream output in real-time.          ║
║  You will see NOTHING until council fully completes.         ║
║                                                              ║
║  CORRECT invocation:                                        ║
║    1. Use Bash tool with run_in_background: true            ║
║    2. Poll every 30s: ls council_runs/<latest>/             ║
║                                                              ║
║  Results: council_runs/run_YYYYMMDD_HHMMSS_*/               ║
╚══════════════════════════════════════════════════════════════╝`

	case "codex":
		msg = `╔══════════════════════════════════════════════════════════════╗
║  COUNCIL INVOCATION GUIDE (Codex)                            ║
║                                                              ║
║  ⚠️  Bash tool buffers output until completion.              ║
║  Run in background and check council_runs/ after done.       ║
║                                                              ║
║  CORRECT invocation:                                        ║
║    1. Run with background execution enabled                 ║
║    2. Check: ls council_runs/<latest>/ every 30s            ║
║                                                              ║
║  Results: council_runs/run_YYYYMMDD_HHMMSS_*/               ║
╚══════════════════════════════════════════════════════════════╝`

	case "copilot":
		msg = `╔══════════════════════════════════════════════════════════════╗
║  COUNCIL INVOCATION GUIDE (Copilot)                          ║
║                                                              ║
║  ⚠️  Output is buffered; check council_runs/ after done.      ║
║  Use background execution for best results.                 ║
║                                                              ║
║  CORRECT invocation:                                        ║
║    1. Run with background/async enabled                     ║
║    2. Poll: ls council_runs/<latest>/ every 30s             ║
║                                                              ║
║  Results: council_runs/run_YYYYMMDD_HHMMSS_*/               ║
╚══════════════════════════════════════════════════════════════╝`

	case "gemini":
		msg = `╔══════════════════════════════════════════════════════════════╗
║  COUNCIL INVOCATION GUIDE (Gemini)                           ║
║                                                              ║
║  ⚠️  Output is buffered; check council_runs/ after done.      ║
║  Use background execution for best results.                 ║
║                                                              ║
║  CORRECT invocation:                                        ║
║    1. Run with background/async enabled                     ║
║    2. Poll: ls council_runs/<latest>/ every 30s             ║
║                                                              ║
║  Results: council_runs/run_YYYYMMDD_HHMMSS_*/               ║
╚══════════════════════════════════════════════════════════════╝`

	default:
		msg = `╔══════════════════════════════════════════════════════════════╗
║  COUNCIL RUNNING                                              ║
║                                                              ║
║  Output may be buffered. Check council_runs/ after done.      ║
║  Results: council_runs/run_YYYYMMDD_HHMMSS_*/                ║
╚══════════════════════════════════════════════════════════════╝`
	}
	fmt.Fprintln(os.Stderr, msg)
}

// printUsage is the full help: subcommands first (they are how you set
// council up), then run flags. Wired into both `council help` and `-h`.
func printUsage() {
	fmt.Fprintf(os.Stderr, `AI Council Orchestrator v%s — run multiple AI agent CLIs in parallel on one task

USAGE
  council "task description"                 convene a council
  council --continue <run_dir> "feedback"    continue a previous session
  council <subcommand> [options]

GETTING STARTED (in order)
  council setup                list the 12 supported agent CLIs; missing ones show install commands
  council setup --apply        install ALL missing agents (latest stable, official installers)
  council setup --apply --free install only the free-tier / open-source agents
  council login                sign-in checklist for every installed agent
  council login <agent>        launch that agent's interactive sign-in flow (e.g. council login copilot)
  council doctor               health check; --json machine-readable; --ping live auth verification

EVERYDAY SUBCOMMANDS
  council models               show each agent's model + how to change it (fast vs deep)
  council config set KEY VALUE persist any COUNCIL_* setting (models, roster, auto-update)
  council config list          show persisted settings
  council update               upgrade all installed agent CLIs to latest stable
  council update --check       report versions only
  council install              copy this binary into your PATH
  council version              print version

RUN FLAGS
  --agents <list>        comma-separated roster: gemini,claude,codex,copilot,cursor,antigravity,aider,opencode,qwen,goose,amp,droid
  --timeout <sec>        per-agent run timeout (default 300)
  --ping-timeout <sec>   pre-flight ping timeout per agent (default 45)
  --check-timeout <sec>  agent binary discovery timeout (default 8)
  --continue <dir>       continue a previous session (args after become feedback)
  --repo <path>          repository root override
  --remote <host>        delegate the session to a remote SSH host
  --unrestricted / --yolo / -y   bypass agent sandboxes and approval prompts
  --verbose=false        silence progress output
`, Version)
	fmt.Fprintln(os.Stderr, `
EXAMPLES
  council "Should we migrate this service from REST to gRPC? List risks."
  council --agents claude,codex --timeout 240 "Review this API design"
  council config set COUNCIL_CLAUDE_MODEL haiku     # cheaper model for small tasks
  council config set COUNCIL_AUTO_UPDATE 1          # keep agents fresh automatically

Results land in council_runs/run_<timestamp>/ (plan.<agent>.txt + critiques).
Docs: https://github.com/alijamal14/council`)
}

func parseFlags() Config {
	// Persisted settings (council config set ...) become env defaults before
	// anything reads the environment. Real env vars still win.
	applyPersistentConfig()

	cfg := Config{
		AgentRunTimeout:   300,
		AgentCheckTimeout: 8,
		PingTimeout:       45,
	}

	// Check for non-flag subcommands first
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Printf("AI Council Orchestrator v%s (%s, built on %s)\n", Version, commit, date)
			os.Exit(0)
		case "help", "--help", "-h", "-help":
			printUsage()
			os.Exit(0)
		case "install":
			cfg.Subcommand = "install"
			return cfg
		case "config":
			// No repo, agents, or network needed — handle immediately.
			os.Exit(handleConfig(os.Args[2:]))
		case "doctor":
			cfg.Subcommand = "doctor"
			// Mirrors defaults when normal flag.Parse path is skipped
			cfg.AgentRunTimeout = 300
			cfg.PingTimeout = 45
			for _, arg := range os.Args[2:] {
				switch arg {
				case "--json":
					cfg.DoctorJSON = true
				case "--ping":
					cfg.DoctorPing = true
				}
			}
			return cfg
		case "setup":
			cfg.Subcommand = "setup"
			for _, arg := range os.Args[2:] {
				switch arg {
				case "--apply":
					cfg.SetupApply = true
				case "--free":
					cfg.SetupFree = true
				}
			}
			return cfg
		case "login":
			cfg.Subcommand = "login"
			cfg.SubArgs = os.Args[2:]
			return cfg
		case "models":
			cfg.Subcommand = "models"
			return cfg
		case "update":
			cfg.Subcommand = "update"
			for _, arg := range os.Args[2:] {
				switch arg {
				case "--check":
					cfg.UpdateCheck = true
				case "--quiet":
					cfg.UpdateQuiet = true
				}
			}
			return cfg
		}
	}

	timeoutPtr := flag.Int("timeout", 300, "Per-agent run timeout (plan/critique) in seconds")
	checkTimeoutPtr := flag.Int("check-timeout", 8, "Agent binary discovery probe timeout in seconds")
	pingTimeoutPtr := flag.Int("ping-timeout", 45, "Pre-flight ping timeout per agent (seconds)")
	agentsPtr := flag.String("agents", "", "Comma-separated list of agents (gemini,claude,codex,copilot,cursor,antigravity,aider,opencode,qwen,goose,amp,droid)")
	verbosePtr := flag.Bool("verbose", true, "Verbose output")
	continuePtr := flag.String("continue", "", "Session directory to continue")
	repoPtr := flag.String("repo", "", "Repository root (explicit override)")
	remotePtr := flag.String("remote", "", "Remote host for delegation (e.g., user@host:port)")
	unrestrictedPtr := flag.Bool("unrestricted", false, "Enable unrestricted mode (bypasses sandbox/approvals)")
	yoloPtr := flag.Bool("yolo", false, "Alias for --unrestricted")
	yShortPtr := flag.Bool("y", false, "Alias for --unrestricted")
	versionPtr := flag.Bool("version", false, "Print version and exit")

	flag.Usage = printUsage
	flag.Parse()

	if *versionPtr {
		fmt.Printf("AI Council Orchestrator v%s (%s, built on %s)\n", Version, commit, date)
		os.Exit(0)
	}

	cfg.AgentRunTimeout = *timeoutPtr
	cfg.AgentCheckTimeout = *checkTimeoutPtr
	cfg.PingTimeout = *pingTimeoutPtr
	cfg.Verbose = *verbosePtr
	cfg.RemoteHost = *remotePtr
	// COUNCIL_UNRESTRICTED=1 lets always-on servers and CI default to headless
	// auto-approval without editing every invocation.
	cfg.Unrestricted = *unrestrictedPtr || *yoloPtr || *yShortPtr || os.Getenv("COUNCIL_UNRESTRICTED") == "1"

	if *repoPtr != "" {
		cfg.RepoRoot = *repoPtr
	} else if envRepo := os.Getenv("COUNCIL_REPO_ROOT"); envRepo != "" {
		cfg.RepoRoot = envRepo
	}

	if *agentsPtr != "" {
		cfg.AllowedAgents = strings.Split(*agentsPtr, ",")
	} else if envAgents := os.Getenv("COUNCIL_AGENTS"); envAgents != "" {
		// Persistent default roster (council config set COUNCIL_AGENTS claude,codex)
		cfg.AllowedAgents = strings.Split(envAgents, ",")
	}

	args := flag.Args()
	if *continuePtr != "" {
		cfg.ContinueDir = *continuePtr
		if len(args) > 0 {
			cfg.ContinueFeedback = strings.Join(args, " ")
		}
	} else {
		if len(args) > 0 {
			cfg.Task = strings.Join(args, " ")
		}
	}

	// Load model overrides from environment
	cfg.GeminiModel = os.Getenv("COUNCIL_GEMINI_MODEL")
	cfg.CodexModel = os.Getenv("COUNCIL_CODEX_MODEL")
	cfg.ClaudeModel = os.Getenv("COUNCIL_CLAUDE_MODEL")
	cfg.CopilotModel = os.Getenv("COUNCIL_COPILOT_MODEL")
	cfg.CursorModel = os.Getenv("COUNCIL_CURSOR_MODEL")
	cfg.AntigravityModel = os.Getenv("COUNCIL_ANTIGRAVITY_MODEL")
	cfg.AiderModel = os.Getenv("COUNCIL_AIDER_MODEL")
	cfg.OpenCodeModel = os.Getenv("COUNCIL_OPENCODE_MODEL")
	cfg.QwenModel = os.Getenv("COUNCIL_QWEN_MODEL")
	cfg.GooseModel = os.Getenv("COUNCIL_GOOSE_MODEL")
	cfg.DroidModel = os.Getenv("COUNCIL_DROID_MODEL")

	return cfg
}

func resolveRepoRoot(cfg Config) (string, error) {
	if cfg.RepoRoot != "" {
		if _, err := os.Stat(cfg.RepoRoot); err == nil {
			return filepath.Abs(cfg.RepoRoot)
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	current := cwd
	for {
		// Look for standard project markers
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return filepath.EvalSymlinks(current)
		}
		if _, err := os.Stat(filepath.Join(current, ".council")); err == nil {
			return filepath.EvalSymlinks(current)
		}
		// Legacy support for CLAUDE.md
		if _, err := os.Stat(filepath.Join(current, "CLAUDE.md")); err == nil {
			return filepath.EvalSymlinks(current)
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return cwd, nil
}

func run(ctx context.Context, cfg Config) int {
	// The buffering warning only matters for real council sessions run by AI
	// callers (whose shell tools buffer output). Humans at a terminal get a
	// one-line header; utility subcommands get nothing.
	if cfg.Subcommand == "" {
		if isTerminalFile(os.Stdout) {
			printCompactHeader()
		} else {
			printInvocationGuide(detectCaller())
		}
	}

	cfg.IsTerminal = os.Getenv("TERM") != "" &&
		os.Getenv("CLAUDECODE") == "" &&
		os.Getenv("CODEX_CI") == "" &&
		os.Getenv("CODEX_THREAD_ID") == ""

	os.Unsetenv("CLAUDECODE")

	repoRoot, err := resolveRepoRoot(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving repo root: %v\n", err)
		return 2
	}

	cfg.RepoRoot = repoRoot
	cfg.CouncilRunsDir = filepath.Join(repoRoot, "council_runs")
	// If legacy council_runs doesn't exist, use .council/runs
	if _, err := os.Stat(cfg.CouncilRunsDir); err != nil {
		cfg.CouncilRunsDir = filepath.Join(repoRoot, ".council", "runs")
	}

	cfg.AuditLogMD = filepath.Join(cfg.CouncilRunsDir, "council_audit.md")
	cfg.AuditLogJSONL = filepath.Join(cfg.CouncilRunsDir, "council_audit.jsonl")

	// Domain context is now optional and configurable
	cfg.DomainsDir = os.Getenv("COUNCIL_DOMAINS_DIR")
	if cfg.DomainsDir == "" {
		cfg.DomainsDir = filepath.Join(repoRoot, "context", "domains")
	}
	cfg.RegistryFile = filepath.Join(cfg.DomainsDir, "_registry.template.yml")
	if _, err := os.Stat(cfg.RegistryFile); err != nil {
		cfg.RegistryFile = filepath.Join(cfg.DomainsDir, "_registry.yml")
	}

	// Normalize cwd so every agent resolves the intended repo (Cursor, Copilot codebase tools, MCP roots).
	if cfg.Subcommand != "install" && cfg.RepoRoot != "" {
		if err := os.Chdir(cfg.RepoRoot); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Cannot chdir to repo root %s: %v (agents may use wrong workspace)\n", cfg.RepoRoot, err)
		}
	}

	// Route subcommands
	switch cfg.Subcommand {
	case "install":
		return handleInstall(cfg)
	case "doctor":
		return handleDoctor(ctx, cfg)
	case "setup":
		return handleSetup(ctx, cfg)
	case "login":
		target := ""
		if len(cfg.SubArgs) > 0 && !strings.HasPrefix(cfg.SubArgs[0], "-") {
			target = cfg.SubArgs[0]
		}
		return handleLogin(ctx, cfg, target)
	case "models":
		return handleModels(ctx, cfg)
	case "update":
		return handleUpdate(ctx, cfg, cfg.UpdateCheck, cfg.UpdateQuiet)
	}

	// Acquire the task BEFORE any agent detection or pinging — a missing task
	// must never cost the user a 45-second ping round. At a terminal this is
	// the interactive mode; piped stdin (e.g. `git diff | council`) also works.
	interactive := false
	if cfg.ContinueDir == "" && cfg.Task == "" {
		if isTerminalFile(os.Stdin) && isTerminalFile(os.Stdout) {
			task, ok := promptForTask()
			if !ok {
				return 0
			}
			cfg.Task = task
			interactive = true
		} else if piped := readPipedTask(); piped != "" {
			cfg.Task = piped
		} else {
			fmt.Fprintf(os.Stderr, "Usage: %s \"Task description\"\nRun `council help` for setup, login, models, and update commands.\n", os.Args[0])
			return 2
		}
	}

	os.MkdirAll(cfg.CouncilRunsDir, 0755)

	if cfg.RemoteHost != "" {
		fmt.Printf("🚀 Delegating Council session to remote host: %s\n", cfg.RemoteHost)
		flush()
		return delegateToRemote(ctx, cfg)
	}

	sigCtx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())
	log, err := newAuditLogger(sessionID, []string{cfg.AuditLogMD}, []string{cfg.AuditLogJSONL})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating audit logger: %v\n", err)
		return 2
	}
	defer log.Close()

	detectedAgents := detectAgents(sigCtx, cfg, log)

	if len(cfg.AllowedAgents) > 0 {
		allowedMap := make(map[string]bool)
		for _, a := range cfg.AllowedAgents {
			allowedMap[strings.ToLower(a)] = true
		}
		for agent := range detectedAgents {
			if !allowedMap[strings.ToLower(string(agent))] {
				delete(detectedAgents, agent)
			}
		}
	}

	// Quarantine: agents that failed pre-flight in several consecutive
	// sessions (broken install, obsolete CLI, expired auth) are skipped
	// automatically so they stop costing a ping timeout every run. An
	// explicit --agents mention re-enables an agent.
	st := loadState()
	explicitlyAllowed := map[string]bool{}
	for _, a := range cfg.AllowedAgents {
		explicitlyAllowed[normalizeAgentKey(strings.TrimSpace(a))] = true
	}
	detectedAgents = filterQuarantined(detectedAgents, st, explicitlyAllowed)

	pingedSet := map[AgentName]bool{}
	for name := range detectedAgents {
		pingedSet[name] = true
	}

	fmt.Printf("\n🏓 Pre-flight ping (timeout: %ds per agent)...\n", cfg.PingTimeout)
	flush()
	detectedAgents = pingAgentsParallel(sigCtx, detectedAgents, cfg.PingTimeout, cfg, log)
	fmt.Println()
	flush()

	healthySet := map[AgentName]bool{}
	for name := range detectedAgents {
		healthySet[name] = true
	}
	for _, name := range st.recordPingResults(pingedSet, healthySet) {
		fmt.Printf("⏸️  %s failed its ping %d sessions in a row — auto-disabled until it works again.\n", name, quarantineThreshold)
		fmt.Println("    Fix: council update (refresh the CLI) or council login " + name + " (re-auth), then council doctor --ping")
		log.Log("QUARANTINE", name+" auto-disabled after repeated ping failures")
	}
	st.save()

	if len(detectedAgents) == 0 {
		fmt.Println("❌ No agents passed pre-flight ping.")
		fmt.Println()
		printNoAgentsHelp()
		fmt.Println("   For a full diagnostic, run: council doctor")
		log.Log("ABORT", "No agents passed pre-flight ping")
		return 2
	}

	// Interactive mode loops: each pass is one full plan+critique round, and
	// the feedback> prompt turns the next pass into a --continue of the same
	// session. Non-interactive callers exit after one pass as before.
	for {
		exitCode := runSessionOnce(sigCtx, &cfg, detectedAgents, log, interactive)
		if !interactive || exitCode == 2 {
			maybeAutoUpdate()
			maybeNudgeUpdate(st)
			return exitCode
		}
		feedback, ok := promptFeedback()
		if !ok {
			maybeAutoUpdate()
			return exitCode
		}
		cfg.ContinueFeedback = feedback
	}
}

// runSessionOnce executes one plan+critique round. On the first pass
// cfg.ContinueDir selects new-vs-resumed session; afterwards it always points
// at the session directory so interactive feedback rounds accumulate there.
func runSessionOnce(sigCtx context.Context, cfg *Config, detectedAgents AgentSet, log *AuditLogger, interactive bool) int {
	sessionStart := time.Now()
	repoRoot := cfg.RepoRoot
	var err error

	var runDir, iterDir string
	var taskContext string
	var promptTask string
	if cfg.ContinueDir != "" {
		runDir = cfg.ContinueDir
		if !filepath.IsAbs(runDir) {
			if _, err := os.Stat(runDir); err == nil {
				runDir, _ = filepath.Abs(runDir)
			} else if _, err := os.Stat(filepath.Join(repoRoot, runDir)); err == nil {
				runDir = filepath.Join(repoRoot, runDir)
			}
		}

		if _, err := os.Stat(runDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Session directory %s not found.\n", runDir)
			return 2
		}

		fmt.Println(bold("=== 🔄 CLI AI Council: Resuming Session ==="))
		flush()

		count, _ := countIterDirs(runDir)
		count++
		iterDir, _, _ = createIterDir(runDir)

		if count == 1 {
			appendHistory(runDir, "#### Initial Council Findings\n\n")
		}

		appendHistory(runDir, fmt.Sprintf("#### User Feedback (Round %d)\n%s\n\n", count, cfg.ContinueFeedback))
		taskContext, _ = buildContext(runDir, cfg.AuditLogMD)
		promptTask = "Feedback: " + cfg.ContinueFeedback
	} else {
		if cfg.Task == "" {
			fmt.Fprintf(os.Stderr, "Usage: %s \"Task description\"\n", os.Args[0])
			return 2
		}

		runDir, err = createRunDir(cfg.CouncilRunsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to create run directory.\n")
			return 2
		}

		iterDir = runDir
		writeBrief(runDir, cfg.Task)
		log.AddSessionFile(filepath.Join(runDir, "audit.jsonl"))

		entries, err := parseRegistry(cfg.RegistryFile)
		if err != nil && cfg.Verbose {
			fmt.Fprintf(os.Stderr, "ℹ️  Context Routing: skipped (registry not found: %s)\n", cfg.RegistryFile)
		}
		domainName, score := resolveDomain(cfg.Task, entries)

		var domainContext string
		if score > 0 {
			domainContext = loadDomainManifest(cfg.DomainsDir, domainName)
		}

		taskContext = "TASK: " + cfg.Task + "\n" + domainContext

		// Optional RAG hook: pipe the task through an external retrieval command
		// (e.g. COUNCIL_CONTEXT_CMD="graphify query") and inject its output.
		if retrieved := retrieveExternalContext(sigCtx, cfg.Task, cfg.Verbose); retrieved != "" {
			taskContext += "\nRETRIEVED CONTEXT (from " + os.Getenv("COUNCIL_CONTEXT_CMD") + "):\n" + retrieved + "\n"
		}
		promptTask = cfg.Task

		fmt.Println(bold("=== 🤖 CLI AI Council Convening ==="))
		flush()
	}

	fmt.Printf("📂 Workspace: %s\n", runDir)
	flush()
	if cfg.Unrestricted {
		fmt.Println("\n☢️  UNRESTRICTED MODE: Sandbox and approvals will be bypassed where supported.")
	} else {
		fmt.Println("\n🛡️  RESTRICTED MODE: Agents will run with default safety and approval prompts.")
	}
	flush()
	log.Log("START", "Council session: "+promptTask)

	fmt.Println(bold(cyan("\n--- Phase 1: Planning ---")))
	flush()

	planPrompt := fmt.Sprintf("ROLE: Planner. CONTEXT: %s. OBJECTIVE: %s. TEXT-ONLY OUTPUT ONLY.", taskContext, promptTask)
	_, hasCopilot := detectedAgents[AgentCopilot]
	results := runAgentsParallel(sigCtx, detectedAgents, planPrompt, iterDir, "plan", *cfg, log)

	validPlans := 0
	for _, r := range results {
		if r.Success {
			validPlans++
		}
	}

	fmt.Printf("\n✅ Planning complete. Valid plans: %d/%d\n", validPlans, len(detectedAgents))
	flush()

	if validPlans == 0 {
		fmt.Println("\n❌ All agents failed in Phase 1. Skipping critique.")
		log.Log("SKIP", "Critique skipped — no valid plans produced")
	} else {
		fmt.Println(bold(cyan("\n--- Phase 2: Critique ---")))
		flush()

		entries, _ := os.ReadDir(iterDir)
		var planContents []string
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "plan.") && strings.HasSuffix(e.Name(), ".txt") {
				path := filepath.Join(iterDir, e.Name())
				if isValidOutput(path) {
					content, err := os.ReadFile(path)
					if err == nil {
						planContents = append(planContents, string(content))
					}
				}
			}
		}

		maxPlansLen := 6000
		totalLen := 0
		for _, c := range planContents {
			totalLen += len(c)
		}

		allPlans := ""
		// Sort entries to ensure deterministic labeling and write label_map.json
		labels := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
		labelIdx := 0
		labelMap := make(map[string]string)

		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "plan.") && strings.HasSuffix(e.Name(), ".txt") {
				path := filepath.Join(iterDir, e.Name())
				if isValidOutput(path) {
					agentName := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "plan."), ".txt")
					label := ""
					if labelIdx < len(labels) {
						label = labels[labelIdx]
						labelMap[label] = agentName
						contentBytes, _ := os.ReadFile(path)
						content := string(contentBytes)
						if totalLen > maxPlansLen && len(planContents) > 0 {
							limit := maxPlansLen / len(planContents)
							if len(content) > limit {
								content = content[:limit] + "\n... [Plan truncated to fit Windows CLI limit]"
							}
						}
						allPlans += fmt.Sprintf("### Plan %s:\n%s\n\n", label, content)
						labelIdx++
					}
				}
			}
		}

		// Write label map
		mapData, _ := json.Marshal(labelMap)
		os.WriteFile(filepath.Join(iterDir, "label_map.json"), mapData, 0644)

		critiquePrompt := buildCritiquePrompt(allPlans)
		var dvAgent AgentName

		if len(detectedAgents) >= 2 {
			// Determine devil's advocate agent (rotate by run directory hash for determinism)
			agentList := sortedAgentNames(detectedAgents)

			// Simple hash of runDir
			var hash uint32 = 0
			for i := 0; i < len(runDir); i++ {
				hash = hash*31 + uint32(runDir[i])
			}
			dvIdx := int(hash % uint32(len(agentList)))
			dvAgent = agentList[dvIdx]

			devilPrompt := buildDevilPrompt(allPlans)

			fmt.Printf("🎭 Assigned %s as Devil's Advocate\n", dvAgent)
			flush()

			// Run agents in parallel, giving the devilPrompt to the selected agent
			results = runAgentsParallelWithDevil(sigCtx, detectedAgents, critiquePrompt, devilPrompt, dvAgent, iterDir, "critique", *cfg, log)
		} else {
			fmt.Println("ℹ️  Single agent mode — generating self-critique")
			flush()
			for name, resolved := range detectedAgents {
				lowerName := strings.ToLower(string(name))
				outFile := filepath.Join(iterDir, "critique."+lowerName+".txt")
				runAgent(sigCtx, name, resolved, detectedAgents[AgentCopilot], critiquePrompt, outFile, *cfg, log, hasCopilot)
				break
			}
		}

		validCritiques := countValidFiles(iterDir, "critique.")
		fmt.Printf("\n✅ Critique complete. Valid critiques: %d\n", validCritiques)
		flush()

		// Aggregate anonymous rankings from critiques (best-effort; empty if none).
		if agg, err := aggregateRankings(iterDir); err == nil && len(agg.ByAgent) > 0 {
			if err := writeRankingsJSON(iterDir, agg); err == nil {
				fmt.Printf("📊 Rankings aggregate written (winner: Plan %s)\n", agg.Winner)
				flush()
			}
		}

		// Phase 3: Chairman synthesis (default on when ≥2 valid plans).
		if synthesisEnabled(validPlans) && (validCritiques >= 1 || validPlans >= 2) {
			fmt.Println(bold(cyan("\n--- Phase 3: Chairman Synthesis ---")))
			flush()

			chair, ok := chooseChairman(detectedAgents, labelMap, dvAgent)
			if !ok {
				fmt.Println("⚠️  No chairman candidate available — skipping synthesis")
				log.Log("SKIP", "Synthesis skipped — no chairman candidate")
			} else {
				critiques := collectCritiqueBodies(iterDir)
				synthPrompt := buildSynthesisPrompt(promptTask, allPlans, critiques)
				outFile := filepath.Join(iterDir, "synthesis.txt")
				fmt.Printf("🧑‍⚖️ Chairman: %s\n", chair)
				flush()
				log.Log("SYNTHESIS", fmt.Sprintf("Chairman %s synthesizing consensus", chair))
				res := runAgent(sigCtx, chair, detectedAgents[chair], detectedAgents[AgentCopilot], synthPrompt, outFile, *cfg, log, hasCopilot)
				results = append(results, res)
				if isValidOutput(outFile) {
					fmt.Println("✅ Synthesis complete → synthesis.txt")
					flush()
				} else {
					fmt.Println("⚠️  Synthesis did not produce valid output")
					log.Log("WARN", "Synthesis output invalid or empty")
					flush()
				}
			}
		} else if validPlans >= 2 {
			fmt.Println("ℹ️  Phase 3 synthesis disabled (COUNCIL_SYNTHESIS=0)")
			flush()
			log.Log("SKIP", "Synthesis disabled via COUNCIL_SYNTHESIS")
		}
	}

	fmt.Println(bold(green("\n=== Council Adjourned ===")))
	flush()
	fmt.Printf("📂 Results: %s\n", iterDir)

	printSummary(iterDir, results)
	fmt.Println(dim(fmt.Sprintf("⏱  %s total", time.Since(sessionStart).Round(time.Second))))

	// Advisory only — council never changes models on its own.
	printModelAdvisory(detectedAgents, *cfg, promptTask)

	log.Log("END", fmt.Sprintf("Council session complete. Plans: %d/%d", validPlans, len(detectedAgents)))

	if validPlans < len(detectedAgents) {
		log.Log("STATUS", "INCOMPLETE: One or more agents failed to produce output.")
	} else {
		log.Log("STATUS", "COMPLETE: All agents produced valid output.")
	}

	// Interactive feedback rounds resume this same session directory.
	cfg.ContinueDir = runDir

	if validPlans == 0 {
		return 2
	}
	if validPlans < len(detectedAgents) {
		return 1
	}
	return 0
}

// retrieveExternalContext runs the optional COUNCIL_CONTEXT_CMD retrieval hook
// with the task appended as the final argument and returns its stdout (capped).
// This lets any RAG or code-graph CLI (graphify, rg wrappers, vector search...)
// enrich the prompt every agent receives. Empty when unset or on failure.
func retrieveExternalContext(ctx context.Context, task string, verbose bool) string {
	raw := strings.TrimSpace(os.Getenv("COUNCIL_CONTEXT_CMD"))
	if raw == "" {
		return ""
	}
	parts := strings.Fields(raw)
	args := append(parts[1:], task)

	hookCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(hookCtx, parts[0], args...)
	out, err := cmd.Output()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "ℹ️  Context hook skipped (%s failed: %v)\n", parts[0], err)
		}
		return ""
	}

	const maxContext = 32 * 1024
	result := strings.TrimSpace(string(out))
	if len(result) > maxContext {
		result = result[:maxContext] + "\n[context truncated by council]"
	}
	if result != "" && verbose {
		fmt.Printf("🧠 Context hook: injected %d bytes from `%s`\n", len(result), raw)
		flush()
	}
	return result
}

// delegateToRemote handles the SSH connection and execution on a remote host
func delegateToRemote(ctx context.Context, cfg Config) int {
	user := os.Getenv("USER")
	host := cfg.RemoteHost
	port := "22"

	// Parse user and host if provided directly like user@1.2.3.4:22
	if strings.Contains(host, "@") {
		parts := strings.Split(host, "@")
		user = parts[0]
		host = parts[1]
	}
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		host = parts[0]
		port = parts[1]
	}

	var identityFile string

	// Attempt to resolve via SSH config (supports ~/.ssh/config aliases)
	cmd := exec.Command("ssh", "-G", cfg.RemoteHost)
	if out, err := cmd.Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "hostname ") {
				host = strings.TrimSpace(strings.TrimPrefix(line, "hostname "))
			} else if strings.HasPrefix(line, "user ") {
				user = strings.TrimSpace(strings.TrimPrefix(line, "user "))
			} else if strings.HasPrefix(line, "port ") {
				port = strings.TrimSpace(strings.TrimPrefix(line, "port "))
			} else if strings.HasPrefix(line, "identityfile ") {
				// We only care if an identity file is specifically set.
				// The first one is typically the primary match.
				if identityFile == "" {
					identityFile = strings.TrimSpace(strings.TrimPrefix(line, "identityfile "))
				}
			}
		}
	}

	dialAddress := fmt.Sprintf("%s:%s", host, port)

	// fmt.Printf("🔍 SSH Delegation Config:\n  User: %s\n  Host: %s\n  Port: %s\n  IdentityFile: %s\n", user, host, port, identityFile)
	// flush()

	sshCfg, err := NewSSHConfig(user, identityFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ SSH Config Error: %v\n", err)
		return 2
	}

	client, err := ssh.Dial("tcp", dialAddress, sshCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Connection Failed (%s): %v\n", dialAddress, err)
		return 2
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ SSH Session Error: %v\n", err)
		return 2
	}
	defer session.Close()

	var args []string
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--remote" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "--remote=") {
			continue
		}
		args = append(args, fmt.Sprintf("%q", arg))
	}

	remoteDir := os.Getenv("COUNCIL_REMOTE_DIR")
	if remoteDir == "" {
		remoteDir = "." // Default to current directory on remote
	}
	innerCmd := fmt.Sprintf("cd %s && council %s", remoteDir, strings.Join(args, " "))
	remoteCmd := fmt.Sprintf("bash -l -c '%s'", strings.ReplaceAll(innerCmd, "'", "'\\''"))

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	fmt.Printf("⛓️  Connected. Starting remote session...\n\n")
	flush()

	done := make(chan error, 1)
	go func() {
		done <- session.Run(remoteCmd)
	}()

	select {
	case <-ctx.Done():
		session.Close()
		fmt.Println("\n⚠️  Local cancellation received. Closing remote session.")
		return 130
	case err := <-done:
		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				return exitErr.ExitStatus()
			}
			fmt.Fprintf(os.Stderr, "❌ Remote execution failed: %v\n", err)
			return 2
		}
	}

	fmt.Println("\n✅ Remote session complete.")
	return 0
}

// handleInstall handles the 'install' subcommand
func handleInstall(cfg Config) int {
	fmt.Println("🔧 Installing AI Council globally...")

	binaryPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error getting binary path: %v\n", err)
		return 2
	}
	binaryPath, _ = filepath.Abs(binaryPath)

	home, _ := os.UserHomeDir()

	binaryName := "council"
	var installDirs []string
	if runtime.GOOS == "windows" {
		binaryName = "council.exe"
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			installDirs = append(installDirs, filepath.Join(localAppData, "Programs", "council"))
		}
		installDirs = append(installDirs, filepath.Join(home, "bin"))
	} else {
		installDirs = []string{
			"/usr/local/bin",
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "bin"),
		}
	}

	var installDir string
	for _, dir := range installDirs {
		if dir == "" || dir == string(filepath.Separator) {
			continue
		}
		if _, err := os.Stat(dir); err != nil {
			// Only auto-create user-owned locations, never system dirs.
			if dir == "/usr/local/bin" {
				continue
			}
			if err := os.MkdirAll(dir, 0755); err != nil {
				continue
			}
		}
		testFile := filepath.Join(dir, ".council_install_test")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err == nil {
			os.Remove(testFile)
			installDir = dir
			break
		}
	}

	if installDir == "" {
		fmt.Println("❌ No writable install directory found.")
		if runtime.GOOS == "windows" {
			fmt.Printf("   Tried %%LOCALAPPDATA%%\\Programs\\council and %%USERPROFILE%%\\bin.\n")
		} else {
			fmt.Println("   Tried /usr/local/bin, ~/.local/bin, and ~/bin.")
		}
		return 2
	}

	dest := filepath.Join(installDir, binaryName)

	if samePath(binaryPath, dest) {
		fmt.Printf("✅ Council is already installed at: %s\n", dest)
		return 0
	}

	// Copy (not symlink): survives deletion of the build directory and works on
	// Windows where symlinks require elevated privileges.
	if err := copyExecutable(binaryPath, dest); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error installing binary: %v\n", err)
		return 2
	}

	fmt.Printf("✅ Success! Council is now installed at: %s\n", dest)

	if !dirInPath(installDir) {
		fmt.Printf("\n⚠️  %s is not in your PATH.\n", installDir)
		if runtime.GOOS == "windows" {
			fmt.Println("   Add it in PowerShell (then restart your terminal):")
			fmt.Printf("     [Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path','User') + ';%s', 'User')\n", installDir)
		} else {
			fmt.Println("   Add this line to your ~/.bashrc or ~/.zshrc:")
			fmt.Printf("     export PATH=\"$PATH:%s\"\n", installDir)
		}
	} else {
		fmt.Println("   You can now run 'council' from any directory.")
	}
	return 0
}

// samePath reports whether two paths refer to the same file location.
func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(absA, absB)
	}
	return absA == absB
}

// copyExecutable copies src to dst with executable permissions, replacing dst if present.
func copyExecutable(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// Write to a temp file in the target dir, then rename for atomicity and to
	// avoid "text file busy" when overwriting a running binary.
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0755); err != nil {
		return err
	}
	os.Remove(dst)
	return os.Rename(tmp, dst)
}

// dirInPath reports whether dir is present in the PATH environment variable.
func dirInPath(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if samePath(p, dir) {
			return true
		}
	}
	return false
}

// collectAgentStatuses builds the full agent inventory: every supported agent,
// whether it is installed, and its best-effort (or ping-verified) auth state.
func collectAgentStatuses(ctx context.Context, cfg Config, withPing bool) ([]AgentStatus, map[string]int) {
	dummyLog, _ := newAuditLogger("doctor", []string{os.DevNull}, []string{os.DevNull})
	agents := detectAgents(ctx, cfg, dummyLog)

	var statuses []AgentStatus
	counts := map[string]int{
		"supported": len(councilRosterAgents), "installed": 0,
		"authenticated": 0, "login_required": 0, "unknown": 0,
	}

	persistedState := loadState()
	for _, name := range councilRosterAgents {
		entry := catalogEntry(name)
		st := AgentStatus{Name: string(name), Auth: "unknown"}
		if entry != nil {
			st.Executable = entry.Executable
			st.Vendor = entry.Vendor
			st.FreeTier = entry.FreeTier
			st.InstallHint = entry.InstallHint
			st.LoginHint = entry.LoginHint
			st.LimitsURL = entry.LimitsURL
			st.LimitsHint = entry.LimitsHint
			st.ModelEnv = entry.ModelEnv
		}
		st.Quarantined = persistedState.isQuarantined(name)

		resolved, installed := agents[name]
		st.Installed = installed
		if installed {
			counts["installed"]++
			st.Path = resolved.Path
			st.Version = probeVersion(ctx, entry, resolved.Path)
			st.Auth, st.AuthMethod = authProbe(ctx, entry, resolved.Path)

			if withPing {
				pt := cfg.PingTimeout
				if pt < 15 {
					pt = 15
				}
				pingCtx, cancel := context.WithTimeout(ctx, time.Duration(pt)*time.Second)
				override := getModelOverride(name, cfg)
				args := councilPingArgs(name, "RESPOND WITH 'OK' ONLY.", "", override, cfg.Unrestricted)
				runner := &LocalRunner{}
				stdout, stderr, err := runner.Run(pingCtx, resolved.Path, args, nil)
				cancel()
				output := string(stdout) + string(stderr)
				if err == nil || strings.Contains(strings.ToUpper(output), "OK") {
					st.Auth, st.AuthMethod = "yes", "ping"
					// A verified live ping re-enables a quarantined agent.
					if st.Quarantined {
						persistedState.clearQuarantine(name)
						st.Quarantined = false
					}
				} else {
					st.Auth, st.AuthMethod = "no", "ping"
				}
			}

			switch st.Auth {
			case "yes", "likely":
				counts["authenticated"]++
			case "no":
				counts["login_required"]++
			default:
				counts["unknown"]++
			}
		}

		statuses = append(statuses, st)
	}
	if withPing {
		persistedState.save() // persists any quarantine clears from verified pings
	}
	return statuses, counts
}

// handleDoctor handles the 'doctor' subcommand
func handleDoctor(ctx context.Context, cfg Config) int {
	if cfg.DoctorJSON {
		statuses, counts := collectAgentStatuses(ctx, cfg, cfg.DoctorPing)
		payload := map[string]any{
			"council_version": Version,
			"platform":        runtime.GOOS + "/" + runtime.GOARCH,
			"repo_root":       cfg.RepoRoot,
			"ping_verified":   cfg.DoctorPing,
			"totals":          counts,
			"agents":          statuses,
		}
		out, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			return 2
		}
		fmt.Println(string(out))
		if counts["installed"] == 0 {
			return 1
		}
		return 0
	}

	fmt.Println("🩺 AI Council Health Check")
	fmt.Println("-------------------------")

	// 1. Check Repo Root
	fmt.Printf("📂 Repo Root: %s\n", cfg.RepoRoot)
	if _, err := os.Stat(cfg.RegistryFile); err == nil {
		fmt.Println("  ✅ Optional domain registry found")
	}

	// 2. Check Remote Connectivity (optional feature — only probe when configured)
	remoteHost := cfg.RemoteHost
	if remoteHost == "" {
		remoteHost = os.Getenv("COUNCIL_REMOTE_HOST")
	}
	if remoteHost != "" {
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			fmt.Printf("🔑 SSH Agent: ONLINE (%s)\n", sock)
		} else {
			fmt.Println("⚠️  SSH Agent: OFFLINE (Remote delegation will require manual keys)")
		}
		fmt.Printf("📡 Remote (%s): ", remoteHost)
		flush()
		cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=2", remoteHost, "echo OK")
		if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) == "OK" {
			fmt.Println("ONLINE")
		} else {
			fmt.Println("OFFLINE / UNREACHABLE")
		}
	} else {
		fmt.Println("📡 Remote delegation: not configured (optional — set with --remote or COUNCIL_REMOTE_HOST)")
	}

	// 3. Agent inventory with auth state
	fmt.Println("\n🤖 Agent Inventory:")
	if cfg.DoctorPing {
		fmt.Println("   (auth verified by live ping — this sends one tiny prompt per agent)")
	}
	flush()

	statuses, counts := collectAgentStatuses(ctx, cfg, cfg.DoctorPing)
	for _, st := range statuses {
		if !st.Installed {
			fmt.Printf("  ❌ %-12s not installed   → %s\n", st.Name, st.InstallHint)
			continue
		}
		authLabel := map[string]string{
			"yes": green("✅ authenticated"), "likely": green("🔑 credentials found"),
			"no": yellow("🔒 login required"), "unknown": "❔ auth unknown",
		}[st.Auth]
		version := ""
		if st.Version != "" && st.Version != "unknown" {
			version = dim(" · " + st.Version)
		}
		quarantine := ""
		if st.Quarantined {
			quarantine = red(" [auto-disabled — fix then council doctor --ping]")
		}
		fmt.Printf("  ✅ %-12s %s%s%s\n", st.Name, authLabel, version, quarantine)
		if st.Auth == "no" && st.LoginHint != "" {
			fmt.Printf("     → council login %s   (%s)\n", normalizeAgentKey(st.Name), st.LoginHint)
		}
	}

	fmt.Printf("\n📊 Summary: %d supported | %d installed | %d authenticated | %d login required | %d unknown\n",
		counts["supported"], counts["installed"], counts["authenticated"], counts["login_required"], counts["unknown"])
	fmt.Println("   Machine-readable: council doctor --json   Definitive auth check: council doctor --ping")
	fmt.Println("   Limits insight: each agent entry in --json carries limits_url / limits_hint for its vendor plan.")
	fmt.Println("   Sign-in help: council login   Model config: council models   Refresh CLIs: council update")

	if counts["installed"] == 0 {
		fmt.Println()
		printNoAgentsHelp()
		return 1
	}

	fmt.Printf("\n✅ Doctor check complete. %d agent(s) ready.\n", counts["installed"])
	return 0
}

// handleSetup handles the 'setup' subcommand: reports missing agent CLIs and
// (with --apply) installs the ones that have a safe, non-interactive installer.
func handleSetup(ctx context.Context, cfg Config) int {
	fmt.Println("🔧 AI Council Setup")
	fmt.Println("-------------------")

	dummyLog, _ := newAuditLogger("setup", []string{os.DevNull}, []string{os.DevNull})
	detectCfg := cfg
	if detectCfg.AgentCheckTimeout <= 0 {
		detectCfg.AgentCheckTimeout = 8
	}
	agents := detectAgents(ctx, detectCfg, dummyLog)

	var missing []*AgentCatalogEntry
	for _, name := range councilRosterAgents {
		entry := catalogEntry(name)
		if entry == nil {
			continue
		}
		if cfg.SetupFree && !entry.FreeTier {
			continue // --free: only free-tier / open-source agents
		}
		if _, ok := agents[name]; ok {
			fmt.Printf("  ✅ %-12s already installed\n", name)
			continue
		}
		missing = append(missing, entry)
	}

	if len(missing) == 0 {
		fmt.Println("\n✅ Every requested agent CLI is already installed. Next: council login")
		return 0
	}

	fmt.Printf("\n📦 %d agent(s) not installed:\n", len(missing))
	for _, entry := range missing {
		tag := ""
		if entry.FreeTier {
			tag = green(" [free]")
		}
		auto := dim(" (manual install)")
		if installArgv(entry) != nil {
			auto = ""
		}
		fmt.Printf("  ❌ %-12s%s %s%s\n", entry.Name, tag, entry.InstallHint, auto)
	}

	if !cfg.SetupApply {
		fmt.Println("\n💡 council setup --apply          install everything above via each vendor's official channel")
		fmt.Println("   council setup --apply --free   install only the free-tier / open-source agents")
		fmt.Println("   Installers always fetch the latest stable release. Afterwards: council login")
		return 0
	}

	fmt.Println("\n🚀 Installing latest stable releases via official installers...")
	failures := 0
	for _, entry := range missing {
		argv := installArgv(entry)
		if argv == nil {
			fmt.Printf("  ⏭️  %-12s manual install required: %s\n", entry.Name, entry.InstallHint)
			continue
		}
		fmt.Printf("  📥 %-12s %s ...\n", entry.Name, strings.Join(argv, " "))
		flush()
		installCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		cmd := exec.CommandContext(installCtx, argv[0], argv[1:]...)
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			failures++
			fmt.Printf("  ❌ %-12s install failed: %v\n", entry.Name, err)
			tail := string(out)
			if len(tail) > 400 {
				tail = tail[len(tail)-400:]
			}
			if strings.TrimSpace(tail) != "" {
				fmt.Printf("     %s\n", strings.TrimSpace(tail))
			}
		} else {
			if len(entry.PostInstall) > 0 {
				fmt.Printf("  📥 %-12s second stage: %s ...\n", entry.Name, strings.Join(entry.PostInstall, " "))
				flush()
				postCtx, postCancel := context.WithTimeout(ctx, 10*time.Minute)
				postCmd := exec.CommandContext(postCtx, entry.PostInstall[0], entry.PostInstall[1:]...)
				postOut, postErr := postCmd.CombinedOutput()
				postCancel()
				if postErr != nil {
					failures++
					fmt.Printf("  ❌ %-12s second stage failed: %v\n", entry.Name, postErr)
					tail := strings.TrimSpace(string(postOut))
					if len(tail) > 400 {
						tail = tail[len(tail)-400:]
					}
					if tail != "" {
						fmt.Printf("     %s\n", tail)
					}
					continue
				}
			}
			fmt.Printf("  ✅ %-12s installed. Sign in: council login %s\n", entry.Name, normalizeAgentKey(string(entry.Name)))
		}
	}

	fmt.Println("\n🔑 Sign in to the new agents:  council login")
	fmt.Println("🩺 Then verify everything:     council doctor --ping")
	if failures > 0 {
		return 1
	}
	return 0
}

// printNoAgentsHelp prints actionable guidance when no agent CLIs are available.
func printNoAgentsHelp() {
	fmt.Println("💡 Council needs at least one supported AI agent CLI installed and authenticated.")
	fmt.Println("   council setup --apply          install all 12 supported agents (latest stable)")
	fmt.Println("   council setup --apply --free   install only the free-tier / open-source ones")
	fmt.Println("   council login                  sign-in checklist for everything installed")
	fmt.Println("   council doctor --ping          verify each agent end-to-end")
}
