package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCopilotFallbackModel(t *testing.T) {
	// Fallback is disabled by default (requires COUNCIL_COPILOT_FALLBACK=1).
	tests := []struct {
		agent    AgentName
		expected string
	}{
		{AgentGemini, ""},
		{AgentClaude, ""},
		{AgentCodex, ""},
		{AgentCopilot, ""},
	}
	for _, test := range tests {
		if got := copilotFallbackModel(test.agent); got != test.expected {
			t.Fatalf("copilotFallbackModel(%s) = %q, want %q (fallback permanently disabled)", test.agent, got, test.expected)
		}
	}

	// Fallback is now permanently disabled regardless of env var.
	// See QA_REPORT.md - Issue #2: Copilot Fallback Model Failure.
	t.Setenv("COUNCIL_COPILOT_FALLBACK", "1")
	disabledTests := []struct {
		agent    AgentName
		expected string
	}{
		{AgentGemini, ""},
		{AgentClaude, ""},
		{AgentCodex, ""},
		{AgentCopilot, ""},
	}
	for _, test := range disabledTests {
		if got := copilotFallbackModel(test.agent); got != test.expected {
			t.Fatalf("copilotFallbackModel(%s) = %q, want %q (still disabled)", test.agent, got, test.expected)
		}
	}
}

func TestCouncilSpawnArgs(t *testing.T) {
	t.Parallel()

	prompt := "probe"

	geminiWant := []string{"--skip-trust", "--approval-mode", "yolo", "-p", prompt}
	if g := councilSpawnArgs(AgentGemini, prompt, "", "", true); !reflect.DeepEqual(g, geminiWant) {
		t.Fatalf("Gemini: got %q want %q", g, geminiWant)
	}

	codexWant := []string{"exec", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox", "--dangerously-bypass-hook-trust", prompt}
	if g := councilSpawnArgs(AgentCodex, prompt, "", "", true); !reflect.DeepEqual(g, codexWant) {
		t.Fatalf("Codex: got %q want %q", g, codexWant)
	}

	claudeWant := []string{"--effort", "high", "--dangerously-skip-permissions", "-p", prompt}
	if g := councilSpawnArgs(AgentClaude, prompt, "", "", true); !reflect.DeepEqual(g, claudeWant) {
		t.Fatalf("Claude: got %q want %q", g, claudeWant)
	}

	copilotWant := []string{"--prompt", prompt, "--allow-all"}
	if g := councilSpawnArgs(AgentCopilot, prompt, "", "", true); !reflect.DeepEqual(g, copilotWant) {
		t.Fatalf("Copilot: got %q want %q", g, copilotWant)
	}

	cursorWant := []string{
		"agent",
		"--print",
		"--yolo",
		"--trust",
		"--approve-mcps",
		"-p", prompt,
	}
	if g := councilSpawnArgs(AgentCursor, prompt, "", "", true); !reflect.DeepEqual(g, cursorWant) {
		t.Fatalf("Cursor: got %q want %q", g, cursorWant)
	}

	agyWant := []string{"--print", "--dangerously-skip-permissions", prompt}
	if g := councilSpawnArgs(AgentAntigravity, prompt, "", "", true); !reflect.DeepEqual(g, agyWant) {
		t.Fatalf("Antigravity: got %q want %q", g, agyWant)
	}

	surrogateWant := []string{"chat", "--message", prompt, "--model", "gpt-5", "--allow-all"}
	if g := councilSpawnArgs(AgentGemini, prompt, "gpt-5", "", true); !reflect.DeepEqual(g, surrogateWant) {
		t.Fatalf("Copilot surrogate: got %q want %q", g, surrogateWant)
	}

	overrideWant := []string{"--model", "gemini-ultra", "--skip-trust", "--approval-mode", "yolo", "-p", prompt}
	if g := councilSpawnArgs(AgentGemini, prompt, "", "gemini-ultra", true); !reflect.DeepEqual(g, overrideWant) {
		t.Fatalf("Model override: got %q want %q", g, overrideWant)
	}
}

func TestCouncilPingArgs(t *testing.T) {
	t.Parallel()
	prompt := "p"
	wantGemini := []string{"--skip-trust", "--approval-mode", "yolo", "-p", prompt}
	if g := councilPingArgs(AgentGemini, prompt, "", "", true); !reflect.DeepEqual(g, wantGemini) {
		t.Fatalf("ping Gemini got %v want %v", g, wantGemini)
	}
	wantCodex := []string{"exec", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox", "--dangerously-bypass-hook-trust", prompt}
	if g := councilPingArgs(AgentCodex, prompt, "", "", true); !reflect.DeepEqual(g, wantCodex) {
		t.Fatalf("ping Codex got %v want %v", g, wantCodex)
	}
	wantClaude := []string{"--dangerously-skip-permissions", "--output-format", "text", "-p", prompt}
	if g := councilPingArgs(AgentClaude, prompt, "", "", true); !reflect.DeepEqual(g, wantClaude) {
		t.Fatalf("ping Claude got %v want %v", g, wantClaude)
	}
	wantCopilot := []string{"--prompt", prompt, "--allow-all"}
	if g := councilPingArgs(AgentCopilot, prompt, "", "", true); !reflect.DeepEqual(g, wantCopilot) {
		t.Fatalf("ping Copilot got %v want %v", g, wantCopilot)
	}
	wantCursor := []string{
		"agent",
		"--print",
		"--yolo",
		"--trust",
		"--approve-mcps",
		"-p", prompt,
	}
	if g := councilPingArgs(AgentCursor, prompt, "", "", true); !reflect.DeepEqual(g, wantCursor) {
		t.Fatalf("ping Cursor got %v want %v", g, wantCursor)
	}
}

func TestDetectAgentsFindsMockBinaries(t *testing.T) {
	tmpdir := t.TempDir()
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })
	_ = os.Setenv("PATH", tmpdir)

	createMockAgent(t, tmpdir, "claude", "echo help ok")
	createMockAgent(t, tmpdir, "copilot", "echo help ok")

	agents := detectAgents(context.Background(), Config{AgentCheckTimeout: 5}, &AuditLogger{sessionID: "test"})

	if _, ok := agents[AgentClaude]; !ok {
		t.Fatal("detectAgents() should find Claude")
	}
	if _, ok := agents[AgentCopilot]; !ok {
		t.Fatal("detectAgents() should find Copilot")
	}
}

