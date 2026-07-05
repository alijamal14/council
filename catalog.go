package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// AgentCatalogEntry is static metadata about a supported agent CLI: how to
// install it on each platform, how to tell whether it is authenticated, how to
// sign in, and where its usage limits are documented. It powers
// `council doctor --json`, `council setup`, `council login`, `council models`,
// and `council update`.
type AgentCatalogEntry struct {
	Name           AgentName
	Executable     string // primary binary name ("cursor-agent" has an alias handled in discovery)
	Vendor         string
	FreeTier       bool     // usable at no cost (free OAuth tier or open source w/ your own keys)
	InstallCmd     []string // argv Council may run for `setup --apply` (empty = script or manual install)
	PostInstall    []string // argv run after a successful InstallCmd (second-stage bootstrappers)
	InstallHint    string   // human-readable install instruction (always set)
	UnixInstall    string   // vendor install script pipeline for sh (macOS/Linux), "" if none
	WinInstall     string   // vendor install command for PowerShell (Windows), "" if none
	AuthStatusArgs []string // cheap CLI command that reports auth state (exit 0 = logged in), empty if none
	LoginArgs      []string // argv (after binary) that starts the CLI's sign-in flow
	LoginBare      bool     // true when running the bare CLI interactively IS the sign-in flow
	LoginHint      string   // how a user completes authentication
	CredFiles      []string // credential file/dir heuristics relative to $HOME (used when AuthStatusArgs is empty)
	CredEnvs       []string // env vars that indicate credentials are configured
	VersionArgs    []string // argv that prints the CLI version (nil = ["--version"])
	UpdateArgs     []string // argv for the CLI's own self-update command, if it has one
	LimitsURL      string   // where plan/rate limits are documented for this vendor CLI
	LimitsHint     string   // one-line guidance on checking remaining quota
	ModelEnv       string   // COUNCIL_<AGENT>_MODEL variable honored by Council ("" = unsupported)
	FastModel      string   // example lighter model for small tasks ("" = n/a); names go stale, treat as examples
	DeepModel      string   // example heavier model for complex tasks ("" = n/a)
}

