# Contributing to AI Council

Thank you for your interest in contributing to the AI Council Orchestrator! This project aims to provide a robust harness for multi-agent collaboration.

## 🛠️ Development Environment

### Prerequisites
- **Go**: 1.25+
- **Agent CLIs**: (Optional for unit tests) Gemini, Claude, Codex, etc.

### Building
```bash
go build -o council .
```

### Running Tests
The test suite uses "Fake Agent" stubs on the `PATH` to verify discovery and orchestration logic without consuming real tokens.
```bash
go test ./...
```

---

## 🚦 Contribution Workflow

1. **Fork the Repository**: Create your own fork and branch from `main`.
2. **Implement Changes**:
    *   **New Agents**: Add a constant in `agent.go`, implement discovery logic in `discoverAgent()`, and define spawn/ping args in `councilSpawnArgs` and `councilPingArgs`.
    *   **Core Logic**: Ensure changes to `council.go` or `run.go` maintain backward compatibility for existing session formats.
3. **Add Tests**: All new features or agent integrations must include corresponding tests in `*_test.go`.
4. **Submit PR**: Provide a clear description of the problem solved and include any relevant logs from a `council doctor` check.

---

## 🛡️ Security Guidelines

- **No Hardcoded Secrets**: Never commit API keys, private hostnames, or local paths.
- **Restricted by Default**: All new agent flags should default to "Restricted Mode". Bypasses for approvals or sandboxing must be explicitly guarded by the `--unrestricted` flag.
- **Path Neutrality**: Do not assume fixed directory layouts. Use `resolveRepoRoot`, `COUNCIL_REPO_ROOT`, or the `--repo` flag to resolve relative paths.

---

## 📜 Coding Standards

- **Formatting**: Run `go fmt ./...` before committing.
- **Linting**: Ensure `go vet ./...` passes without warnings.
- **Documentation**: Update `README.md` and `PROGRESS.md` if your change introduces new CLI flags or features.

---

## ⚖️ License

By contributing, you agree that your contributions will be licensed under the **Apache License 2.0**.
