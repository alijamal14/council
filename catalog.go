package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// AgentCatalogEntry is static metadata about a supported agent CLI: how to
// install it, how to tell whether it is authenticated, and where its usage
// limits are documented. It powers `council doctor --json` and `council setup`.
type AgentCatalogEntry struct {
	Name           AgentName
	Executable     string // primary binary name ("cursor-agent" has an alias handled in discovery)
	Vendor         string
	InstallCmd     []string // argv Council may run for `setup --apply` (empty = manual install only)
	InstallHint    string   // human-readable install instruction (always set)
	AuthStatusArgs []string // cheap CLI command that reports auth state (exit 0 = logged in), empty if none
	LoginHint      string   // how a user completes authentication
	CredFiles      []string // credential file/dir heuristics relative to $HOME (used when AuthStatusArgs is empty)
	CredEnvs       []string // env vars that indicate credentials are configured
	LimitsURL      string   // where plan/rate limits are documented for this vendor CLI
	LimitsHint     string   // one-line guidance on checking remaining quota
	ModelEnv       string   // COUNCIL_<AGENT>_MODEL variable honored by Council ("" = unsupported)
}

// agentCatalog lists every agent Council knows how to drive.
// Auth heuristics are best-effort: a definitive answer requires `council doctor --ping`,
// which sends a real (tiny) prompt through each CLI.
var agentCatalog = []AgentCatalogEntry{
	{
		Name: AgentGemini, Executable: "gemini", Vendor: "Google",
		InstallCmd:  []string{"npm", "install", "-g", "@google/gemini-cli"},
		InstallHint: "npm install -g @google/gemini-cli",
		LoginHint:   "run `gemini` once and complete the OAuth flow (or set GEMINI_API_KEY)",
		CredFiles:   []string{".gemini/oauth_creds.json"},
		CredEnvs:    []string{"GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_APPLICATION_CREDENTIALS"},
		LimitsURL:   "https://github.com/google-gemini/gemini-cli",
		LimitsHint:  "free OAuth tier has daily request caps; API-key usage follows your Gemini API quota",
		ModelEnv:    "COUNCIL_GEMINI_MODEL",
	},
	{
		Name: AgentCodex, Executable: "codex", Vendor: "OpenAI",
		InstallCmd:     []string{"npm", "install", "-g", "@openai/codex"},
		InstallHint:    "npm install -g @openai/codex",
		AuthStatusArgs: []string{"login", "status"},
		LoginHint:      "run `codex login`",
		CredFiles:      []string{".codex/auth.json"},
		CredEnvs:       []string{"OPENAI_API_KEY"},
		LimitsURL:      "https://github.com/openai/codex",
		LimitsHint:     "usage draws on your ChatGPT plan or OpenAI API quota; `codex login status` shows the active account",
		ModelEnv:       "COUNCIL_CODEX_MODEL",
	},
	{
		Name: AgentClaude, Executable: "claude", Vendor: "Anthropic",
		InstallCmd:  []string{"npm", "install", "-g", "@anthropic-ai/claude-code"},
		InstallHint: "npm install -g @anthropic-ai/claude-code",
		LoginHint:   "run `claude` once and complete the login flow",
		CredFiles:   []string{".claude/.credentials.json", ".claude.json"},
		CredEnvs:    []string{"ANTHROPIC_API_KEY"},
		LimitsURL:   "https://docs.claude.com/en/docs/claude-code/costs",
		LimitsHint:  "subscription plans use rolling usage windows; run /status inside claude to see remaining capacity",
		ModelEnv:    "COUNCIL_CLAUDE_MODEL",
	},
	{
		Name: AgentCopilot, Executable: "copilot", Vendor: "GitHub",
		InstallCmd:  []string{"npm", "install", "-g", "@github/copilot"},
		InstallHint: "npm install -g @github/copilot",
		LoginHint:   "run `copilot` once and authenticate with GitHub (or set GITHUB_TOKEN)",
		CredFiles:   []string{".config/github-copilot", ".copilot"},
		CredEnvs:    []string{"GITHUB_COPILOT_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"},
		LimitsURL:   "https://docs.github.com/en/copilot",
		LimitsHint:  "premium-request allowances depend on your Copilot plan",
		ModelEnv:    "COUNCIL_COPILOT_MODEL",
	},
	{
		Name: AgentCursor, Executable: "cursor-agent", Vendor: "Cursor",
		InstallHint:    "curl https://cursor.com/install -fsS | bash  (see https://cursor.com/cli)",
		AuthStatusArgs: []string{"status"},
		LoginHint:      "run `cursor-agent login`",
		CredEnvs:       []string{"CURSOR_API_KEY"},
		LimitsURL:      "https://cursor.com/docs/cli/overview",
		LimitsHint:     "usage follows your Cursor plan; `cursor-agent status` shows the signed-in account",
		ModelEnv:       "COUNCIL_CURSOR_MODEL",
	},
	{
		Name: AgentAntigravity, Executable: "agy", Vendor: "Google",
		InstallHint: "install the Antigravity app, then enable its CLI (`agy`)",
		LoginHint:   "sign in through the Antigravity app",
		LimitsURL:   "https://antigravity.google",
		LimitsHint:  "quota is managed by your Antigravity account",
		ModelEnv:    "COUNCIL_ANTIGRAVITY_MODEL",
	},
	{
		Name: AgentAider, Executable: "aider", Vendor: "Aider (open source)",
		InstallCmd:  []string{"python", "-m", "pip", "install", "aider-install"},
		InstallHint: "python -m pip install aider-install && aider-install",
		LoginHint:   "set a model API key such as ANTHROPIC_API_KEY, OPENAI_API_KEY, or GEMINI_API_KEY",
		CredFiles:   []string{".aider.conf.yml"},
		CredEnvs:    []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "OPENROUTER_API_KEY", "DEEPSEEK_API_KEY"},
		LimitsURL:   "https://aider.chat/docs/",
		LimitsHint:  "limits are those of whichever model API key you configure",
		ModelEnv:    "COUNCIL_AIDER_MODEL",
	},
	{
		Name: AgentOpenCode, Executable: "opencode", Vendor: "OpenCode (open source)",
		InstallCmd:     []string{"npm", "install", "-g", "opencode-ai"},
		InstallHint:    "npm install -g opencode-ai",
		AuthStatusArgs: []string{"auth", "list"},
		LoginHint:      "run `opencode auth login`",
		LimitsURL:      "https://opencode.ai/docs",
		LimitsHint:     "limits are those of the provider you connect (Anthropic, OpenAI, local models, ...)",
		ModelEnv:       "COUNCIL_OPENCODE_MODEL",
	},
	{
		Name: AgentQwen, Executable: "qwen", Vendor: "Alibaba (open source)",
		InstallCmd:  []string{"npm", "install", "-g", "@qwen-code/qwen-code"},
		InstallHint: "npm install -g @qwen-code/qwen-code",
		LoginHint:   "run `qwen` once and complete the OAuth flow (or set DASHSCOPE_API_KEY)",
		CredFiles:   []string{".qwen/oauth_creds.json"},
		CredEnvs:    []string{"DASHSCOPE_API_KEY", "OPENAI_API_KEY"},
		LimitsURL:   "https://github.com/QwenLM/qwen-code",
		LimitsHint:  "Qwen OAuth offers a generous free daily request quota; API-key usage follows your provider plan",
		ModelEnv:    "COUNCIL_QWEN_MODEL",
	},
	{
		Name: AgentGoose, Executable: "goose", Vendor: "Block (open source)",
		InstallHint: "see https://block.github.io/goose/docs/getting-started/installation",
		LoginHint:   "run `goose configure` to set a provider and API key",
		CredFiles:   []string{".config/goose/config.yaml"},
		CredEnvs:    []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GOOGLE_API_KEY"},
		LimitsURL:   "https://block.github.io/goose/",
		LimitsHint:  "limits are those of the configured provider",
		ModelEnv:    "COUNCIL_GOOSE_MODEL",
	},
	{
		Name: AgentAmp, Executable: "amp", Vendor: "Sourcegraph",
		InstallCmd:  []string{"npm", "install", "-g", "@sourcegraph/amp"},
		InstallHint: "npm install -g @sourcegraph/amp",
		LoginHint:   "run `amp login` (or set AMP_API_KEY)",
		CredFiles:   []string{".config/amp"},
		CredEnvs:    []string{"AMP_API_KEY"},
		LimitsURL:   "https://ampcode.com/manual",
		LimitsHint:  "usage is billed to your Amp account",
		ModelEnv:    "", // Amp selects models itself; no --model flag
	},
	{
		Name: AgentDroid, Executable: "droid", Vendor: "Factory",
		InstallHint: "curl -fsSL https://app.factory.ai/cli | sh  (see https://docs.factory.ai/cli)",
		LoginHint:   "run `droid` once and complete the login flow",
		CredFiles:   []string{".factory"},
		LimitsURL:   "https://docs.factory.ai/cli",
		LimitsHint:  "usage follows your Factory plan",
		ModelEnv:    "COUNCIL_DROID_MODEL",
	},
}

