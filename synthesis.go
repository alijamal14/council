package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var finalRankingHeader = regexp.MustCompile(`(?i)final\s+ranking\s*:`)

// synthesisEnabled reports whether Phase 3 should run.
// Default is on when there are at least two valid plans; COUNCIL_SYNTHESIS=0 disables.
func synthesisEnabled(validPlans int) bool {
	if validPlans < 2 {
		return false
	}
	v := strings.TrimSpace(os.Getenv("COUNCIL_SYNTHESIS"))
	if v == "0" || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") {
		return false
	}
	return true
}

// parseFinalRanking extracts ordered plan labels after a FINAL RANKING: header.
// Accepts forms like "B > A > C", "B, A, C", "1. B 2. A", and lowercase letters.
func parseFinalRanking(text string) []string {
	loc := finalRankingHeader.FindStringIndex(text)
	if loc == nil {
		return nil
	}
	rest := text[loc[1]:]
	// Take the remainder of the line containing the ranking (and allow a short
	// continuation if the ranking spills to the next line with only labels/separators).
	lineEnd := strings.IndexAny(rest, "\r\n")
	line := rest
	if lineEnd >= 0 {
		line = rest[:lineEnd]
		restAfter := rest[lineEnd:]
		restAfter = strings.TrimLeft(restAfter, "\r\n")
		if nextEnd := strings.IndexAny(restAfter, "\r\n"); nextEnd >= 0 {
			restAfter = restAfter[:nextEnd]
		}
		if looksLikeRankingContinuation(restAfter) {
			line = line + " " + restAfter
		}
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// Normalize separators to spaces, then pull single-letter labels A–H.
	replacer := strings.NewReplacer(
		">", " ",
		",", " ",
		";", " ",
		"|", " ",
		"→", " ",
		"->", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		".", " ",
		":", " ",
	)
	normalized := replacer.Replace(line)
	fields := strings.Fields(normalized)

	seen := make(map[string]bool)
	var out []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		// Skip ordinal noise like "1" "2nd" "first"
		if isRankingNoise(f) {
			continue
		}
		label := strings.ToUpper(f)
		if len(label) != 1 || label[0] < 'A' || label[0] > 'H' {
			continue
		}
		if seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
	}
	return out
}

func looksLikeRankingContinuation(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// Continuation should be mostly labels/separators, not a new paragraph.
	if len(s) > 40 {
		return false
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "because") || strings.Contains(lower, "plan ") {
		return false
	}
	return parseFinalRanking("FINAL RANKING: "+s) != nil
}

func isRankingNoise(tok string) bool {
	lower := strings.ToLower(tok)
	switch lower {
	case "1", "2", "3", "4", "5", "6", "7", "8",
		"1st", "2nd", "3rd", "4th", "5th", "6th", "7th", "8th",
		"first", "second", "third", "fourth", "fifth",
		"best", "worst", "rank", "ranking", "order":
		return true
	}
	// Pure digits
	for _, r := range tok {
		if r < '0' || r > '9' {
			return false
		}
	}
	return tok != ""
}

// RankingAggregate is written to rankings.json for machine use.
type RankingAggregate struct {
	ByAgent map[string][]string `json:"by_agent"`
	Borda   []BordaScore        `json:"borda"`
	Winner  string              `json:"winner,omitempty"`
}

type BordaScore struct {
	Label string `json:"label"`
	Score int    `json:"score"`
}

// aggregateRankings parses FINAL RANKING blocks from critique files and
// computes a simple Borda count across reviewers (Devil's Advocate files
// without a ranking contribute nothing).
func aggregateRankings(dir string) (*RankingAggregate, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	agg := &RankingAggregate{ByAgent: make(map[string][]string)}
	nLabels := 0

	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "critique.") || !strings.HasSuffix(name, ".txt") {
			continue
		}
		path := filepath.Join(dir, name)
		if !isValidOutput(path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		agent := strings.TrimSuffix(strings.TrimPrefix(name, "critique."), ".txt")
		ranking := parseFinalRanking(string(data))
		if len(ranking) == 0 {
			continue
		}
		agg.ByAgent[agent] = ranking
		if len(ranking) > nLabels {
			nLabels = len(ranking)
		}
	}

	scores := make(map[string]int)
	for _, ranking := range agg.ByAgent {
		for i, label := range ranking {
			// Best rank gets the most points (Borda with shared slate size).
			scores[label] += nLabels - i
		}
	}

	for label, score := range scores {
		agg.Borda = append(agg.Borda, BordaScore{Label: label, Score: score})
	}
	sort.Slice(agg.Borda, func(i, j int) bool {
		if agg.Borda[i].Score != agg.Borda[j].Score {
			return agg.Borda[i].Score > agg.Borda[j].Score
		}
		return agg.Borda[i].Label < agg.Borda[j].Label
	})
	if len(agg.Borda) > 0 {
		agg.Winner = agg.Borda[0].Label
	}
	return agg, nil
}

