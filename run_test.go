package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateRunDir(t *testing.T) {
	tmpdir := t.TempDir()
	councilRunsDir := filepath.Join(tmpdir, "council_runs")
	os.MkdirAll(councilRunsDir, 0755)

	runDir, err := createRunDir(councilRunsDir)
	if err != nil {
		t.Fatalf("createRunDir() error = %v", err)
	}

	if !strings.Contains(runDir, "run_") {
		t.Errorf("createRunDir() = %s, should contain 'run_'", runDir)
	}

	if !strings.Contains(runDir, "_") {
		t.Error("createRunDir() should contain date/time separators")
	}

	// Verify directory exists
	if _, err := os.Stat(runDir); err != nil {
		t.Errorf("createRunDir() created directory doesn't exist: %v", err)
	}
}

func TestWriteBrief(t *testing.T) {
	tmpdir := t.TempDir()
	runDir := filepath.Join(tmpdir, "run_test")
	os.Mkdir(runDir, 0755)

	task := "Test task description"
	err := writeBrief(runDir, task)
	if err != nil {
		t.Fatalf("writeBrief() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(runDir, "brief.txt"))
	if err != nil {
		t.Fatalf("Failed to read brief.txt: %v", err)
	}

	if string(content) != task {
		t.Errorf("writeBrief() content = %q, want %q", string(content), task)
	}
}

func TestCountIterDirs(t *testing.T) {
	tmpdir := t.TempDir()

	// Create iteration directories
	os.Mkdir(filepath.Join(tmpdir, "iter_1"), 0755)
	os.Mkdir(filepath.Join(tmpdir, "iter_2"), 0755)
	os.Mkdir(filepath.Join(tmpdir, "iter_3"), 0755)
	os.Mkdir(filepath.Join(tmpdir, "other_dir"), 0755)

	count, err := countIterDirs(tmpdir)
	if err != nil {
		t.Fatalf("countIterDirs() error = %v", err)
	}

	if count != 3 {
		t.Errorf("countIterDirs() = %d, want 3", count)
	}
}

func TestCreateIterDir(t *testing.T) {
	tmpdir := t.TempDir()

	iterDir1, num1, err := createIterDir(tmpdir)
	if err != nil {
		t.Fatalf("createIterDir() error = %v", err)
	}

	if num1 != 1 {
		t.Errorf("createIterDir() number = %d, want 1", num1)
	}

	if !strings.Contains(iterDir1, "iter_1") {
		t.Errorf("createIterDir() path should contain 'iter_1'")
	}

	iterDir2, num2, _ := createIterDir(tmpdir)
	if num2 != 2 {
		t.Errorf("Second createIterDir() number = %d, want 2", num2)
	}

	if strings.Contains(iterDir2, "iter_1") {
		t.Error("Second createIterDir() should create iter_2, not iter_1")
	}
}

func TestAppendHistory(t *testing.T) {
	tmpdir := t.TempDir()
	runDir := filepath.Join(tmpdir, "run")
	os.Mkdir(runDir, 0755)

	err := appendHistory(runDir, "First line\n")
	if err != nil {
		t.Fatalf("appendHistory() error = %v", err)
	}

	err = appendHistory(runDir, "Second line\n")
	if err != nil {
		t.Fatalf("appendHistory() error = %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(runDir, "history.md"))
	contentStr := string(content)

	if !strings.Contains(contentStr, "First line") {
		t.Error("appendHistory() should contain first line")
	}

	if !strings.Contains(contentStr, "Second line") {
		t.Error("appendHistory() should contain second line")
	}
}

func TestBuildContext(t *testing.T) {
	tmpdir := t.TempDir()
	runDir := filepath.Join(tmpdir, "run")
	os.Mkdir(runDir, 0755)

	// Write brief
	os.WriteFile(filepath.Join(runDir, "brief.txt"), []byte("Test task"), 0644)

	// Write history
	os.WriteFile(filepath.Join(runDir, "history.md"), []byte("Test history"), 0644)

	context, err := buildContext(runDir, "")
	if err != nil {
		t.Fatalf("buildContext() error = %v", err)
	}

	if !strings.Contains(context, "INITIAL BRIEF") {
		t.Error("buildContext() should include INITIAL BRIEF")
	}

	if !strings.Contains(context, "Test task") {
		t.Error("buildContext() should include task content")
	}

	if !strings.Contains(context, "CONVERSATION HISTORY") {
		t.Error("buildContext() should include CONVERSATION HISTORY")
	}
}

func TestLastNLines(t *testing.T) {
	tmpdir := t.TempDir()
	file := filepath.Join(tmpdir, "test.txt")

	content := "line1\nline2\nline3\nline4\nline5\n"
	os.WriteFile(file, []byte(content), 0644)

	result := lastNLines(file, 3)
	lines := strings.Split(strings.TrimSpace(result), "\n")

	if len(lines) < 3 {
		t.Errorf("lastNLines(3) returned %d lines", len(lines))
	}

	if !contains(result, "line3") || !contains(result, "line4") || !contains(result, "line5") {
		t.Error("lastNLines() should return last 3 lines")
	}
}

func TestAuditLoggerLog(t *testing.T) {
	tmpdir := t.TempDir()
	mdPath := filepath.Join(tmpdir, "audit.md")
	jsonlPath := filepath.Join(tmpdir, "audit.jsonl")

	logger, err := newAuditLogger("test-session", []string{mdPath}, []string{jsonlPath})
	if err != nil {
		t.Fatalf("newAuditLogger() error = %v", err)
	}
	defer logger.Close()

	logger.Log("TEST", "Test message")

	// Check markdown file
	mdContent, _ := os.ReadFile(mdPath)
	mdStr := string(mdContent)
	if !contains(mdStr, "TEST") || !contains(mdStr, "Test message") {
		t.Error("Audit log markdown should contain logged message")
	}

	// Check JSONL file
	jsonlContent, _ := os.ReadFile(jsonlPath)
	jsonlStr := string(jsonlContent)
	if !contains(jsonlStr, "TEST") || !contains(jsonlStr, "Test message") {
		t.Error("Audit log JSONL should contain logged message")
	}
}

func TestAuditLoggerConcurrent(t *testing.T) {
	tmpdir := t.TempDir()
	mdPath := filepath.Join(tmpdir, "audit.md")
	jsonlPath := filepath.Join(tmpdir, "audit.jsonl")

	logger, err := newAuditLogger("test-session", []string{mdPath}, []string{jsonlPath})
	if err != nil {
		t.Fatalf("newAuditLogger() error = %v", err)
	}
	defer logger.Close()

	// Write from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func(i int) {
			logger.LogAgent("TEST", "Concurrent message", "Agent", i)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify all messages were written
	mdContent, _ := os.ReadFile(mdPath)
	mdStr := string(mdContent)
	lines := strings.Split(mdStr, "\n")

	auditLines := 0
	for _, line := range lines {
		if contains(line, "TEST") {
			auditLines++
		}
	}

	if auditLines < 5 {
		t.Errorf("Expected at least 5 audit lines, got %d", auditLines)
	}
}

