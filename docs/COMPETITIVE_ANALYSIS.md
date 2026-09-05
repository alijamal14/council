# Competitive Analysis — AI Council vs peers

Living document. **Refresh after every Council release or material feature ship** so we do not miss competitor moves.

Last refreshed: **2026-09-05** (post Phase 3 ranking/synthesis + Windows prompt-file fix).  
Method: GitHub API (`gh api repos/…`, `releases/latest`, last 5 commits) + public READMEs.

## How to refresh (required after each update)

```bash
# From any machine with gh authenticated:
for r in karpathy/llm-council onevcat/argue MrLesk/agents-council am-will/llm-council \
         Sentry01/AgentCouncil aiwithremy/claude-skills-llm-council DmitryBMsk/llm-council-plus \
         tenfoldmarc/llm-council-skill machine-theory/lm-council; do
  echo "==== $r ===="
  gh api "repos/$r" --jq '{stars:.stargazers_count,pushed:.pushed_at,desc:.description}'
  gh api "repos/$r/releases/latest" --jq '{tag:.tag_name,published:.published_at}' 2>/dev/null || echo "(no releases)"
  gh api "repos/$r/commits?per_page=5" --jq '.[]|{sha:.sha[0:7],date:.commit.author.date,msg:.commit.message|split("\n")[0]}'
done
gh search repos "llm-council" --limit 10 --json fullName,stargazersCount,updatedAt,description
```

Then update the snapshot table below, the “What changed since last refresh” section, and ROADMAP gaps.

Also search: `agents-council`, `llm council skill`, `multi-agent debate CLI`.

---

## Snapshot (2026-09-05)

