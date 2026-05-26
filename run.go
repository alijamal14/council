package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	Agent     string `json:"agent,omitempty"`
	Attempt   int    `json:"attempt,omitempty"`
}

type AuditLogger struct {
	mu         sync.Mutex
	sessionID  string
	mdFiles    []*os.File
	jsonlFiles []*os.File
}

// newAuditLogger creates a new audit logger with initial global files
func newAuditLogger(sessionID string, mdPaths, jsonlPaths []string) (*AuditLogger, error) {
	l := &AuditLogger{
		sessionID: sessionID,
	}

	for _, path := range mdPaths {
		if path == os.DevNull {
			continue
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			l.mdFiles = append(l.mdFiles, f)
		}
	}

	for _, path := range jsonlPaths {
		if path == os.DevNull {
			continue
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			l.jsonlFiles = append(l.jsonlFiles, f)
		}
	}

	return l, nil
}

// AddSessionFile adds a session-specific JSONL log file
func (l *AuditLogger) AddSessionFile(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	l.jsonlFiles = append(l.jsonlFiles, f)
	return nil
}

// Log logs a message with a level
func (l *AuditLogger) Log(level, message string) {
	l.LogAgent(level, message, "", 0)
}

// LogAgent logs a message with agent and attempt info
func (l *AuditLogger) LogAgent(level, message, agent string, attempt int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Write markdown format
	mdLine := fmt.Sprintf("#### %s [%s] %s\n", level, timestamp, message)
	for _, f := range l.mdFiles {
		if _, err := f.WriteString(mdLine); err != nil {
			fmt.Fprintf(os.Stderr, "audit log write error (md): %v\n", err)
		}
	}

	// Write JSONL format
	entry := AuditEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		SessionID: l.sessionID,
		Message:   message,
		Agent:     agent,
		Attempt:   attempt,
	}
	data, _ := json.Marshal(entry)
	jsonLine := string(data) + "\n"
	for _, f := range l.jsonlFiles {
		if _, err := f.WriteString(jsonLine); err != nil {
			fmt.Fprintf(os.Stderr, "audit log write error (jsonl): %v\n", err)
		}
	}
}

// Close closes all log files
func (l *AuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var err error
	for _, f := range l.mdFiles {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	for _, f := range l.jsonlFiles {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

const defaultCouncilRunRetention = 200

// createRunDir creates a new run directory with timestamp-based name.
func createRunDir(councilRunsDir string) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	// Create temp directory - we'll use a numbered approach since Go's TempDir
	// may not handle the pattern the same way
	for i := 0; i < 1000; i++ {
		suffix := fmt.Sprintf("%06d", i)
		runDir := filepath.Join(councilRunsDir, "run_"+timestamp+"_"+suffix)
		err := os.Mkdir(runDir, 0755)
		if err == nil {
			// Rotate after creating the new run so retention is exact and includes this run.
			if os.Getenv("COUNCIL_NO_ROTATE") != "1" {
				rotateRunDirs(councilRunsDir, councilRunRetention())
			}
			return runDir, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("could not create unique run directory")
}

func councilRunRetention() int {
	value := strings.TrimSpace(os.Getenv("COUNCIL_KEEP_RUNS"))
	if value == "" {
		return defaultCouncilRunRetention
	}
	keep, err := strconv.Atoi(value)
	if err != nil || keep < 1 {
		return defaultCouncilRunRetention
	}
	return keep
}

// rotateRunDirs deletes old run directories to save space.
func rotateRunDirs(dir string, keep int) {
	if keep < 1 {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	type runEntry struct {
		name    string
		modTime time.Time
	}

	var runs []runEntry
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "run_") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		runs = append(runs, runEntry{name: entry.Name(), modTime: info.ModTime()})
	}

	if len(runs) <= keep {
		return
	}

	// Sort by modification time, with name as a deterministic tie-breaker. This handles
	// both normal timestamped run dirs and manually named run_* directories.
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].modTime.Equal(runs[j].modTime) {
			return runs[i].name < runs[j].name
		}
		return runs[i].modTime.Before(runs[j].modTime)
	})

	// Delete oldest.
	toDelete := len(runs) - keep
	for i := 0; i < toDelete; i++ {
		os.RemoveAll(filepath.Join(dir, runs[i].name))
	}
}

// writeBrief writes the task brief to a file
func writeBrief(runDir, task string) error {
	briefFile := filepath.Join(runDir, "brief.txt")
	return os.WriteFile(briefFile, []byte(task), 0644)
}

// countIterDirs counts iteration directories in a run
func countIterDirs(runDir string) (int, error) {
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "iter_") {
			count++
		}
	}
	return count, nil
}

