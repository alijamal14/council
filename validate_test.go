package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsValidOutput_Valid(t *testing.T) {
	// Create temp file with valid content
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "valid.txt")
	os.WriteFile(file, []byte("This is valid output content"), 0644)

	if !isValidOutput(file) {
		t.Error("Expected isValidOutput to return true for valid content")
	}
}

func TestIsValidOutput_RateLimit(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "ratelimit.txt")
	os.WriteFile(file, []byte("Error: 429 Too Many Requests"), 0644)

	if isValidOutput(file) {
		t.Error("Expected isValidOutput to return false for rate limit error")
	}
}

// A substantial answer that *discusses* error conditions (HTTP 429, panics,
// rate limits) must not be rejected: only the head of a long output is scanned.
// Regression test for a real council run where a correct plan mentioning
// "returning HTTP 429/503" was failed four times.
func TestIsValidOutput_SubstantialAnswerDiscussingErrors(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "plan.txt")

	body := "1. A huge buffer hides a throughput mismatch: workers cannot keep up and memory grows.\n" +
		"2. Correct backpressure design: bounded channel; when full, shed load or slow intake "
	for len(body) < 1100 {
		body += "and propagate pressure upstream by rate-limiting producers with bounded queues. "
	}
	body += "\n3. Under sustained overload return HTTP 429/503 to callers instead of buffering forever.\n"

	os.WriteFile(file, []byte(body), 0644)
	if !isValidOutput(file) {
		t.Error("substantial answer mentioning HTTP 429 must be valid")
	}
}

// A long Codex banner followed by a trailing "requires a newer version" API
// error must be invalid — the mismatch lands after substantial preamble.
func TestIsValidOutput_CodexNeedsNewerCLI(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "plan.codex.txt")
	banner := "OpenAI Codex v0.142.5\n--------\nworkdir: C:\\Users\\ali\\ai\nmodel: gpt-5.6-sol\n"
	for len(banner) < 1100 {
		banner += "provider: openai\napproval: never\nsandbox: danger-full-access\n"
	}
	banner += `ERROR: {"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The 'gpt-5.6-sol' model requires a newer version of Codex. Please upgrade to the latest app or CLI and try again."}}`
	os.WriteFile(file, []byte(banner), 0644)
	if isValidOutput(file) {
		t.Error("Codex CLI-too-old error after a long banner must be invalid")
	}
	if !isCLIVersionMismatch(banner) {
		t.Error("isCLIVersionMismatch should detect requires-a-newer-version")
	}
}

func TestCodexCompatibleFallbackModel(t *testing.T) {
	t.Setenv("COUNCIL_CODEX_FALLBACK_MODEL", "")
	if got := codexCompatibleFallbackModel(); got != "" {
		t.Fatalf("default fallback = %q, want empty (ChatGPT-safe)", got)
	}
	t.Setenv("COUNCIL_CODEX_FALLBACK_MODEL", "o4-mini")
	if got := codexCompatibleFallbackModel(); got != "o4-mini" {
		t.Fatalf("env fallback = %q, want o4-mini", got)
	}
}

func TestVersionSemver(t *testing.T) {
	if got := versionSemver("codex-cli 0.153.3"); got != "0.153.3" {
		t.Fatalf("got %q", got)
	}
	if !semverNewer(versionSemver("codex-cli 0.153.3"), versionSemver("codex-cli 0.142.5")) {
		t.Fatal("0.153.3 should be newer than 0.142.5")
	}
}

