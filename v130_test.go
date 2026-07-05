package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- config file ---

func TestConfigFileRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(userConfigEnvVar(), tmp)

	if got := readConfigFile(); len(got) != 0 {
		t.Fatalf("expected empty config, got %v", got)
	}

	want := map[string]string{
		"COUNCIL_CLAUDE_MODEL": "haiku",
		"COUNCIL_AGENTS":       "claude,codex",
	}
	if err := writeConfigFile(want); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	got := readConfigFile()
	for k, v := range want {
		if got[k] != v {
			t.Errorf("key %s: got %q want %q", k, got[k], v)
		}
	}
}

func TestApplyPersistentConfigRespectsRealEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(userConfigEnvVar(), tmp)

	writeConfigFile(map[string]string{
		"COUNCIL_QWEN_MODEL": "from-config",
		"NOT_COUNCIL_KEY":    "must-not-leak",
	})
	t.Setenv("COUNCIL_QWEN_MODEL", "from-env")
	os.Unsetenv("NOT_COUNCIL_KEY")

	applyPersistentConfig()

	if got := os.Getenv("COUNCIL_QWEN_MODEL"); got != "from-env" {
		t.Errorf("real env must win: got %q", got)
	}
	if _, set := os.LookupEnv("NOT_COUNCIL_KEY"); set {
		t.Error("non-COUNCIL_ keys must never be injected")
	}
}

// userConfigEnvVar returns the env var os.UserConfigDir consults per OS, so
// tests can sandbox the council config dir.
func userConfigEnvVar() string {
	switch runtime.GOOS {
	case "windows":
		return "AppData"
	case "darwin":
		return "HOME" // UserConfigDir = $HOME/Library/Application Support
	default:
		return "XDG_CONFIG_HOME"
	}
}

// --- model advisor ---

func TestClassifyTask(t *testing.T) {
	cases := []struct {
		task string
		want taskWeight
	}{
		{"fix typo in README", taskLight},
		{"what is a goroutine", taskLight},
		{"design a system for multi-region failover with consensus and disaster recovery planning across services", taskHeavy},
		{"please review the trade-offs between kafka and rabbitmq for our event pipeline architecture and make a recommendation", taskHeavy},
	}
	for _, c := range cases {
		if got := classifyTask(c.task); got != c.want {
			t.Errorf("classifyTask(%q) = %v, want %v", c.task, got, c.want)
		}
	}
}

func TestClassifyModel(t *testing.T) {
	if classifyModel("claude-opus-4") != taskHeavy {
		t.Error("opus should classify heavy")
	}
	if classifyModel("gemini-2.5-flash") != taskLight {
		t.Error("flash should classify light")
	}
	if classifyModel("gpt-5.5-codex") != taskStandard {
		t.Error("unknown model should classify standard")
	}
}

// --- quarantine state ---

func TestQuarantineLifecycle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(userConfigEnvVar(), tmp)

	st := loadState()
	pinged := map[AgentName]bool{AgentQwen: true}
	unhealthy := map[AgentName]bool{}

	// Two failures: not yet quarantined.
	st.recordPingResults(pinged, unhealthy)
	if newly := st.recordPingResults(pinged, unhealthy); len(newly) != 0 {
		t.Fatalf("quarantined too early: %v", newly)
	}
	// Third consecutive failure trips the threshold.
	newly := st.recordPingResults(pinged, unhealthy)
	if len(newly) != 1 || newly[0] != string(AgentQwen) {
		t.Fatalf("expected qwen quarantined on 3rd failure, got %v", newly)
	}
	if !st.isQuarantined(AgentQwen) {
		t.Fatal("isQuarantined should report true")
	}

	// Persistence across load.
	st.save()
	st2 := loadState()
	if !st2.isQuarantined(AgentQwen) {
		t.Fatal("quarantine must persist to disk")
	}

	// A healthy ping clears both the counter and the quarantine.
	st2.recordPingResults(pinged, map[AgentName]bool{AgentQwen: true})
	if st2.isQuarantined(AgentQwen) || st2.PingFailures[string(AgentQwen)] != 0 {
		t.Fatal("healthy ping must clear quarantine and failures")
	}
}

func TestFilterQuarantinedRespectsExplicitAllow(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(userConfigEnvVar(), tmp)

	st := loadState()
	st.Quarantined[string(AgentAmp)] = "2026-01-01T00:00:00Z"

	agents := AgentSet{AgentAmp: &ResolvedAgent{Name: AgentAmp, Path: "amp"}}
	// Explicit --agents amp wins and clears the quarantine.
	filtered := filterQuarantined(agents, st, map[string]bool{"amp": true})
	if _, ok := filtered[AgentAmp]; !ok {
		t.Fatal("explicitly allowed agent must not be filtered")
	}
	if st.isQuarantined(AgentAmp) {
		t.Fatal("explicit allow must clear quarantine")
	}

	// Without the explicit allow it is removed.
	st.Quarantined[string(AgentAmp)] = "2026-01-01T00:00:00Z"
	filtered = filterQuarantined(agents, st, map[string]bool{})
	if _, ok := filtered[AgentAmp]; ok {
		t.Fatal("quarantined agent must be filtered out")
	}
}

// --- catalog v2 invariants ---

func TestCatalogV2Coverage(t *testing.T) {
	freeCount := 0
	for _, name := range councilRosterAgents {
		entry := catalogEntry(name)
		if entry == nil {
			t.Fatalf("%s missing from catalog", name)
		}
		if entry.LoginHint == "" {
			t.Errorf("%s: LoginHint required", name)
		}
		if len(versionArgv(entry)) == 0 {
			t.Errorf("%s: versionArgv must never be empty", name)
		}
		// Every agent must be installable or carry a clear manual hint.
		if installArgv(entry) == nil && entry.InstallHint == "" {
			t.Errorf("%s: needs installArgv or InstallHint", name)
		}
		if entry.FreeTier {
			freeCount++
		}
	}
	if freeCount < 4 {
		t.Errorf("expected at least 4 free-tier agents, got %d", freeCount)
	}
}

func TestSemverNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.4.0", "1.3.2", true},
		{"1.3.2", "1.4.0", false},
		{"1.3.2", "1.3.2", false},
		{"2.0.0", "1.9.9", true},
		{"1.10.0", "1.9.0", true}, // numeric, not lexicographic
	}
	for _, c := range cases {
		if got := semverNewer(c.a, c.b); got != c.want {
			t.Errorf("semverNewer(%q,%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestShortPingReasonAuthHint(t *testing.T) {
	reason := "err: exit status 41 | stderr: Please set an Auth method in your settings.json\nsecond line"
	out := shortPingReason(AgentGemini, reason)
	if !strings.Contains(out, "council login gemini") {
		t.Errorf("auth-smelling failure should point at council login, got %q", out)
	}
	if strings.Contains(out, "second line") {
		t.Errorf("reason should be condensed to one line, got %q", out)
	}

	plain := shortPingReason(AgentCodex, "err: context deadline exceeded")
	if strings.Contains(plain, "council login") {
		t.Errorf("non-auth failure must not suggest login, got %q", plain)
	}
}

func TestUpdateArgvPrefersSelfUpdater(t *testing.T) {
	cursor := catalogEntry(AgentCursor)
	argv := updateArgv(cursor, filepath.Join("bin", "cursor-agent"))
	if len(argv) != 2 || argv[1] != "update" {
		t.Errorf("cursor should use its self-updater, got %v", argv)
	}
}
