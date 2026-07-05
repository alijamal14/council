package main

import (
	"os"
	"sync"
)

// Minimal ANSI styling with zero dependencies. Colors are enabled only when
// stdout is a terminal and NO_COLOR (https://no-color.org) is unset, so piped
// output (Bash tools, CI, logs) stays plain text.

var colorOnce sync.Once
var colorEnabled bool

func useColor() bool {
	colorOnce.Do(func() {
		if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
			return
		}
		if os.Getenv("TERM") == "dumb" {
			return
		}
		info, err := os.Stdout.Stat()
		if err != nil {
			return
		}
		colorEnabled = info.Mode()&os.ModeCharDevice != 0
	})
	return colorEnabled
}

func paint(code, s string) string {
	if !useColor() {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func bold(s string) string   { return paint("1", s) }
func dim(s string) string    { return paint("2", s) }
func green(s string) string  { return paint("32", s) }
func yellow(s string) string { return paint("33", s) }
func red(s string) string    { return paint("31", s) }
func cyan(s string) string   { return paint("36", s) }
