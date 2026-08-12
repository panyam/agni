package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// This file runs the tutorials against the fixture they claim to come from.
//
// It exists because nothing did, and the drift was not hypothetical: rung 9's coverage table showed
// `8 fail, 1 n/a` where the fixture produced `9 fail, 0 n/a`, with a Board row that contradicted the
// Total row in the same table, and rung 8 carried the same numbers plus a board example that had
// stopped being true when designs gained declared companions. Both were caught by accident, while
// regenerating them for an unrelated change.
//
// The existing gates cannot see this. `nav_test.go` checks page WIRING — index, nav template, header
// links, sidebar dispatch — and never reads a page's body. `catalog-docs-check` is git-status based
// and covers only the GENERATED reference pages. A tutorial is hand-authored prose whose promise to
// the reader is "run this, see this", and that promise was unverified.
//
// Verification is OPT-IN, marked on the fence:
//
//	```console verify
//	$ agni check designs/gateway --fail-on error
//	$ echo $?
//	2
//	```
//
// Opt-in rather than opt-out because the blocks are heterogeneous. Many show a command with no
// output, several show an excerpt rather than the whole thing, and `agni serve` never returns. An
// opt-out scheme would need more markers than this one, added under pressure, and a missed one would
// hang the gate rather than fail it.
//
// A marked block is checked STRICTLY: every line the doc shows is a line the command produced. That
// is the whole value, so a block that cannot survive it should stay unmarked rather than be softened.
//
// Only some blocks are marked today: the four that had already drifted, plus the exit-code
// transcripts most likely to. Marking a rung's remaining blocks is incremental work, best done when
// that rung is next edited.

// tutorialFixture is the project every marked block runs against, relative to this package.
const tutorialFixture = "../examples/tutorial-project"

var (
	buildOnce sync.Once
	agniBin   string
	buildErr  error
)

// agniBinary builds the CLI once per test run and returns its path.
//
// Built rather than `go run`, because a marked block may invoke agni several times and paying the
// compile once keeps the check cheap enough to sit in the default gate.
func agniBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "agni-tutorial")
		if err != nil {
			buildErr = err
			return
		}
		agniBin = filepath.Join(dir, "agni")
		// Built from the REPO ROOT, because docsite/ is its own Go module and cmd/agni is not one of its
		// dependencies. Naming the parent's package from here would be outside this module.
		cmd := exec.Command("go", "build", "-o", agniBin, "./cmd/agni")
		cmd.Dir = ".."

		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = err
			agniBin = string(out)
		}
	})
	if buildErr != nil {
		t.Fatalf("building the CLI for tutorial verification: %v\n%s", buildErr, agniBin)
	}
	return agniBin
}

// verifiedBlock is one marked fenced block: the shell lines to run and the output they must produce.
type verifiedBlock struct {
	file    string
	line    int // 1-based line of the opening fence, so a failure points at the block
	script  string
	expect  string
	rawInfo string
}

// parseVerifiedBlocks extracts the marked blocks from one markdown file.
//
// A `$ `-prefixed line is a command; everything else is expected output. The two are separated rather
// than interleaved because the check compares the WHOLE block's stdout against the whole expected
// text: a transcript that interleaved them per command would have to model which output belonged to
// which line, and the tutorials do not write them that way.
func parseVerifiedBlocks(file, content string) []verifiedBlock {
	var out []verifiedBlock
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "```") {
			continue
		}
		info := strings.TrimSpace(strings.TrimPrefix(lines[i], "```"))
		start := i
		// Find the closing fence.
		var body []string
		i++
		for ; i < len(lines) && !strings.HasPrefix(lines[i], "```"); i++ {
			body = append(body, lines[i])
		}
		if !strings.Contains(info, "verify") {
			continue
		}
		var script, expect []string
		for _, l := range body {
			if cmd, ok := strings.CutPrefix(l, "$ "); ok {
				script = append(script, cmd)
			} else {
				expect = append(expect, l)
			}
		}
		out = append(out, verifiedBlock{
			file: file, line: start + 1, rawInfo: info,
			script: strings.Join(script, "\n"),
			expect: strings.TrimRight(strings.Join(expect, "\n"), "\n"),
		})
	}
	return out
}

// TestTutorialsMatchTheFixture runs every `verify`-marked block and compares its stdout to the output
// the page promises.
//
// STDOUT only. Resolution notes ("reading gateway.edn, the entry designs/gateway/design.yaml
// declares") go to stderr precisely so a redirect of a report stays clean, and the tutorials do not
// paste them. Comparing combined output would make every marked block carry noise the page does not
// show.
func TestTutorialsMatchTheFixture(t *testing.T) {
	pages, err := filepath.Glob("content/tutorials/*.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Fatal("no tutorial pages found; this test would silently verify nothing")
	}
	var blocks []verifiedBlock
	for _, p := range pages {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		blocks = append(blocks, parseVerifiedBlocks(p, string(b))...)
	}
	if len(blocks) == 0 {
		t.Fatal("no ```console verify blocks found; the marker or the glob is wrong, and this test " +
			"would pass while checking nothing")
	}

	bin := agniBinary(t)
	fixture, err := filepath.Abs(tutorialFixture)
	if err != nil {
		t.Fatal(err)
	}
	// agni is put on PATH rather than substituted into the script, so a block reads exactly as a user
	// would type it. A tutorial that had to say `/tmp/build/agni check` would be teaching a lie.
	path := filepath.Dir(bin) + string(os.PathListSeparator) + os.Getenv("PATH")

	for _, b := range blocks {
		t.Run(b.file+":"+itoa(b.line), func(t *testing.T) {
			// A SHELL is genuinely needed: marked blocks use `echo $?` to show a gate exit code, and some
			// pipe through `head`. The script is checked-in markdown from this repo own
			// content/tutorials/, never input from elsewhere, so there is no injection boundary here:
			// anyone who can edit a tutorial page can already edit this test.
			cmd := exec.Command("sh", "-c", b.script)
			cmd.Dir = fixture
			cmd.Env = append(os.Environ(), "PATH="+path)
			var stdout strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = nil
			// The exit status is deliberately ignored: several blocks demonstrate a GATE tripping, so a
			// non-zero exit is the lesson rather than a failure. What must match is the output.
			_ = cmd.Run()
			got := strings.TrimRight(stdout.String(), "\n")
			if got != b.expect {
				t.Errorf("%s:%d is out of date with the fixture.\n\n--- the page says ---\n%s\n\n--- the command produced ---\n%s\n\nRegenerate the block by running, in %s:\n%s",
					b.file, b.line, b.expect, got, tutorialFixture, b.script)
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
