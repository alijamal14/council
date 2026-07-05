package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Persistent configuration: a plain KEY=VALUE file mapping onto the same
// COUNCIL_* environment variables council already honors. Real environment
// variables always win over the file, so one-off overrides still work:
//
//	council config set COUNCIL_CLAUDE_MODEL haiku     # persist
//	COUNCIL_CLAUDE_MODEL=opus council "task"          # override once
//
// Location: <os user config dir>/council/config.env
//   - Linux:   ~/.config/council/config.env
//   - macOS:   ~/Library/Application Support/council/config.env
//   - Windows: %AppData%\council\config.env

func councilConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "council")
}

func configFilePath() string {
	return filepath.Join(councilConfigDir(), "config.env")
}

// readConfigFile parses the KEY=VALUE config file. Missing file = empty map.
func readConfigFile() map[string]string {
	values := map[string]string{}
	data, err := os.ReadFile(configFilePath())
	if err != nil {
		return values
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if key != "" {
			values[key] = val
		}
	}
	return values
}

func writeConfigFile(values map[string]string) error {
	if err := os.MkdirAll(councilConfigDir(), 0755); err != nil {
		return err
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# council persistent configuration (council config set KEY VALUE)\n")
	b.WriteString("# real environment variables override these values\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, values[k])
	}
	return os.WriteFile(configFilePath(), []byte(b.String()), 0644)
}

// applyPersistentConfig loads COUNCIL_* keys from the config file into the
// process environment unless the variable is already set. Called before flag
// parsing so everything downstream (model overrides, COUNCIL_AGENTS,
// COUNCIL_CONTEXT_CMD, COUNCIL_AUTO_UPDATE, ...) sees persisted values.
func applyPersistentConfig() {
	for key, val := range readConfigFile() {
		if !strings.HasPrefix(key, "COUNCIL_") {
			continue // only council's own namespace may be injected
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

// handleConfig implements: council config [list|get KEY|set KEY VALUE|unset KEY|path]
func handleConfig(args []string) int {
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "path":
		fmt.Println(configFilePath())
		return 0

	case "list":
		values := readConfigFile()
		if len(values) == 0 {
			fmt.Println("No persistent settings. Set one with: council config set COUNCIL_CLAUDE_MODEL haiku")
			fmt.Printf("File: %s\n", configFilePath())
			return 0
		}
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			marker := ""
			if env, set := os.LookupEnv(k); set && env != values[k] {
				marker = fmt.Sprintf("   (overridden by environment: %s)", env)
			}
			fmt.Printf("%s=%s%s\n", k, values[k], marker)
		}
		fmt.Printf("\nFile: %s\n", configFilePath())
		return 0

	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: council config get KEY")
			return 2
		}
		if val, ok := readConfigFile()[args[1]]; ok {
			fmt.Println(val)
			return 0
		}
		return 1

	case "set":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: council config set KEY VALUE   (e.g. council config set COUNCIL_CLAUDE_MODEL haiku)")
			return 2
		}
		key := args[1]
		if !strings.HasPrefix(key, "COUNCIL_") {
			fmt.Fprintf(os.Stderr, "Error: only COUNCIL_* keys can be persisted (got %q).\n", key)
			fmt.Fprintln(os.Stderr, "Model overrides: COUNCIL_<AGENT>_MODEL. Roster: COUNCIL_AGENTS. See: council models")
			return 2
		}
		values := readConfigFile()
		values[key] = strings.Join(args[2:], " ")
		if err := writeConfigFile(values); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
			return 2
		}
		fmt.Printf("✅ %s=%s saved to %s\n", key, values[key], configFilePath())
		return 0

	case "unset":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: council config unset KEY")
			return 2
		}
		values := readConfigFile()
		if _, ok := values[args[1]]; !ok {
			fmt.Printf("%s was not set.\n", args[1])
			return 0
		}
		delete(values, args[1])
		if err := writeConfigFile(values); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
			return 2
		}
		fmt.Printf("✅ %s removed.\n", args[1])
		return 0

	default:
		fmt.Fprintln(os.Stderr, "Usage: council config [list|get KEY|set KEY VALUE|unset KEY|path]")
		return 2
	}
}