func TestRefreshResolvedAgentPathPrefersNewer(t *testing.T) {
	tmpdir := t.TempDir()
	old := createMockAgent(t, tmpdir, "codex-old", "echo codex-cli 0.142.5")
	newerDir := filepath.Join(tmpdir, "npm")
	_ = os.MkdirAll(newerDir, 0755)
	// Put a newer stub where refreshResolvedAgentPath looks on Windows (APPDATA/npm)
	// and via LookPath by putting tmpdir first on PATH.
	var newer string
	if runtime.GOOS == "windows" {
		newer = filepath.Join(newerDir, "codex.cmd")
		_ = os.WriteFile(newer, []byte("@echo off\r\necho codex-cli 0.153.3\r\n"), 0755)
		t.Setenv("APPDATA", tmpdir)
		_ = os.MkdirAll(filepath.Join(tmpdir, "npm"), 0755)
		_ = os.WriteFile(filepath.Join(tmpdir, "npm", "codex.cmd"), []byte("@echo off\r\necho codex-cli 0.153.3\r\n"), 0755)
	} else {
		newer = filepath.Join(tmpdir, "codex-new")
		_ = os.WriteFile(newer, []byte("#!/bin/sh\necho codex-cli 0.153.3\n"), 0755)
		t.Setenv("HOME", tmpdir)
		_ = os.MkdirAll(filepath.Join(tmpdir, ".local", "bin"), 0755)
		_ = os.WriteFile(filepath.Join(tmpdir, ".local", "bin", "codex"), []byte("#!/bin/sh\necho codex-cli 0.153.3\n"), 0755)
	}

	resolved := &ResolvedAgent{Name: AgentCodex, Path: old, RunnerType: "local"}
	refreshResolvedAgentPath(context.Background(), AgentCodex, resolved)
	ver := probeVersion(context.Background(), catalogEntry(AgentCodex), resolved.Path)
	if !semverNewer(versionSemver(ver), "0.142.5") {
		t.Fatalf("expected refresh to pick newer binary, path=%s ver=%s", resolved.Path, ver)
	}
	_ = newer
}

// A long output is still invalid when the error appears at the start (real CLI
// failures fail fast) or when a council marker is appended at the end.
func TestIsValidOutput_LongOutputRealFailures(t *testing.T) {
	tmpdir := t.TempDir()
	filler := ""
	for len(filler) < 1200 {
		filler += "retrying with exponential backoff while the service recovers... "
	}

	headFail := filepath.Join(tmpdir, "head.txt")
	os.WriteFile(headFail, []byte("Error: 429 Too Many Requests\n"+filler), 0644)
	if isValidOutput(headFail) {
		t.Error("long output with a leading rate-limit error must be invalid")
	}

	markerFail := filepath.Join(tmpdir, "marker.txt")
	os.WriteFile(markerFail, []byte(filler+"\n[COUNCIL_AGENT_FAILED] X did not produce valid output after 4 attempts.\n"), 0644)
	if isValidOutput(markerFail) {
		t.Error("long output with a trailing failure marker must be invalid")
	}
}

func TestIsValidOutput_TimeoutMarker(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "timeout.txt")
	os.WriteFile(file, []byte("[COUNCIL_AGENT_TIMEOUT] Agent exceeded 180s timeout"), 0644)

	if isValidOutput(file) {
		t.Error("Expected isValidOutput to return false for timeout marker")
	}
}

func TestIsValidOutput_FailedMarker(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "failed.txt")
	os.WriteFile(file, []byte("[COUNCIL_AGENT_FAILED] Agent did not produce valid output"), 0644)

	if isValidOutput(file) {
		t.Error("Expected isValidOutput to return false for failed marker")
	}
}

func TestIsValidOutput_AllWhitespace(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "whitespace.txt")
	os.WriteFile(file, []byte("   \n\n  \t  \n"), 0644)

	if isValidOutput(file) {
		t.Error("Expected isValidOutput to return false for all-whitespace content")
	}
}

func TestIsValidOutput_EmptyFile(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "empty.txt")
	os.WriteFile(file, []byte(""), 0644)

	if isValidOutput(file) {
		t.Error("Expected isValidOutput to return false for empty file")
	}
}

func TestIsValidOutput_NetworkError(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "network.txt")
	os.WriteFile(file, []byte("Connection refused: ECONNREFUSED"), 0644)

	if isValidOutput(file) {
		t.Error("Expected isValidOutput to return false for network error")
	}
}

func TestIsValidOutput_MissingFile(t *testing.T) {
	if isValidOutput("/nonexistent/file.txt") {
		t.Error("Expected isValidOutput to return false for missing file")
	}
}

func TestContainsErrorPattern(t *testing.T) {
	tests := []struct {
		content  string
		expected bool
	}{
		{"Error: 429 Too Many Requests", true},
		{"HTTP 429 ", true},
		{"429 rate limit reached", true},
		{"Error executing tool failed", true},
		{"connection refused ECONNREFUSED", true},
		{"normal output text", false},
		{"this is valid content", false},
		{"See line 429 in the spec for details.", false},
	}

	for _, test := range tests {
		result := containsErrorPattern(test.content)
		if result != test.expected {
			t.Errorf("containsErrorPattern(%q) = %v, want %v", test.content, result, test.expected)
		}
	}
}

