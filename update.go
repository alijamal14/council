package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// handleUpdate implements `council update [--check] [--quiet]`.
//
//	update           upgrade every installed agent to its latest stable
//	                 release through the same official channel it was
//	                 installed from (npm @latest, pip --upgrade, the CLI's
//	                 own self-updater, or the vendor install script)
//	update --check   report versions and available council releases only
//	update --quiet   same as update, minimal output (used by auto-update)
//
// Council itself is version-checked against GitHub releases; the binary is
// never replaced in-place — the user is told the exact upgrade command for
// their install channel instead (brew/scoop/script), which is safer.
func handleUpdate(ctx context.Context, cfg Config, checkOnly, quiet bool) int {
	logf := func(format string, a ...any) {
		if !quiet {
			fmt.Printf(format, a...)
		}
	}

	logf("%s\n", bold("🔄 Council Update"))
	logf("-----------------\n")

	dummyLog, _ := newAuditLogger("update", []string{os.DevNull}, []string{os.DevNull})
	detectCfg := cfg
	if detectCfg.AgentCheckTimeout <= 0 {
		detectCfg.AgentCheckTimeout = 8
	}
	installed := detectAgents(ctx, detectCfg, dummyLog)

	st := loadState()
	failures := 0

	for _, name := range councilRosterAgents {
		resolved, ok := installed[name]
		if !ok {
			continue
		}
		entry := catalogEntry(name)
		version := probeVersion(ctx, entry, resolved.Path)
		if checkOnly {
			logf("  %-12s %s\n", name, cyan(version))
			continue
		}

		argv := updateArgv(entry, resolved.Path)
		if argv == nil {
			logf("  ⏭️  %-12s %s — no automated updater (auto-updates itself or manual)\n", name, version)
			continue
		}

		logf("  🔄 %-12s %s → %s ...\n", name, version, strings.Join(argv, " "))
		flush()
		updCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		cmd := exec.CommandContext(updCtx, argv[0], argv[1:]...)
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			failures++
			logf("  ❌ %-12s update failed: %v\n", name, err)
			tail := strings.TrimSpace(string(out))
			if len(tail) > 300 {
				tail = tail[len(tail)-300:]
			}
			if tail != "" && !quiet {
				logf("     %s\n", tail)
			}
		} else {
			newVersion := probeVersion(ctx, entry, resolved.Path)
			if newVersion != version {
				logf("  ✅ %-12s %s → %s\n", name, version, green(newVersion))
			} else {
				logf("  ✅ %-12s already latest (%s)\n", name, version)
			}
			// A refreshed CLI deserves a fresh start in the quarantine ledger.
			st.clearQuarantine(name)
		}
	}

	// Council self-check: compare against the latest GitHub release tag.
	if latest := latestCouncilRelease(ctx); latest != "" && !quiet {
		if semverNewer(strings.TrimPrefix(latest, "v"), Version) {
			fmt.Printf("\n⬆️  Council %s is available (you run v%s). Upgrade:\n", latest, Version)
			fmt.Println("    Homebrew:  brew upgrade council")
			fmt.Println("    Scoop:     scoop update council")
			fmt.Println("    Script:    curl -fsSL https://raw.githubusercontent.com/alijamal14/council/main/scripts/install.sh | bash")
		} else {
			fmt.Printf("\n✅ Council v%s is the latest release.\n", Version)
		}
	}

	st.LastUpdateCheck = time.Now()
	st.save()

	if failures > 0 {
		return 1
	}
	return 0
}

// updateArgv picks the upgrade command for an installed agent: the CLI's own
// self-updater when it has one, otherwise re-running its install channel.
func updateArgv(entry *AgentCatalogEntry, path string) []string {
	if entry == nil {
		return nil
	}
	if len(entry.UpdateArgs) > 0 {
		return append([]string{path}, entry.UpdateArgs...)
	}
	return installArgv(entry) // npm @latest / pip --upgrade / vendor script re-run
}

// probeVersion best-effort reads a CLI's version string (first line, capped).
func probeVersion(ctx context.Context, entry *AgentCatalogEntry, path string) string {
	vCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	runner := &LocalRunner{}
	stdout, stderr, err := runner.Run(vCtx, path, versionArgv(entry), nil)
	if err != nil && len(stdout)+len(stderr) == 0 {
		return "unknown"
	}
	line := strings.TrimSpace(string(stdout))
	if line == "" {
		line = strings.TrimSpace(string(stderr))
	}
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	if len(line) > 60 {
		line = line[:60]
	}
	if line == "" {
		return "unknown"
	}
	return line
}

