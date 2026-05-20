# 🚀 AI Council Orchestrator: Refactoring Progress

**Goal:** Transform the legacy bash-based Council orchestrator into a robust, cross-platform, native Go application with SSH delegation capabilities.

## 🟢 Phase 1: Logic Consolidation (COMPLETED)
- [x] Port `council.sh` core orchestration loop to Go.
- [x] Implement Native PathResolver (`Flag > Env > Git > Home`).
- [x] Create clean `Runner` interface abstraction.
- [x] Implement process-group aware `LocalRunner` (prevents zombie processes).
- [x] Implement robust cross-platform `DiscoverAgent` with 500ms version probes.
- [x] Decouple `yq` dependency: Native YAML parsing for domain resolution.
- [x] Enhance Continue-Mode: Aggregates previous critiques automatically.

## 🟢 Phase 2: Resiliency & Native SSH Delegation (COMPLETED)
- [x] **Copilot Failover:** Automatically use Copilot if primary agent (Gemini, Claude, Codex) fails pre-flight ping.
- [x] **SSHRunner Foundation:** Implement `SSHRunner` using `golang.org/x/crypto/ssh`.
- [x] **Smart Auth:** SSH-Agent support with automatic fallback to standard private keys.
- [x] **Remote Flag:** Wire `--remote` into the CLI configuration.
- [x] **Cross-Platform Robustness:** Refactored `LocalRunner` into `_windows.go` and `_unix.go`.
- [x] **Windows Process Trees:** Implemented Windows Job Objects (`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`) to prevent orphaned child processes.
- [x] **Validation:** Run End-to-End test of the remote delegation to a remote host (Handoff and streaming verified).

## 🟢 Phase 3: Open Source & Deployment UX (COMPLETED)
- [x] Implement `council install` for global cross-platform binary symlinking.
- [x] Implement `council doctor` to diagnose environment/SSH issues.
- [x] Add `--version` and `VERSION` constant.
- [x] Update build tags to modern `//go:build` syntax.
- [x] Finalize `AGENTS.md` documentation for AI-to-AI interaction protocols.
- [x] Update root `README.md` with universal setup instructions.

## 🟢 Phase 4: Hardening & Parity (v1.1.0 - COMPLETED)
- [x] **Model Pinning:** Support for `COUNCIL_<AGENT>_MODEL` for consistent high-end reasoning.
- [x] **Audit Isolation:** Per-session `audit.jsonl` to prevent cross-session context pollution.
- [x] **Roster Parity:** Integrated **Antigravity** (`agy`) as the 6th seat.
- [x] **Disk Management:** Automated rotation/cleanup of old `run_*/` directories (keep latest 50).
- [x] **Security Parity:** Synchronized permissive flags (`--yolo`, `--trust`) across all phases.

---
**Status:** ✅ **v1.1.1 MAINTENANCE RELEASE**

## 🔵 Phase 5: Maintenance & Standalone Migration (v1.1.1 - COMPLETED)
- [x] **Repository Decoupling:** Updated legacy paths (`tools/council`) to reflect standalone repository structure.
- [x] **Dependency Refresh:** Cleaned up `go.mod` and `go.sum` via `go mod tidy`.
- [x] **Documentation Alignment:** Updated `BUILD.md` and script wrappers (`council.sh`, `wait-council.sh`) with correct usage examples.
- [x] **Path Resolution Hardening:** Verified binary discovery across all six agents.
