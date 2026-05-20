# 🏗️ AI Council: Build & Architecture

The AI Council is a native Go orchestrator designed for multi-agent consensus and automated project orchestration. The previous shell-based implementation (`council.sh`) has been deprecated and is now a simple wrapper for the Go binary.

## 🚀 Quick Start

To build and run the Council locally:

```bash
go build -o council .
./council "Your task description here"
```

## 🛠️ Architecture Overview

The orchestrator utilizes a **Bridge Pattern** to decouple the orchestration logic from the execution platform, enabling both local and future remote transports.

### Agent roster (six)

All are optional at runtime; unavailable CLIs are skipped after discovery/ping:

| Name | Executable(s) | Notes |
|------|----------------|-------|
| Gemini | `gemini` | |
| Codex | `codex` | `exec` subcommand |
| Claude | `claude` | |
| Copilot | `copilot` | Surrogate failover for Gemini/Claude/Codex on failure |
| Cursor | `cursor-agent`, then `agent` | Cursor Agent CLI ([docs](https://cursor.com/docs/cli/overview)) |
| Antigravity | `agy` | The 6th seat (High-Fidelity Agentic CLI) |

**Default invocation** is unrestricted/headless-capable argv for each CLI (`councilSpawnArgs` in `agent.go`); use only where the workspace trust model permits it.

### Key Components
*   **`council.go`**: Primary entry point. Handles CLI flag parsing, repository root resolution, and the high-level planning/critique session lifecycle.
*   **`agent.go`**: Core orchestration engine. Manages parallel execution of AI agent CLIs, process-group termination (preventing orphan processes), and cross-platform binary discovery.
*   **`domain.go`**: Context Routing engine. Uses native YAML parsing (`gopkg.in/yaml.v3`) to resolve project-specific domains from `_registry.template.yml` without external dependencies like `yq`.
*   **`run.go`**: Persistence and Audit layer. Manages session directory creation, structured logging (Markdown and JSONL), and Continue-Mode history aggregation.

### Path Resolution Hierarchy
The orchestrator resolves the repository root (to find manifests and logs) using the following priority:
1.  **CLI Flag**: `--repo <path>`
2.  **Environment Variable**: `COUNCIL_REPO_ROOT`
3.  **Git Root**: Automated walk-up from the current working directory.
4.  **Fallback**: `~/ai` (via native system home detection).

## 💻 Multi-Platform Support

The Council is designed to work seamlessly across macOS, Linux, and Windows.

*   **macOS/Linux**: Use `./council.sh` (wrapper) or the `./council` binary directly.
*   **Windows**: Use `council.bat` (CMD) or `council.ps1` (PowerShell).

All wrappers will automatically attempt to build the Go binary if it is missing.

## 🧪 Testing

Run the full test suite to verify cross-platform logic:

```bash
go test ./...
```

For integration testing, ensure the CLIs you expect are authenticated (`cursor-agent login`, Gemini OAuth, etc.) and on `PATH`; run `go run . doctor` to probe all six.
