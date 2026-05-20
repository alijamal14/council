package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

var errorPatterns = []string{
	"429 too many requests",
	"http 429",
	"https 429",
	": 429 ",
	"[429]",
	"(429)",
	"status 429",
	"429 rate-limit",
	"429 rate limit",
	"too many requests",
	"quota exceeded",
	"rate limit exceeded",
	"rate-limit exceeded",
	"Error executing tool",
	"FATAL", "panic:", "Traceback (most recent",
	"connection refused", "ECONNREFUSED", "ETIMEDOUT", "socket hang up", "network error",
	"command not found", "cannot execute", "permission denied",
	"not recognized as", "no such file or directory",
	"internal server error", "503 service", "502 bad gateway",
	"authentication failed",
	"invalid api key",
	"[COUNCIL_AGENT_TIMEOUT]", "[COUNCIL_AGENT_FAILED]",
}

// isValidOutput checks if a file exists, is non-empty, and doesn't contain error patterns
func isValidOutput(outFile string) bool {
	// File must exist and be non-empty
	data, err := os.ReadFile(outFile)
	if err != nil || len(data) == 0 {
		return false
	}

	content := string(data)

	// Check for error patterns across the full file
	if containsErrorPattern(content) {
		return false
	}

	// Must not be all whitespace
	if isAllWhitespace(content) {
		return false
	}

	return true
}

// readHead reads up to n bytes from a file
func readHead(outFile string, n int) string {
	data, err := os.ReadFile(outFile)
	if err != nil {
		return ""
	}
	if len(data) > n {
		data = data[:n]
	}
	return string(data)
}

// containsErrorPattern checks if content matches any error pattern (case-insensitive)
func containsErrorPattern(content string) bool {
	lower := strings.ToLower(content)
	for _, pattern := range errorPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	// Special case: "exec:" AND "not found"
	if strings.Contains(lower, "exec:") && strings.Contains(lower, "not found") {
		return true
	}
	return false
}

// isAllWhitespace checks if a string contains only whitespace
func isAllWhitespace(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// isPingResponseValid checks if an agent responded to the ping prompt.
// More lenient than isValidOutput: strips known auth preamble lines (e.g.
// Gemini's "Loaded cached credentials."), then checks if any real content remains.
func isPingResponseValid(outFile string) bool {
	data, err := os.ReadFile(outFile)
	if err != nil || len(data) == 0 {
		return false
	}
	content := string(data)
	if containsErrorPattern(content) {
		return false
	}
	// Walk lines — return true on the first meaningful (non-preamble) line found
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "loaded cached credentials") ||
			strings.Contains(lower, "loading credentials") {
			continue
		}
		return true
	}
	return false
}

// fileSize returns the size of a file in bytes
func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// countValidFiles counts valid output files matching a glob pattern
func countValidFiles(dir, prefix string) int {
	count := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".txt") {
				path := filepath.Join(dir, name)
				if isValidOutput(path) {
					count++
				}
			}
		}
	}
	return count
}

func writeTimeoutMarker(outFile string, agent AgentName, timeoutSec int) {
	f, err := os.OpenFile(outFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "\n[COUNCIL_AGENT_TIMEOUT] %s exceeded %ds timeout.\n", agent, timeoutSec)
}

func writeFailedMarker(outFile string, agent AgentName, attempts int) {
	f, err := os.OpenFile(outFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "\n[COUNCIL_AGENT_FAILED] %s did not produce valid output after %d attempts.\n", agent, attempts)
}
