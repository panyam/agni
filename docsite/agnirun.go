package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sync"

	"gopkg.in/yaml.v3"
)

var (
	agniBuildOnce sync.Once
	agniBuildPath string
	agniBuildErr  error
)

// AgniRun transcludes a captured command output into a page, from a declaration beside it, so the
// output has one source the way `includeCard` gives the rule catalog one (#238). Hand-pasted output
// rotted twice before this existed.
//
// Three files:
//
//	09-read-the-verdicts.md      {{ agniRun "runs/coverage.yaml" }}   never rewritten
//	runs/coverage.yaml           what to run, and in which fixture     hand-authored
//	runs/coverage.yaml.output    what it printed                       generated, COMMITTED
//
// The page is never written, and the output only when stale, so a rebuild converges. The output is
// committed so a regression shows as a reviewable diff, and so a docs build with fresh outputs needs
// no `agni` binary and no fixture. Authoring rules are in docsite/README.md#generated-command-output.
func AgniRun(relativePath string) string {
	out, err := renderRun(relativePath)
	if err != nil {
		// Rendered into the page rather than swallowed, since a tutorial showing an error gets fixed.
		// Also to stderr, because the build still exits 0 and the operator never sees the page.
		fmt.Fprintf(os.Stderr, "agniRun %s failed: %v\n", relativePath, err)
		return "```\nagniRun " + relativePath + " failed: " + err.Error() + "\n```"
	}
	return out
}

// renderRun composes the blocks, each command as the reader types it followed by what it printed.
// The command comes from the spec rather than the page, so the two cannot drift apart (#239).
func renderRun(relativePath string) (string, error) {
	spec, bodies, err := runOrLoad(relativePath)
	if err != nil {
		return "", err
	}
	steps := spec.steps()
	if len(bodies) != len(steps) {
		return "", fmt.Errorf("%s: %d captured sections for %d steps", relativePath, len(bodies), len(steps))
	}
	// ONE BLOCK PER STEP, so a command sits with its own output and the copy button hands back
	// something pasteable (#525).
	blocks := make([]string, 0, len(steps))
	for i, step := range steps {
		blocks = append(blocks, block(step.shown(), bodies[i]))
	}
	return strings.Join(blocks, "\n\n"), nil
}

// block renders one command and its output as a BARE fence. Tagging it `console` makes Chroma's
// console lexer mark the whole body as error tokens, crimson on near-black.
func block(shown, body string) string {
	var b strings.Builder
	b.WriteString("```\n")
	// Only a line that STARTS a command gets the prompt. A `$` on a backslash-continued line reads as
	// a second command and copies as a broken one.
	continued := false
	for _, l := range strings.Split(strings.TrimRight(shown, "\n"), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if continued {
			// Verbatim, because the spec's own indentation already aligns it under the prompt.
			b.WriteString(l + "\n")
		} else {
			b.WriteString("$ " + l + "\n")
		}
		continued = isContinued(l)
	}
	// A step can print nothing (rung 11 writes a results file and redirects the report), and then the
	// block is the command alone.
	if out := strings.TrimRight(body, "\n"); out != "" {
		b.WriteString(out + "\n")
	}
	b.WriteString("```")
	return b.String()
}

