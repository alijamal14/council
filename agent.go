package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type AgentName string

const (
	AgentGemini      AgentName = "Gemini"
	AgentCodex       AgentName = "Codex"
	AgentClaude      AgentName = "Claude"
	AgentCopilot     AgentName = "Copilot"
	AgentCursor      AgentName = "Cursor"
	AgentAntigravity AgentName = "Antigravity"
)

// councilRosterAgents is the subprocess roster this binary discovers and runs (order only affects goroutine scheduling).
//
// Cursor Agent CLI integration follows non-interactive / print semantics described at:
// https://cursor.com/docs/cli/overview https://cursor.com/docs/cli/using https://cursor.com/docs/cli/reference/output-format
var councilRosterAgents = []AgentName{AgentGemini, AgentCodex, AgentClaude, AgentCopilot, AgentCursor, AgentAntigravity}

// retrySleep is the sleep function used between runAgent retry attempts.
// Tests may replace it with a no-op to avoid real delays.
var retrySleep = time.Sleep

// Runner defines the interface for executing agent commands
type Runner interface {
	Run(ctx context.Context, name string, args []string, stdin io.Reader) ([]byte, []byte, error)
	Type() string
}

// LocalRunner executes commands on the local machine
type LocalRunner struct{}

func (r *LocalRunner) Type() string { return "local" }

// SSHRunner executes commands on a remote machine
type SSHRunner struct {
	Host   string
	User   string
	Config *ssh.ClientConfig
}

func (r *SSHRunner) Type() string { return "ssh" }

func (r *SSHRunner) Run(ctx context.Context, name string, args []string, stdin io.Reader) ([]byte, []byte, error) {
	client, err := ssh.Dial("tcp", r.Host, r.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("ssh dial error: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("ssh session error: %v", err)
	}
	defer session.Close()

	if stdin != nil {
		session.Stdin = stdin
	}

	var stdout, stderr strings.Builder
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Format command string for remote shell
	fullCmd := name
	for _, arg := range args {
		fullCmd += fmt.Sprintf(" %q", arg)
	}

	// Start command in a goroutine to support context cancellation
	err = session.Start(fullCmd)
	if err != nil {
		return nil, nil, err
	}

	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case <-ctx.Done():
		// Note: session.Signal(ssh.SIGKILL) is not supported by all SSH servers.
		// A more robust way would be to kill the process on the remote side via a separate shell command.
		// For now, closing the session is the standard approach.
		session.Close()
		return []byte(stdout.String()), []byte(stderr.String()), ctx.Err()
	case err := <-done:
		return []byte(stdout.String()), []byte(stderr.String()), err
	}
}

// NewSSHConfig creates a client config using SSH agent, a specific private key, or default keys
func NewSSHConfig(user, identityFile string) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	// 1. Try SSH Agent
	if socket := os.Getenv("SSH_AUTH_SOCK"); socket != "" {
		conn, err := net.Dial("unix", socket)
		if err == nil {
			agentClient := agent.NewClient(conn)
			authMethods = append(authMethods, ssh.PublicKeysCallback(agentClient.Signers))
		}
	}

	home, _ := os.UserHomeDir()
	var keyPaths []string

	// 2. Add specific identity file if provided
	if identityFile != "" {
		if strings.HasPrefix(identityFile, "~/") {
			identityFile = filepath.Join(home, identityFile[2:])
		}
		keyPaths = append(keyPaths, identityFile)
	}

	// 3. Add Default Private Keys
	keyPaths = append(keyPaths,
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_rsa"),
	)

	for _, path := range keyPaths {
		key, err := os.ReadFile(path)
		if err == nil {
			signer, err := ssh.ParsePrivateKey(key)
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
				// fmt.Printf("🔑 Loaded SSH Key: %s\n", path)
			} else {
				fmt.Fprintf(os.Stderr, "⚠️  Failed to parse SSH Key %s: %v\n", path, err)
			}
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no valid SSH auth methods found (check ssh-agent, %s, or ~/.ssh/id_rsa)", identityFile)
	}

	var hostKeyCallback ssh.HostKeyCallback
	knownHostsPath := os.Getenv("COUNCIL_KNOWN_HOSTS")
	insecureSSH := os.Getenv("COUNCIL_SSH_INSECURE") == "1"

	if knownHostsPath == "" {
		// Default to system known_hosts
		knownHostsPath = filepath.Join(home, ".ssh", "known_hosts")
	} else if strings.HasPrefix(knownHostsPath, "~/") {
		knownHostsPath = filepath.Join(home, knownHostsPath[2:])
	}

	if _, err := os.Stat(knownHostsPath); err == nil {
		var err error
		hostKeyCallback, err = knownhosts.New(knownHostsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Failed to parse known_hosts from %s: %v\n", knownHostsPath, err)
			if insecureSSH {
				fmt.Fprintln(os.Stderr, "⚠️  Falling back to insecure host key verification (COUNCIL_SSH_INSECURE=1)")
				hostKeyCallback = ssh.InsecureIgnoreHostKey()
			} else {
				return nil, fmt.Errorf("host key verification failed and COUNCIL_SSH_INSECURE is not set")
			}
		}
	} else {
		if insecureSSH {
			fmt.Fprintln(os.Stderr, "⚠️  known_hosts not found; using insecure verification (COUNCIL_SSH_INSECURE=1)")
			hostKeyCallback = ssh.InsecureIgnoreHostKey()
		} else {
			return nil, fmt.Errorf("known_hosts file not found at %s. Use COUNCIL_KNOWN_HOSTS or set COUNCIL_SSH_INSECURE=1 to bypass (not recommended)", knownHostsPath)
		}
	}

	return &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}, nil
}

