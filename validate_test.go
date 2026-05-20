package main

import (
	"os"
	"path/filepath"
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
