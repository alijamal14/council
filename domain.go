package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type DomainManifest struct {
	Domain      string   `yaml:"domain"`
	Description string   `yaml:"description"`
	Keywords    []string `yaml:"keywords"`
	AllowGlobs  []string `yaml:"allow_globs"`
	IgnoreGlobs []string `yaml:"ignore_globs"`
}

type Registry struct {
	Domains map[string]struct {
		Description string `yaml:"description"`
		Keywords    []string `yaml:"keywords"`
		Manifest    string `yaml:"manifest"`
	} `yaml:"domains"`
}

type DomainEntry struct {
	Name     string
	Keywords []string
}

// parseRegistry parses the domain registry YAML file using gopkg.in/yaml.v3
func parseRegistry(registryFile string) ([]DomainEntry, error) {
	data, err := os.ReadFile(registryFile)
	if err != nil {
		return nil, err
	}

	var registry Registry
	if err := yaml.Unmarshal(data, &registry); err != nil {
		return nil, err
	}

	var entries []DomainEntry
	for name, domain := range registry.Domains {
		entries = append(entries, DomainEntry{
			Name:     name,
			Keywords: domain.Keywords,
		})
	}

	return entries, nil
}

// scoreTask calculates a score for how well a domain matches a task
func scoreTask(task string, entry DomainEntry) int {
	taskLower := strings.ToLower(task)
	score := 0
	for _, kw := range entry.Keywords {
		if strings.Contains(taskLower, strings.ToLower(kw)) {
			score++
		}
	}
	return score
}

// resolveDomain finds the best matching domain for a task
func resolveDomain(task string, entries []DomainEntry) (string, int) {
	bestDomain := ""
	bestScore := 0

	for _, entry := range entries {
		score := scoreTask(task, entry)
		if score > bestScore {
			bestScore = score
			bestDomain = entry.Name
		}
	}

	return bestDomain, bestScore
}

// loadDomainManifest loads the domain manifest file and returns its content
// (first 20 lines of key properties, excluding globs)
func loadDomainManifest(domainsDir, domainName string) string {
	if domainName == "" {
		return ""
	}

	// Try common extensions
	manifestPath := filepath.Join(domainsDir, domainName+".template.yml")
	if _, err := os.Stat(manifestPath); err != nil {
		manifestPath = filepath.Join(domainsDir, domainName+".yml")
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Sprintf("### DOMAIN CONTEXT: %s (no manifest file)\n", domainName)
	}

	var manifest DomainManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		// Fallback to raw read if YAML is invalid
		return fmt.Sprintf("### DOMAIN CONTEXT: %s (invalid manifest)\n", domainName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### DOMAIN CONTEXT: %s\n", domainName))
	sb.WriteString(fmt.Sprintf("Description: %s\n", manifest.Description))
	if len(manifest.Keywords) > 0 {
		sb.WriteString(fmt.Sprintf("Keywords: %s\n", strings.Join(manifest.Keywords, ", ")))
	}
	
	// Add some raw lines from the file but skip the big glob lists
	lines := strings.Split(string(data), "\n")
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "allow_globs:") || strings.HasPrefix(trimmed, "ignore_globs:") || strings.HasPrefix(trimmed, "-") {
			continue
		}
		if count >= 15 {
			break
		}
		sb.WriteString(line + "\n")
		count++
	}

	return sb.String()
}