func writeRankingsJSON(dir string, agg *RankingAggregate) error {
	if agg == nil {
		return nil
	}
	data, err := json.MarshalIndent(agg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "rankings.json"), data, 0644)
}

// chooseChairman picks the synthesis agent: COUNCIL_CHAIRMAN if present in the
// roster with a valid plan; otherwise the first sorted planner, preferring to
// exclude the Devil's Advocate when another candidate remains.
func chooseChairman(agents AgentSet, labelMap map[string]string, devil AgentName) (AgentName, bool) {
	planners := make(map[string]bool)
	for _, agent := range labelMap {
		planners[strings.ToLower(agent)] = true
	}

	if pref := strings.TrimSpace(os.Getenv("COUNCIL_CHAIRMAN")); pref != "" {
		prefLower := strings.ToLower(pref)
		for name := range agents {
			if strings.ToLower(string(name)) == prefLower && planners[prefLower] {
				return name, true
			}
		}
		// Prefer named agent even if they only critiqued (still in roster).
		for name := range agents {
			if strings.ToLower(string(name)) == prefLower {
				return name, true
			}
		}
	}

	sorted := sortedAgentNames(agents)
	var candidates []AgentName
	for _, name := range sorted {
		if planners[strings.ToLower(string(name))] {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) == 0 {
		// Fall back to any roster member.
		if len(sorted) == 0 {
			return "", false
		}
		return sorted[0], true
	}
	if devil != "" && len(candidates) > 1 {
		var without []AgentName
		for _, c := range candidates {
			if c != devil {
				without = append(without, c)
			}
		}
		if len(without) > 0 {
			return without[0], true
		}
	}
	return candidates[0], true
}

func buildCritiquePrompt(allPlans string) string {
	return fmt.Sprintf(
		"ROLE: Reviewer. PLANS:\n%s\n"+
			"GOAL: Briefly critique each labeled plan (strengths and weaknesses). "+
			"Then end with a mandatory block on its own line in exactly this form:\n"+
			"FINAL RANKING: B > A > C\n"+
			"(ordered best to worst; use plan letter labels only; include every plan label exactly once). "+
			"Do not reveal or guess which CLI authored which plan. TEXT-ONLY OUTPUT ONLY.",
		allPlans,
	)
}

func buildDevilPrompt(allPlans string) string {
	return fmt.Sprintf(
		"ROLE: Devil's Advocate. PLANS:\n%s\n"+
			"GOAL: Your job is NOT to agree with the consensus. Identify the strongest objections, "+
			"hidden assumptions, failure modes, and risks across ALL plans. Force-rank the risks by severity. "+
			"Do not recommend a winner and do not emit a FINAL RANKING of plan labels. TEXT-ONLY OUTPUT ONLY.",
		allPlans,
	)
}

func buildSynthesisPrompt(task, allPlans, critiques string) string {
	return fmt.Sprintf(
		"ROLE: Council Chairman / Synthesizer.\n"+
			"ORIGINAL TASK:\n%s\n\n"+
			"ANONYMIZED PLANS:\n%s\n"+
			"PEER CRITIQUES (may include FINAL RANKING blocks):\n%s\n"+
			"GOAL: Produce a single actionable consensus plan. Resolve disagreements, note residual risks, "+
			"and state a clear recommended path. Refer to plans only by their letter labels (A, B, C, …). "+
			"Do not invent which CLI wrote which plan. TEXT-ONLY OUTPUT ONLY.",
		task, allPlans, critiques,
	)
}

func collectCritiqueBodies(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "critique.") && strings.HasSuffix(name, ".txt") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		path := filepath.Join(dir, name)
		if !isValidOutput(path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		agent := strings.TrimSuffix(strings.TrimPrefix(name, "critique."), ".txt")
		b.WriteString(fmt.Sprintf("### Critique from %s:\n%s\n\n", agent, string(data)))
	}
	return b.String()
}
