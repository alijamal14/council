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
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

var Version = "1.1.1"

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
	Subcommand        string // "install", "doctor", or "" (default)

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

func parseFlags() Config {
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
		case "install":
			cfg.Subcommand = "install"
			return cfg
		case "doctor":
			cfg.Subcommand = "doctor"
			// Mirrors defaults when normal flag.Parse path is skipped
			cfg.AgentRunTimeout = 300
			cfg.PingTimeout = 45
			return cfg
		}
	}

	timeoutPtr := flag.Int("timeout", 300, "Per-agent run timeout (plan/critique) in seconds")
	checkTimeoutPtr := flag.Int("check-timeout", 8, "Agent binary discovery probe timeout in seconds")
	pingTimeoutPtr := flag.Int("ping-timeout", 45, "Pre-flight ping timeout per agent (seconds)")
	agentsPtr := flag.String("agents", "", "Comma-separated list of agents (antigravity,gemini,claude,codex,copilot,cursor)")
	verbosePtr := flag.Bool("verbose", true, "Verbose output")
	continuePtr := flag.String("continue", "", "Session directory to continue")
	repoPtr := flag.String("repo", "", "Repository root (explicit override)")
	remotePtr := flag.String("remote", "", "Remote host for delegation (e.g., user@host:port)")
	unrestrictedPtr := flag.Bool("unrestricted", false, "Enable unrestricted mode (bypasses sandbox/approvals)")
	yoloPtr := flag.Bool("yolo", false, "Alias for --unrestricted")
	yShortPtr := flag.Bool("y", false, "Alias for --unrestricted")
	versionPtr := flag.Bool("version", false, "Print version and exit")

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
	cfg.Unrestricted = *unrestrictedPtr || *yoloPtr || *yShortPtr

	if *repoPtr != "" {
		cfg.RepoRoot = *repoPtr
	} else if envRepo := os.Getenv("COUNCIL_REPO_ROOT"); envRepo != "" {
		cfg.RepoRoot = envRepo
	}

	if *agentsPtr != "" {
		cfg.AllowedAgents = strings.Split(*agentsPtr, ",")
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
	caller := detectCaller()
	printInvocationGuide(caller)

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

	fmt.Printf("\n🏓 Pre-flight ping (timeout: %ds per agent)...\n", cfg.PingTimeout)
	flush()
	detectedAgents = pingAgentsParallel(sigCtx, detectedAgents, cfg.PingTimeout, cfg, log)
	fmt.Println()
	flush()

	if len(detectedAgents) == 0 {
		fmt.Println("❌ No agents passed pre-flight ping.")
		log.Log("ABORT", "No agents passed pre-flight ping")
		return 2
	}

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

		fmt.Println("=== 🔄 CLI AI Council: Resuming Session ===")
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
		promptTask = cfg.Task

		fmt.Println("=== 🤖 CLI AI Council Convening ===")
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

	fmt.Println("\n--- Phase 1: Planning ---")
	flush()

	planPrompt := fmt.Sprintf("ROLE: Planner. CONTEXT: %s. OBJECTIVE: %s. TEXT-ONLY OUTPUT ONLY.", taskContext, promptTask)
	_, hasCopilot := detectedAgents[AgentCopilot]
	results := runAgentsParallel(sigCtx, detectedAgents, planPrompt, iterDir, "plan", cfg, log)

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
		fmt.Println("\n--- Phase 2: Critique ---")
		flush()

		allPlans := ""
		// Sort entries to ensure deterministic labeling and write label_map.json
		labels := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
		labelIdx := 0
		labelMap := make(map[string]string)
		entries, _ := os.ReadDir(iterDir)

		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "plan.") && strings.HasSuffix(e.Name(), ".txt") {
				path := filepath.Join(iterDir, e.Name())
				if isValidOutput(path) {
					agentName := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "plan."), ".txt")
					label := ""
					if labelIdx < len(labels) {
						label = labels[labelIdx]
						labelIdx++
						labelMap[label] = agentName
						content, _ := os.ReadFile(path)
						allPlans += fmt.Sprintf("### Plan %s:\n%s\n\n", label, string(content))
					}
				}
			}
		}

		// Write label map
		mapData, _ := json.Marshal(labelMap)
		os.WriteFile(filepath.Join(iterDir, "label_map.json"), mapData, 0644)

		critiquePrompt := fmt.Sprintf("ROLE: Reviewer. PLANS:\n%s\nGOAL: Critique and recommend the best path forward. TEXT-ONLY OUTPUT ONLY.", allPlans)

		if len(detectedAgents) >= 2 {
			// Determine devil's advocate agent (rotate by run directory hash for determinism)
			agentList := sortedAgentNames(detectedAgents)

			// Simple hash of runDir
			var hash uint32 = 0
			for i := 0; i < len(runDir); i++ {
				hash = hash*31 + uint32(runDir[i])
			}
			dvIdx := int(hash % uint32(len(agentList)))
			dvAgent := agentList[dvIdx]

			devilPrompt := fmt.Sprintf("ROLE: Devil's Advocate. PLANS:\n%s\nGOAL: Your job is NOT to agree with the consensus. Identify the strongest objections, hidden assumptions, failure modes, and risks across ALL plans. Force-rank the risks by severity. Do not recommend a winner. TEXT-ONLY OUTPUT ONLY.", allPlans)

			fmt.Printf("🎭 Assigned %s as Devil's Advocate\n", dvAgent)
			flush()

			// Run agents in parallel, giving the devilPrompt to the selected agent
			results = runAgentsParallelWithDevil(sigCtx, detectedAgents, critiquePrompt, devilPrompt, dvAgent, iterDir, "critique", cfg, log)
		} else {
			fmt.Println("ℹ️  Single agent mode — generating self-critique")
			flush()
			for name, resolved := range detectedAgents {
				lowerName := strings.ToLower(string(name))
				outFile := filepath.Join(iterDir, "critique."+lowerName+".txt")
				runAgent(sigCtx, name, resolved, detectedAgents[AgentCopilot], critiquePrompt, outFile, cfg, log, hasCopilot)
				break
			}
		}

		validCritiques := countValidFiles(iterDir, "critique.")
		fmt.Printf("\n✅ Critique complete. Valid critiques: %d\n", validCritiques)
		flush()
	}

	fmt.Println("\n=== Council Adjourned ===")
	flush()
	fmt.Printf("📂 Results: %s\n", iterDir)

	printSummary(iterDir, results)

	log.Log("END", fmt.Sprintf("Council session complete. Plans: %d/%d", validPlans, len(detectedAgents)))

	if validPlans < len(detectedAgents) {
		log.Log("STATUS", "INCOMPLETE: One or more agents failed to produce output.")
	} else {
		log.Log("STATUS", "COMPLETE: All agents produced valid output.")
	}

	if validPlans == 0 {
		return 2
	} else if validPlans < len(detectedAgents) {
		return 1
	}
	return 0
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
	installDirs := []string{
		"/usr/local/bin",
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "bin"),
	}

	var installDir string
	for _, dir := range installDirs {
		if _, err := os.Stat(dir); err == nil {
			// Check if writable
			testFile := filepath.Join(dir, ".council_install_test")
			if err := os.WriteFile(testFile, []byte("test"), 0644); err == nil {
				os.Remove(testFile)
				installDir = dir
				break
			}
		}
	}

	if installDir == "" {
		fmt.Println("❌ No writable install directory found in PATH.")
		fmt.Println("   Please add ~/.local/bin to your PATH and try again.")
		return 2
	}

	dest := filepath.Join(installDir, "council")

	// Create symlink
	os.Remove(dest) // Remove existing if any
	err = os.Symlink(binaryPath, dest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error creating symlink: %v\n", err)
		return 2
	}

	fmt.Printf("✅ Success! Council is now installed at: %s\n", dest)
	fmt.Println("   You can now run 'council' from any directory.")
	return 0
}

