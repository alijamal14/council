# AI Council Orchestrator

[![CI](https://github.com/alijamal14/council/actions/workflows/ci.yml/badge.svg)](https://github.com/alijamal14/council/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/alijamal14/council)](https://github.com/alijamal14/council/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/alijamal14/council)](https://golang.org/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Website](https://img.shields.io/badge/website-ali.mk313.com%2Fcouncil-3ecf8e)](https://ali.mk313.com/council/)

**Site:** https://ali.mk313.com/council/

**Get a second (and third, and fourth) opinion from your AI coding tools — in one command.**

AI Council is a free, open-source, cross-platform CLI that runs multiple AI agent command-line tools — **Claude Code, Gemini CLI, OpenAI Codex, GitHub Copilot, Cursor, Antigravity, Aider, OpenCode, Qwen Code, Goose, Amp, and Factory Droid** — in parallel on the same task, collects their independent plans, runs a critique pass (including a Devil's Advocate), and saves the full multi-agent session as auditable output. Works on **macOS, Linux, and Windows**.

It is designed for developers — and for AI coding agents themselves — who want a repeatable way to compare model perspectives on complex engineering tasks.

> **Council is not an AI model, hosted service, or credential manager.** It orchestrates the vendor CLIs you have already installed and authenticated. No API keys are collected; nothing leaves your machine except through the vendor CLIs you already trust.

---

## Quick Start (60 seconds)

```bash
# 1. Install (macOS/Linux)
brew install alijamal14/tap/council
# ...or Windows (PowerShell)
#   scoop bucket add alijamal14 https://github.com/alijamal14/scoop-bucket
#   scoop install council

# 2. Install every supported AI CLI (latest stable, official channels)
council setup --apply          # or: council setup --apply --free  (free agents only)

# 3. Sign in to each one (guided)
council login

# 4. Convene your first council
council "Should we migrate this service from REST to gRPC? List risks."
```

Results land in `council_runs/run_<timestamp>/` as one plan file per agent plus critiques. You need **at least one** supported AI CLI installed and signed in — Council skips the rest automatically. `council doctor` shows what Council can see at any time.

---

## What Council Does

When you run:

```bash
council "Design a migration plan for the authentication service."
```

Council will:

1. Discover available agent CLIs on your `PATH`.
2. Run selected agents concurrently against the same task.
3. Collect each agent's response (the **Planning** phase).
4. Run a critique pass to identify risks, gaps, and disagreements (the **Critique** phase).
5. Write the full session output to a timestamped run directory for review and follow-up.

This makes Council useful for:

- Comparing architecture plans across multiple models.
- Reviewing risky implementation ideas before coding.
- Generating second opinions on debugging, refactoring, or migration work.
- Keeping a durable audit trail of AI-assisted planning sessions.

---

## Features

- **Multi-agent orchestration** — run available AI CLIs concurrently from one command.
- **Planning & Critique workflow** — collect independent plans, then challenge the result with a Devil's Advocate agent.
- **Selectable roster** — choose exactly which agents participate via `--agents`.
- **Continue mode** — resume a previous Council session with additional feedback.
- **Restricted by default** — agents run with safer arguments unless unrestricted mode is explicitly enabled.
- **Remote delegation** — run Council over SSH on a remote host when needed.
- **Structured artifacts** — Markdown and JSONL logs are saved for every session.
- **Cross-platform** — macOS, Linux, and Windows binaries are all supported.

---

## Supported Agents

Council can use the following 12 CLIs when they are installed and authenticated. Unavailable agents are skipped during discovery — you do not need all of them installed; **one is enough to start**.

| Agent | Executable | Vendor | Install |
|-------|------------|--------|---------|
| Claude Code | `claude` | Anthropic | 
pm i -g @anthropic-ai/claude-code` |
| Gemini CLI | `gemini` | Google | 
pm i -g @google/gemini-cli` |
| Codex | `codex` | OpenAI | 
pm i -g @openai/codex` |
| Copilot CLI | `copilot` | GitHub | 
pm i -g @github/copilot` |
| Cursor | `cursor-agent` or `agent` | Cursor | [cursor.com/cli](https://cursor.com/cli) |
| Antigravity | `agy` | Google | [antigravity.google](https://antigravity.google) |
| Aider | `aider` | open source | `pip install aider-install && aider-install` |
| OpenCode | `opencode` | open source | 
pm i -g opencode-ai` |
| Qwen Code | `qwen` | Alibaba (open source) | 
pm i -g @qwen-code/qwen-code` |
| Goose | `goose` | Block (open source) | [goose install docs](https://block.github.io/goose/docs/getting-started/installation) |
| Amp | `amp` | Sourcegraph | 
pm i -g @sourcegraph/amp` |
| Droid | `droid` | Factory | [docs.factory.ai/cli](https://docs.factory.ai/cli) |

Run `council setup` to see which are missing on your machine, or `council setup --apply` to install every agent that has a package-manager installer.

Notes on how Council drives the newer agents: Aider runs in **ask mode** (analysis only, no file edits), Droid runs at its default **read-focused autonomy** unless you pass `--unrestricted`, and Qwen Code uses the same headless surface as Gemini CLI (it is a fork).

---

## Requirements

- **One or more supported AI agent CLIs** installed on your system.
- **Authentication** completed through each vendor CLI before running Council.
- **Go 1.21+** — only if building from source (the required toolchain downloads automatically).

```bash
# Examples of vendor authentication:
claude login
gemini auth login
cursor-agent login
```

Council does not store or manage API keys. Authentication is the responsibility of each underlying CLI.

---

## Installation

### Homebrew (macOS / Linux) — Recommended

```bash
brew install alijamal14/tap/council
```

### Scoop (Windows) — Recommended

```powershell
scoop bucket add alijamal14 https://github.com/alijamal14/scoop-bucket
scoop install council
```

### Prebuilt Binary (all platforms)

**Linux / macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/alijamal14/council/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
powershell -c "irm https://raw.githubusercontent.com/alijamal14/council/main/scripts/install.ps1 | iex"
```

### Go Install (if you already have Go 1.21+)

```bash
go install github.com/alijamal14/council@latest
```

### Build From Source

```bash
git clone https://github.com/alijamal14/council.git
cd council
go build -o council .
```

Optionally install the binary to your `PATH`:
```bash
./council install
```

After any installation method, verify it worked:
```bash
council --version
```

---

## Verify Your Setup

```bash
council doctor
```

The `doctor` command checks all supported agent CLIs for availability **and authentication state**, and probes SSH connectivity if configured. Run this before your first real session, and after installing or updating any agent tool.

### Agent Inventory as Data (`doctor --json`)

Scripts, CI jobs, and AI agents can consume the inventory programmatically:

```bash
council doctor --json          # fast: install + best-effort auth detection
council doctor --json --ping   # definitive: sends one tiny prompt through each installed CLI
```

```json
{
  "council_version": "1.2.0",
  "platform": "linux/amd64",
  "totals": { "supported": 12, "installed": 4, "authenticated": 3, "login_required": 1, "unknown": 0 },
  "agents": [
    {
      "name": "Claude", "executable": "claude", "installed": true,
      "path": "/usr/local/bin/claude", "auth": "yes", "auth_method": "ping",
      "login_hint": "run `claude` once and complete the login flow",
      "limits_url": "https://docs.claude.com/en/docs/claude-code/costs",
      "limits_hint": "subscription plans use rolling usage windows; run /status inside claude to see remaining capacity",
      "model_override_env": "COUNCIL_CLAUDE_MODEL"
    }
  ]
}
```

- `auth` is `yes`, 
o`, `likely` (credentials found on disk/env), or `unknown`. Only `--ping` gives a definitive answer, because most vendor CLIs expose no offline auth query.
- `limits_url` / `limits_hint` tell you (or your orchestrating AI) where each vendor documents plan quotas — useful for deciding whether the council can afford another round.
- Exit code is `1` when zero agents are installed, so the command doubles as a CI gate.

---

## Interactive Mode

Run `council` with no arguments in a terminal and it opens a small REPL:

```
🏛  AI Council v1.4.0 — interactive mode
council> should we shard the orders table or move to a queue?
   ... agents plan and cross-critique ...
feedback> what about operational cost of each option?
   ... a --continue round in the same session ...
feedback>            (blank line finishes)
```

Piped stdin works too — the input becomes the task:

```bash
git diff | council            # council reviews your working-tree diff
echo "review api.go" | council
```

---

## Basic Usage

**Run Council on a task:**
```bash
council "Refactor the authentication middleware to support JWT and OAuth2."
```

**Run only specific agents:**
```bash
council --agents gemini,claude,codex "Review this database migration plan."
```

**Continue a previous session with feedback:**
```bash
council --continue council_runs/run_20260520_044511 "Revise the plan to include token revocation."
```

**Set a custom repository root:**
```bash
council --repo /path/to/project "Review the current architecture."
```

---

## Output

Council writes all session artifacts to a run directory.

**Default location:**
```
council_runs/run_YYYYMMDD_HHMMSS_000000/
```

**Fallback** (when no standard repository root is found):
```
.council/runs/run_YYYYMMDD_HHMMSS_000000/
```

Each run directory contains:
- Per-agent `plan.<agent>.txt` files from the planning phase.
- Per-agent `critique.<agent>.txt` files from the critique phase.
- A `brief.txt` summary of the task.
- Structured `audit.jsonl` logs for programmatic inspection.

Council keeps the newest **200** `run_*` directories by default. Rotation happens after a new run is created, so the retained count includes the latest run. Use `COUNCIL_KEEP_RUNS=<n>` to override the count, or `COUNCIL_NO_ROTATE=1` to disable run-directory deletion. Global audit files such as `council_audit.md` and `council_audit.jsonl` are not truncated by run-directory rotation.

---

## CLI Reference

| Command / Flag | Description | Default |
|----------------|-------------|---------|
| `council install` | Install the Council binary to your `PATH`. | |
| `council doctor` | Check agent CLI discovery, auth state, and connectivity. | |
| `council doctor --json` | Machine-readable agent inventory (installed / authenticated / login required / limits). | |
| `council doctor --ping` | Verify auth definitively by sending one tiny prompt per installed agent. | |
| `council setup` | List missing agent CLIs with install commands (free-tier agents are tagged). | |
| `council setup --apply` | Install every missing agent's latest stable release via its official channel (npm/pip/vendor script). | |
| `council setup --apply --free` | Install only the free-tier / open-source agents. | |
| `council login` | Sign-in checklist: shows which installed agents still need auth and how to fix each. | |
| `council login <agent>` | Launch that agent's interactive sign-in flow, then re-verify. | |
| `council models` | Show each agent's active model (override or vendor default) and how to change it. | |
| `council config set KEY VALUE` | Persist any `COUNCIL_*` setting (models, roster, hooks) — survives across runs. | |
| `council config list` / `get` / `unset` / `path` | Inspect or edit persisted settings. | |
| `council update` | Upgrade every installed agent CLI to latest stable; also checks for new Council releases. | |
| `council update --check` | Report installed agent versions without changing anything. | |
| `--version`, `-v` | Print version information. | |
| `--agents` | Comma-separated agents to run (`antigravity`, `gemini`, `claude`, `codex`, `copilot`, `cursor`). | All discovered |
| `--timeout` | Per-agent run timeout in seconds. | `300` |
| `--check-timeout` | Agent binary discovery timeout in seconds. | `8` |
| `--ping-timeout` | Pre-flight ping timeout per agent in seconds (minimum effective: 15s). | `45` |
| `--repo` | Override repository root detection. | Git root or CWD |
| `--continue <dir>` | Continue a previous Council run. All args after `<dir>` become the feedback text. | |
| `--remote <host>` | Delegate execution to a remote SSH host (e.g., `user@host:port`). | |
| `--unrestricted`, `--yolo`, `-y` | Enable unrestricted/headless agent execution. | `false` |
| `--verbose` | Enable verbose output. Pass `--verbose=false` to silence. | `true` |

For built-in help: `council -h`

---

## Restricted and Unrestricted Modes

Council runs in **restricted mode by default**. Agent CLIs are launched with safer default arguments where the vendor CLI supports it.

Use **unrestricted mode** only when you understand and trust the behavior of the underlying agents in the target workspace:

```bash
council --yolo "Fix all lint errors in this repository."
```

> **Warning:** Unrestricted mode may allow agents to perform broader automated file changes or commands depending on the vendor CLI. Review your agent configuration before using it in sensitive repositories.

---

## Remote Delegation

Council can delegate execution to a remote machine over SSH:

```bash
council --remote user@example.com "Analyze the production deployment plan."
```

Remote delegation uses **strict host key verification** by default. The host key must exist in your `known_hosts` file before connecting.

**SSH authentication order:**
1. SSH agent via `SSH_AUTH_SOCK`
2. `IdentityFile` entries from your SSH config (`ssh -G`)
3. Common default keys (`~/.ssh/id_ed25519`, `~/.ssh/id_rsa`)

**Custom remote directory:**
```bash
COUNCIL_REMOTE_DIR=/path/to/project council --remote user@example.com "Review this service."
```

To bypass host key verification (not recommended):
```bash
COUNCIL_SSH_INSECURE=1 council --remote user@example.com "..."
```

---

## Configuration

Every setting is an environment variable, and any of them can be **persisted** with
`council config set KEY VALUE` (stored in `<user config dir>/council/config.env`;
real environment variables always override the file):

```bash
council config set COUNCIL_CLAUDE_MODEL haiku    # pin a lighter Claude model permanently
council config set COUNCIL_AGENTS claude,codex   # default roster without typing --agents
council config set COUNCIL_AUTO_UPDATE 1         # background agent refresh after runs (max once/day)
council config list                              # see everything (env overrides are flagged)
```

| Variable | Description |
|----------|-------------|
| `COUNCIL_REPO_ROOT` | Override repository root detection. |
| `COUNCIL_REMOTE_HOST` | Default host used by `council doctor` SSH probe. Does **not** set delegation — use `--remote` for that. |
| `COUNCIL_REMOTE_DIR` | Remote working directory during SSH delegation. Default: `.` |
| `COUNCIL_KNOWN_HOSTS` | Path to a custom `known_hosts` file. Default: `~/.ssh/known_hosts` |
| `COUNCIL_SSH_INSECURE` | Set to `1` to bypass SSH host key verification. |
| `COUNCIL_DOMAINS_DIR` | Path to custom domain context manifests. |
| `COUNCIL_CONTEXT_CMD` | Optional RAG hook: a command run with the task appended as final argument; its stdout (up to 32 KB) is injected into every agent's context. Example: `COUNCIL_CONTEXT_CMD="graphify query"`. |
| `COUNCIL_<AGENT>_MODEL` | Pin a specific model for an agent (e.g., `COUNCIL_GEMINI_MODEL=gemini-2.0-pro`). Supported for all agents except Amp, which selects models itself. See `council models`. |
| `COUNCIL_AGENTS` | Default agent roster, comma-separated (same as `--agents`, but persistent). |
| `COUNCIL_AUTO_UPDATE` | Set to `1` to refresh agent CLIs in the background after successful runs (at most once per day, never blocking). |
| `COUNCIL_UNRESTRICTED` | Set to `1` to default to unrestricted mode (equivalent to passing `--yolo` on every run) — intended for dedicated always-on runners and CI. |
| `COUNCIL_KEEP_RUNS` | Number of newest run directories to retain. Invalid values and values below `1` fall back to `200`. |
| `COUNCIL_NO_ROTATE` | Set to `1` to disable automatic pruning of old run directories. |

---

## Models: defaults, overrides, and the advisor

Council **does not casually override** an agent's model — each vendor's default (or the user's `COUNCIL_<AGENT>_MODEL` / `~/.codex/config.toml`) is preferred. Three tools sit on top of that:

- **`council models`** — one table: every agent's active model (override or vendor default),
  example fast/deep model names, and the exact command to change each.
- **Easy switching** — persistent: `council config set COUNCIL_CLAUDE_MODEL haiku`;
  one run only: `COUNCIL_CLAUDE_MODEL=opus council "task"`.
- **Post-run advisor** — after each session Council prints at most a few dim `💡` lines when
  the pairing looks off: a heavy model (opus/pro-class) on a trivial task is flagged as token
  overkill with a cheaper suggestion, and a light model (haiku/flash-class) on a deep
  architecture task gets a nudge toward a stronger one. Advice only — behavior never changes.

**Exception — Codex mid-run heal:** if Codex fails because the configured model needs a newer
CLI (or ChatGPT auth rejects the model id), Council upgrades Codex once, re-points at the newest
binary (npm vs WinGet), and retries. Optional `COUNCIL_CODEX_FALLBACK_MODEL` forces a `--model`
override after a failed heal. See `context/CLI_COUNCIL_SOP.md` (Mid-run Codex CLI heal).

## Self-maintaining roster

- **`council update`** upgrades every installed agent CLI to its latest stable release through
  the same official channel it was installed from (npm `@latest`, pip `--upgrade`, the CLI's own
  self-updater, or the vendor's install script), and tells you when a new Council release exists.
- **Auto-update (opt-in):** `council config set COUNCIL_AUTO_UPDATE 1` refreshes agents in the
  background after successful runs — at most once a day, never blocking, log in the config dir.
- **Quarantine:** an agent that fails its pre-flight ping in **3 consecutive sessions** (broken
  install, obsolete binary, expired auth) is auto-disabled with a notice, so it stops costing a
  45-second timeout every run. It is re-enabled automatically the moment it passes a ping again
  (`council doctor --ping`), after a `council update`, or by naming it explicitly in `--agents`.
- **Antigravity argv:** unrestricted mode uses `--print=<prompt> --dangerously-skip-permissions`.
  Bare `--print` immediately before the skip flag makes `agy` treat the flag as the prompt.
- A gentle once-a-week reminder suggests `council update` if no check has happened; there is no
  other nagging.

---

## Troubleshooting

**No agents found / agent silently missing from a run:**
Confirm the CLI is on `PATH` (`where claude` / `which claude`). Run `council doctor` — `installed: false` means discovery failed, not quarantine. On Windows, WinGet may keep the package under `...\WinGet\Packages\` while deleting `...\WinGet\Links\<agent>.exe`; recreate that Links shim (or use the npm global shim) so LookPath succeeds.

**An agent is found but fails:**
Run that vendor CLI directly and confirm authentication is current (e.g., `claude --version`, then `claude login`).

**Codex: model requires a newer version of Codex:**
Your `~/.codex/config.toml` model is ahead of the CLI on `PATH` (common when WinGet trails npm). Newer Council builds self-heal mid-run (upgrade + pick newest binary). Manually: 
pm install -g @openai/codex@latest`, `winget upgrade OpenAI.Codex`, or ensure `%APPDATA%\npm` precedes the WinGet Links directory on `PATH`. Check `run_*/audit.jsonl` for `UPGRADE_OK`.

**Antigravity prints “--print took --dangerously-skip-permissions as its prompt”:**
Upgrade Council. Unrestricted spawn must bind the prompt as `--print=<prompt>`.

**Claude present on disk but absent from council roster:**
Not quarantine — PATH. Verify `council doctor --json` shows Claude `installed: true` and a `path`. If the binary only exists under WinGet Packages, hardlink/symlink it into WinGet Links (already on PATH) or install `@anthropic-ai/claude-code` via npm.

**Remote delegation fails:**
Verify SSH access manually first:
```bash
ssh user@example.com
```
Then check Council's connectivity:
```bash
council doctor
```

**Output is in an unexpected location:**
Use `--repo` or set `COUNCIL_REPO_ROOT` to explicitly set the repository root:
```bash
council --repo /path/to/project "Your task."
```

---

## Using Council from AI Coding Agents

Council is built to be invoked *by* AI agents, not just by humans. If you are an AI coding agent (Claude Code, Cursor, Copilot, Codex, Gemini CLI) reading this:

1. **Run Council in the background.** Council sessions take minutes and most agent shell tools buffer output until completion. Launch with background execution, then poll the newest `council_runs/run_*/` directory for `plan.*.txt` and `critique.*.txt` files.
2. **Read the artifacts, not stdout.** Every result is written to disk: per-agent plans, critiques, `label_map.json`, and `audit.jsonl`.
3. **Continue sessions with feedback** using `council --continue <run_dir> "feedback"`.

Council detects common agent callers (Claude Code, Codex, Copilot, Gemini) and prints an invocation guide to stderr automatically. A machine-readable project summary is available in [`llms.txt`](llms.txt).

Example instruction you can add to your agent's project config (e.g., `CLAUDE.md`, `AGENTS.md`, `.cursorrules`):

```markdown
When a task needs multi-model consensus, run `council "<question>"` in the
background, then poll `council_runs/` for plan.*.txt and critique.*.txt results.
```

---

## FAQ

### What is AI Council?

AI Council is an open-source command-line tool that orchestrates multiple AI coding assistants (Claude, Gemini, Codex, Copilot, Cursor, Antigravity, Aider, OpenCode, Qwen Code, Goose, Amp, Droid) in parallel on one task, then has them critique each other's plans. Think of it as a design-review meeting for AI models, with a written record.

### How do I check which AI agents are installed and logged in?

Run `council doctor` for a human-readable inventory or `council doctor --json` for machine-readable output with totals: how many agents are supported, installed, authenticated, and how many still need login. Add `--ping` for a definitive auth check (it sends one tiny prompt through each installed CLI). Each agent's entry also carries `limits_url` and `limits_hint` so you can check your remaining vendor quota before running a large council session.

### How do I install all the AI agents at once?

Run `council setup --apply`. It installs the latest stable release of every missing agent CLI
through its official channel (npm, pip, or the vendor's install script — Windows PowerShell
installers included). Use `--free` to install only free-tier/open-source agents (Gemini CLI,
Qwen Code, OpenCode, Aider, Goose). Then run `council login` for a guided sign-in checklist.

### How do I change which model each AI agent uses?

`council models` shows every agent's active model. Change one persistently with
`council config set COUNCIL_<AGENT>_MODEL <name>` or per run with an environment variable.
Council uses each vendor's default model unless you override it, and after each session it
advises (one dim line, never automatic) if a heavy model was overkill for a small task or a
light model was underpowered for a complex one.

### How does Council keep agent CLIs up to date?

`council update` upgrades every installed agent to its latest stable release and reports new
Council versions. Optionally set `COUNCIL_AUTO_UPDATE=1` to refresh agents in the background
after runs (max once/day). Agents whose CLI breaks or goes obsolete are auto-disabled after 3
consecutive failed pings — with a notice and a one-command re-enable — so they never silently
slow your sessions.

### Is AI Council free?

Yes. The orchestrator is free and Apache 2.0 licensed. You pay only whatever your existing AI CLI subscriptions or API plans cost — Council adds no fees and calls no APIs of its own.

### Do I need API keys to use Council?

No. Council never asks for or stores API keys. It shells out to vendor CLIs you have already installed and authenticated (e.g., via `claude login` or `gemini auth login`).

### Which operating systems does Council support?

macOS (Intel and Apple Silicon), Linux (x86_64 and ARM64), and Windows (x86_64 and ARM64). Prebuilt binaries are published for all of them on the [releases page](https://github.com/alijamal14/council/releases).

### How many AI CLIs do I need installed?

One is enough (Council runs a self-critique pass). Two or more unlocks the full experience: independent plans plus a Devil's Advocate critique that challenges the consensus.

### How is this different from just asking each AI tool myself?

Council asks all of them the *same* question *simultaneously*, anonymizes the plans (labeled A, B, C...) so the critique phase can't play favorites, assigns one model to argue against the consensus, and archives everything for audit. Doing that by hand takes 30+ minutes; Council does it in one command.

### Can I use Council in CI or scripts?

Yes. Council is a plain CLI with meaningful exit codes (0 = all agents succeeded, 1 = partial, 2 = failure) and writes machine-readable `audit.jsonl` logs. Use `--agents` to pin a deterministic roster and `--timeout` to bound runtime.

### Does Council send my code anywhere?

Council itself makes no network calls during a normal local session (SSH is used only if you opt into `--remote`). Your prompts and repository context reach model providers only through the vendor CLIs you invoke, subject to each vendor's own settings.

---

## Development

Build locally:
```bash
go build -o council .
```

Run the full test suite:
```bash
go test ./...
```

Formatting and vet checks (required before submitting a PR):
```bash
go fmt ./...
go vet ./...
```

The test suite uses fake agent stubs, so unit tests do not require live AI credentials.

---

## Documentation

| Document | Description |
|----------|-------------|
| [README.md](README.md) | This file — getting started, usage, CLI reference, configuration |
| [BUILD.md](BUILD.md) | Architecture, agent roster, release workflow, multi-platform support |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Contribution guide, dev environment, coding standards, security |
| [PROGRESS.md](PROGRESS.md) | Project history and completed milestones |

---

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

New agent integrations should include discovery logic, spawn and ping arguments, documentation updates, and tests.

---

## License

AI Council is distributed under the **Apache License 2.0**. See [LICENSE](LICENSE) for details.

---

*Built with ❤️ by [alijamal14](https://github.com/alijamal14).*
