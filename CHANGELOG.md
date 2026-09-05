# Changelog

All notable changes to Council are documented here.

## [Unreleased]

### What's new

- **Anonymous peer ranking** — Phase 2 critique prompts require a machine-parseable `FINAL RANKING: B > A > C` block using letter labels only (authors stay anonymized via `label_map.json`).
- **Chairman synthesis (Phase 3)** — after critiques, one agent writes `synthesis.txt` consensus. Enabled by default when ≥2 valid plans; disable with `COUNCIL_SYNTHESIS=0`. Pick the chair with `COUNCIL_CHAIRMAN=<agent>`.
- **`rankings.json`** — optional Borda aggregate of `FINAL RANKING` blocks across critique files.

## [1.5.0] — 2026-09-05

### What's new

- **Codex mid-run heal** — when Codex rejects a model because the CLI is too old (or ChatGPT auth rejects the model id), Council upgrades the CLI once (`npm` / `winget`), re-points at the newest binary (WinGet vs npm), and retries. Optional: `COUNCIL_CODEX_FALLBACK_MODEL`.
- **Antigravity (`agy`) unrestricted argv fix** — unrestricted mode now uses `--print=<prompt> --dangerously-skip-permissions` so the skip flag is no longer swallowed as the prompt.
- **Stricter version-mismatch validation** — trailing “requires a newer version…” / ChatGPT “not supported” API errors invalidate long banner outputs (no more false “valid plan”).
- **Public website** — https://ali.mk313.com/council/ with SEO/AEO (FAQ + SoftwareApplication schema, `llms.txt`, sitemap, robots) and a **live animated demo** of discover → plan → critique → audit.

### Docs

- PATH vs quarantine diagnostics (Windows WinGet Links shim).
- README links to the animated session demo.

## [1.4.0] — 2026-07-05

- Interactive REPL, piped-stdin tasks, humane terminal UX.
- Weekly nudge timestamp persistence.

## [1.3.0] — 2026-07-05

- Twelve-agent roster install (`setup --apply`), `council login`, `council config`, `council models`, `council update`, quarantine, timings.
