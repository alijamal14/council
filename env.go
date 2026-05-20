package main

import (
	"os"
	"strings"
)

// agentSubprocessEnv returns a mutable copy of the current environment tuned for headless agent CLIs:
// avoids TERM=dumb / empty freezes (Gemini and others); suppresses credential prompts during tool use.
func agentSubprocessEnv() []string {
	env := append([]string(nil), os.Environ()...)

	set := func(key, val string) {
		prefix := key + "="
		for i, kv := range env {
			if strings.HasPrefix(kv, prefix) {
				env[i] = prefix + val
				return
			}
		}
		env = append(env, prefix+val)
	}

	term := ""
	for _, kv := range env {
		if strings.HasPrefix(kv, "TERM=") {
			term = kv[len("TERM="):]
			break
		}
	}
	if term == "" || term == "dumb" {
		set("TERM", "xterm-256color")
	}
	set("GIT_TERMINAL_PROMPT", "0")

	// Ensure auth credentials are forwarded so agents don't block on interactive auth flows.
	// These are pass-through only: if not set in the parent env, they remain absent.
	for _, key := range []string{
		"GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_APPLICATION_CREDENTIALS", "GOOGLE_CLOUD_PROJECT",
		"GITHUB_TOKEN",
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
	} {
		if val := os.Getenv(key); val != "" {
			set(key, val)
		}
	}

	return env
}