// isContinued reports whether a line runs on into the next one, which it does when it ends in an ODD
// number of backslashes. A trailing `\\` is an escaped backslash and ends the command.
func isContinued(line string) bool {
	n := 0
	for i := len(line) - 1; i >= 0 && line[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

// runSpec is what a `.yaml` beside a tutorial declares.
type runSpec struct {
	// Fixture is the project the command runs in, relative to the repo root. It is COPIED to a scratch
	// directory first, so a destructive step (rung 11's `mv params params-old`) cannot mutate the
	// checked-in fixture.
	Fixture string `yaml:"fixture"`
	// FromRoot runs the script at the scratch ROOT with the fixture at its full relative path, so
	// commands read as typed from a clone. The learn course wants this, since a chapter picks whichever
	// fixture fits; the tutorials default to running inside the fixture, matching their one
	// `cd examples/tutorial-project` (#434).
	//
	// A mode rather than a `show` override, so the displayed path is the one that ran. Output is the
	// same either way, because provenance is reported relative to the design's project.
	FromRoot bool `yaml:"from_root"`
	// Script is the shell to run. A shell rather than an argv because a step may be several commands
	// or a pipe.
	Script string `yaml:"script"`
	// Capture selects which stream the block shows: "stdout" (default), "stderr", "both", or "none"
	// for a lesson that is only about the exit code. A field rather than a shell redirect, since
	// resolution notes and a gate's message share stderr (#241).
	Capture string `yaml:"capture"`
	// Exit appends "exit N" to the block, for the rungs whose lesson IS the exit code. Use it rather
	// than `echo $?` in the script (#241).
	Exit bool `yaml:"exit"`
	// Match keeps only the lines matching this RE2 pattern, empty to keep everything. It selects by
	// content rather than position, so the output gaining a line cannot silently shift it (#241).
	// Matching nothing is an ERROR, not an empty block.
	Match string `yaml:"match"`
	// Show is the command as the READER should see it, defaulting to Script. It hides plumbing such as
	// a stderr redirect or a narrowing `sed`, and sits beside Script so a reviewer reads both in one
	// file (#239). Omit it when the script has nothing to hide.
	Show string `yaml:"show"`
	// Steps replaces Script when a lesson is several commands and each one's OUTPUT is part of the
	// point, so each renders as its own block (#525). Boundaries are DECLARED, because neither
	// splitting a heredoc-bearing script nor mapping output onto `show` can infer them.
	//
	// Each step is its own shell in the SAME scratch directory, so files carry across and shell
	// variables do not. Script and Steps are mutually exclusive. Capture and Match apply to every
	// step, Exit to the last.
	Steps []runStep `yaml:"steps"`
}

// runStep is one command and the output it produced, which the page renders as its own block.
type runStep struct {
	// Script is the shell to run, Show what the reader should see, defaulting to Script, as on runSpec.
	Script string `yaml:"script"`
	Show   string `yaml:"show"`
}

// steps normalizes a spec to the list the runner and the renderer both walk. A bare `script` is the
// one-step case (80 of 96 specs when #525 landed) and captures byte-identically to before.
func (s runSpec) steps() []runStep {
	if len(s.Steps) > 0 {
		return s.Steps
	}
	return []runStep{{Script: s.Script, Show: s.Show}}
}

// shown is what the block prints as the command, which is Show when the step hides plumbing.
func (st runStep) shown() string {
	if strings.TrimSpace(st.Show) != "" {
		return st.Show
	}
	return st.Script
}

// outputSuffix is appended to the spec's path to name its captured output.
const outputSuffix = ".output"

// stampPrefix marks the hash line at the top of a generated output file.
const stampPrefix = "#agni-run "

// stepDelim separates one step's captured output from the next inside a single capture file. The
// commands are NOT written there, since the spec holds them. A one-step spec writes no delimiter.
const stepDelim = "#agni-step\n"

// runOrLoad returns the spec's output, reusing the committed capture when it is current.
//
// Freshness is a HASH of the inputs, never mtime, which a git checkout makes arbitrary. The hash
// covers the spec and the fixture, NOT the engine build, so an engine change regenerates nothing on
// its own and only `tutorial-runs-check` catches it. See
// docsite/content/build/the-gate.md#generated-captures-are-checked-by-regenerating-them.
func runOrLoad(relativePath string) (runSpec, []string, error) {
	var spec runSpec
	specPath, ok := safeJoin(relativePath)
	if !ok {
		return spec, nil, fmt.Errorf("path escapes the docsite directory")
	}
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return spec, nil, err
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return spec, nil, fmt.Errorf("%s: %w", relativePath, err)
	}
	if err := spec.validate(relativePath); err != nil {
		return spec, nil, err
	}
	want, err := inputHash(raw, spec.Fixture)
	if err != nil {
		return spec, nil, err
	}
	outPath := specPath + outputSuffix
	// A capture whose section count does not match the step count is stale whatever its stamp says,
	// since that is what a hand-edited capture looks like.
	if bodies, stamp, err := readOutput(outPath); err == nil && stamp == want && len(bodies) == len(spec.steps()) {
		return spec, bodies, nil
	}
	bodies, err := execute(spec)
	if err != nil {
		return spec, nil, err
	}
	if err := os.WriteFile(outPath, []byte(stampPrefix+want+"\n"+strings.Join(bodies, stepDelim)), 0o644); err != nil {
		return spec, nil, err
	}
	return spec, bodies, nil
}

// validate rejects a self-contradictory spec rather than silently preferring one field over another.
func (s runSpec) validate(path string) error {
	hasScript := strings.TrimSpace(s.Script) != ""
	if hasScript && len(s.Steps) > 0 {
		return fmt.Errorf("%s declares both script and steps; a spec is one or the other", path)
	}
	if !hasScript && len(s.Steps) == 0 {
		return fmt.Errorf("%s declares no script", path)
	}
	if hasScript && strings.TrimSpace(s.Show) == "" && strings.Contains(s.Script, stepDelim) {
		return fmt.Errorf("%s: a script cannot contain %q, which delimits a capture", path, strings.TrimSpace(stepDelim))
	}
	for i, st := range s.Steps {
		if strings.TrimSpace(st.Script) == "" {
			return fmt.Errorf("%s: step %d declares no script", path, i+1)
		}
	}
	return nil
}

// readOutput splits a captured output into its stamp and one body per step.
func readOutput(path string) (bodies []string, stamp string, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	s := string(b)
	first, rest, ok := strings.Cut(s, "\n")
	if !ok || !strings.HasPrefix(first, stampPrefix) {
		// No stamp means someone hand-edited it, so treat it as stale.
		return nil, "", fmt.Errorf("no stamp")
	}
	return strings.Split(rest, stepDelim), strings.TrimPrefix(first, stampPrefix), nil
}

// inputHash covers the spec and every TRACKED file in its fixture, so any COMMITTED edit to either
// regenerates and nothing else in a working tree can move it. The stamp is itself committed, so a
// working-tree hash would be right for one machine only (agni issue 357).
//
// A FETCHED fixture has nothing tracked, so its stamp covers hack/samples.pin instead, which changes
// exactly when the corpus does and needs no corpus on disk to check (agni issue 682).
func inputHash(spec []byte, fixture string) (string, error) {
	h := sha256.New()
	h.Write(spec)
	if fixture == "" {
		return hex.EncodeToString(h.Sum(nil))[:16], nil
	}
	if isFetched(fixture) {
		pinPath := samplesPin
		if !filepath.IsAbs(pinPath) {
			pinPath = filepath.Join("..", pinPath)
		}
		pin, err := os.ReadFile(pinPath)
		if err != nil {
			return "", fmt.Errorf("hashing fetched fixture %s: %w", fixture, err)
		}
		h.Write([]byte(path.Clean(fixture)))
		h.Write(pin)
		return hex.EncodeToString(h.Sum(nil))[:16], nil
	}
	if stray, err := untrackedFixtureFiles(fixture); err != nil {
		return "", err
	} else if len(stray) > 0 {
		return "", fmt.Errorf("fixture %s has files git does not track yet:\n  %s\n"+
			"The stamp hashes COMMITTED content, so it cannot be computed for this tree: it would come "+
			"out one way now and another once these are committed, which is a capture that passes here "+
			"and is stale in CI. Commit them, then regenerate (agni issue 588)",
			fixture, strings.Join(stray, "\n  "))
	}
	files, err := trackedFiles(fixture)
	if err != nil {
		return "", err
	}
	for _, rel := range files {
		// Generated outputs inside the fixture would make the hash depend on itself.
		if strings.HasSuffix(rel, outputSuffix) {
			continue
		}
		b, err := os.ReadFile(filepath.Join("..", rel))
		if err != nil {
			return "", fmt.Errorf("hashing fixture %s: %w", fixture, err)
		}
		h.Write([]byte(rel))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// fetchedRoot is where `make samples` extracts the pinned board corpus (hack/fetch_samples.sh). It
// is gitignored, because the boards are other people's and carry their own licences.
const fetchedRoot = "tools/samples"

// samplesPin is the tracked file a fetched fixture's stamp covers in place of its content, relative
// to the repo root. A variable so a test can point it at an absolute path it is free to edit.
var samplesPin = "hack/samples.pin"

func isFetched(fixture string) bool {
	c := path.Clean(fixture)
	return c == fetchedRoot || strings.HasPrefix(c, fetchedRoot+"/")
}

// untrackedFixtureFiles lists the files under fixture that git neither tracks nor ignores. Such a
// file is outside the hash, so the local gate passes and CI fails once it is committed; inputHash
// refuses instead (agni issue 588). Ignored files are not listed, since counting them was agni
// issue 357.
func untrackedFixtureFiles(fixture string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z", "--others", "--exclude-standard", "--", fixture)
	cmd.Dir = ".."
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing untracked files for fixture %s: %w", fixture, err)
	}
	var files []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			files = append(files, p)
		}
	}
	sort.Strings(files)
	return files, nil
}