// semverNewer reports whether a is a strictly newer x.y.z version than b.
// Non-numeric or missing segments compare as 0; equal versions are not newer.
func semverNewer(a, b string) bool {
	pa, pb := strings.SplitN(a, ".", 3), strings.SplitN(b, ".", 3)
	for i := 0; i < 3; i++ {
		var na, nb int
		if i < len(pa) {
			fmt.Sscanf(strings.TrimSpace(pa[i]), "%d", &na)
		}
		if i < len(pb) {
			fmt.Sscanf(strings.TrimSpace(pb[i]), "%d", &nb)
		}
		if na != nb {
			return na > nb
		}
	}
	return false
}

// latestCouncilRelease returns the latest release tag (e.g. "v1.3.0") from
// GitHub, or "" on any failure — the check must never block or break update.
func latestCouncilRelease(ctx context.Context) string {
	reqCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, "GET",
		"https://api.github.com/repos/alijamal14/council/releases/latest", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if json.NewDecoder(resp.Body).Decode(&payload) != nil {
		return ""
	}
	return payload.TagName
}

// maybeAutoUpdate spawns a detached background `council update --quiet` after
// a successful run when COUNCIL_AUTO_UPDATE=1 (set once via
// `council config set COUNCIL_AUTO_UPDATE 1`). The user's session is never
// blocked; output lands in the state directory for inspection.
func maybeAutoUpdate() {
	if os.Getenv("COUNCIL_AUTO_UPDATE") != "1" {
		return
	}
	st := loadState()
	if time.Since(st.LastUpdateCheck) < 24*time.Hour {
		return // at most once a day
	}
	self, err := os.Executable()
	if err != nil {
		return
	}
	logPath := filepath.Join(councilConfigDir(), "auto-update.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	cmd := exec.Command(self, "update", "--quiet")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return
	}
	fmt.Println(dim(fmt.Sprintf("🔄 auto-update started in background (log: %s)", logPath)))
	// Detach: the child outlives this process; Release avoids a zombie handle.
	_ = cmd.Process.Release()
	logFile.Close()
}

// agentUpgradeMu serializes mid-run CLI upgrades so parallel agents do not race
// npm/winget installs for the same binary.
var agentUpgradeMu sync.Mutex

// upgradeAgentCLIFn is the mid-run upgrade hook; tests may replace it.
var upgradeAgentCLIFn = upgradeAgentCLI

// upgradeAgentCLI upgrades a single installed agent through its catalog channel
// (same path as `council update`). Returns true when the upgrade command exits 0.
func upgradeAgentCLI(ctx context.Context, name AgentName, resolved *ResolvedAgent, log *AuditLogger) bool {
	if resolved == nil || resolved.Path == "" {
		return false
	}
	entry := catalogEntry(name)
	argv := updateArgv(entry, resolved.Path)
	if argv == nil {
		if log != nil {
			log.LogAgent("UPGRADE_SKIP", fmt.Sprintf("%s has no automated updater", name), string(name), 0)
		}
		return false
	}

	agentUpgradeMu.Lock()
	defer agentUpgradeMu.Unlock()

	before := probeVersion(ctx, entry, resolved.Path)
	fmt.Printf("🔄 %s CLI may be too old for its model — upgrading (%s)...\n", name, strings.Join(argv, " "))
	flush()
	if log != nil {
		log.LogAgent("UPGRADE_START", fmt.Sprintf("%s upgrade: %s (was %s)", name, strings.Join(argv, " "), before), string(name), 0)
	}

	updCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	cmd := exec.CommandContext(updCtx, argv[0], argv[1:]...)
	out, err := cmd.CombinedOutput()
	cancel()

	// On Windows, Codex is often the WinGet shim while npm installs a newer
	// binary elsewhere — also try winget upgrade, then re-point at the newest.
	if name == AgentCodex && runtime.GOOS == "windows" {
		wCtx, wCancel := context.WithTimeout(ctx, 5*time.Minute)
		wCmd := exec.CommandContext(wCtx, "winget", "upgrade", "--id", "OpenAI.Codex", "-e",
			"--accept-package-agreements", "--accept-source-agreements")
		_, _ = wCmd.CombinedOutput()
		wCancel()
	}

	refreshResolvedAgentPath(ctx, name, resolved)
	after := probeVersion(ctx, entry, resolved.Path)
	if err != nil {
		tail := strings.TrimSpace(string(out))
		if len(tail) > 300 {
			tail = tail[len(tail)-300:]
		}
		// npm may have succeeded enough to expose a newer binary even if the
		// primary argv reported an error (e.g. permission noise).
		if !semverNewer(versionSemver(after), versionSemver(before)) {
			fmt.Printf("❌ %s upgrade failed: %v\n", name, err)
			if tail != "" {
				fmt.Printf("   %s\n", tail)
			}
			flush()
			if log != nil {
				log.LogAgent("UPGRADE_FAIL", fmt.Sprintf("%s upgrade failed: %v", name, err), string(name), 0)
			}
			return false
		}
	}

	st := loadState()
	st.clearQuarantine(name)
	st.save()

	if after != before {
		fmt.Printf("✅ %s upgraded: %s → %s (%s)\n", name, before, after, resolved.Path)
	} else {
		fmt.Printf("✅ %s upgrade finished (still %s) — will retry / fall back if needed\n", name, after)
	}
	flush()
	if log != nil {
		log.LogAgent("UPGRADE_OK", fmt.Sprintf("%s upgraded: %s → %s path=%s", name, before, after, resolved.Path), string(name), 0)
	}
	return true
}

