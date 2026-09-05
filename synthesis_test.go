package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFinalRanking(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "standard",
			in:   "Some critique.\n\nFINAL RANKING: B > A > C\n",
			want: []string{"B", "A", "C"},
		},
		{
			name: "lowercase header and labels",
			in:   "final ranking: c > a > b",
			want: []string{"C", "A", "B"},
		},
		{
			name: "comma separated",
			in:   "FINAL RANKING: A, C, B",
			want: []string{"A", "C", "B"},
		},
		{
			name: "missing header",
			in:   "I like plan B best, then A.",
			want: nil,
		},
		{
			name: "partial list still returns what was found",
			in:   "FINAL RANKING: B > A",
			want: []string{"B", "A"},
		},
		{
			name: "dedupe labels",
			in:   "FINAL RANKING: B > A > B > C",
			want: []string{"B", "A", "C"},
		},
		{
			name: "arrow unicode",
			in:   "FINAL RANKING: A → B → C",
			want: []string{"A", "B", "C"},
		},
		{
			name: "empty after header",
			in:   "FINAL RANKING:\n\nMore text without labels.",
			want: nil,
		},
		{
			name: "spaced header",
			in:   "Final Ranking : D > C > A > B",
			want: []string{"D", "C", "A", "B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFinalRanking(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSynthesisEnabled(t *testing.T) {
	t.Setenv("COUNCIL_SYNTHESIS", "")
	if !synthesisEnabled(2) {
		t.Fatal("expected synthesis on by default with 2 plans")
	}
	if synthesisEnabled(1) {
		t.Fatal("expected synthesis off with fewer than 2 plans")
	}
	t.Setenv("COUNCIL_SYNTHESIS", "0")
	if synthesisEnabled(3) {
		t.Fatal("expected synthesis off when COUNCIL_SYNTHESIS=0")
	}
	t.Setenv("COUNCIL_SYNTHESIS", "off")
	if synthesisEnabled(3) {
		t.Fatal("expected synthesis off when COUNCIL_SYNTHESIS=off")
	}
}

func TestAggregateRankingsBorda(t *testing.T) {
	dir := t.TempDir()
	// Agent1: B > A > C
	os.WriteFile(filepath.Join(dir, "critique.claude.txt"), []byte("notes\nFINAL RANKING: B > A > C\n"), 0644)
	// Agent2: A > B > C
	os.WriteFile(filepath.Join(dir, "critique.codex.txt"), []byte("notes\nFINAL RANKING: A > B > C\n"), 0644)
	// DA with no ranking — ignored
	os.WriteFile(filepath.Join(dir, "critique.gemini.txt"), []byte("Risks: ... no winner.\n"), 0644)

	agg, err := aggregateRankings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(agg.ByAgent) != 2 {
		t.Fatalf("by_agent=%v", agg.ByAgent)
	}
	// Borda with n=3: B gets 3+2=5, A gets 2+3=5, C gets 1+1=2 — tie A/B, winner A (label sort)
	if agg.Winner != "A" && agg.Winner != "B" {
		t.Fatalf("winner=%s scores=%v", agg.Winner, agg.Borda)
	}
	if agg.Borda[0].Score != agg.Borda[1].Score {
		t.Fatalf("expected A/B tie at top, got %v", agg.Borda)
	}
	if err := writeRankingsJSON(dir, agg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "rankings.json")); err != nil {
		t.Fatal(err)
	}
}

func TestChooseChairman(t *testing.T) {
	agents := AgentSet{
		AgentClaude: &ResolvedAgent{},
		AgentCodex:  &ResolvedAgent{},
		AgentGemini: &ResolvedAgent{},
	}
	labelMap := map[string]string{"A": "claude", "B": "codex", "C": "gemini"}

	t.Setenv("COUNCIL_CHAIRMAN", "")
	// Exclude devil gemini → first sorted planner is claude
	chair, ok := chooseChairman(agents, labelMap, AgentGemini)
	if !ok || chair != AgentClaude {
		t.Fatalf("got %s ok=%v, want claude", chair, ok)
	}

	t.Setenv("COUNCIL_CHAIRMAN", "codex")
	chair, ok = chooseChairman(agents, labelMap, AgentGemini)
	if !ok || chair != AgentCodex {
		t.Fatalf("got %s ok=%v, want codex from env", chair, ok)
	}
}

func TestBuildCritiquePromptRequiresRanking(t *testing.T) {
	p := buildCritiquePrompt("### Plan A:\nhello\n")
	if !contains(p, "FINAL RANKING:") {
		t.Fatal("critique prompt must require FINAL RANKING")
	}
	devil := buildDevilPrompt("### Plan A:\nhello\n")
	if !contains(devil, "Do not recommend a winner") {
		t.Fatal("devil prompt should forbid winners")
	}
}