func TestPingAgentsParallelTriggersCopilotFailover(t *testing.T) {
	// Fallback is permanently disabled. See QA_REPORT.md - Issue #2.
	// This test now verifies that failed agents are dropped (not fallen back).
	tmpdir := t.TempDir()
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })
	_ = os.Setenv("PATH", tmpdir)

	createMockAgent(t, tmpdir, "gemini", `
if [ "$1" = "-v" ] || [ "$1" = "--version" ] || [ "$1" = "--help" ]; then
  exit 0
fi
echo "mock ping failure"
exit 1`)
	createMockAgent(t, tmpdir, "copilot", `
if [ "$1" = "-v" ] || [ "$1" = "--version" ] || [ "$1" = "--help" ]; then
  exit 0
fi
echo "OK"
exit 0`)

	log, err := newAuditLogger(
		"test-ping",
		[]string{filepath.Join(tmpdir, "audit.md")},
		[]string{filepath.Join(tmpdir, "audit.jsonl")},
	)
	if err != nil {
		t.Fatalf("newAuditLogger() error = %v", err)
	}
	defer log.Close()

	healthy := pingAgentsParallel(context.Background(), AgentSet{
		AgentGemini:  {Name: AgentGemini, Path: filepath.Join(tmpdir, "gemini"), RunnerType: "local"},
		AgentCopilot: {Name: AgentCopilot, Path: filepath.Join(tmpdir, "copilot"), RunnerType: "local"},
	}, 25, Config{}, log)

	_, ok := healthy[AgentGemini]
	if ok {
		t.Fatal("pingAgentsParallel() should drop failed Gemini (no fallback)")
	}
	// Only Copilot should remain
	if _, ok := healthy[AgentCopilot]; !ok {
		t.Fatal("pingAgentsParallel() should keep healthy Copilot")
	}
}