func catalogEntry(name AgentName) *AgentCatalogEntry {
	for i := range agentCatalog {
		if agentCatalog[i].Name == name {
			return &agentCatalog[i]
		}
	}
	return nil
}

// AgentStatus is the runtime status of one agent, as reported by doctor.
type AgentStatus struct {
	Name        string `json:"name"`
	Executable  string `json:"executable"`
	Vendor      string `json:"vendor,omitempty"`
	Installed   bool   `json:"installed"`
	Path        string `json:"path,omitempty"`
	Auth        string `json:"auth"`                  // "yes" | "no" | "likely" | "unknown"
	AuthMethod  string `json:"auth_method,omitempty"` // "status-cmd" | "credential-file" | "env" | "ping" | ""
	InstallHint string `json:"install_hint,omitempty"`
	LoginHint   string `json:"login_hint,omitempty"`
	LimitsURL   string `json:"limits_url,omitempty"`
	LimitsHint  string `json:"limits_hint,omitempty"`
	ModelEnv    string `json:"model_override_env,omitempty"`
}

// authProbe determines best-effort auth status for an installed agent without
// spending model tokens. Order: vendor status command, credential files, env vars.
func authProbe(ctx context.Context, entry *AgentCatalogEntry, path string) (status, method string) {
	if entry == nil {
		return "unknown", ""
	}

	if len(entry.AuthStatusArgs) > 0 {
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		runner := &LocalRunner{}
		stdout, stderr, err := runner.Run(probeCtx, path, entry.AuthStatusArgs, nil)
		if probeCtx.Err() == nil {
			if err == nil && len(stdout)+len(stderr) > 0 {
				return "yes", "status-cmd"
			}
			if err != nil {
				return "no", "status-cmd"
			}
		}
	}

	home, _ := os.UserHomeDir()
	for _, rel := range entry.CredFiles {
		if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(rel))); err == nil {
			return "likely", "credential-file"
		}
	}
	for _, env := range entry.CredEnvs {
		if os.Getenv(env) != "" {
			return "likely", "env"
		}
	}
	return "unknown", ""
}

// installArgv returns the argv Council can execute to install the agent on
// this platform, or nil when only a manual InstallHint is available.
func installArgv(entry *AgentCatalogEntry) []string {
	if entry == nil || len(entry.InstallCmd) == 0 {
		return nil
	}
	argv := append([]string(nil), entry.InstallCmd...)
	if runtime.GOOS == "windows" && argv[0] == "python" {
		argv[0] = "py" // python launcher is the more reliable default on Windows
	}
	return argv
}