func TestIsAllWhitespace(t *testing.T) {
	tests := []struct {
		s        string
		expected bool
	}{
		{"   ", true},
		{"\n\n", true},
		{"\t\t", true},
		{"hello", false},
		{"  text  ", false},
	}

	for _, test := range tests {
		result := isAllWhitespace(test.s)
		if result != test.expected {
			t.Errorf("isAllWhitespace(%q) = %v, want %v", test.s, result, test.expected)
		}
	}
}

func TestFileSize(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "test.txt")
	content := "Hello, World!"
	os.WriteFile(file, []byte(content), 0644)

	size := fileSize(file)
	expected := int64(len(content))
	if size != expected {
		t.Errorf("fileSize() = %d, want %d", size, expected)
	}
}

func TestCountValidFiles(t *testing.T) {
	tmpdir := t.TempDir()

	// Create valid files
	os.WriteFile(filepath.Join(tmpdir, "plan.gemini.txt"), []byte("valid output"), 0644)
	os.WriteFile(filepath.Join(tmpdir, "plan.claude.txt"), []byte("valid output"), 0644)

	// Create invalid file
	os.WriteFile(filepath.Join(tmpdir, "plan.codex.txt"), []byte("[COUNCIL_AGENT_FAILED] error"), 0644)

	count := countValidFiles(tmpdir, "plan.")
	if count != 2 {
		t.Errorf("countValidFiles() = %d, want 2", count)
	}
}

func TestIsPingResponseValid_ValidContent(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "ping.txt")
	os.WriteFile(file, []byte("ready\n"), 0644)
	if !isPingResponseValid(file) {
		t.Error("Expected isPingResponseValid to return true for 'ready' response")
	}
}

func TestIsPingResponseValid_PreambleOnly(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "ping.txt")
	// Only auth preamble, no actual response — should fail
	os.WriteFile(file, []byte("Loaded cached credentials.\n"), 0644)
	if isPingResponseValid(file) {
		t.Error("Expected isPingResponseValid to return false when only preamble is present")
	}
}

func TestIsPingResponseValid_PreambleThenContent(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "ping.txt")
	// Preamble followed by real response — should pass
	os.WriteFile(file, []byte("Loaded cached credentials.\nready\n"), 0644)
	if !isPingResponseValid(file) {
		t.Error("Expected isPingResponseValid to return true when preamble is followed by real content")
	}
}

func TestIsPingResponseValid_Empty(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "ping.txt")
	os.WriteFile(file, []byte(""), 0644)
	if isPingResponseValid(file) {
		t.Error("Expected isPingResponseValid to return false for empty file")
	}
}

func TestIsPingResponseValid_ErrorPattern(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "ping.txt")
	os.WriteFile(file, []byte("Loaded cached credentials.\n429 Too Many Requests\n"), 0644)
	if isPingResponseValid(file) {
		t.Error("Expected isPingResponseValid to return false when error pattern present")
	}
}

func TestIsPingResponseValid_MissingFile(t *testing.T) {
	if isPingResponseValid("/nonexistent/file.txt") {
		t.Error("Expected isPingResponseValid to return false for missing file")
	}
}

func TestWriteFailedMarker_Appends(t *testing.T) {
	tmpdir := t.TempDir()
	outFile := filepath.Join(tmpdir, "failed.txt")

	// Write partial output first
	os.WriteFile(outFile, []byte("partial agent output\n"), 0644)

	writeFailedMarker(outFile, AgentClaude, 3)

	content, _ := os.ReadFile(outFile)
	contentStr := string(content)

	if !contains(contentStr, "partial agent output") {
		t.Error("writeFailedMarker() should preserve existing partial output")
	}
	if !contains(contentStr, "[COUNCIL_AGENT_FAILED]") {
		t.Error("writeFailedMarker() should append the failed marker")
	}
}

func TestWriteTimeoutMarker_Appends(t *testing.T) {
	tmpdir := t.TempDir()
	outFile := filepath.Join(tmpdir, "timeout.txt")

	// Write partial output first
	os.WriteFile(outFile, []byte("partial agent output\n"), 0644)

	writeTimeoutMarker(outFile, AgentGemini, 180)

	content, _ := os.ReadFile(outFile)
	contentStr := string(content)

	if !contains(contentStr, "partial agent output") {
		t.Error("writeTimeoutMarker() should preserve existing partial output")
	}
	if !contains(contentStr, "[COUNCIL_AGENT_TIMEOUT]") {
		t.Error("writeTimeoutMarker() should append the timeout marker")
	}
}
