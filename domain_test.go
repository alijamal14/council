package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRegistry(t *testing.T) {
	// Create a test registry file
	tmpdir := t.TempDir()
	registryFile := filepath.Join(tmpdir, "_registry.template.yml")

	registryContent := `_meta:
  schema: context/registry
  version: '1.0'

domains:
  council:
    description: "AI Council meetings"
    manifest: council.template.yml
    keywords:
      - council
      - consensus
      - meeting
      - decision
      - consultation
      - whatsapp

  minecraft:
    description: "Minecraft game servers"
    manifest: minecraft.template.yml
    keywords:
      - minecraft
      - server
      - modpack
      - bluemap
`

	os.WriteFile(registryFile, []byte(registryContent), 0644)

	entries, err := parseRegistry(registryFile)
	if err != nil {
		t.Fatalf("parseRegistry() error = %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("parseRegistry() returned %d entries, want 2", len(entries))
	}

	// Find council entry
	var councilEntry *DomainEntry
	for i := range entries {
		if entries[i].Name == "council" {
			councilEntry = &entries[i]
			break
		}
	}

	if councilEntry == nil {
		t.Fatal("council domain not found in parsed registry")
	}

	expectedKeywords := []string{"council", "consensus", "meeting", "decision", "consultation", "whatsapp"}
	if len(councilEntry.Keywords) != len(expectedKeywords) {
		t.Errorf("council domain has %d keywords, want %d", len(councilEntry.Keywords), len(expectedKeywords))
	}

	for i, kw := range expectedKeywords {
		if i >= len(councilEntry.Keywords) {
			break
		}
		if councilEntry.Keywords[i] != kw {
			t.Errorf("keyword %d = %q, want %q", i, councilEntry.Keywords[i], kw)
		}
	}
}

func TestScoreTask(t *testing.T) {
	entry := DomainEntry{
		Name:     "council",
		Keywords: []string{"council", "consensus", "meeting", "decision"},
	}

	tests := []struct {
		task     string
		expected int
	}{
		{"consensus meeting decision", 3},
		{"council discussion", 1},
		{"random task", 0},
	}

	for _, test := range tests {
		score := scoreTask(test.task, entry)
		if score != test.expected {
			t.Errorf("scoreTask(%q) = %d, want %d", test.task, score, test.expected)
		}
	}
}

func TestResolveDomain(t *testing.T) {
	entries := []DomainEntry{
		{Name: "council", Keywords: []string{"council", "consensus", "meeting"}},
		{Name: "minecraft", Keywords: []string{"minecraft", "server", "modpack"}},
		{Name: "general", Keywords: []string{}},
	}

	tests := []struct {
		task            string
		expectedDomain  string
		expectedScore   int
	}{
		{"council consensus meeting", "council", 3},
		{"minecraft server setup", "minecraft", 2},
		{"random task", "", 0},
	}

	for _, test := range tests {
		domain, score := resolveDomain(test.task, entries)
		if domain != test.expectedDomain {
			t.Errorf("resolveDomain(%q) domain = %q, want %q", test.task, domain, test.expectedDomain)
		}
		if score != test.expectedScore {
			t.Errorf("resolveDomain(%q) score = %d, want %d", test.task, score, test.expectedScore)
		}
	}
}

func TestLoadDomainManifest(t *testing.T) {
	tmpdir := t.TempDir()

	// Create a test manifest
	manifestPath := filepath.Join(tmpdir, "council.template.yml")
	manifestContent := `description: "AI Council context"
allow_globs:
  - "*.md"
ignore_globs:
  - "*.log"
content: "This is test content"
more: "More content"`

	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	result := loadDomainManifest(tmpdir, "council")

	if !contains(result, "DOMAIN CONTEXT") {
		t.Error("loadDomainManifest() should include DOMAIN CONTEXT header")
	}

	if contains(result, "allow_globs") {
		t.Error("loadDomainManifest() should exclude allow_globs lines")
	}

	if !contains(result, "content:") {
		t.Error("loadDomainManifest() should include content lines")
	}
}

func TestLoadDomainManifest_NoManifest(t *testing.T) {
	tmpdir := t.TempDir()

	result := loadDomainManifest(tmpdir, "nonexistent")

	if !contains(result, "no manifest file") {
		t.Error("loadDomainManifest() should indicate no manifest found")
	}
}

// Helper function for test assertions
func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
