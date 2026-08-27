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
	return out
}

// renderRun composes the blocks: each command as the reader types it, then what that command printed.
//
// The command comes from the spec rather than from the page, because two places is one place too many
// — that split is what let the output drift in the first place, and leaving the command behind would
// have preserved the bug for the half nobody was looking at.
func renderRun(relativePath string) (string, error) {
	spec, bodies, err := runOrLoad(relativePath)
	if err != nil {
		return "", err
	}
	steps := spec.steps()
	if len(bodies) != len(steps) {
		return "", fmt.Errorf("%s: %d captured sections for %d steps", relativePath, len(bodies), len(steps))
	}
	// ONE BLOCK PER STEP, so a command sits with the output it produced. A run of several commands used
	// to render every command and then every output, which left the reader matching halves by eye, and
	// the copy button handing back a transcript instead of something to paste.
	blocks := make([]string, 0, len(steps))
	for i, step := range steps {
		blocks = append(blocks, block(step.shown(), bodies[i]))
	}
	return strings.Join(blocks, "\n\n"), nil
}

// block renders one command and its output as a bare fence.
//
// A BARE fence. Tagging it `console` makes Chroma apply its console lexer, which marks the whole body
// as error tokens and renders it crimson on near-black.
func block(shown, body string) string {
	var b strings.Builder
	b.WriteString("```\n")
	// Only a line that STARTS a command gets the prompt. A line continued with a trailing backslash
	// runs on into the next, and prefixing that next line too rendered a second `$` where there is no
	// second command, so the block read as two commands and copying it produced a broken one.
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
	// A step can legitimately print nothing — rung 11 writes a results file and redirects the report —
	// and then the block is the command alone rather than a command above a blank line.
	if out := strings.TrimRight(body, "\n"); out != "" {
		b.WriteString(out + "\n")
	}
	b.WriteString("```")
	return b.String()
}

// isContinued reports whether a line runs on into the next one, which it does when it ends in an ODD
// number of backslashes: a trailing `\\` is an escaped backslash and ends the command.
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
	// directory first, so a rung that documents a destructive step (rung 11 walks the reader through
	// `mv params params-old`) cannot mutate the checked-in fixture. That is not hypothetical: doing it
	// by hand once left the real params/ renamed and two stray artifacts in the tree.
	Fixture string `yaml:"fixture"`
	// FromRoot runs the script at the scratch ROOT with the fixture at its full relative path, so
	// commands read exactly as a reader would type them standing in a clone.
	//
	// The default is the opposite, and deliberately so: the tutorials establish one working directory
	// at the top of the course ("cd agni/examples/tutorial-project") and every rung is relative to it,
	// which is how somebody actually works through them. The learn course has no such setting, since a
	// chapter reaches for whichever fixture demonstrates its point, so a bare `designs/gateway...`
	// there is a path the reader cannot use and cannot locate without searching.
	//
	// It is a mode rather than something a spec fakes with `show` so that the command displayed is
	// exactly the command that ran. `show` exists to hide plumbing, and using it to swap in a
	// different PATH would put an untested command in front of the reader: nothing would check that
	// the displayed form still resolves, which is the class of rot the whole generated-capture
	// mechanism exists to remove.
	//
	// Output is unaffected either way. Provenance and resolution notes are reported relative to the
	// design's project rather than to the invocation, so `designs/gateway/gateway.edn` reads the same
	// from either working directory.
	FromRoot bool `yaml:"from_root"`
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
	// Steps replaces Script when a lesson is several commands and each one's OUTPUT is part of the
	// point. One script produced one capture, so a two-command lesson rendered both commands and then
	// both outputs, and a reader had to work out which half answered which. Four specs papered over it
	// with `echo "no dot:"` labels between the runs, which is the workaround this field removes.
	//
	// Boundaries are DECLARED rather than inferred. Inferring them means either splitting the script on
	// newlines, which writes a marker into the middle of the four specs that build a fixture with a
	// heredoc, or mapping output back onto `show`, which cannot work: every spec that sets both has a
	// different line count in each, since `show` is what hides the `echo` labels and the `| grep` in
	// the first place.
	//
	// Each step runs as its own shell in the SAME scratch directory, so a step reads what an earlier
	// one wrote (rung 11 stores a results document and then re-renders it) without the runner having
	// to keep one shell alive and delimit its output. Shell variables would not carry across, and
	// nothing here uses them.
	//
	// Script and Steps are mutually exclusive; a spec sets whichever fits. Capture and Match apply to
	// every step, Exit appends to the last.
	Steps []runStep `yaml:"steps"`
}

// runStep is one command and the output it produced, which the page renders as its own block.
type runStep struct {
	// Script is the shell to run, Show what the reader should see, defaulting to Script. Same split and
	// same reasoning as the spec-level pair.
	Script string `yaml:"script"`
	Show   string `yaml:"show"`
}

// steps normalizes a spec to the list the runner and the renderer both walk. A spec with a bare
// `script` is the one-step case, which is 80 of the 96 specs, and it keeps rendering and capturing
// byte-identically so adding this field regenerated nothing.
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

// stepDelim separates one step's captured output from the next inside a single capture file.
//
// The commands themselves are NOT written here. They live in the spec, which the stamp already
// covers, so the capture stays a file of nothing but output and its diff reads as one.
//
// A one-step spec writes no delimiter at all, which is why adding steps left all 80 single-command
// captures byte-identical.
const stepDelim = "#agni-step\n"

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
	// A capture whose section count no longer matches the spec's step count is stale even when the
	// stamp says otherwise, which is what a hand-edited capture looks like. Regenerating beats
	// rendering a command against another command's output.
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