// agentCatalog lists every agent Council knows how to drive.
// Install commands pin @latest so `setup --apply` and `update` always fetch the
// latest stable release through each vendor's own channel; the curl/irm script
// installers are the vendors' official ones and likewise resolve to latest.
// Auth heuristics are best-effort: a definitive answer requires `council doctor --ping`,
// which sends a real (tiny) prompt through each CLI.
var agentCatalog = []AgentCatalogEntry{
	{
		Name: AgentGemini, Executable: "gemini", Vendor: "Google", FreeTier: true,
		InstallCmd:  []string{"npm", "install", "-g", "@google/gemini-cli@latest"},
		InstallHint: "npm install -g @google/gemini-cli",
		LoginBare:   true,
		LoginHint:   "run `gemini` once and complete the OAuth flow (or set GEMINI_API_KEY)",
		CredFiles:   []string{".gemini/oauth_creds.json"},
		CredEnvs:    []string{"GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_APPLICATION_CREDENTIALS"},
		LimitsURL:   "https://github.com/google-gemini/gemini-cli",
		LimitsHint:  "free OAuth tier has daily request caps; API-key usage follows your Gemini API quota",
		ModelEnv:    "COUNCIL_GEMINI_MODEL",
		FastModel:   "gemini-2.5-flash", DeepModel: "gemini-2.5-pro",
	},
	{
		Name: AgentCodex, Executable: "codex", Vendor: "OpenAI",
		InstallCmd:     []string{"npm", "install", "-g", "@openai/codex@latest"},
		InstallHint:    "npm install -g @openai/codex",
		AuthStatusArgs: []string{"login", "status"},
		LoginArgs:      []string{"login"},
		LoginHint:      "run `codex login`",
		CredFiles:      []string{".codex/auth.json"},
		CredEnvs:       []string{"OPENAI_API_KEY"},
		LimitsURL:      "https://github.com/openai/codex",
		LimitsHint:     "usage draws on your ChatGPT plan or OpenAI API quota; `codex login status` shows the active account",
		ModelEnv:       "COUNCIL_CODEX_MODEL",
	},
	{
		Name: AgentClaude, Executable: "claude", Vendor: "Anthropic",
		InstallCmd:  []string{"npm", "install", "-g", "@anthropic-ai/claude-code@latest"},
		InstallHint: "npm install -g @anthropic-ai/claude-code",
		LoginBare:   true,
		LoginHint:   "run `claude` once and complete the login flow",
		CredFiles:   []string{".claude/.credentials.json", ".claude.json"},
		CredEnvs:    []string{"ANTHROPIC_API_KEY"},
		UpdateArgs:  []string{"update"},
		LimitsURL:   "https://docs.claude.com/en/docs/claude-code/costs",
		LimitsHint:  "subscription plans use rolling usage windows; run /status inside claude to see remaining capacity",
		ModelEnv:    "COUNCIL_CLAUDE_MODEL",
		FastModel:   "haiku", DeepModel: "opus",
	},
	{
		Name: AgentCopilot, Executable: "copilot", Vendor: "GitHub",
		InstallCmd:  []string{"npm", "install", "-g", "@github/copilot@latest"},
		InstallHint: "npm install -g @github/copilot",
		LoginBare:   true,
		LoginHint:   "run `copilot` once and authenticate with GitHub (or set GITHUB_TOKEN)",
		CredFiles:   []string{".config/github-copilot", ".copilot"},
		CredEnvs:    []string{"GITHUB_COPILOT_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"},
		LimitsURL:   "https://docs.github.com/en/copilot",
		LimitsHint:  "premium-request allowances depend on your Copilot plan",
		ModelEnv:    "COUNCIL_COPILOT_MODEL",
	},
	{
		Name: AgentCursor, Executable: "cursor-agent", Vendor: "Cursor",
		InstallHint:    "curl https://cursor.com/install -fsS | bash  (Windows: irm 'https://cursor.com/install?win32=true' | iex)",
		UnixInstall:    "curl https://cursor.com/install -fsS | bash",
		WinInstall:     "irm 'https://cursor.com/install?win32=true' | iex",
		AuthStatusArgs: []string{"status"},
		LoginArgs:      []string{"login"},
		LoginHint:      "run `cursor-agent login`",
		CredEnvs:       []string{"CURSOR_API_KEY"},
		UpdateArgs:     []string{"update"},
		LimitsURL:      "https://cursor.com/docs/cli/overview",
		LimitsHint:     "usage follows your Cursor plan; `cursor-agent status` shows the signed-in account",
		ModelEnv:       "COUNCIL_CURSOR_MODEL",
	},
	{
		Name: AgentAntigravity, Executable: "agy", Vendor: "Google",
		InstallHint: "install the Antigravity app from https://antigravity.google, then enable its CLI (`agy`)",
		LoginHint:   "sign in through the Antigravity app",
		LimitsURL:   "https://antigravity.google",
		LimitsHint:  "quota is managed by your Antigravity account",
		ModelEnv:    "COUNCIL_ANTIGRAVITY_MODEL",
	},
	{
		Name: AgentAider, Executable: "aider", Vendor: "Aider (open source)", FreeTier: true,
		InstallCmd: []string{"python", "-m", "pip", "install", "--upgrade", "aider-install"},
		// aider-install is a bootstrapper: the pip package installs a console
		// script that then installs the real aider via uv.
		PostInstall: []string{"aider-install"},
		InstallHint: "python -m pip install aider-install && aider-install",
		LoginHint:   "set a model API key such as ANTHROPIC_API_KEY, OPENAI_API_KEY, or GEMINI_API_KEY",
		CredFiles:   []string{".aider.conf.yml"},
		CredEnvs:    []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "OPENROUTER_API_KEY", "DEEPSEEK_API_KEY"},
		LimitsURL:   "https://aider.chat/docs/",
		LimitsHint:  "limits are those of whichever model API key you configure",
		ModelEnv:    "COUNCIL_AIDER_MODEL",
	},
	{
		Name: AgentOpenCode, Executable: "opencode", Vendor: "OpenCode (open source)", FreeTier: true,
		InstallCmd:     []string{"npm", "install", "-g", "opencode-ai@latest"},
		InstallHint:    "npm install -g opencode-ai",
		UnixInstall:    "curl -fsSL https://opencode.ai/install | bash",
		AuthStatusArgs: []string{"auth", "list"},
		LoginArgs:      []string{"auth", "login"},
		LoginHint:      "run `opencode auth login`",
		LimitsURL:      "https://opencode.ai/docs",
		LimitsHint:     "limits are those of the provider you connect (Anthropic, OpenAI, local models, ...)",
		ModelEnv:       "COUNCIL_OPENCODE_MODEL",
	},
	{
		Name: AgentQwen, Executable: "qwen", Vendor: "Alibaba (open source)", FreeTier: true,
		InstallCmd:  []string{"npm", "install", "-g", "@qwen-code/qwen-code@latest"},
		InstallHint: "npm install -g @qwen-code/qwen-code",
		LoginBare:   true,
		LoginHint:   "run `qwen` once and complete the OAuth flow (or set DASHSCOPE_API_KEY)",
		CredFiles:   []string{".qwen/oauth_creds.json"},
		CredEnvs:    []string{"DASHSCOPE_API_KEY", "OPENAI_API_KEY"},
		LimitsURL:   "https://github.com/QwenLM/qwen-code",
		LimitsHint:  "Qwen OAuth offers a generous free daily request quota; API-key usage follows your provider plan",
		ModelEnv:    "COUNCIL_QWEN_MODEL",
	},
	{
		Name: AgentGoose, Executable: "goose", Vendor: "Block / AAIF (open source)", FreeTier: true,
		InstallHint: "curl -fsSL https://github.com/aaif-goose/goose/releases/download/stable/download_cli.sh | bash",
		// CONFIGURE must reach the bash that runs the script (not curl),
		// otherwise the installer opens an interactive /dev/tty configurator.
		UnixInstall: "curl -fsSL https://github.com/aaif-goose/goose/releases/download/stable/download_cli.sh | CONFIGURE=false bash",
		LoginArgs:   []string{"configure"},
		LoginHint:   "run `goose configure` to set a provider and API key",
		CredFiles:   []string{".config/goose/config.yaml"},
		CredEnvs:    []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GOOGLE_API_KEY"},
		UpdateArgs:  []string{"update"},
		LimitsURL:   "https://block.github.io/goose/",
		LimitsHint:  "limits are those of the configured provider",
		ModelEnv:    "COUNCIL_GOOSE_MODEL",
	},
	{
		Name: AgentAmp, Executable: "amp", Vendor: "Sourcegraph",
		InstallCmd:  []string{"npm", "install", "-g", "@sourcegraph/amp@latest"},
		InstallHint: "npm install -g @sourcegraph/amp",
		LoginArgs:   []string{"login"},
		LoginHint:   "run `amp login` (or set AMP_API_KEY)",
		CredFiles:   []string{".config/amp"},
		CredEnvs:    []string{"AMP_API_KEY"},
		LimitsURL:   "https://ampcode.com/manual",
		LimitsHint:  "usage is billed to your Amp account",
		ModelEnv:    "", // Amp selects models itself; no --model flag
	},
	{
		Name: AgentDroid, Executable: "droid", Vendor: "Factory",
		InstallHint: "curl -fsSL https://app.factory.ai/cli | sh  (Windows: irm https://app.factory.ai/cli/windows | iex)",
		UnixInstall: "curl -fsSL https://app.factory.ai/cli | sh",
		WinInstall:  "irm https://app.factory.ai/cli/windows | iex",
		LoginHint:   "run `droid` once and use /login (or set FACTORY_API_KEY)",
		CredFiles:   []string{".factory"},
		CredEnvs:    []string{"FACTORY_API_KEY"},
		VersionArgs: []string{"-v"},
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
	FreeTier    bool   `json:"free_tier,omitempty"`
	Installed   bool   `json:"installed"`
	Path        string `json:"path,omitempty"`
	Version     string `json:"version,omitempty"`
	Quarantined bool   `json:"quarantined,omitempty"`
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
// this platform: a package manager when available, otherwise the vendor's
// official install script for this OS, or nil when only manual install works.
func installArgv(entry *AgentCatalogEntry) []string {
	if entry == nil {
		return nil
	}
	if len(entry.InstallCmd) > 0 {
		argv := append([]string(nil), entry.InstallCmd...)
		if runtime.GOOS == "windows" && argv[0] == "python" {
			argv[0] = "py" // python launcher is the more reliable default on Windows
		}
		return argv
	}
	if runtime.GOOS == "windows" {
		if entry.WinInstall != "" {
			return []string{"powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", entry.WinInstall}
		}
		// No native Windows installer, but Git Bash / MSYS2 can run the unix one.
		if entry.UnixInstall != "" {
			if bash, err := exec.LookPath("bash"); err == nil {
				return []string{bash, "-c", entry.UnixInstall}
			}
		}
		return nil
	}
	if entry.UnixInstall != "" {
		return []string{"sh", "-c", entry.UnixInstall}
	}
	return nil
}

// versionArgv returns the argv (after the binary) that prints the CLI version.
func versionArgv(entry *AgentCatalogEntry) []string {
	if entry != nil && len(entry.VersionArgs) > 0 {
		return entry.VersionArgs
	}
	return []string{"--version"}
}