// refreshResolvedAgentPath picks the newest installed binary for an agent when
// multiple install channels coexist (WinGet shim vs npm global, etc.).
func refreshResolvedAgentPath(ctx context.Context, name AgentName, resolved *ResolvedAgent) {
	if resolved == nil {
		return
	}
	entry := catalogEntry(name)
	if entry == nil {
		return
	}

	seen := map[string]bool{}
	var candidates []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		key := strings.ToLower(p)
		if seen[key] {
			return
		}
		seen[key] = true
		candidates = append(candidates, p)
	}

	add(resolved.Path)
	if p, err := exec.LookPath(entry.Executable); err == nil {
		add(p)
	}
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			add(filepath.Join(appdata, "npm", entry.Executable+".cmd"))
			add(filepath.Join(appdata, "npm", entry.Executable+".exe"))
			add(filepath.Join(appdata, "npm", entry.Executable))
		}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			add(filepath.Join(local, "Microsoft", "WinGet", "Links", entry.Executable+".exe"))
		}
	} else if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".npm-global", "bin", entry.Executable))
		add(filepath.Join(home, ".local", "bin", entry.Executable))
	}

	bestPath := resolved.Path
	bestVer := probeVersion(ctx, entry, resolved.Path)
	for _, c := range candidates {
		if _, err := os.Stat(c); err != nil {
			continue
		}
		ver := probeVersion(ctx, entry, c)
		if semverNewer(versionSemver(ver), versionSemver(bestVer)) {
			bestPath, bestVer = c, ver
		}
	}
	if bestPath != "" && bestPath != resolved.Path {
		resolved.Path = bestPath
	}
}

// versionSemver extracts the first x.y[.z] token from a CLI version string.
func versionSemver(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			continue
		}
		j := i
		dots := 0
		for j < len(s) {
			c := s[j]
			if c >= '0' && c <= '9' {
				j++
				continue
			}
			if c == '.' && dots < 2 && j+1 < len(s) && s[j+1] >= '0' && s[j+1] <= '9' {
				dots++
				j++
				continue
			}
			break
		}
		if dots >= 1 {
			return s[i:j]
		}
	}
	return strings.TrimSpace(s)
}

// codexCompatibleFallbackModel returns a model id to try when the configured
// Codex model requires a newer CLI and upgrade did not clear the error.
// Override with COUNCIL_CODEX_FALLBACK_MODEL.
func codexCompatibleFallbackModel() string {
	if v := strings.TrimSpace(os.Getenv("COUNCIL_CODEX_FALLBACK_MODEL")); v != "" {
		return v
	}
	// Prefer leaving ChatGPT-linked accounts on whatever the upgraded CLI
	// defaults to only when an explicit override is set; gpt-5.2 is rejected
	// by ChatGPT-auth Codex, so default empty disables --model fallback.
	return ""
}
