# 🤖 AI Council Orchestrator

[![Go Version](https://img.shields.io/github/go-mod/go-version/alijamal14/council)](https://golang.org/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

**AI Council** is a high-performance, cross-platform Go orchestrator designed to run multiple AI agent CLIs in parallel. It facilitates a robust **Planning & Critique** workflow where different models (Gemini, Claude, Codex, Copilot, Cursor, etc.) work together to solve complex coding tasks, review each other's work, and provide a multi-perspective consensus.

---

## ✨ Key Features

- **🚀 Multi-Agent Concurrency**: Run up to 6 unique AI agents (or model variants) simultaneously to compare architectural plans.
- **🎭 Devil's Advocate Mode**: Automatically assigns one agent to critique the consensus, identifying hidden risks and edge cases.
- **⛓️ Native SSH Delegation**: Seamlessly hand off heavy computations or sensitive infrastructure tasks to a remote server (e.g., `remote-host`) while keeping the local environment clean.
- **🛡️ Execution Policy Control**: Toggle between **Restricted Mode** (default safety) and **Unrestricted Mode** (`--yolo`) for automated, headless workflows.
- **📂 Modular Context Routing**: Opt-in domain-specific context injection based on task keywords (supports custom manifests).
- **📝 Audit Logs**: Comprehensive session logging in both Markdown and JSONL formats for traceability and debugging.

---

## 📋 Prerequisites

Before using Council, ensure you have the following:

1. **Go**: Version 1.25 or higher (for building from source).
2. **Agent CLIs**: Install the AI agent CLIs you intend to use (e.g., `claude`, `gemini`, `cursor-agent`).
3. **Authentication**: **IMPORTANT**: You must be logged into each service through its respective vendor CLI. Council does not manage credentials; it assumes the underlying CLIs are already authenticated (e.g., via `claude login`, `gemini auth login`, or `cursor-agent login`).

To quickly verify your installation and agent authentication, run:
```bash
council doctor
```

## 🚀 Quick Start

### 🚀 Quick Install

Install the pre-built binary for your platform with a single command:

**Linux / macOS**:
```bash
curl -fsSL https://raw.githubusercontent.com/alijamal14/council/main/scripts/install.sh | bash
```

**Windows (PowerShell)**:
```powershell
powershell -c "irm https://raw.githubusercontent.com/alijamal14/council/main/scripts/install.ps1 | iex"
```

---

### Manual Installation (from source)

If you prefer to build from source:

1. **Clone the repository**:
```bash
git clone https://github.com/alijamal14/council.git
cd council

# Build and install to your PATH (~/.local/bin, /usr/local/bin, etc.)
go build -o council .
```

### Basic Usage

```bash
# Convene the council for a coding task
council "Refactor the authentication middleware to support JWT and OAuth2 simultaneously."

# Continue a previous session with feedback
council --continue council_runs/run_20260520_044511 "Great plan, but ensure we handle token revocation."
```

### Advanced Features

- **Remote Delegation**:
  ```bash
  council --remote user@remote-host "Deploy the new microservice architecture."
  ```
- **Selective Roster**:
  ```bash
  council --agents gemini,claude,cursor "Generate a unit test suite for the payment gateway."
  ```
- **Unrestricted Automation**:
  ```bash
  council --yolo "Fix all lint errors in the current directory."
  ```

> **Note on Artifacts**: Output is stored in `council_runs/`. If you are outside a standard repository, results will be stored in `.council/runs/` as a fallback.

---

## 📖 CLI Reference

| Command / Flag | Description | Default |
|----------------|-------------|---------|
| `council install` | Installs the binary to your PATH. Handled before flag parsing. | |
| `council doctor` | Probes all agent CLIs and SSH connectivity. Handled before flag parsing. | |
| `council --version` / `-v` | Prints version info (matches git tag if installed via quick install). | |
| `--timeout` | Per-agent run timeout (plan/critique) in seconds. | `300` |
| `--check-timeout` | Agent binary discovery probe timeout in seconds. | `8` |
| `--ping-timeout` | Pre-flight ping timeout per agent in seconds (minimum effective 15s). | `45` |
| `--verbose` | Toggles verbose output. Pass `--verbose=false` to silence. | `true` |
| `--agents` | Comma-separated list of agents to run (`antigravity`, `gemini`, `claude`, `codex`, `copilot`, `cursor`). | All found |
| `--repo` | Repository root override. | Git root |
| `--unrestricted` / `--yolo` / `-y` | Run agents in unrestricted/headless mode. | `false` |
| `--remote` | Remote host for SSH delegation (e.g., `user@host:port`). Delegation uses strict host key verification. | |
| `--continue <dir>` | Continues a session. All subsequent arguments become the feedback text. | |

*Tip for first-time users: Run `council -h` for standard flag help.*

---

## 🔒 SSH Delegation & Security

When using `--remote` to delegate tasks to a remote server, Council adheres to strict SSH security:
1. **First Connection**: The remote host's key **must** exist in your `known_hosts` file (or you must explicitly bypass via `COUNCIL_SSH_INSECURE=1`).
2. **Authentication Order**: The orchestrator checks your SSH agent (`SSH_AUTH_SOCK`) first, followed by specific `IdentityFile` overrides from `ssh -G`, and finally defaults like `~/.ssh/id_ed25519` and `~/.ssh/id_rsa`.

---

## 🛠️ Configuration

Council can be customized via environment variables or a `.council` directory:

| Variable | Description |
|----------|-------------|
| `COUNCIL_REPO_ROOT` | Manual override for the project root. |
| `COUNCIL_REMOTE_HOST` | Default host for `council doctor` SSH probe (BatchMode echo); **does not** set delegation — use `--remote` for that. |
| `COUNCIL_REMOTE_DIR` | Remote directory to `cd` into before running council via delegation. (Default: `.`) |
| `COUNCIL_KNOWN_HOSTS` | Custom path to known_hosts file (default: `~/.ssh/known_hosts`). |
| `COUNCIL_SSH_INSECURE` | Set to `1` to bypass host key verification (not recommended). |
| `COUNCIL_DOMAINS_DIR` | Path to your custom domain manifests. |
| `COUNCIL_<AGENT>_MODEL` | Pin specific models (e.g., `COUNCIL_ANTIGRAVITY_MODEL=agy-pro`, `COUNCIL_GEMINI_MODEL=...`). |
| `COUNCIL_NO_ROTATE` | If not `1`, prunes old run directories (keeps the latest 50). |

---

## 🧪 Testing & CI/CD

Council includes a robust testing suite that uses "Fake Agent" stubs to verify orchestration logic without requiring real AI API credits.

```bash
# Run all unit tests
go test ./...
```

---

## 📚 Documentation

| Document | Description |
|----------|-------------|
| [README.md](README.md) | This file — Quick Start, CLI Reference, Configuration |
| [BUILD.md](BUILD.md) | Architecture deep-dive, agent roster, release workflow, multi-platform support |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute, dev environment, coding standards, security guidelines |
| [PROGRESS.md](PROGRESS.md) | Refactoring history and completed milestones |

---

## 📄 License

Distributed under the **Apache License 2.0**. See [LICENSE](LICENSE) for more information.

---

## 🤝 Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

---

*Built with ❤️ by the [alijamal14](https://github.com/alijamal14) team.*
