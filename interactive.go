package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Interactive mode: running bare `council` in a terminal opens a small REPL —
// type a task, the council convenes, and afterwards a feedback> prompt feeds
// `--continue` rounds in the same session. Piped stdin also works:
//
//	git diff | council            (the diff becomes the task context)
//	echo "review api.go" | council

// isTerminalFile reports whether f is attached to a terminal.
func isTerminalFile(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// readPipedTask reads a task from non-terminal stdin (capped at 64 KB so a
// piped diff or log can serve as task context without unbounded prompts).
func readPipedTask() string {
	if isTerminalFile(os.Stdin) {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 64*1024))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// promptForTask shows the interactive welcome and reads one task line.
// ok is false when the user submits a blank line or stdin closes (exit).
func promptForTask() (task string, ok bool) {
	fmt.Println(bold("🏛  AI Council ") + dim("v"+Version+" — interactive mode"))
	fmt.Println(dim("   Describe a task; every agent plans it in parallel, then they cross-critique."))
	fmt.Println(dim("   Blank line exits · council help shows all commands"))
	fmt.Println()
	return readPromptLine(cyan("council> "))
}

// promptFeedback asks for a follow-up after a session completes.
func promptFeedback() (feedback string, ok bool) {
	fmt.Println()
	fmt.Println(dim("↩  Continue this session with feedback — blank line to finish"))
	return readPromptLine(cyan("feedback> "))
}

// readPromptLine prints a prompt and reads one trimmed line from stdin.
func readPromptLine(prompt string) (string, bool) {
	fmt.Print(prompt)
	flush()
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		fmt.Println()
		return "", false
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}
	return line, true
}

// printCompactHeader is the human-terminal replacement for the big ASCII
// invocation box (which is guidance for AI callers with buffered output).
func printCompactHeader() {
	fmt.Fprintln(os.Stderr, bold("🏛  AI Council ")+dim("v"+Version))
}

// shortPingReason condenses a ping failure into one readable line and, when
// it looks like an auth problem, points at the exact fix.
func shortPingReason(agent AgentName, reason string) string {
	// First meaningful line only, hard cap for width.
	line := reason
	for _, cand := range strings.Split(reason, "\n") {
		cand = strings.TrimSpace(cand)
		if cand != "" {
			line = cand
			break
		}
	}
	if len(line) > 100 {
		line = line[:100] + "…"
	}

	lower := strings.ToLower(reason)
	authSmell := strings.Contains(lower, "auth") || strings.Contains(lower, "login") ||
		strings.Contains(lower, "api key") || strings.Contains(lower, "api_key") ||
		strings.Contains(lower, "credential") || strings.Contains(lower, "sign in") ||
		strings.Contains(lower, "set an auth")
	if authSmell {
		return dim(line) + "\n" + yellow(fmt.Sprintf("               → council login %s", normalizeAgentKey(string(agent))))
	}
	return dim(line)
}
