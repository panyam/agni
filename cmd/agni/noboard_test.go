package main

import (
	"bytes"
	"strings"
	"testing"
)

// A revision with no board must say its board-tier rules did not run, or a check of it reads as
// clean copper (agni issue 848). The tutorial leaves rev C undeclared, so it reads with no board.
func TestCheckSaysWhenNoBoardWasRead(t *testing.T) {
	freshWorkspace(t)
	cmd := checkCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{tutorialGateway + "gateway-rev-c.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check rev C: %v", err)
	}
	note := errOut.String()
	for _, want := range []string{"no board was read", "copper-clearance", "track-width", "design.yaml"} {
		if !strings.Contains(note, want) {
			t.Errorf("stderr is missing %q:\n%s", want, note)
		}
	}
	// A revision that declares its board says nothing of the kind.
	freshWorkspace(t)
	cmd = checkCmd()
	errOut.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{tutorialGateway + "gateway-rev-b.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check rev B: %v", err)
	}
	if strings.Contains(errOut.String(), "no board was read") {
		t.Errorf("rev B declares its board, yet stderr says none was read:\n%s", errOut.String())
	}
}