// validate rejects the two ways a spec can be self-contradictory, rather than silently preferring one
// field over the other and rendering a lesson nobody wrote.
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
		// No stamp: an output someone hand-edited. Treat it as stale rather than trusting it, so the
		// generator remains the only author.
		return nil, "", fmt.Errorf("no stamp")
	}
	return strings.Split(rest, stepDelim), strings.TrimPrefix(first, stampPrefix), nil
}

// inputHash covers the spec and every TRACKED file in its fixture, so any COMMITTED edit to either
// regenerates and nothing a working tree happens to contain can move it.
//
// Tracked rather than "every file on disk", because the stamp is written into a file that is itself
// committed, and a hash of the working tree makes that committed value right for one machine. The
// tutorial's own Makefile has a `report` target writing examples/tutorial-project/reports/, which its
// .gitignore covers, so every reader who followed the tutorial hashed two files nobody else had.
// Their gate runs rewrote a committed output they had not touched, and `git checkout --` on it became
// part of the routine (agni issue 357).
//
// Regenerating the output would have moved the staleness rather than fixed it: the new stamp would
// have been right for a tree that had run the tutorial and wrong for every clean checkout.
func inputHash(spec []byte, fixture string) (string, error) {
	h := sha256.New()
	h.Write(spec)
	if fixture == "" {
		return hex.EncodeToString(h.Sum(nil))[:16], nil
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

// trackedFiles lists a fixture's git-tracked files, repo-root-relative and in git's own sorted order
// so the hash is deterministic.
//
// It REFUSES rather than falling back to walking the directory. A fallback would restore the exact
// bug this exists to fix, and restore it invisibly: the stamp would start depending on the working
// tree again with nothing on screen to say so. A fixture with no tracked files is the same mistake
// wearing a different hat, so an empty listing is an error too, not a hash of no content.
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

// execute runs the spec's script in a scratch copy of its fixture and applies the spec's capture
// rules, returning the block body.
//
// The stream selection, exit code and line filter are applied HERE rather than by shell plumbing in
// the script. That is the whole point of them being fields: a spec says what the lesson is about, and
// the fragile mechanics of getting there are written once, in Go, where they can be tested.
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
		// Where the copy LANDS is what decides how paths read in the block. Under from_root it keeps
		// its full relative path and the script runs at the scratch root, so `examples/x/y.edn` is
		// both what runs and what a reader can type; otherwise it lands as a bare basename and the
		// script runs inside it, which is the tutorials' cd-once-then-work-relative shape.
		dest := filepath.Join(dir, filepath.Base(spec.Fixture))
		if spec.FromRoot {
			dest = filepath.Join(dir, spec.Fixture)
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return nil, fmt.Errorf("preparing fixture path: %w", err)
			}
		} else {
			work = dest
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
	// A SHELL is still needed: a step may itself be several commands (rung 11 moves a directory before
	// running anything), and those are the tutorial's content rather than plumbing. The script is
	// checked-in yaml from this repo own content tree, never input from elsewhere, so there is no
	// injection boundary: anyone who can edit a run spec can already edit this file.
	cmd := exec.Command("sh", "-c", step.Script)
	cmd.Dir = work
	cmd.Env = captureEnv(dir, bin)
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
	// A filter runs against what a step actually printed, and a step that printed NOTHING is not a
	// filter that stopped matching. Rung 11's first step redirects its report to /dev/null because the
	// lesson is that the results file it also wrote can be re-rendered without the design, so its
	// capture is empty by design and running the pattern over it would fail the build.
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
	// The scratch directory must not survive into a committed capture. It is machine-specific and
	// changes every run, so leaving it in would churn the file on every regeneration and put a host
	// path in a public repo. `agni query` prints one: its provenance column resolves the design to an
	// absolute path, so a capture taken here would read `/var/folders/.../tutorial-project/designs/...`
	// where the page means `designs/...`.
	// BOTH forms of the scratch path, because a temp dir is reached through a symlink on macOS
	// (/var/folders/... resolves to /private/var/folders/...) and a command printing the resolved form
	// leaves the unresolved replacement useless. Getting this wrong is not subtle-but-harmless: the
	// first attempt stripped the middle of the resolved path and produced "/privatedesigns/gateway",
	// which is neither a real path nor an obvious mistake at a glance.
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

// scratchForms returns the scratch directory as a command might print it: as handed to the process,
// and with symlinks resolved.
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
		// From the repo root: docsite/ is its own module and cmd/agni is not one of its dependencies.
		cmd.Dir = ".."
		if out, err := cmd.CombinedOutput(); err != nil {
			agniBuildErr = fmt.Errorf("building agni: %v: %s", err, out)
		}
	})
	return agniBuildPath, agniBuildErr
}

// captureEnv is the environment a captured run sees, built rather than inherited.
//
// These outputs are COMMITTED, so whatever the machine running this target happens to have configured
// is what a reader of the docs ends up seeing. Inheriting os.Environ() made a capture a function of
// the operator as well as of the fixture, in two ways with very different costs.
//
// HOME and XDG_CONFIG_HOME reach agni.yaml, so a developer with one folded a
// `note: using N mount(s) ... from ~/.config/agni/agni.yaml` line into unrelated captures. Cosmetic,
// and it names a path out of someone's home directory in a public repo.
//
// AGNI_SYMBOL_PATH is the one that matters. It changes what a read RESOLVES, so a schematic naming
// external symbols reads more completely on a configured machine than on a bare one: different pins,
// different nets, different findings. A capture regenerated locally would disagree with the same
// capture regenerated in CI, and both would look correct.
//
// So the run gets a scratch HOME inside the same temp directory the fixture copy lives in, and only
// the variables a shell genuinely needs. Anything agni reads from the environment is absent by
// construction rather than by a deny-list this function would have to keep up to date.
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
