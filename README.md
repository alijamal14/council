# 🤖 AI Council Orchestrator

[![Go Version](https://img.shields.io/github/go-mod/go-version/alijamal14/council)](https://golang.org/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

**AI Council** is a high-performance, cross-platform Go orchestrator designed to run multiple AI agent CLIs in parallel. It facilitates a robust **Planning & Critique** workflow where different models (Gemini, Claude, Codex, Copilot, Cursor, etc.) work together to solve complex coding tasks, review each other's work, and provide a multi-perspective consensus.

---

## ✨ Key Features

- **🚀 Multi-Agent Concurrency**: Run up to 6 unique AI agents (or model variants) simultaneously to compare architectural plans.
- **🎭 Devil's Advocate Mode**: Automatically assigns one agent to critique the consensus, identifying hidden risks and edge cases.
- **⛓️ Native SSH Delegation**: Seamlessly hand off heavy computations or sensitive infrastructure tasks to a remote server (e.g., `remote-host`) while keeping the local environment clean.
- **🔄 Intelligent Failover**: Automatically surrogates failed agents through Copilot or other stable fallbacks to ensure session reliability.
- **🛡️ Execution Policy Control**: Toggle between **Restricted Mode** (default safety) and **Unrestricted Mode** (`--yolo`) for automated, headless workflows.
- **📂 Modular Context Routing**: Opt-in domain-specific context injection based on task keywords (supports custom manifests).
- **📝 Audit Logs**: Comprehensive session logging in both Markdown and JSONL formats for traceability and debugging.

---

## 📋 Prerequisites

Before using Council, ensure you have the following:

1. **Go**: Version 1.25 or higher (for building from source).
2. **Agent CLIs**: Install the AI agent CLIs you intend to use (e.g., `claude`, `gemini`, `cursor-agent`).
3. **Authentication**: **IMPORTANT**: You must be logged into each service through its respective vendor CLI. Council does not manage credentials; it assumes the underlying CLIs are already authenticated (e.g., via `claude login`, `gemini auth login`, or `cursor-agent login`).

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

---

## 🛠️ Configuration

Council can be customized via environment variables or a `.council` directory:

| Variable | Description |
|----------|-------------|
| `COUNCIL_REPO_ROOT` | Manual override for the project root. |
| `COUNCIL_REMOTE_HOST` | Default host for SSH delegation (default: none). |
| `COUNCIL_KNOWN_HOSTS` | Custom path to known_hosts file (default: `~/.ssh/known_hosts`). |
| `COUNCIL_SSH_INSECURE` | Set to `1` to bypass host key verification (not recommended). |
| `COUNCIL_DOMAINS_DIR` | Path to your custom domain manifests. |
| `COUNCIL_<AGENT>_MODEL` | Pin specific models (e.g., `COUNCIL_GEMINI_MODEL=gemini-2.0-pro-exp-02-05`). |

---

## 🧪 Testing & CI/CD

Council includes a robust testing suite that uses "Fake Agent" stubs to verify orchestration logic without requiring real AI API credits.

```bash
# Run all unit tests
go test ./...
```

---

## 📄 License

Distributed under the **Apache License 2.0**. See `LICENSE` for more information.

---

## 🤝 Contributing

Contributions are welcome! Please read `CONTRIBUTING.md` (coming soon) for details on our code of conduct and the process for submitting pull requests.

---

*Built with ❤️ by the [alijamal14](https://github.com/alijamal14) team.*