type AgentSet map[AgentName]*ResolvedAgent

type ResolvedAgent struct {
	Name       AgentName
	Path       string
	Version    string
	RunnerType string
}

type RunResult struct {
	Agent      AgentName
	OutFile    string
	Success    bool
	IsFallback bool
	Attempts   int
	Error      string
}

// detectAgents detects which agents are available on the system
func detectAgents(ctx context.Context, cfg Config, log *AuditLogger) AgentSet {
	agents := make(AgentSet)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, agentName := range councilRosterAgents {
		wg.Add(1)
		go func(name AgentName) {
			defer wg.Done()
			resolved := discoverAgent(ctx, name, cfg.AgentCheckTimeout)
			if resolved != nil {
				mu.Lock()
				agents[name] = resolved
				mu.Unlock()
			}
		}(agentName)
	}

	wg.Wait()
	return agents
}

// discoverAgent checks if an agent can be executed and returns its metadata
func discoverAgent(ctx context.Context, name AgentName, timeoutSec int) *ResolvedAgent {
	if name == AgentCursor {
		return discoverCursorAgent(ctx, timeoutSec)
	}

	cmdName := strings.ToLower(string(name))
	if name == AgentAntigravity {
		cmdName = "agy"
	}

	runner := &LocalRunner{}

	// probeInstalled runs --version then --help; returns true if the binary produces any output.
	// Discovery answers "is the CLI installed?" — ping answers "can it run headlessly?"
	probeTimeout := time.Duration(timeoutSec) * time.Second
	if probeTimeout <= 0 {
		probeTimeout = 5 * time.Second
	}
	probeInstalled := func(path string) bool {
		for _, probe := range [][]string{{"--version"}, {"--help"}} {
			probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
			stdout, stderr, _ := runner.Run(probeCtx, path, probe, nil)
			cancel()
			if len(stdout) > 0 || len(stderr) > 0 {
				return true
			}
		}
		return false
	}

	// 1. Try system PATH first (most reliable)
	if path, err := exec.LookPath(cmdName); err == nil {
		if probeInstalled(path) {
			return &ResolvedAgent{Name: name, Path: path, RunnerType: "local"}
		}
	}

	// 2. Fallback search chain for non-PATH locations
	home, _ := os.UserHomeDir()
	searchPaths := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "bin"),
		"/usr/local/bin",
		"/opt/homebrew/bin",
	}

	for _, basePath := range searchPaths {
		path := filepath.Join(basePath, cmdName)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if probeInstalled(path) {
			absPath, _ := filepath.Abs(path)
			return &ResolvedAgent{Name: name, Path: absPath, RunnerType: "local"}
		}
	}

	return nil
}

