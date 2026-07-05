package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CouncilState is small cross-run bookkeeping persisted next to the config
// file. It drives the agent quarantine (auto-disable agents that keep failing)
// and the non-intrusive update reminders. All reads/writes are best-effort:
// a missing or corrupt state file never breaks a council run.
type CouncilState struct {
	// LastUpdateCheck is when `council update`/`update --check` last ran.
	LastUpdateCheck time.Time `json:"last_update_check"`
	// LastUpdateNudge is when the weekly reminder was last printed.
	LastUpdateNudge time.Time `json:"last_update_nudge"`
	// PingFailures counts consecutive failed pre-flight pings per agent.
	PingFailures map[string]int `json:"ping_failures,omitempty"`
	// Quarantined maps agent name -> RFC3339 time it was auto-disabled.
	Quarantined map[string]string `json:"quarantined,omitempty"`
}

// quarantineThreshold is how many consecutive failed pre-flight pings
// auto-disable an agent (an obsolete/broken CLI stops costing 45s per run).
const quarantineThreshold = 3

func stateFilePath() string {
	return filepath.Join(councilConfigDir(), "state.json")
}

func loadState() *CouncilState {
	st := &CouncilState{}
	data, err := os.ReadFile(stateFilePath())
	if err == nil {
		_ = json.Unmarshal(data, st)
	}
	if st.PingFailures == nil {
		st.PingFailures = map[string]int{}
	}
	if st.Quarantined == nil {
		st.Quarantined = map[string]string{}
	}
	return st
}

func (st *CouncilState) save() {
	if err := os.MkdirAll(councilConfigDir(), 0755); err != nil {
		return
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(stateFilePath(), data, 0644)
}

// recordPingResults updates failure counters after a pre-flight ping round.
// pinged is every agent that was pinged; healthy is the subset that passed.
// Returns the names newly quarantined this round (for user notification).
func (st *CouncilState) recordPingResults(pinged, healthy map[AgentName]bool) []string {
	var newlyQuarantined []string
	for agent := range pinged {
		key := string(agent)
		if healthy[agent] {
			delete(st.PingFailures, key)
			if _, wasQuarantined := st.Quarantined[key]; wasQuarantined {
				delete(st.Quarantined, key)
			}
			continue
		}
		st.PingFailures[key]++
		if st.PingFailures[key] >= quarantineThreshold {
			if _, already := st.Quarantined[key]; !already {
				st.Quarantined[key] = time.Now().Format(time.RFC3339)
				newlyQuarantined = append(newlyQuarantined, key)
			}
		}
	}
	return newlyQuarantined
}

// isQuarantined reports whether an agent is currently auto-disabled.
func (st *CouncilState) isQuarantined(agent AgentName) bool {
	_, ok := st.Quarantined[string(agent)]
	return ok
}

// clearQuarantine re-enables an agent (successful doctor --ping or update).
func (st *CouncilState) clearQuarantine(agent AgentName) {
	delete(st.Quarantined, string(agent))
	delete(st.PingFailures, string(agent))
}

// filterQuarantined removes quarantined agents from the set, unless the user
// explicitly allow-listed them (an explicit --agents choice always wins).
// Prints one notice line per skipped agent so the user knows why it's absent.
func filterQuarantined(agents AgentSet, st *CouncilState, explicitlyAllowed map[string]bool) AgentSet {
	for name := range agents {
		if !st.isQuarantined(name) {
			continue
		}
		if explicitlyAllowed[normalizeAgentKey(string(name))] {
			st.clearQuarantine(name) // user insisted — give it a fresh start
			continue
		}
		fmt.Printf("⏸️  %s is auto-disabled after %d failed sessions (broken or obsolete CLI).\n", name, quarantineThreshold)
		fmt.Printf("    Re-enable: council update, then council doctor --ping   (or force with --agents %s)\n", normalizeAgentKey(string(name)))
		delete(agents, name)
	}
	return agents
}

func normalizeAgentKey(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b = append(b, c)
	}
	return string(b)
}

// maybeNudgeUpdate prints (at most once a week) a one-line reminder to check
// for agent updates, so users stay current without being nagged every run.
func maybeNudgeUpdate(st *CouncilState) {
	const week = 7 * 24 * time.Hour
	if time.Since(st.LastUpdateCheck) < week || time.Since(st.LastUpdateNudge) < week {
		return
	}
	st.LastUpdateNudge = time.Now()
	st.save() // persist immediately — callers may not save again after this
	fmt.Println(dim("💡 Agent CLIs haven't been update-checked in over a week — run: council update"))
}
