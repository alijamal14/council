//go:build !windows

package main

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"syscall"
)

// Run executes commands on the local machine with process group management for Unix
func (r *LocalRunner) Run(ctx context.Context, name string, args []string, stdin io.Reader) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	cmd.Env = agentSubprocessEnv() // auth + TERM/Git fixes for non-interactive CLIs

	// Process group management for Unix to ensure sub-processes are killed on timeout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Start()
	if err != nil {
		return nil, nil, err
	}

	// Handle graceful termination via process group
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Kill the entire process group
		pgid, err := syscall.Getpgid(cmd.Process.Pid)
		if err == nil {
			syscall.Kill(-pgid, syscall.SIGKILL)
		}
		// Drain the wait goroutine
		<-done
		return []byte(stdout.String()), []byte(stderr.String()), ctx.Err()
	case err := <-done:
		return []byte(stdout.String()), []byte(stderr.String()), err
	}
}
