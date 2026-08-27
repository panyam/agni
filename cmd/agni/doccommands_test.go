package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// docContentDir is the docsite content tree, relative to this package.
const docContentDir = "../../docsite/content"

// docCommandCount is how many `agni ...` commands the extractor finds in hand-written fences across
// the whole content tree. Asserted so a block written in a shape the extractor does not recognise
// shows up as a coverage DROP rather than as silence, which is the failure mode of every scanner
// that reports only on what it managed to read.
//
// It moves when a page gains or loses a hand-written command, and it goes DOWN when a fence converts
// to a generated capture (an agniRun spec runs its command for real, so it needs no parse check).
// Either way, update it deliberately.
const docCommandCount = 35

// TestDocumentedCommandsParse runs every hand-written `agni ...` line in the docsite through the
// real command tree, checking that the command and its flags exist.
//
// The docsite carries two kinds of command and only one of them was checked. A run spec under
// `runs/*.yaml` is EXECUTED, and `tutorial-runs-check` regenerates every capture on each gate run, so
// a command that stops working fails the build. A command typed into a fenced block was checked by
// nothing, and three PRs that each changed how a command behaves updated the page documenting the
// flag while leaving the pages that USE it. Four tutorial commands stopped working and the ladder
// still read as though they did (agni issue 471).
//
// This checks that a command PARSES, not that it runs, and the difference is the whole reason it can
// exist at all. Several of these commands cannot be executed by a test: `agni serve` blocks until
// interrupted, some name a board the reader has rather than a fixture in this repo, and one is on the
// page precisely to show a mistake. Parsing needs no fixture, no server, and no terminating process,
// and every one of the four regressions was an unknown subcommand or an unknown flag.
//
// What it does NOT catch is a command that parses and does the wrong thing. `--format jsn` is a
// well-formed string for a flag that takes one, so cobra accepts it and so does this. A fence whose
// output matters wants an agniRun spec instead, which runs it.
func TestDocumentedCommandsParse(t *testing.T) {
	cmds := docCommands(t)
	if len(cmds) != docCommandCount {
		t.Errorf("found %d documented commands, expected %d. A fence written in a shape the extractor "+
			"does not recognise is invisible to this test, so a drop is a coverage loss rather than a "+
			"cleanup; update docCommandCount only once you know which way it moved", len(cmds), docCommandCount)
	}
	for _, c := range cmds {
		t.Run(c.where, func(t *testing.T) {
			if err := parseArgs(c.args); err != nil {
				t.Errorf("%s\n  %s\n  %v", c.where, strings.Join(c.args, " "), err)
			}
		})
	}
}

// parseArgs resolves argv against the real command tree and validates it without running anything.
//
// The three steps are separate because each catches a different regression and cobra reports them at
// different moments. Find resolves the subcommand path, ParseFlags rejects a flag that no longer
// exists, and ValidateArgs rejects a positional the command does not take, which is what an
// `agni serve web` reads as once serve stopped accepting one.
func parseArgs(argv []string) error {
	root := rootCmd()
	root.SetArgs(argv)
	target, rest, err := root.Find(argv)
	if err != nil {
		return err
	}
	// --help is a documented thing to type and pflag reports it as an error sentinel rather than a
	// parse failure, so getting-started's first command would otherwise fail for succeeding.
	if err := target.ParseFlags(rest); err != nil && !errors.Is(err, pflag.ErrHelp) {
		return err
	}
	return target.ValidateArgs(target.Flags().Args())
}

// docCommand is one command as a page shows it, with where to find it.
type docCommand struct {
	where string
	args  []string
}

// docCommands extracts every hand-written `agni ...` command from a fenced block in the content tree.
//
// The awkward part is telling a command block from an OUTPUT block, since a page shows both and
// `agni version` prints a first line that begins with the word agni. Two rules settle it. A fence
// containing any `$ `-prefixed line is a transcript, so only the prompted lines are commands. And a
// line indented under a line that did not end in a backslash is output, which disqualifies the whole
// fence: that is exactly the shape of `agni v0.1.1` followed by its indented build detail.
func docCommands(t *testing.T) []docCommand {
	t.Helper()
	var out []docCommand
	seen := map[string]int{}
	err := filepath.WalkDir(docContentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(docContentDir, path)
		for _, fence := range fencedBlocks(string(b)) {
			for _, line := range shellCommands(fence) {
				argv := shellSplit(line)
				if len(argv) == 0 || argv[0] != "agni" {
					continue
				}
				seen[rel]++
				out = append(out, docCommand{
					where: fmt.Sprintf("%s#%d", rel, seen[rel]),
					args:  argv[1:],
				})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", docContentDir, err)
	}
	if len(out) == 0 {
		t.Fatalf("no commands found under %s at all; the extractor or the path is wrong", docContentDir)
	}
	return out
}

// fencedBlocks returns the body of every ``` fenced block in a markdown document.
func fencedBlocks(md string) []string {
	var out []string
	var cur []string
	in := false
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if in {
				out = append(out, strings.Join(cur, "\n"))
				cur = nil
			}
			in = !in
			continue
		}
		if in {
			cur = append(cur, line)
		}
	}
	return out
}

// shellCommands returns the command lines in one fenced block, joining backslash continuations and
// returning nothing at all for a block that turns out to be output. See docCommands for the rules.
func shellCommands(fence string) []string {
	var lines []string
	for _, l := range strings.Split(fence, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	prompted := false
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "$ ") {
			prompted = true
		}
	}
	var out []string
	var cur string
	curPrompted := false
	continuing := false
	keep := func(cmd string, wasPrompted bool) {
		if !prompted || wasPrompted {
			out = append(out, cmd)
		}
	}
	for _, l := range lines {
		if continuing {
			cur += " " + strings.TrimSuffix(strings.TrimSpace(l), "\\")
			if !strings.HasSuffix(strings.TrimRight(l, " \t"), "\\") {
				keep(cur, curPrompted)
				continuing = false
			}
			continue
		}
		if l[0] == ' ' || l[0] == '\t' {
			return nil // indented with no continuation to belong to: this fence is output
		}
		s := strings.TrimSpace(l)
		wasPrompted := false
		if after, ok := strings.CutPrefix(s, "$ "); ok {
			s, wasPrompted = strings.TrimSpace(after), true
		}
		if strings.HasPrefix(s, "#") {
			continue
		}
		if strings.HasSuffix(s, "\\") {
			cur, curPrompted, continuing = strings.TrimSpace(strings.TrimSuffix(s, "\\")), wasPrompted, true
			continue
		}
		keep(s, wasPrompted)
	}
	if continuing {
		keep(cur, curPrompted)
	}
	return out
}

// shellSplit tokenizes a command line the way a shell would for the parts that matter here: quoting,
// then a trailing comment or redirect. Both are stripped only OUTSIDE quotes, because the datalog
// query on rules-and-checks.md carries `<`, `>` and `=>` inside its quoted program.
//
// `<` is NOT a terminator even unquoted. No page redirects input, and every unquoted `<` in the tree
// opens a placeholder (`agni check <file> --fail-on error`), which has to survive as a positional or
// the command loses the argument it is there to show.
func shellSplit(line string) []string {
	var args []string
	var cur strings.Builder
	var quote rune
	started := false
	flush := func() {
		if started {
			args = append(args, cur.String())
			cur.Reset()
			started = false
		}
	}
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, started = r, true
		case r == ' ' || r == '\t':
			flush()
		case (r == '#' || r == '>' || r == '|') && !started:
			flush()
			return args // a comment or a redirect: the command ends here
		default:
			cur.WriteRune(r)
			started = true
		}
	}
	flush()
	return args
}