// trackedFiles lists a fixture's git-tracked files, repo-root-relative and in git's sorted order so
// the hash is deterministic. It REFUSES rather than falling back to walking the directory, which
// would silently make the stamp depend on the working tree again, and an empty listing is an error.
func trackedFiles(fixture string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z", "--", fixture)
	cmd.Dir = ".." // specs name their fixture relative to the repo root, as the rest of this file does
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing tracked files for fixture %s: %w "+
			"(the run-output stamp is a hash of COMMITTED content, so it needs git)", fixture, err)
	}
	var files []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			files = append(files, p)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("fixture %s has no git-tracked files; a stamp over uncommitted content "+
			"would look stable and mean nothing", fixture)
	}
	return files, nil
}

// execute runs each step in a scratch copy of the spec's fixture and returns one block body per
// step, with the spec's capture rules applied here rather than by shell plumbing in the script.
func execute(spec runSpec) ([]string, error) {
	bin, err := buildAgni()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "agni-run")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	work := dir
	if spec.Fixture != "" {
		// Where the copy LANDS decides how paths read in the block. Under from_root it keeps its full
		// relative path and the script runs at the scratch root; otherwise it lands as a bare basename
		// and the script runs inside it.
		dest := filepath.Join(dir, filepath.Base(spec.Fixture))
		if spec.FromRoot {
			dest = filepath.Join(dir, spec.Fixture)
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return nil, fmt.Errorf("preparing fixture path: %w", err)
			}
		} else {
			work = dest
		}
		if _, err := os.Stat(filepath.Join("..", spec.Fixture)); err != nil && isFetched(spec.Fixture) {
			return nil, fmt.Errorf("fixture %s is fetched rather than committed, and is not here: run `make samples`", spec.Fixture)
		}
		if out, err := exec.Command("cp", "-R", filepath.Join("..", spec.Fixture), dest).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("copying fixture: %v: %s", err, out)
		}
	}
	steps := spec.steps()
	bodies := make([]string, 0, len(steps))
	for i, step := range steps {
		body, err := runOne(spec, step, work, dir, bin, i == len(steps)-1)
		if err != nil {
			return nil, err
		}
		bodies = append(bodies, body)
	}
	return bodies, nil
}

