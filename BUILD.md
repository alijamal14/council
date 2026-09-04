# 🏗️ AI Council: Build & Architecture

The AI Council is a native Go orchestrator designed for multi-agent consensus and automated project orchestration. The previous shell-based implementation (`council.sh`) has been deprecated and is now a simple wrapper for the Go binary.

## 🚀 Quick Start

To build and run the Council locally:

```bash
cd tools/council
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
*   **`run.go`**: Persistence and Audit layer. Manages session directory creation, structured logging (Markdown and JSONL), Continue-Mode history aggregation, and run-directory retention.

### Run Directory Retention

Council writes each new session to `council_runs/run_YYYYMMDD_HHMMSS_*/`. These directories are audit artifacts, but they are not intended to accumulate forever.

Default behavior:
* Keep the newest **200** `council_runs/run_*` directories.
* Rotate after creating a new run, so the retained count includes the new run exactly.
* Delete oldest directories by filesystem modification time, with name as a deterministic tie-breaker.
* Keep global audit logs (`council_runs/council_audit.md` and `council_runs/council_audit.jsonl`) separately; run-directory rotation does not truncate those files.

Environment controls:
* `COUNCIL_KEEP_RUNS=<n>` — override the retained run-directory count. Invalid values and values below `1` fall back to `200`.
* `COUNCIL_NO_ROTATE=1` — disable run-directory deletion entirely.

### Path Resolution Hierarchy
The orchestrator resolves the repository root (to find manifests and logs) using the following priority:
1.  **CLI Flag**: `--repo <path>`
2.  **Environment Variable**: `COUNCIL_REPO_ROOT`
3.  **Git Root**: Automated walk-up from the current working directory.
4.  **Fallback**: `~/ai` (via native system home detection).

## 💻 Multi-Platform Support

The Council is designed to work seamlessly across macOS, Linux, and Windows.

*   **Linux repo default**: Use `./council-linux-amd64` directly when running from `tools/council/`.
*   **macOS/Linux from source**: Use `go build -o council .` and then `./council`.
*   **Compatibility fallback**: `./council.sh` is only a wrapper/autobuild fallback, not the default agent invocation.
*   **Windows**: Use `council.bat` (CMD) or `council.ps1` (PowerShell).

All wrappers will automatically attempt to build the Go binary if it is missing.

## 🔧 Agent maintenance lifecycle

Council now runs a maintenance pass before agent discovery/ping.

- Default mode is `--maintenance check` (or `COUNCIL_MAINTENANCE=check`), which reports what is installed and what would be updated.
- `--maintenance apply` runs configured update commands before the council run.
- `--maintenance off` skips the maintenance phase entirely.
- `--install-missing` (or `COUNCIL_INSTALL_MISSING=1`) allows trusted auto-installers where configured (currently includes Antigravity via `agy` installer).
- `COUNCIL_SKIP_AGENT_UPDATE=1` hard-skips maintenance regardless of flags.
- `COUNCIL_AGENT_UPDATE_TIMEOUT` controls per-agent maintenance command timeout (default: 180s).

Example:

```bash
./tools/council/council-linux-amd64 --maintenance apply --install-missing "Plan migration rollout"
```

## 🗂️ Runtime diagnostics output

Each run now writes `runtime_diagnostics.json` in the run directory (or iteration directory for `--continue`) with:

- Active/discovered agents and resolved binary paths
- Maintenance results and command outcomes
- Timeout/model configuration and invocation metadata

Audit entries also carry structured `details` in `council_audit.jsonl` and run-local `audit.jsonl`.

## 🧪 Testing

Run the full test suite to verify cross-platform logic:

```bash
go test ./...
```

For integration testing, ensure the CLIs you expect are authenticated (`cursor-agent login`, Gemini OAuth, etc.) and on `PATH`; run `go run . doctor` from `tools/council/` to probe all six.