// discoverCursorAgent resolves the Cursor Agent CLI (`cursor-agent` preferred, then `agent` per Cursor install docs).
func discoverCursorAgent(ctx context.Context, timeoutSec int) *ResolvedAgent {
	candidates := []string{"cursor-agent", "agent"}
	home, _ := os.UserHomeDir()
	searchPaths := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "bin"),
		"/usr/local/bin",
		"/opt/homebrew/bin",
	}

	runner := &LocalRunner{}
	probeTimeout := time.Duration(timeoutSec) * time.Second
	if probeTimeout <= 0 {
		probeTimeout = 5 * time.Second
	}
	tryProbe := func(path string) *ResolvedAgent {
		for _, probe := range [][]string{{"--version"}, {"--help"}} {
			probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
			stdout, stderr, _ := runner.Run(probeCtx, path, probe, nil)
			cancel()
			if len(stdout) > 0 || len(stderr) > 0 {
				absPath, _ := filepath.Abs(path)
				return &ResolvedAgent{Name: AgentCursor, Path: absPath, RunnerType: "local"}
			}
		}
		return nil
	}

	for _, cand := range candidates {
		if path, err := exec.LookPath(cand); err == nil {
			if r := tryProbe(path); r != nil {
				return r
			}
		}
	}
	for _, cand := range candidates {
		for _, base := range searchPaths {
			var path string
			if base == "" {
				continue
			}
			path = filepath.Join(base, cand)
			if _, err := os.Stat(path); err != nil {
				continue
			}
			if r := tryProbe(path); r != nil {
				return r
			}
		}
	}
	return nil
}

// When copilotSubstituteModel is non-empty, the logical agent is Gemini/Claude/Codex but the binary path
// is Copilot surrogating via --model (failover roster).
func councilSpawnArgs(logical AgentName, prompt string, copilotSubstituteModel string, modelOverride string, unrestricted bool) []string {
	// Copilot surrogate: use long-form --message for version stability.
	if copilotSubstituteModel != "" {
		args := []string{"chat", "--message", prompt, "--model", copilotSubstituteModel}
		if unrestricted {
			args = append(args, "--allow-all")
		}
		return args
	}

	var args []string
	switch logical {
	case AgentGemini:
		if unrestricted {
			args = []string{"--skip-trust", "--approval-mode", "yolo", "-p", prompt}
		} else {
			args = []string{"-p", prompt}
		}
	case AgentCodex:
		if unrestricted {
			args = []string{"exec", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox", "--dangerously-bypass-hook-trust", prompt}
		} else {
			args = []string{"exec", prompt}
		}
	case AgentClaude:
		if unrestricted {
			args = []string{"--effort", "high", "--dangerously-skip-permissions", "-p", prompt}
		} else {
			args = []string{"--effort", "high", "-p", prompt}
		}
	case AgentCopilot:
		args = []string{"--prompt", prompt}
		if unrestricted {
			args = append(args, "--allow-all")
		}
	case AgentCursor:
		args = []string{"agent", "--print"}
		if unrestricted {
			args = append(args, "--yolo", "--trust", "--approve-mcps")
		}
		args = append(args, "-p", prompt)
	case AgentAntigravity:
		args = []string{"--print"}
		if unrestricted {
			args = append(args, "--dangerously-skip-permissions")
		}
		args = append(args, prompt)
	default:
		args = []string{"-p", prompt}
	}

	if modelOverride != "" {
		// If first arg is a subcommand, insert after it.
		if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
			newArgs := make([]string, 0, len(args)+2)
			newArgs = append(newArgs, args[0], "--model", modelOverride)
			newArgs = append(newArgs, args[1:]...)
			return newArgs
		}
		// Otherwise prepend
		return append([]string{"--model", modelOverride}, args...)
	}
	return args
}

