# Roadmap & Future Improvements

Ideas are grouped by theme. Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).
Concepts marked *(inspired by)* borrow proven designs from other open-source multi-agent projects.

Refresh competitor gaps after every release — see [docs/COMPETITIVE_ANALYSIS.md](docs/COMPETITIVE_ANALYSIS.md).

## Near-term adoption plan (90 days)

Consensus from the 2026-09-05 competitive strategy council (`cursor` / `claude` / `codex` / `antigravity`): **v1.6.0 closed deliberation parity; adoption is the bottleneck.** Do not chase vanity stars — ship ambient invocation + reliability.

### Days 1–21 — Reliability & machine core

- **Exclude `*.prompt.txt` from plan labeling** — file-backed prompts must never appear in `label_map.json` / Phase 2 plan set (bug found in live ranking runs).
- **Live `status.json`** — phase + per-agent progress for agents that poll runs.
- **Safer defaults** — prefer read-only / sandboxed spawn unless `--unrestricted` / explicit write trust.
- **Ranking compliance** — one-shot reprompt when a non-DA critique omits `FINAL RANKING:`.
- **Clear exit-code contract** for programmatic callers.

### Days 22–56 — Ambient invocation & distribution

- **Official MCP server** (`convene_council`) for Cursor / Claude Desktop / Antigravity / peers.
- **Claude Code skill wrapper** (`/council`) that shells to the binary (match skill-pack distribution).
- **Local git hook / agent-precommit** path (honest CI story: self-hosted or local, not “zero-key on ephemeral GHA” without keys).
- **Doctor / setup polish** so Time-To-First-Run stays short.

### Days 57–90 — Evidence & presets

- **`council bench`** — small blind suite proving (or falsifying) decision lift vs single-model baseline.
- **`fast` preset** (2-agent peer review) vs **`deep`** (full roster + DA).
- **2–3 public case studies** + self-hosted runner kit notes.

## Consensus & Quality

- **Structured verdicts** — ask each critique to emit a machine-readable score block (risk, confidence, recommendation) in addition to prose, so runs can be compared quantitatively across sessions.
- **Cross-examination rounds** — let agents answer the Devil's Advocate's objections before the session closes *(inspired by multi-agent debate: Du et al. 2023, ChatEval)*.
- **Multi-round claim lifecycle / early stop** — optional argue-style rebuttal mode for tied or disputed rankings *(inspired by onevcat/argue)*.
- **Accuracy benchmarking harness** — a `council bench` command that replays a fixed task suite (with known-good rubrics) across roster subsets and reports per-agent latency, token cost proxy, and rubric scores over time.

## Roster & Providers

- **Config-defined custom agents** — let users declare any CLI in `~/.config/council/agents.yml` (name, binary, spawn args template, ping args) so new tools work without a Council release *(inspired by aider's model-settings YAML and OpenCode's provider config)*.
- **Local model support** — first-class Ollama/llama.cpp roster entries for fully offline councils.
- **Per-agent roles** — pin an agent to a persistent role (security reviewer, performance reviewer) via config, not just the rotating Devil's Advocate.

## Insight & Limits

- **Usage ledger** — aggregate per-agent invocation counts and durations from `audit.jsonl` into `council stats`, giving users a local view of how heavily each vendor CLI is being used.
- **Deeper quota introspection** — where vendors expose usage endpoints or status commands, surface remaining quota directly in `doctor --json` instead of `limits_hint` prose.
- **Cost estimation** — optional per-run token estimates (prompt sizes are known; response sizes are on disk).

## Context & RAG

- **Richer retrieval hooks** — multiple `COUNCIL_CONTEXT_CMD`-style hooks with per-hook byte budgets; built-in adapters for common tools (ripgrep, ctags, tree-sitter outlines) *(inspired by aider's repo-map and Claude Code's @-file mentions)*.
- **Artifact-aware continues** — `--continue` currently replays history text; teach it to re-run the retrieval hook against the feedback.

## Distribution & Integration

- **MCP server mode** — expose Council as a Model Context Protocol tool so any MCP-capable agent can convene a council natively. *(priority — Days 22–56)*
- **Agent skill packs** — first-class Claude Code / Cursor skill wrappers that invoke the local binary. *(priority — Days 22–56)*
- **GitHub Action** — `uses: alijamal14/council-action` for self-hosted / keyed runners; do not oversell zero-key ephemeral Actions.
- **Winget & apt packages** — extend beyond Homebrew/Scoop.

## Reliability

- **Streaming progress file** — a `status.json` in the run directory updated live (phase, per-agent state), so orchestrating AIs can poll cheaply. *(priority — Days 1–21)*
- **Prompt-file labeling hygiene** — never treat `*.prompt.txt` as plans/critiques in `label_map` / aggregation. *(priority — Days 1–21)*
- **Re-enable Copilot surrogate failover** once model aliases are stable across Copilot CLI versions (see QA notes in code).

## Delivered

- **v1.6.0** — anonymized Phase 2 `FINAL RANKING`, Phase 3 chairman `synthesis.txt`, `rankings.json`, `COUNCIL_SYNTHESIS` / `COUNCIL_CHAIRMAN`, Windows file-backed long prompts, living `docs/COMPETITIVE_ANALYSIS.md`.
- **v1.5.0** — Codex mid-run CLI heal (upgrade + newest-binary refresh), Antigravity `--print=<prompt>` unrestricted fix, version-mismatch validation, public site at https://ali.mk313.com/council/ (SEO/AEO + animated demo).
- **Post-v1.4 heal (workspace)** — Antigravity unrestricted argv uses `--print=<prompt>`; Codex mid-run CLI upgrade + newest-binary refresh when the configured model needs a newer Codex; trailing version-mismatch API errors invalidate long banner outputs.
- **v1.3.0** — one-command roster install for all 12 agents incl. vendor scripts & Windows (`setup --apply [--free]`), guided sign-in (`council login`), persistent settings (`council config`), model visibility + easy switching + post-run overkill/underkill advisor (`council models`), agent updater with self-release check (`council update`), opt-in daily background auto-update, failing-agent quarantine with auto re-enable, per-agent timings and ANSI-colored output.
- **v1.2.x** — 12-agent roster, `doctor --json` machine-readable inventory with auth + limits insight, `setup` installer, `COUNCIL_CONTEXT_CMD` RAG hook, validator fixes from live benchmarking.