// handleDoctor handles the 'doctor' subcommand
func handleDoctor(ctx context.Context, cfg Config) int {
	fmt.Println("🩺 AI Council Health Check")
	fmt.Println("-------------------------")

	// 1. Check Repo Root
	fmt.Printf("📂 Repo Root: %s\n", cfg.RepoRoot)
	if _, err := os.Stat(filepath.Join(cfg.RepoRoot, "CLAUDE.md")); err == nil {
		fmt.Println("  ✅ CLAUDE.md found")
	} else {
		fmt.Println("  ⚠️  CLAUDE.md not found (Registry resolution might be limited)")
	}

	// 2. Check SSH Agent
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		fmt.Printf("🔑 SSH Agent: ONLINE (%s)\n", sock)
	} else {
		fmt.Println("  ⚠️  SSH Agent: OFFLINE (Remote delegation will require manual keys)")
	}

	// 3. Check Remote Connectivity
	remoteHost := cfg.RemoteHost
	if remoteHost == "" {
		remoteHost = os.Getenv("COUNCIL_REMOTE_HOST")
	}
	if remoteHost == "" {
		remoteHost = "localhost" // Default safe fallback
	}

	fmt.Printf("📡 Remote (%s): ", remoteHost)
	flush()
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=2", remoteHost, "echo OK")
	if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) == "OK" {
		fmt.Println("ONLINE")
	} else {
		fmt.Println("OFFLINE / UNREACHABLE")
	}

	// 4. Check Local Agents
	fmt.Println("\n🤖 Local Agent Discovery:")
	flush()

	dummyLog, _ := newAuditLogger("doctor", []string{os.DevNull}, []string{os.DevNull})
	agents := detectAgents(ctx, cfg, dummyLog)

	for _, name := range councilRosterAgents {
		if resolved, ok := agents[name]; ok {
			fmt.Printf("  ✅ %-10s: %s\n", name, resolved.Path)
		} else {
			fmt.Printf("  ❌ %-10s: NOT FOUND\n", name)
		}
	}

	fmt.Println("\n✅ Doctor check complete.")
	return 0
}