// createIterDir creates a new iteration directory
func createIterDir(runDir string) (string, int, error) {
	count, err := countIterDirs(runDir)
	if err != nil {
		return "", 0, err
	}
	count++
	iterDir := filepath.Join(runDir, "iter_"+strconv.Itoa(count))
	err = os.Mkdir(iterDir, 0755)
	return iterDir, count, err
}

// appendHistory appends content to the run's history file and includes previous critiques
func appendHistory(runDir, content string) error {
	historyFile := filepath.Join(runDir, "history.md")
	f, err := os.OpenFile(historyFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// If this is user feedback, we should first append any critiques from the LATEST iteration
	if strings.Contains(content, "User Feedback") {
		latestIter, _ := countIterDirs(runDir)
		if latestIter > 0 {
			iterPath := filepath.Join(runDir, "iter_"+strconv.Itoa(latestIter))
			entries, _ := os.ReadDir(iterPath)
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), "critique.") && strings.HasSuffix(e.Name(), ".txt") {
					agentName := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "critique."), ".txt")
					critiqueData, _ := os.ReadFile(filepath.Join(iterPath, e.Name()))
					if len(critiqueData) > 0 {
						f.WriteString(fmt.Sprintf("#### Critique from %s (Round %d)\n%s\n\n", agentName, latestIter, string(critiqueData)))
					}
				}
			}
		}
	}

	_, err = f.WriteString(content)
	return err
}

// buildContext builds the context from brief, audit log, and history
func buildContext(runDir, mdAuditLog string) (string, error) {
	result := ""

	// Initial brief
	briefFile := filepath.Join(runDir, "brief.txt")
	briefData, err := os.ReadFile(briefFile)
	if err == nil {
		result += "### INITIAL BRIEF\n" + string(briefData) + "\n\n"
	}

	// Lessons learned (from audit log)
	if mdAuditLog != "" {
		lessons := lastNLinesWithFilter(mdAuditLog, 10, []string{"SUCCESS", "FAILURE", "SKIP"})
		if lessons != "" {
			result += "### LESSONS LEARNED\n" + lessons + "\n\n"
		}
	}

	// Conversation history
	historyFile := filepath.Join(runDir, "history.md")
	history := lastNLines(historyFile, 50)
	if history != "" {
		result += "### CONVERSATION HISTORY (RECENT)\n" + history + "\n\n"
	}

	return result, nil
}

// lastNLines returns the last N lines of a file as a string
func lastNLines(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	// Strip trailing empty element caused by a trailing newline
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}

	// Return last n lines
	start := len(lines) - n
	return strings.Join(lines[start:], "\n")
}

// lastNLinesWithFilter returns last N lines matching any of the filter strings
func lastNLinesWithFilter(path string, n int, filters []string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	var matching []string

	// Collect matching lines
	for _, line := range lines {
		for _, filter := range filters {
			if strings.Contains(line, filter) {
				matching = append(matching, line)
				break
			}
		}
	}

	// Return last n matching lines
	if len(matching) <= n {
		return strings.Join(matching, "\n")
	}
	return strings.Join(matching[len(matching)-n:], "\n")
}

// printSummary prints a summary of results, distinguishing timeout vs failure vs missing.
// When a file came from Copilot failover, preserve that state in the final report.
func printSummary(dir string, results []RunResult) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	failoverFiles := make(map[string]bool, len(results))
	for _, result := range results {
		if result.IsFallback {
			failoverFiles[result.OutFile] = true
		}
	}

	fmt.Println("\n--- Agent Report ---")
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
			path := filepath.Join(dir, entry.Name())
			if isValidOutput(path) {
				size := fileSize(path)
				status := "✅"
				if failoverFiles[path] {
					status = "🔄"
				}
				fmt.Printf("  %s %s (%d bytes)\n", status, entry.Name(), size)
			} else {
				// Read file to distinguish timeout vs failed vs other
				data, readErr := os.ReadFile(path)
				if readErr != nil || len(data) == 0 {
					fmt.Printf("  ❌ %s (missing/empty)\n", entry.Name())
				} else if strings.Contains(string(data), "[COUNCIL_AGENT_TIMEOUT]") {
					fmt.Printf("  ⏱️  %s (timeout — increase --timeout or retry)\n", entry.Name())
				} else if strings.Contains(string(data), "[COUNCIL_AGENT_FAILED]") {
					fmt.Printf("  ❌ %s (failed — check auth or agent availability)\n", entry.Name())
				} else {
					fmt.Printf("  ❌ %s (invalid output)\n", entry.Name())
				}
			}
		}
	}
}