// councilPingArgs uses lightweight headless argv so pre-flight pings finish quickly without full tool/agent cold starts.
func councilPingArgs(logical AgentName, prompt string, copilotSubstituteModel string, modelOverride string, unrestricted bool) []string {
	if copilotSubstituteModel != "" {
		args := []string{"chat", "--message", prompt, "--model", copilotSubstituteModel}
		if unrestricted {
			args = append(args, "--allow-all")
		}
		return args
	}

	var args []string
	switch logical {
	case AgentGemini:
		if unrestricted {
			args = []string{"--skip-trust", "--approval-mode", "yolo", "-p", prompt}
		} else {
			args = []string{"-p", prompt}
		}
	case AgentCodex:
		if unrestricted {
			args = []string{"exec", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox", "--dangerously-bypass-hook-trust", prompt}
		} else {
			args = []string{"exec", prompt}
		}
	case AgentClaude:
		if unrestricted {
			args = []string{"--dangerously-skip-permissions", "--output-format", "text", "-p", prompt}
		} else {
			args = []string{"--output-format", "text", "-p", prompt}
		}
	case AgentCopilot:
		args = []string{"--prompt", prompt}
		if unrestricted {
			args = append(args, "--allow-all")
		}
	case AgentCursor:
		args = []string{"agent", "--print"}
		if unrestricted {
			args = append(args, "--yolo", "--trust", "--approve-mcps")
		}
		args = append(args, "-p", prompt)
	case AgentAntigravity:
		args = []string{"--print"}
		if unrestricted {
			args = append(args, "--dangerously-skip-permissions")
		}
		args = append(args, prompt)
	default:
		args = []string{"-p", prompt}
	}

	if modelOverride != "" {
		if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
			newArgs := make([]string, 0, len(args)+2)
			newArgs = append(newArgs, args[0], "--model", modelOverride)
			newArgs = append(newArgs, args[1:]...)
			return newArgs
		}
		return append([]string{"--model", modelOverride}, args...)
	}
	return args
}

// pingAgentsParallel sends a minimal prompt to every detected agent in parallel.
func pingAgentsParallel(ctx context.Context, agents AgentSet, pingTimeoutSecs int, cfg Config, log *AuditLogger) AgentSet {
	if len(agents) == 0 {
		return agents
	}

	healthyAgents := make(AgentSet)
	var wg sync.WaitGroup
	var mu sync.Mutex

	type pingResult struct {
		agent      AgentName
		ok         bool
		isFailover bool
		elapsed    time.Duration
		reason     string
	}
	var results []pingResult
	var resultsMu sync.Mutex

	for name, resolved := range agents {
		wg.Add(1)
		go func(n AgentName, r *ResolvedAgent) {
			defer wg.Done()
			start := time.Now()

			runner := &LocalRunner{}
			pt := pingTimeoutSecs
			if pt < 15 {
				pt = 15
			}
			pingCtx, cancel := context.WithTimeout(ctx, time.Duration(pt)*time.Second)
			defer cancel()

			// Simple ping prompt
			prompt := "RESPOND WITH 'OK' ONLY."

			override := getModelOverride(n, cfg)
			args := councilPingArgs(n, prompt, "", override, cfg.Unrestricted)

			stdout, stderr, err := runner.Run(pingCtx, r.Path, args, nil)
			elapsed := time.Since(start)

			output := string(stdout) + string(stderr)
			ok := err == nil || strings.Contains(strings.ToUpper(output), "OK")

			if ok {
				resultsMu.Lock()
				results = append(results, pingResult{n, true, false, elapsed, ""})
				resultsMu.Unlock()

				mu.Lock()
				healthyAgents[n] = r
				mu.Unlock()
				log.LogAgent("PING_OK", fmt.Sprintf("%s responded in %s", n, elapsed.Round(time.Millisecond)), string(n), 0)
			} else {
				// FAILOVER ATTEMPT: If primary fails, try Copilot version
				fallbackModel := copilotFallbackModel(n)
				if n != AgentCopilot && fallbackModel != "" {
					if copilot, exists := agents[AgentCopilot]; exists {
						log.LogAgent("PING_FAILOVER", fmt.Sprintf("%s failed, attempting Copilot failover (%s)", n, fallbackModel), string(n), 0)

						fstart := time.Now()
						fargs := councilPingArgs(n, prompt, fallbackModel, "", cfg.Unrestricted)
						fstdout, fstderr, ferr := runner.Run(pingCtx, copilot.Path, fargs, nil)
						foutput := string(fstdout) + string(fstderr)

						if ferr == nil || strings.Contains(strings.ToUpper(foutput), "OK") {
							resultsMu.Lock()
							results = append(results, pingResult{n, true, true, time.Since(fstart), ""})
							resultsMu.Unlock()

							mu.Lock()
							// Create a failover agent entry
							failoverAgent := *r
							failoverAgent.Path = copilot.Path
							failoverAgent.RunnerType = "copilot-failover"
							healthyAgents[n] = &failoverAgent
							mu.Unlock()
							return
						}
					}
				}

				resultsMu.Lock()
				results = append(results, pingResult{n, false, false, elapsed, fmt.Sprintf("err: %v | stderr: %s", err, string(stderr))})
				resultsMu.Unlock()
				log.LogAgent("WARN", fmt.Sprintf("Ping failed for %s: %v | stderr: %s", n, err, string(stderr)), string(n), 1)
			}
		}(name, resolved)
	}

	wg.Wait()

	// Sort for deterministic, diffable output
	sort.Slice(results, func(i, j int) bool {
		return string(results[i].agent) < string(results[j].agent)
	})
	for _, r := range results {
		status := "✅"
		suffix := ""
		if r.isFailover {
			status = "🔄"
			suffix = " (via Copilot)"
		}

		if r.ok {
			fmt.Printf("  %s %-10s (%s)%s\n", status, r.agent, r.elapsed.Round(time.Millisecond), suffix)
		} else {
			fmt.Printf("  ❌ %-10s (%s — skipping)\n", r.agent, r.reason)
		}
		flush()
	}

	return healthyAgents
}

