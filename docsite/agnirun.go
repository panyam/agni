package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sync"

	"gopkg.in/yaml.v3"
)

var (
	agniBuildOnce sync.Once
	agniBuildPath string
	agniBuildErr  error
)

// AgniRun transcludes a captured command output into a tutorial, from a declaration beside the page.
//
// A tutorial's promise is "run this, see this", and until now the "see this" half was pasted in by
// hand. It rotted twice — rung 9's coverage table showed 8 fail / 1 n/a where the fixture produced
// 9 / 0, with a Board row contradicting the Total row in the same table — and both times it was found
// by accident. The engine has no equivalent problem because generated content has one source, which
// is what `includeCard` above already does for the rule catalog. This is that, for command output.
//
// Three files, and the split is what keeps it from thrashing:
//
//	09-read-the-verdicts.md      {{ agniRun "runs/coverage.yaml" }}   never rewritten
//	runs/coverage.yaml           what to run, and in which fixture     hand-authored
//	runs/coverage.yaml.output    what it printed                       generated, COMMITTED
//
// The page is never written, so the site's own file watcher cannot see a build modify the content it
// is watching. The output file IS written, but only when stale, so a watcher rebuild converges after
// one cycle instead of looping. Writing the output back into the page — the obvious first design —
// would loop forever under `Site.Watch()`.
//
// The output is committed for two reasons. A regression then shows up as a reviewable diff rather
// than silently re-rendering, which is the property hand-pasting accidentally had and pure
// render-time generation would lose. And the docs build needs no `agni` binary and no fixture when
// the outputs are fresh, so publishing stays fast and hermetic.
func AgniRun(relativePath string) string {
	out, err := runOrLoad(relativePath)
	if err != nil {
		// Rendered rather than swallowed. A silently empty block is the failure this whole mechanism
		// exists to remove, and a tutorial showing an error is a tutorial someone fixes.
		return "```\nagniRun " + relativePath + " failed: " + err.Error() + "\n```"
	}
	// A BARE fence. Tagging it `console` makes Chroma apply its console lexer, which marks the whole
	// body as error tokens and renders it crimson on near-black. The hand-written blocks it replaces
	// are bare too, so this also keeps the page visually unchanged.
	return "```\n" + strings.TrimRight(out, "\n") + "\n```"
}

// runSpec is what a `.yaml` beside a tutorial declares.
type runSpec struct {
	// Fixture is the project the command runs in, relative to the repo root. It is COPIED to a scratch
	// directory first, so a rung that documents a destructive step (rung 11 walks the reader through
	// `mv params params-old`) cannot mutate the checked-in fixture. That is not hypothetical: doing it
	// by hand once left the real params/ renamed and two stray artifacts in the tree.
	Fixture string `yaml:"fixture"`
	// Script is the shell to run. A shell rather than an argv because the transcripts show `echo $?`
	// to teach exit codes, and some pipe through `head`.
	Script string `yaml:"script"`
}

// outputSuffix is appended to the spec's path to name its captured output.
const outputSuffix = ".output"

// stampPrefix marks the hash line at the top of a generated output file.
const stampPrefix = "#agni-run "

// runOrLoad returns the spec's output, reusing the committed capture when it is current.
//
// Freshness is a HASH of the inputs, never mtime. A git checkout gives every file the checkout time,
// so mtime comparisons are arbitrary on a fresh clone and would either regenerate everything or
// nothing depending on the order files happened to land.
//
// The hash deliberately covers the spec and the fixture, NOT the engine build. Including the binary
// would mark every output stale on every code change, which is exactly the per-push cost this design
// avoids. The consequence is that a code change does not regenerate anything on its own, so a
// regression is caught by the periodic `make tutorial-runs` sweep rather than immediately — a
// deliberate trade for a docs pipeline that stays out of the way.
func runOrLoad(relativePath string) (string, error) {
	specPath, ok := safeJoin(relativePath)
	if !ok {
		return "", fmt.Errorf("path escapes the docsite directory")
	}
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return "", err
	}
	var spec runSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return "", fmt.Errorf("%s: %w", relativePath, err)
	}
	if strings.TrimSpace(spec.Script) == "" {
		return "", fmt.Errorf("%s declares no script", relativePath)
	}
	want, err := inputHash(raw, spec.Fixture)
	if err != nil {
		return "", err
	}
	outPath := specPath + outputSuffix
	if body, stamp, err := readOutput(outPath); err == nil && stamp == want {
		return body, nil
	}
	body, err := execute(spec)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(outPath, []byte(stampPrefix+want+"\n"+body), 0o644); err != nil {
		return "", err
	}
	return body, nil
}

// readOutput splits a captured output into its stamp and its body.
func readOutput(path string) (body, stamp string, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	s := string(b)
	first, rest, ok := strings.Cut(s, "\n")
	if !ok || !strings.HasPrefix(first, stampPrefix) {
		// No stamp: an output someone hand-edited. Treat it as stale rather than trusting it, so the
		// generator remains the only author.
		return "", "", fmt.Errorf("no stamp")
	}
	return rest, strings.TrimPrefix(first, stampPrefix), nil
}

// inputHash covers the spec and every file in its fixture, so any edit to either regenerates.
func inputHash(spec []byte, fixture string) (string, error) {
	h := sha256.New()
	h.Write(spec)
	if fixture != "" {
		root := filepath.Join("..", fixture)
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			// Generated outputs inside the fixture would make the hash depend on itself.
			if strings.HasSuffix(p, outputSuffix) {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			h.Write([]byte(p))
			h.Write(b)
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("hashing fixture %s: %w", fixture, err)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// execute runs the spec's script in a scratch copy of its fixture, with a freshly built agni on PATH.
func execute(spec runSpec) (string, error) {
	bin, err := buildAgni()
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "agni-run")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	work := dir
	if spec.Fixture != "" {
		work = filepath.Join(dir, filepath.Base(spec.Fixture))
		if out, err := exec.Command("cp", "-R", filepath.Join("..", spec.Fixture), work).CombinedOutput(); err != nil {
			return "", fmt.Errorf("copying fixture: %v: %s", err, out)
		}
	}
	// A SHELL is needed: the transcripts use echo $? to teach exit codes and some pipe through head.
	// The script is checked-in yaml from this repo own content tree, never input from elsewhere, so
	// there is no injection boundary: anyone who can edit a run spec can already edit this file.
	cmd := exec.Command("sh", "-c", spec.Script)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stdout strings.Builder
	cmd.Stdout = &stdout
	// STDOUT only. Resolution notes ("reading gateway.edn, the entry design.yaml declares") go to
	// stderr precisely so a redirected report stays clean, and the pages do not show them.
	//
	// The exit status is ignored: several rungs exist to demonstrate a gate TRIPPING, so a non-zero
	// exit is the lesson rather than a failure.
	_ = cmd.Run()
	return stdout.String(), nil
}

// buildAgni compiles the CLI once per process into a cached location.
func buildAgni() (string, error) {
	agniBuildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "agni-bin")
		if err != nil {
			agniBuildErr = err
			return
		}
		agniBuildPath = filepath.Join(dir, "agni")
		cmd := exec.Command("go", "build", "-o", agniBuildPath, "./cmd/agni")
		// From the repo root: docsite/ is its own module and cmd/agni is not one of its dependencies.
		cmd.Dir = ".."
		if out, err := cmd.CombinedOutput(); err != nil {
			agniBuildErr = fmt.Errorf("building agni: %v: %s", err, out)
		}
	})
	return agniBuildPath, agniBuildErr
}
