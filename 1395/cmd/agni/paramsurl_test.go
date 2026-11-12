package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// serveRefusal runs `agni serve` with args from an empty directory and an empty home config, and
// returns the error it refused with. A regression starts a server that never stops, so the wait is
// bounded.
func serveRefusal(t *testing.T, args ...string) error {
	t.Helper()
	clearNamedWebDir(t)
	t.Setenv(envWebDir, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Chdir(t.TempDir())
	cmd := rootCmd()
	cmd.SetArgs(append([]string{"serve", "--addr", "127.0.0.1:0"}, args...))
	cmd.SetErr(new(strings.Builder))
	done := make(chan error, 1)
	go func() { done <- cmd.Execute() }()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("serve started instead of refusing")
		return nil
	}
}

// An unreachable --params-url fails at startup by name, before the listener opens, rather than as an
// unavailable error on the first check someone runs.
func TestServeRefusesAnUnreachableParamsURL(t *testing.T) {
	err := serveRefusal(t, "--params-url", "http://127.0.0.1:1")
	if err == nil || !strings.Contains(err.Error(), "--params-url") {
		t.Fatalf("serve with an unreachable --params-url = %v", err)
	}
}

// --params and --params-url both name the server-wide corpus, so passing both is refused rather than
// one silently winning.
func TestServeRefusesBothParamsFlags(t *testing.T) {
	err := serveRefusal(t, "--params", "../../examples/tutorial-project/params", "--params-url", "http://127.0.0.1:1")
	if err == nil || !strings.Contains(err.Error(), "pass one") {
		t.Fatalf("serve with --params and --params-url = %v", err)
	}
}