// runOne executes a single step and returns the text its block will show. Every step shares the
// scratch directory, so what one writes the next can read.
func runOne(spec runSpec, step runStep, work, dir, bin string, last bool) (string, error) {
	// A shell, because a step may itself be several commands (rung 11 moves a directory first). The
	// script is checked-in yaml from this repo's content tree, so there is no injection boundary.
	cmd := exec.Command("sh", "-c", step.Script)
	cmd.Dir = work
	cmd.Env = captureEnv(dir, bin)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// The exit status is not an error here, because several rungs demonstrate a gate TRIPPING.
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
		// A rung whose lesson IS the exit code shows no report.
		body = ""
	default:
		return "", fmt.Errorf("unknown capture %q (want stdout, stderr or both)", spec.Capture)
	}
	// A step that printed NOTHING is not a filter that stopped matching, so Match skips it. Rung 11's
	// first step redirects its report to /dev/null and is empty by design.
	if spec.Match != "" && strings.TrimSpace(body) != "" {
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
	// The scratch directory must not survive into a committed capture, where it would churn every run
	// and put a host path in a public repo (#243). Strip BOTH forms, because a macOS temp dir is a
	// symlink (/var/folders/... resolves to /private/var/folders/...), and stripping only one left
	// "/privatedesigns/gateway".
	for _, prefix := range scratchForms(work) {
		body = strings.ReplaceAll(body, prefix+string(os.PathSeparator), "")
		body = strings.ReplaceAll(body, prefix, ".")
	}
	if strings.Contains(body, os.TempDir()) || strings.Contains(body, "/var/folders/") || strings.Contains(body, "/private") {
		return "", fmt.Errorf("capture still contains a scratch path: it would churn on every run and put a host path in the repo. The command prints an absolute path this runner cannot rewrite")
	}
	// Exit belongs to the run as a whole, so it lands on the last step's block.
	if spec.Exit && last {
		body = strings.TrimRight(body, "\n")
		if body != "" {
			body += "\n"
		}
		body += fmt.Sprintf("exit %d\n", exitStatus(runErr))
	}
	return body, nil
}

// scratchForms returns the scratch directory in both forms a command might print, as handed to the
// process and with symlinks resolved.
func scratchForms(work string) []string {
	forms := []string{work}
	if real, err := filepath.EvalSymlinks(work); err == nil && real != work {
		forms = append(forms, real)
	}
	return forms
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
		// From the repo root, since docsite/ is its own module and does not depend on cmd/agni.
		cmd.Dir = ".."
		if out, err := cmd.CombinedOutput(); err != nil {
			agniBuildErr = fmt.Errorf("building agni: %v: %s", err, out)
		}
	})
	return agniBuildPath, agniBuildErr
}

// captureEnv is the environment a captured run sees, built rather than inherited, because a
// committed capture must not depend on the operator's machine. HOME and XDG_CONFIG_HOME reach
// agni.yaml, which adds a mount note naming a home path. AGNI_SYMBOL_PATH changes what a read
// RESOLVES, so a local capture would disagree with CI's and both would look correct.
//
// The run gets a scratch HOME and only what a shell needs, so anything agni reads from the
// environment is absent by construction rather than by a deny-list.
func captureEnv(scratch, bin string) []string {
	home := filepath.Join(scratch, "home")
	_ = os.MkdirAll(filepath.Join(home, ".config"), 0o755)
	return []string{
		"PATH=" + filepath.Dir(bin) + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		// A shell wants these, and neither reaches the engine.
		"SHELL=/bin/sh",
		"TMPDIR=" + os.Getenv("TMPDIR"),
	}
}