func TestRunAgentFallsBackToCopilotAfterFailure(t *testing.T) {
	// Fallback is permanently disabled. See QA_REPORT.md - Issue #2.
	// This test now verifies that failed agents do not fall back (fail honestly).
	old := retrySleep
	retrySleep = func(time.Duration) {}
	t.Cleanup(func() { retrySleep = old })

	tmpdir := t.TempDir()
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })
	_ = os.Setenv("PATH", tmpdir)

	createMockAgent(t, tmpdir, "gemini", `
if [ "$1" = "-v" ] || [ "$1" = "--version" ] || [ "$1" = "--help" ]; then
  exit 0
fi
echo "Error executing tool: forced primary failure"
exit 1`)
	createMockAgent(t, tmpdir, "copilot", `
if [ "$1" = "-v" ] || [ "$1" = "--version" ] || [ "$1" = "--help" ]; then
  exit 0
fi
has_message=
has_model=
for arg in "$@"; do
  [ "$arg" = "--message" ] && has_message=1
  [ "$arg" = "--model" ] && has_model=1
done
if [ -n "$has_message" ] && [ -n "$has_model" ]; then
  echo "copilot fallback output"
  exit 0
fi
echo "copilot normal output"
exit 0`)

	log, err := newAuditLogger(
		"test-run",
		[]string{filepath.Join(tmpdir, "audit.md")},
		[]string{filepath.Join(tmpdir, "audit.jsonl")},
	)
	if err != nil {
		t.Fatalf("newAuditLogger() error = %v", err)
	}
	defer log.Close()

	result := runAgent(
		context.Background(),
		AgentGemini,
		&ResolvedAgent{Name: AgentGemini, Path: filepath.Join(tmpdir, "gemini"), RunnerType: "local"},
		&ResolvedAgent{Name: AgentCopilot, Path: filepath.Join(tmpdir, "copilot"), RunnerType: "local"},
		"test prompt",
		filepath.Join(tmpdir, "plan.gemini.txt"),
		Config{AgentRunTimeout: 5},
		log,
		true,
	)

	if result.Success {
		t.Fatal("runAgent() should fail without fallback")
	}
	if result.IsFallback {
		t.Fatal("runAgent() should not mark fallback (disabled)")
	}

	// Verify the failure was recorded
	content, err := os.ReadFile(filepath.Join(tmpdir, "plan.gemini.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), "Error executing tool: forced primary failure") {
		t.Fatalf("expected error message, got %q", string(content))
	}
}

