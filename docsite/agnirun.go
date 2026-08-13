package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	out, err := renderRun(relativePath)
	if err != nil {
		// Rendered rather than swallowed. A silently empty block is the failure this whole mechanism
		// exists to remove, and a tutorial showing an error is a tutorial someone fixes.
		return "```\nagniRun " + relativePath + " failed: " + err.Error() + "\n```"
	}
	// A BARE fence. Tagging it `console` makes Chroma apply its console lexer, which marks the whole
	// body as error tokens and renders it crimson on near-black. The hand-written blocks it replaces
	// are bare too, so this also keeps the page visually unchanged.
	return out
}

// renderRun composes the whole block: the command as the reader types it, then what it printed.
//
// One block rather than the command in the page and the output beside it, because two places is one
// place too many — that split is what let the output drift in the first place, and leaving the command
// behind would have preserved the bug for the half nobody was looking at.
func renderRun(relativePath string) (string, error) {
	spec, body, err := runOrLoad(relativePath)
	if err != nil {
		return "", err
	}
	shown := spec.Show
	if strings.TrimSpace(shown) == "" {
		shown = spec.Script
	}
	var b strings.Builder
	b.WriteString("```\n")
	for _, l := range strings.Split(strings.TrimRight(shown, "\n"), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		b.WriteString("$ " + l + "\n")
	}
	b.WriteString(strings.TrimRight(body, "\n") + "\n```")
	return b.String(), nil
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
	// Capture selects which stream the block shows: "stdout" (default), "stderr", "both", or "none"
	// for a lesson that is only about the exit code.
	//
	// A field rather than a shell redirect because the redirect was the thing making specs fragile.
	// Resolution notes and a gate's message share stderr, so a spec that wanted the message was writing
	// `> /dev/null 2>/tmp/err` and then grepping a temp file — plumbing that had to be re-derived every
	// time and that `show` then had to hide.
	Capture string `yaml:"capture"`
	// Exit appends "exit N" to the block, for the rungs whose lesson IS the exit code.
	//
	// It replaces `echo $?`, which only worked because the runner used a shell, and which forced the
	// preceding command to redirect its real output away to keep the block small.
	Exit bool `yaml:"exit"`
	// Match keeps only the lines matching this RE2 pattern, empty to keep everything.
	//
	// It replaces positional filtering. A spec once said `sed -n '5p'` to pull one line out of a
	// coverage rollup, which is correct until that output gains a line and then silently shows the
	// wrong one — the exact failure mode this whole mechanism exists to remove, reintroduced in the
	// tool meant to prevent it. A pattern selects what the lesson is ABOUT rather than where it
	// happened to sit.
	//
	// Matching nothing is an ERROR, not an empty block. A filter that stops matching has to say so:
	// silently rendering nothing is how a page ends up teaching from a blank space.
	Match string `yaml:"match"`
	// Show is the command as the READER should see it, defaulting to Script.
	//
	// It exists because the page used to hand-write the command in its own fence above the generated
	// output, which put the command back in exactly the position the output had just been rescued
	// from: editable in one place, run from another, free to disagree. Now both halves come from this
	// file and the page holds only the directive.
	//
	// It is a separate field rather than the script itself because a script may carry plumbing a reader
	// should not have to look at — a stderr redirect, a `sed` narrowing a table to the one line the
	// lesson is about. Keeping them apart is what lets the page stay clean without the command becoming
	// a fiction: they sit adjacent in one small file a reviewer reads whole, rather than in two files
	// nobody diffs together. Omit it whenever the script has nothing to hide, which is most of the time.
	Show string `yaml:"show"`
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
func runOrLoad(relativePath string) (runSpec, string, error) {
	var spec runSpec
	specPath, ok := safeJoin(relativePath)
	if !ok {
		return spec, "", fmt.Errorf("path escapes the docsite directory")
	}
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return spec, "", err
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return spec, "", fmt.Errorf("%s: %w", relativePath, err)
	}
	if strings.TrimSpace(spec.Script) == "" {
		return spec, "", fmt.Errorf("%s declares no script", relativePath)
	}
	want, err := inputHash(raw, spec.Fixture)
	if err != nil {
		return spec, "", err
	}
	outPath := specPath + outputSuffix
	if body, stamp, err := readOutput(outPath); err == nil && stamp == want {
		return spec, body, nil
	}
	body, err := execute(spec)
	if err != nil {
		return spec, "", err
	}
	if err := os.WriteFile(outPath, []byte(stampPrefix+want+"\n"+body), 0o644); err != nil {
		return spec, "", err
	}
	return spec, body, nil
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

// execute runs the spec's script in a scratch copy of its fixture and applies the spec's capture
// rules, returning the block body.
//
// The stream selection, exit code and line filter are applied HERE rather than by shell plumbing in
// the script. That is the whole point of them being fields: a spec says what the lesson is about, and
// the fragile mechanics of getting there are written once, in Go, where they can be tested.
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
	// A SHELL is still needed: a lesson may be several steps (rung 11 moves a directory before running
	// anything), and those steps are the tutorial's content rather than plumbing. The script is
	// checked-in yaml from this repo own content tree, never input from elsewhere, so there is no
	// injection boundary: anyone who can edit a run spec can already edit this file.
	cmd := exec.Command("sh", "-c", spec.Script)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// The exit status is not an error here: several rungs exist to demonstrate a gate TRIPPING, so a
	// non-zero exit is the lesson.
	runErr := cmd.Run()

	var body string
	switch spec.Capture {
	case "", "stdout":
		body = stdout.String()
	case "stderr":
		body = stderr.String()
	case "both":
		body = stdout.String() + stderr.String()
	case "none":
		// A rung whose lesson IS the exit code shows no report. Expressed as a capture rather than by
		// filtering everything out, because "keep only the lines matching nothing" is the sort of idiom
		// that reads as a mistake and gets helpfully "fixed" later.
		body = ""
	default:
		return "", fmt.Errorf("unknown capture %q (want stdout, stderr or both)", spec.Capture)
	}
	if spec.Match != "" {
		re, err := regexp.Compile(spec.Match)
		if err != nil {
			return "", fmt.Errorf("match %q: %w", spec.Match, err)
		}
		var kept []string
		for _, l := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
			if re.MatchString(l) {
				kept = append(kept, l)
			}
		}
		if len(kept) == 0 {
			return "", fmt.Errorf("match %q selected no lines; the output it filtered has changed shape", spec.Match)
		}
		body = strings.Join(kept, "\n") + "\n"
	}
	if spec.Exit {
		body = strings.TrimRight(body, "\n")
		if body != "" {
			body += "\n"
		}
		body += fmt.Sprintf("exit %d\n", exitStatus(runErr))
	}
	return body, nil
}

// exitStatus extracts a process exit code, 0 when it succeeded.
func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
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