// sortedAgentNames returns agent names in a deterministic order
func sortedAgentNames(agents AgentSet) []AgentName {
	var names []AgentName
	for name := range agents {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return string(names[i]) < string(names[j])
	})
	return names
}

// runAgentsParallelWithDevil runs all available agents in parallel, with one receiving a special prompt
func runAgentsParallelWithDevil(ctx context.Context, agents AgentSet, prompt, devilPrompt string, devilAgent AgentName, dir, prefix string, cfg Config, log *AuditLogger) []RunResult {
	var results []RunResult
	var wg sync.WaitGroup
	var mu sync.Mutex

	copilotResolved, hasCopilot := agents[AgentCopilot]

	// Track which agents are still running for heartbeat
	running := make(map[AgentName]bool)
	var runningMu sync.Mutex
	for agentName := range agents {
		running[agentName] = true
	}

	// Heartbeat goroutine
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		elapsed := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				elapsed += 15
				runningMu.Lock()
				var still []string
				for a := range running {
					still = append(still, string(a))
				}
				runningMu.Unlock()
				sort.Strings(still)
				if len(still) > 0 {
					fmt.Printf("⏳ [%ds] Still running: %s\n", elapsed, strings.Join(still, ", "))
					flush()
				}
			}
		}
	}()

	// Run all agents in parallel
	for agentName, resolved := range agents {
		wg.Add(1)
		go func(name AgentName, res *ResolvedAgent) {
			defer wg.Done()

			// Determine which prompt to use
			p := prompt
			if name == devilAgent {
				p = devilPrompt
			}

			// Determine output file
			lowerName := strings.ToLower(string(name))
			outFile := filepath.Join(dir, prefix+"."+lowerName+".txt")

			result := runAgent(ctx, name, res, copilotResolved, p, outFile, cfg, log, hasCopilot)

			runningMu.Lock()
			delete(running, name)
			runningMu.Unlock()

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(agentName, resolved)
	}

	wg.Wait()
	close(done)
	return results
}

// runAgentsParallel runs all available agents in parallel
func runAgentsParallel(ctx context.Context, agents AgentSet, prompt, dir, prefix string, cfg Config, log *AuditLogger) []RunResult {
	return runAgentsParallelWithDevil(ctx, agents, prompt, "", "", dir, prefix, cfg, log)
}