func TestPrintSummaryDisplaysFailoverIcon(t *testing.T) {
	tmpdir := t.TempDir()
	outFile := filepath.Join(tmpdir, "plan.gemini.txt")
	if err := os.WriteFile(outFile, []byte("valid fallback output"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	output := captureStdout(t, func() {
		printSummary(tmpdir, []RunResult{{
			Agent:      AgentGemini,
			OutFile:    outFile,
			Success:    true,
			IsFallback: true,
			Attempts:   1,
		}})
	})

	if !strings.Contains(output, "🔄 plan.gemini.txt") {
		t.Fatalf("printSummary() should show failover icon, got:\n%s", output)
	}
}

func TestResolveRepoRootUsesExplicitExistingRepo(t *testing.T) {
	tmpdir := t.TempDir()
	repoRoot := filepath.Join(tmpdir, "repo")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	resolved, err := resolveRepoRoot(Config{RepoRoot: repoRoot})
	if err != nil {
		t.Fatalf("resolveRepoRoot() error = %v", err)
	}

	expected, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	if resolved != expected {
		t.Fatalf("resolveRepoRoot() = %q, want %q", resolved, expected)
	}
}

func createMockAgent(t *testing.T, dir, name, script string) {
	t.Helper()

	var exePath string
	var content string
	if runtime.GOOS == "windows" {
		exePath = filepath.Join(dir, name+".bat")
		content = "@echo off\n" + script + "\n"
	} else {
		exePath = filepath.Join(dir, name)
		content = "#!/bin/sh\n" + script + "\n"
	}

	if err := os.WriteFile(exePath, []byte(content), 0755); err != nil {
		t.Fatalf("createMockAgent() error = %v", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = oldStdout
	return <-done
}

// TestAgentSubprocessEnvForwardsCredentials verifies that set credential vars are forwarded
// and unset ones are absent.
func TestAgentSubprocessEnvForwardsCredentials(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")
	t.Setenv("GITHUB_TOKEN", "test-gh-token")
	// Ensure OPENAI_API_KEY is unset so we can assert absence.
	os.Unsetenv("OPENAI_API_KEY")

	env := agentSubprocessEnv()

	find := func(key string) string {
		prefix := key + "="
		for _, kv := range env {
			if strings.HasPrefix(kv, prefix) {
				return strings.TrimPrefix(kv, prefix)
			}
		}
		return ""
	}

	if got := find("GEMINI_API_KEY"); got != "test-gemini-key" {
		t.Errorf("GEMINI_API_KEY = %q, want test-gemini-key", got)
	}
	if got := find("GITHUB_TOKEN"); got != "test-gh-token" {
		t.Errorf("GITHUB_TOKEN = %q, want test-gh-token", got)
	}
	if got := find("OPENAI_API_KEY"); got != "" {
		t.Errorf("OPENAI_API_KEY = %q, want absent (not set in parent env)", got)
	}
	if got := find("GIT_TERMINAL_PROMPT"); got != "0" {
		t.Errorf("GIT_TERMINAL_PROMPT = %q, want 0", got)
	}
}

// TestCursorSpawnPingParity asserts spawn and ping use matching trust/permissive flags.
func TestCursorSpawnPingParity(t *testing.T) {
	prompt := "test"
	spawn := councilSpawnArgs(AgentCursor, prompt, "", "", true)
	ping := councilPingArgs(AgentCursor, prompt, "", "", true)

	contains := func(args []string, flag string) bool {
		for _, a := range args {
			if a == flag {
				return true
			}
		}
		return false
	}

	// Both must include --yolo, --trust, and --approve-mcps for unrestricted tooling parity.
	requiredFlags := []string{"--yolo", "--trust", "--approve-mcps"}
	for _, flag := range requiredFlags {
		if !contains(spawn, flag) {
			t.Errorf("councilSpawnArgs(Cursor) should contain %s, got %v", flag, spawn)
		}
		if !contains(ping, flag) {
			t.Errorf("councilPingArgs(Cursor) should contain %s, got %v", flag, ping)
		}
	}

	// Both must start with "agent" subcommand and "--print".
	for label, args := range map[string][]string{"spawn": spawn, "ping": ping} {
		if len(args) < 2 || args[0] != "agent" || args[1] != "--print" {
			t.Errorf("councilSpawnArgs/PingArgs(Cursor) %s should start with [agent --print], got %v", label, args)
		}
	}
}

// TestDiscoverAgentSilentBinaryReturnsNil verifies that a binary that exists but produces
// no stdout/stderr on --version or --help is not added to the roster.
func TestDiscoverAgentSilentBinaryReturnsNil(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script mocks not supported on Windows")
	}
	tmpdir := t.TempDir()

	// Use a fake agent name unlikely to exist anywhere in the system fallback paths.
	const fakeName = "council-test-silent-agent-zzz"

	// Place the silent binary in tmpdir and put tmpdir first in PATH.
	createMockAgent(t, tmpdir, fakeName, "exit 1")

	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })
	_ = os.Setenv("PATH", tmpdir+string(os.PathListSeparator)+oldPath)

	// Temporarily register it as a known agent name by calling discoverAgent directly
	// with a synthetic AgentName, bypassing the roster.
	result := discoverAgent(context.Background(), AgentName(fakeName), 5)
	if result != nil {
		t.Fatalf("discoverAgent() = %+v, want nil for silent binary", result)
	}
}
