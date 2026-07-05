package main

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Model policy: council never silently overrides an agent's model — each
// vendor's default IS the standard, tuned by the vendor for general use.
// What council does instead is (a) make per-agent overrides easy and
// persistent, and (b) advise after each run when the task/model pairing
// looks wasteful (heavy model on a trivial task) or underpowered (light
// model on a deep task). Advice is one dim line — never a behavior change.

type taskWeight int

const (
	taskLight taskWeight = iota
	taskStandard
	taskHeavy
)

var heavyTaskMarkers = []string{
	"architect", "architecture", "design a system", "system design", "distributed",
	"concurrency", "migration", "migrate", "security review", "threat model",
	"trade-off", "tradeoff", "scalab", "refactor across", "root cause", "deadlock",
	"consensus", "high availability", "disaster recovery", "data model",
}

var lightTaskMarkers = []string{
	"typo", "rename", "one-liner", "one liner", "quick question", "what is",
	"briefly", "in one sentence", "small fix", "format this", "regex for",
	"convert this", "syntax for",
}

// classifyTask estimates how much reasoning a task needs. Heuristic on
// purpose: it only powers advisory output, never routing decisions.
func classifyTask(task string) taskWeight {
	lower := strings.ToLower(task)
	for _, m := range heavyTaskMarkers {
		if strings.Contains(lower, m) {
			return taskHeavy
		}
	}
	for _, m := range lightTaskMarkers {
		if strings.Contains(lower, m) {
			return taskLight
		}
	}
	switch {
	case len(task) < 120:
		return taskLight
	case len(task) > 700:
		return taskHeavy
	default:
		return taskStandard
	}
}

var heavyModelMarkers = []string{"opus", "pro", "o3", "o4", "ultra", "max", "deep", "sonnet-4.5-thinking", "high"}
var lightModelMarkers = []string{"haiku", "flash", "mini", "lite", "nano", "small", "fast", "low"}

// classifyModel estimates the weight class of a model override string.
func classifyModel(model string) taskWeight {
	lower := strings.ToLower(model)
	for _, m := range lightModelMarkers {
		if strings.Contains(lower, m) {
			return taskLight
		}
	}
	for _, m := range heavyModelMarkers {
		if strings.Contains(lower, m) {
			return taskHeavy
		}
	}
	return taskStandard
}

// printModelAdvisory prints at most three dim advisory lines after a run:
// per-agent overkill/underkill notes plus how to change models. Silent when
// there is nothing useful to say (the common case).
func printModelAdvisory(agents AgentSet, cfg Config, task string) {
	weight := classifyTask(task)
	var lines []string

	for name := range agents {
		entry := catalogEntry(name)
		if entry == nil || entry.ModelEnv == "" {
			continue
		}
		override := getModelOverride(name, cfg)
		if override == "" {
			continue
		}
		switch {
		case weight == taskLight && classifyModel(override) == taskHeavy:
			hint := "a lighter model"
			if entry.FastModel != "" {
				hint = fmt.Sprintf("e.g. %s", entry.FastModel)
			}
			lines = append(lines, fmt.Sprintf("%s ran %q on a small task — %s would save tokens: council config set %s <model>",
				name, override, hint, entry.ModelEnv))
		case weight == taskHeavy && classifyModel(override) == taskLight:
			hint := "a deeper model"
			if entry.DeepModel != "" {
				hint = fmt.Sprintf("e.g. %s", entry.DeepModel)
			}
			lines = append(lines, fmt.Sprintf("%s ran %q on a complex task — %s may reason better: council config set %s <model>",
				name, override, hint, entry.ModelEnv))
		}
		if len(lines) >= 3 {
			break
		}
	}

	if len(lines) == 0 && weight == taskHeavy {
		// No overrides set on a heavy task: one gentle pointer, then stay quiet.
		lines = append(lines, "complex task — vendor-default models were used; see `council models` to pin deeper ones per agent")
	}

	if len(lines) == 0 {
		return
	}
	fmt.Println()
	for _, l := range lines {
		fmt.Println(dim("💡 " + l))
	}
}

// handleModels implements `council models`: shows each agent's active model
// (override or vendor default) and exactly how to change it, persistently or
// per run.
func handleModels(ctx context.Context, cfg Config) int {
	fmt.Println(bold("🎛  Council Model Configuration"))
	fmt.Println("--------------------------------")
	fmt.Println("Council uses each vendor CLI's default model unless you override it — the")
	fmt.Println("default is the vendor's recommended standard and the safest choice.")
	fmt.Println()

	dummyLog, _ := newAuditLogger("models", []string{os.DevNull}, []string{os.DevNull})
	detectCfg := cfg
	if detectCfg.AgentCheckTimeout <= 0 {
		detectCfg.AgentCheckTimeout = 8
	}
	installed := detectAgents(ctx, detectCfg, dummyLog)

	persisted := readConfigFile()
	for _, name := range councilRosterAgents {
		entry := catalogEntry(name)
		if entry == nil {
			continue
		}
		_, isInstalled := installed[name]
		marker := dim("not installed")
		if isInstalled {
			marker = green("installed")
		}

		if entry.ModelEnv == "" {
			fmt.Printf("  %-12s %-14s model fixed by vendor (no override flag)\n", name, marker)
			continue
		}

		current := "vendor default"
		source := ""
		if v := os.Getenv(entry.ModelEnv); v != "" {
			current = v
			source = " (env)"
			if pv, ok := persisted[entry.ModelEnv]; ok && pv == v {
				source = " (config)"
			}
		}
		examples := ""
		if entry.FastModel != "" && entry.DeepModel != "" {
			examples = dim(fmt.Sprintf("   fast: %s · deep: %s", entry.FastModel, entry.DeepModel))
		}
		fmt.Printf("  %-12s %-14s %s%s%s\n", name, marker, cyan(current), source, examples)
	}

	fmt.Println()
	fmt.Println("Change a model:")
	fmt.Println("  persistent:  council config set COUNCIL_CLAUDE_MODEL haiku")
	fmt.Println("  one run:     COUNCIL_CLAUDE_MODEL=opus council \"task\"")
	fmt.Println("  reset:       council config unset COUNCIL_CLAUDE_MODEL")
	fmt.Println(dim("  model names are vendor-defined and change over time — check each vendor's docs for current ids"))
	return 0
}
