package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// handleLogin implements `council login [agent]`.
//
// Without an argument it audits every installed agent's auth state and prints
// a sign-in checklist. With an agent name it launches that CLI's interactive
// sign-in flow attached to the user's terminal, then re-probes to confirm.
func handleLogin(ctx context.Context, cfg Config, target string) int {
	dummyLog, _ := newAuditLogger("login", []string{os.DevNull}, []string{os.DevNull})
	detectCfg := cfg
	if detectCfg.AgentCheckTimeout <= 0 {
		detectCfg.AgentCheckTimeout = 8
	}
	installed := detectAgents(ctx, detectCfg, dummyLog)

	if target == "" {
		return loginChecklist(ctx, installed)
	}

	var match AgentName
	for name := range installed {
		if normalizeAgentKey(string(name)) == normalizeAgentKey(target) {
			match = name
			break
		}
	}
	if match == "" {
		fmt.Fprintf(os.Stderr, "❌ %q is not an installed agent. Installed: %s\n", target, strings.Join(installedNames(installed), ", "))
		fmt.Fprintln(os.Stderr, "   Install missing agents with: council setup --apply")
		return 2
	}

	entry := catalogEntry(match)
	resolved := installed[match]
	if entry == nil {
		return 2
	}

	switch {
	case len(entry.LoginArgs) > 0:
		fmt.Printf("🔑 Launching sign-in: %s %s\n\n", resolved.Path, strings.Join(entry.LoginArgs, " "))
		if err := runInteractive(resolved.Path, entry.LoginArgs); err != nil {
			fmt.Fprintf(os.Stderr, "\n⚠️  Sign-in command exited with an error: %v\n", err)
		}
	case entry.LoginBare:
		fmt.Printf("🔑 %s signs in on first interactive run. Launching `%s` — complete the flow, then exit.\n\n", match, entry.Executable)
		if err := runInteractive(resolved.Path, nil); err != nil {
			fmt.Fprintf(os.Stderr, "\n⚠️  %s exited with an error: %v\n", entry.Executable, err)
		}
	default:
		fmt.Printf("ℹ️  %s has no CLI sign-in command.\n   → %s\n", match, entry.LoginHint)
		return 0
	}

	status, method := authProbe(ctx, entry, resolved.Path)
	switch status {
	case "yes", "likely":
		fmt.Printf("\n✅ %s looks authenticated (%s). Verify for certain: council doctor --ping\n", match, method)
		return 0
	default:
		fmt.Printf("\n❔ Could not confirm %s auth yet. → %s\n   Verify: council doctor --ping\n", match, entry.LoginHint)
		return 1
	}
}

// loginChecklist prints per-agent auth state with the exact next step for
// every agent that still needs a sign-in.
func loginChecklist(ctx context.Context, installed AgentSet) int {
	if len(installed) == 0 {
		printNoAgentsHelp()
		return 1
	}

	fmt.Println(bold("🔑 Council Sign-in Checklist"))
	fmt.Println("----------------------------")
	needsLogin := 0
	for _, name := range councilRosterAgents {
		resolved, ok := installed[name]
		if !ok {
			continue
		}
		entry := catalogEntry(name)
		status, method := authProbe(ctx, entry, resolved.Path)
		switch status {
		case "yes":
			fmt.Printf("  %s %-12s authenticated (%s)\n", green("✅"), name, method)
		case "likely":
			fmt.Printf("  %s %-12s credentials found (%s) — assume signed in\n", green("🔑"), name, method)
		default:
			needsLogin++
			fmt.Printf("  %s %-12s sign-in needed → council login %s\n", yellow("🔒"), name, normalizeAgentKey(string(name)))
			if entry != nil {
				fmt.Printf("     %s\n", dim(entry.LoginHint))
			}
		}
	}

	if needsLogin == 0 {
		fmt.Println("\n✅ Every installed agent has credentials. Definitive check: council doctor --ping")
	} else {
		fmt.Printf("\n%d agent(s) need sign-in. Run the `council login <agent>` lines above one at a time.\n", needsLogin)
	}
	return 0
}

// runInteractive runs a CLI attached to the user's terminal (stdin/out/err),
// which is what OAuth device flows and TUI login prompts require.
func runInteractive(path string, args []string) error {
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installedNames(agents AgentSet) []string {
	var names []string
	for _, name := range councilRosterAgents {
		if _, ok := agents[name]; ok {
			names = append(names, normalizeAgentKey(string(name)))
		}
	}
	return names
}