| Rank | Project | Stars | Latest release / tip | Primary form | Auth model | Deliberation |
|------|---------|------:|----------------------|--------------|------------|--------------|
| 1 | [karpathy/llm-council](https://github.com/karpathy/llm-council) | ~24.6k | no releases; tip `92e1fcc` 2025-11-22 | Local web (FastAPI + React) | **OpenRouter API key** | Stage1 opinions → Stage2 **anonymous peer rank** → Stage3 **Chairman** |
| 2 | [aiwithremy/claude-skills-llm-council](https://github.com/aiwithremy/claude-skills-llm-council) | ~1.8k | tip 2026-04-26 | Claude Code **skill** | Claude / skill harness | 5 advisors + peer review (skill UX) |
| 3 | [tenfoldmarc/llm-council-skill](https://github.com/tenfoldmarc/llm-council-skill) | ~740 | tip 2026-04-18 | Claude Code skill | Claude skill | Skill-packaged council |
| 4 | [gcpdev/llm-council-skill](https://github.com/gcpdev/llm-council-skill) | ~430 | (search hit) | Claude Code skill | Claude + other LLMs | Brainstorm before implementation plan |
| 5 | [machine-theory/lm-council](https://github.com/machine-theory/lm-council) | ~320 | tip 2025-07 (quiet) | Consensus “who is best” | varies | Meta-ranking of models |
| 6 | [onevcat/argue](https://github.com/onevcat/argue) | ~273 | **v0.6.1** 2026-05-31 | Harness-agnostic engine / CLI | Local providers / agent CLIs | Multi-round **claim lifecycle**, voting, early stop |
| 7 | [DmitryBMsk/llm-council-plus](https://github.com/DmitryBMsk/llm-council-plus) | ~127 | tip 2026-07-02 | Web + 3-stage + ranking parser | Multi-model / Ollama presets | Karpathy-style stages + free-LLM ranking list |
| 8 | [MrLesk/agents-council](https://github.com/MrLesk/agents-council) | ~66 | **v0.4.0** 2026-01-18; tip 2026-02-22 | **MCP** + Electrobun desktop | Local CLI credentials | Dynamic handoffs (`summon_agent`), hall UI |
| 9 | [sherifkozman/the-llm-council](https://github.com/sherifkozman/the-llm-council) | ~89 | (search hit) | Claude planning framework | Claude | Multi-LLM planning agents |
| 10 | [Sentry01/AgentCouncil](https://github.com/Sentry01/AgentCouncil) | ~16 | no releases; tip 2026-06-05 | Copilot CLI skill | Copilot CLI | Collaborative / adversarial routing + synthesis |
| 11 | [am-will/llm-council](https://github.com/am-will/llm-council) | ~15 | no releases; tip 2026-01-22 | Claude Code multi-agent + UI | Local CLIs (Claude/Codex/OpenCode) | Skill + UI; SSE-style session feel |
| — | **[alijamal14/council](https://github.com/alijamal14/council)** (us) | — | **v1.6.0** (this train) | **Headless CLI** orchestrator | **Reuses local CLI logins** (zero OpenRouter keys) | Plan → anonymized `FINAL RANKING` + DA → Chairman `synthesis.txt` |

---

## What changed since previous internal review (2026-09-05)

### Us (alijamal14/council)

- Closed the largest quality gap vs Karpathy / am-will: **anonymous peer ranking** + **chairman synthesis**.
- Artifacts: `label_map.json`, `FINAL RANKING:` in critiques, `rankings.json` (Borda), `synthesis.txt`.
- Env: `COUNCIL_SYNTHESIS`, `COUNCIL_CHAIRMAN`.
- Windows: **file-backed prompts** when argv would exceed CreateProcess limits (Codex was failing synthesis with “command line is too long”).

### Competitors (API snapshot)

| Project | Signal |
|---------|--------|
| **karpathy/llm-council** | Still the category reference (~24k★). No tagged releases; last commits Nov 2025 (progressive update, single-turn, label maker, vibe-code warning). **No new code since then** — but mindshare remains #1. |
| **argue v0.6.1** | OpenCode prompt via **stdin** (argv-length fix — same class of bug we hit). Viewer deploy pin. Still strongest **multi-round debate** story. |
| **MrLesk/agents-council** | Post-v0.4 work: Electrobun multi-platform packaging, canonical **Council Hall** UI, session chronicle. MCP distribution still ahead of us. |
| **am-will/llm-council** | Jan 2026: cross-platform setup, skip git-repo checks for CLIs, Claude CLI flag fix, UI polish. Small stars; idea density high (skill + UI). |
| **Sentry01/AgentCouncil** | Apr–Jun 2026: routing/synthesis streamline, adversarial diagram, hero image. Copilot-CLI niche. |
| **aiwithremy / tenfoldmarc / gcpdev skills** | High-star **Claude Code skill** packaging — distribution we lack. Mostly docs/README polish in 2026, not deep engine work. |
| **llm-council-plus** | Jul 2026: ranking parser fixes, free-LLM daily list presets, token stats — actively iterating on Karpathy’s 3-stage model. |

---

## Feature matrix (quality / UX / moat)

| Capability | Karpathy | argue | MrLesk | am-will / plus | Skills packs | **Us** |
|------------|:--------:|:-----:|:------:|:--------------:|:------------:|:----:|
| Anonymous peer ranking | ✅ | — | — | ✅ / ✅ | partial | ✅ (`FINAL RANKING` + `rankings.json`) |
| Chairman / synthesis artifact | ✅ | vote/converge | handoff | ✅ | varies | ✅ (`synthesis.txt`) |
| Devil’s Advocate | — | adversarial claims | — | — | — | ✅ |
| Zero API keys (reuse local CLIs) | ❌ OpenRouter | partial | ✅ | ✅ | Claude-centric | ✅ **moat** |
| Headless CI / agent-callable CLI | weak | ✅ | MCP | skill/UI | skill | ✅ **moat** |
| Multi-round claim lifecycle | — | ✅ | — | — | — | ❌ gap |
| MCP / in-editor ambient | — | — | ✅ | — | skill | ❌ gap (ROADMAP) |
| Live status / SSE | progressive UI | viewer | UI | UI | — | ❌ gap (`status.json`) |
| Intake interview | — | — | — | — | some | ❌ gap |
| Auditable run dirs | — | yes | yes | yes | weak | ✅ |

---

## Steal / don’t steal

**Worth adopting next (priority):**

1. **Live `status.json` (or SSE)** — councils take minutes; agents need cheap poll progress (am-will / ROADMAP).
2. **MCP server or first-class agent skill install** — MrLesk + high-star Claude skills win distribution.
3. **Multi-round / early-stop** — argue’s claim lifecycle beats single critique for hard disputes.
4. **Stricter ranking compliance** — retry or soft-fail critiques missing `FINAL RANKING:` (models still skip the block).

**Do not copy:**

- OpenRouter-only paths as default (destroys zero-key moat).
- Replacing CLI orchestration with a hosted SaaS.

---

## Process note for maintainers / agents

After **every** Council update that ships to `main` or a release tag:

1. Re-run the refresh commands above.
2. Append a dated subsection under “What changed since…”.
3. Adjust ROADMAP Delivered / open gaps.
4. Mirror this file into the private `ai` workspace (`tools/council/docs/COMPETITIVE_ANALYSIS.md`) after the public repo is updated.
5. Re-index RAG (`context-query index`) in the private workspace so agents retrieve the new matrix.
