package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runPullUpCheck(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := checkCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(append([]string{"--verdicts", "--rule", "i2c-pull-up"}, args...))
	err := cmd.Execute()
	return buf.String(), err
}

// The end-to-end proof for agni issue 820 that a declared pull-up reach travels from the YAML, as a
// value through the request, into the model the rule walks. Both routes a declaration takes are
// asked: --intent-path, and the design's own descriptor.
func TestCheckDeclaredPullUpReachCLI(t *testing.T) {
	const fail = "no rail is reachable from SDA_MAIN through a resistor within 3 hops"
	const pass = "SDA_MAIN reaches rail VCC_3V3 through R1 then R2 then R3 then R4"

	out, err := runPullUpCheck(t, "../../examples/common/designs/pullup-reach.edn")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, fail) {
		t.Fatalf("with no declaration SDA_MAIN fails at the default reach:\n%s", out)
	}

	out, err = runPullUpCheck(t, "../../examples/common/designs/pullup-reach.edn", "--intent-path", "../../examples/common/designs/pullup-reach.intent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, pass) || strings.Contains(out, fail) {
		t.Errorf("--intent-path declaring reach 4 must pass SDA_MAIN through all four resistors:\n%s", out)
	}

	edn, err := os.ReadFile("../../examples/common/designs/pullup-reach.edn")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, body := range map[string]string{
		"pullup-reach.edn": string(edn),
		"design.yaml":      "name: pullup\nentry: pullup-reach.edn\nintent:\n  nets:\n    SDA_MAIN: {pull_up_reach_hops: 4}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err = runPullUpCheck(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, pass) {
		t.Errorf("a descriptor declaring reach 4 must pass SDA_MAIN through all four resistors:\n%s", out)
	}
}

// A negative reach is refused at load, naming the field, rather than running a walk that crosses
// nothing.
func TestCheckNegativePullUpReachRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("name: bad\nintent:\n  nets:\n    SDA_MAIN: {pull_up_reach_hops: -1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runPullUpCheck(t, "../../examples/common/designs/pullup-reach.edn", "--intent-path", path)
	if err == nil {
		t.Fatalf("a negative reach must fail the load, not run:\n%s", out)
	}
	if !strings.Contains(err.Error(), "pull_up_reach_hops") {
		t.Errorf("error should name the field: %v", err)
	}
}
