# AI Council Orchestrator

[![Go Version](https://img.shields.io/github/go-mod/go-version/alijamal14/council)](https://golang.org/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

AI Council is a cross-platform Go CLI that runs multiple AI agent command-line tools in parallel, collects their plans, asks one agent to critique the result, and saves the full session as auditable output.

It is designed for developers who already use tools such as Claude, Gemini, Codex, Copilot, Cursor, or Antigravity and want a repeatable way to compare model perspectives on complex engineering tasks.

> **Council is not an AI model, hosted service, or credential manager.** It orchestrates the vendor CLIs you have already installed and authenticated.

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

Council can use the following CLIs when they are installed and authenticated. Unavailable agents are skipped during discovery — you do not need all of them installed.

| Agent | Executable |
|-------|------------|
| Gemini | `gemini` |
| Claude | `claude` |
| Codex | `codex` |
| Copilot | `copilot` |
| Cursor | `cursor-agent` or `agent` |
| Antigravity | `agy` |

---

## Requirements

- **Go 1.25+** — only if building from source.
- **One or more supported AI agent CLIs** installed on your system.
- **Authentication** completed through each vendor CLI before running Council.

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

### Go Install (if you already have Go 1.25+)

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

The `doctor` command checks all supported agent CLIs for availability and probes SSH connectivity if configured. Run this before your first real session, and after installing or updating any agent tool.

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
| `council doctor` | Check agent CLI discovery and connectivity. | |
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

All configuration is done through environment variables:

| Variable | Description |
|----------|-------------|
| `COUNCIL_REPO_ROOT` | Override repository root detection. |
| `COUNCIL_REMOTE_HOST` | Default host used by `council doctor` SSH probe. Does **not** set delegation — use `--remote` for that. |
| `COUNCIL_REMOTE_DIR` | Remote working directory during SSH delegation. Default: `.` |
| `COUNCIL_KNOWN_HOSTS` | Path to a custom `known_hosts` file. Default: `~/.ssh/known_hosts` |
| `COUNCIL_SSH_INSECURE` | Set to `1` to bypass SSH host key verification. |
| `COUNCIL_DOMAINS_DIR` | Path to custom domain context manifests. |
| `COUNCIL_<AGENT>_MODEL` | Pin a specific model for an agent (e.g., `COUNCIL_GEMINI_MODEL=gemini-2.0-pro`). |
| `COUNCIL_KEEP_RUNS` | Number of newest run directories to retain. Invalid values and values below `1` fall back to `200`. |
| `COUNCIL_NO_ROTATE` | Set to `1` to disable automatic pruning of old run directories. |

---

## Troubleshooting

**No agents found:**
Confirm the relevant CLI executables are installed and on your `PATH`. Run `council doctor` for a diagnostic.

**An agent is found but fails:**
Run that vendor CLI directly and confirm authentication is current (e.g., `claude --version`, then `claude login`).

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
