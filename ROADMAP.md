# Roadmap & Future Improvements

Ideas are grouped by theme. Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).
Concepts marked *(inspired by)* borrow proven designs from other open-source multi-agent projects.

## Consensus & Quality

- **Structured verdicts** — ask each critique to emit a machine-readable score block (risk, confidence, recommendation) in addition to prose, so runs can be compared quantitatively across sessions.
- **Synthesis phase** — an optional third phase where one agent merges the surviving plan and critiques into a single actionable plan *(inspired by ensemble/judge patterns in LLM-as-judge literature and tools like PR-Agent's reflect step)*.
- **Cross-examination rounds** — let agents answer the Devil's Advocate's objections before the session closes *(inspired by multi-agent debate: Du et al. 2023, ChatEval)*.
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

- **MCP server mode** — expose Council as a Model Context Protocol tool so any MCP-capable agent can convene a council natively.
- **GitHub Action** — `uses: alijamal14/council-action` to run a council on PR diffs and post the critique as a review comment.
- **Winget & apt packages** — extend beyond Homebrew/Scoop.

## Reliability

- **Streaming progress file** — a `status.json` in the run directory updated live (phase, per-agent state), so orchestrating AIs can poll cheaply.
- **Re-enable Copilot surrogate failover** once model aliases are stable across Copilot CLI versions (see QA notes in code).