// runAgent runs a single agent with retry and fallback logic
func runAgent(ctx context.Context, name AgentName, resolved, copilotResolved *ResolvedAgent, prompt, outFile string, cfg Config, log *AuditLogger, hasCopilot bool) RunResult {
	maxRetries := 3
	totalAttempts := maxRetries + 1
	attempt := 0
	isFallback := false

	for attempt < totalAttempts || (!isFallback && hasCopilot) {
		attempt++

		// Build command with timeout context
		timeoutSec := cfg.AgentRunTimeout
		agentCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)

		execPath := resolved.Path
		var args []string

		if resolved.RunnerType == "copilot-failover" || isFallback {
			model := copilotFallbackModel(name)
			if copilotResolved != nil {
				execPath = copilotResolved.Path
			}
			args = councilSpawnArgs(name, prompt, model, "", cfg.Unrestricted)
			isFallback = true // ensure logging reflects fallback state
		} else {
			override := getModelOverride(name, cfg)
			args = councilSpawnArgs(name, prompt, "", override, cfg.Unrestricted)
		}

		stdinAny := io.Reader(bytes.NewReader(nil))

		runner := &LocalRunner{}
		stdout, stderr, _ := runner.Run(agentCtx, execPath, args, stdinAny)
		cancel()

		// Write to file
		outF, _ := os.Create(outFile)

		outF.Write(stdout)
		outF.Write(stderr)

		// Write timeout marker BEFORE validity check
		if agentCtx.Err() == context.DeadlineExceeded {
			log.LogAgent("TIMEOUT", fmt.Sprintf("%s timed out after %ds (attempt %d)", name, timeoutSec, attempt), string(name), attempt)
			fmt.Fprintf(outF, "\n[COUNCIL_AGENT_TIMEOUT] %s exceeded %ds timeout.\n", name, timeoutSec)
		}
		outF.Close()

		// Check if output is valid
		if isValidOutput(outFile) {
			if isFallback {
				log.Log("FALLBACK_SUCCESS", fmt.Sprintf("%s succeeded via Copilot", name))
				fmt.Printf("✅ %s recovered via Copilot!\n", name)
				flush()
			} else {
				log.LogAgent("SUCCESS", fmt.Sprintf("%s produced valid output (attempt %d)", name, attempt), string(name), attempt)
			}
			return RunResult{
				Agent:      name,
				OutFile:    outFile,
				Success:    true,
				IsFallback: isFallback,
				Attempts:   attempt,
			}
		}

		// Check if we should try Copilot fallback
		if attempt >= totalAttempts && !isFallback && hasCopilot && copilotFallbackModel(name) != "" {
			log.Log("FALLBACK_START", fmt.Sprintf("Attempting Copilot fallback for %s", name))
			fmt.Printf("🔄 %s failed, attempting Copilot fallback...\n", name)
			flush()
			isFallback = true
			attempt = 0
			totalAttempts = 1
			continue
		}

		// Log retry or final failure
		if attempt < totalAttempts && !isFallback {
			log.LogAgent("RETRY", fmt.Sprintf("%s attempt %d failed", name, attempt), string(name), attempt)
			fmt.Printf("⚠️  %s failed (attempt %d/%d), retrying...\n", name, attempt, totalAttempts)
			flush()
			sleep := attempt * 3
			if sleep > 18 {
				sleep = 18
			}
			retrySleep(time.Duration(sleep) * time.Second)
		} else {
			break
		}
	}

	// Final failure
	log.LogAgent("FAILURE", fmt.Sprintf("%s failed after %d attempts", name, totalAttempts), string(name), totalAttempts)
	fmt.Printf("❌ %s failed after %d attempts. See %s\n", name, totalAttempts, outFile)
	flush()

	writeFailedMarker(outFile, name, totalAttempts)

	return RunResult{
		Agent:      name,
		OutFile:    outFile,
		Success:    false,
		IsFallback: isFallback,
		Attempts:   totalAttempts,
	}
}

// copilotFallbackModel returns the --model alias to pass Copilot when surrogating for a failed primary agent.
// Model strings are Copilot-version-dependent; only enable when COUNCIL_COPILOT_FALLBACK=1 is set
// and the caller has validated the aliases against their installed Copilot CLI version.
func copilotFallbackModel(agent AgentName) string {
	// DISABLED: Copilot CLI -m flag is version-dependent and causes false positive fallback.
	// Better to fail honestly than mask failures with unreliable fallover.
	// Re-enable only after validating stable model routing across all Copilot CLI versions.
	// See: QA_REPORT.md - Issue #2: Copilot Fallback Model Failure
	return ""
}

func getModelOverride(agent AgentName, cfg Config) string {
	switch agent {
	case AgentGemini:
		return cfg.GeminiModel
	case AgentCodex:
		return cfg.CodexModel
	case AgentClaude:
		return cfg.ClaudeModel
	case AgentCopilot:
		return cfg.CopilotModel
	case AgentCursor:
		return cfg.CursorModel
	case AgentAntigravity:
		return cfg.AntigravityModel
	default:
		return ""
	}
}
