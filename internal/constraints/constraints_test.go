package constraints

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// repoRoot is two levels up from internal/constraints. Every check here reads the tree rather than
// the package graph alone, so it needs the module root rather than a package path.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Join(wd, "..", "..")
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("go.mod not found at %s: %v", root, err)
	}
	return root
}

// goList runs `go list -deps` over a pattern and returns the import paths it names.
//
// It fails on a result naming no package under want, which is the positive control: a graph check
// reads as clean when the pattern matched nothing, and a mistyped pattern is exactly the edit that
// would produce that. Without this the layering tests would still pass after the tier was renamed
// out from under them.
func goList(t *testing.T, pattern, want string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", pattern).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pattern, err, out)
	}
	deps := strings.Fields(string(out))
	if !slices.ContainsFunc(deps, func(d string) bool { return strings.HasPrefix(d, want) }) {
		t.Fatalf("go list -deps %s named no package under %s, so this check proves nothing", pattern, want)
	}
	return deps
}

// nonTestSources returns every .go file under dir that is not a test file.
func nonTestSources(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return out
}

// C6: each reader declares its fidelity, in the package doc of the file that owns Read.
//
// The declaration is a comment rather than a type because what it has to say is prose: WHICH subset
// survives the read and what is dropped. A `Fidelity` enum would record the word "lossy-bounded" and
// none of the bound. This test asserts the line exists, which is the half a machine can check;
// whether the bound it states is honest is a review question.
//
// It is here because readers/telesis shipped without one in August 2026 and nothing said so. The
// constraint had carried no Verify at all since it was written.
func TestC6EveryReaderDeclaresFidelity(t *testing.T) {
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "readers"))
	if err != nil {
		t.Fatalf("read readers/: %v", err)
	}
	for _, e := range entries {
		// formats is the registry and the loader, not a reader; it parses nothing itself.
		if !e.IsDir() || e.Name() == "formats" {
			continue
		}
		dir := filepath.Join(root, "readers", e.Name())
		found := false
		for _, f := range nonTestSources(t, dir) {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			if strings.Contains(string(b), "Fidelity:") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("readers/%s declares no fidelity contract (C6): no non-test file carries a "+
				"`// Fidelity: ...` line stating which subset the read preserves", e.Name())
		}
	}
}

// C17: the reader tier depends downward only. A reader produces IR and geom; it never reaches up
// into the presentation tier or the application tail.
//
// This subsumes what C15 said about render/svg specifically. A stray import from a reader into
// internal/server would pull servicekit and connect into every consumer of the reader tier and
// foreclose extracting it as its own module, so the check is over the transitive graph rather than
// over the import blocks.
func TestC17ReaderTierDependsDownwardOnly(t *testing.T) {
	forbidden := []string{
		"servicekit",
		"connectrpc",
		"github.com/panyam/agni/core/render",
		"github.com/panyam/agni/core/svg",
		"github.com/panyam/agni/serve",
		"github.com/panyam/agni/service",
		"github.com/panyam/agni/internal/server",
	}
	for _, dep := range goList(t, "github.com/panyam/agni/readers/...", "github.com/panyam/agni/readers/") {
		for _, f := range forbidden {
			if strings.Contains(dep, f) {
				t.Errorf("the reader tier pulls %q (C17): readers depend downward on the contract "+
					"and shared parse/geom helpers only", dep)
			}
		}
	}
}

// C17, second half: the generated contract imports no first-party package, so a consumer can take
// gen/ alone. Anything under gen/ importing back into agni would make the contract carry the tree.
func TestC17ContractImportsNoFirstPartyPackage(t *testing.T) {
	const self = "github.com/panyam/agni/"
	for _, dep := range goList(t, "github.com/panyam/agni/gen/...", "github.com/panyam/agni/gen/") {
		if strings.HasPrefix(dep, self) && !strings.Contains(dep, "/gen/") {
			t.Errorf("the generated contract pulls the first-party package %q (C17)", dep)
		}
	}
}

// C18: dependencies point overlay -> engine. The engine module requires no overlay module.
//
// The graph half of this constraint cannot fail and so is not tested: examples/overlay is its own
// module, so `go list -deps ./...` from the engine can never name it whatever anyone writes. What
// CAN change is go.mod, which is how an overlay would actually get pulled in, so that is what this
// reads. A `replace` counts as much as a `require`: it is the edit that makes a local overlay
// resolvable, and it is the one somebody adds while debugging and forgets to remove.
func TestC18EngineModuleRequiresNoOverlay(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for i, line := range strings.Split(string(b), "\n") {
		code, _, _ := strings.Cut(line, "//")
		if !strings.Contains(code, "panyam/agni/") {
			continue
		}
		t.Errorf("go.mod:%d names a module inside this repo (C18): %q. The engine is composed BY an "+
			"overlay through formats.Register and check.RegisterSource, never coupled to one.",
			i+1, strings.TrimSpace(line))
	}
}

// C20: convention interpretation is left-shifted to ingestion, so the check path reads the stamped
// fact rather than re-running name matching per entity per rule.
//
// The sweep is for the process-wide ACTIVE vocabulary, which is the spelling a rule reaches for when
// it wants to ask "is this ground" and does not know the model already answers it. That is two
// violations in one call: the convention matching runs in the hot path (C20) and the vocabulary
// arrives as ambient state rather than travelling with the run (C22). Read it off the model instead,
// through m.IsGroundNet, m.IsRailNet or check.NetHasRole, which trust ir.Net.roles and fall back to
// THIS model's lexicon for an IR that skipped the loader.
//
// The one call site this would have caught is stdlib/rules/intent/protections.go, whose groundRefs
// matched ground net names itself; both left-shift tickets (WS3-071, WS3-072) had landed by then and
// the constraint's Verify still read "becomes checkable once those land".
func TestC20CheckPathReadsStampedFacts(t *testing.T) {
	root := repoRoot(t)
	// check.ActiveRoleVocab and friends are aliases of the classify originals, so both spellings
	// reach the same package-level state and both have to be named here.
	forbidden := []string{"classify.Active", "check.Active"}
	for _, tier := range []string{"stdlib/rules", "stdlib/relations"} {
		for _, f := range nonTestSources(t, filepath.Join(root, tier)) {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			rel, _ := filepath.Rel(root, f)
			for _, bad := range forbidden {
				if strings.Contains(string(b), bad) {
					t.Errorf("%s calls %s (C20/C22): the check path reads the role and class facts "+
						"stamped at ingestion, through the model (m.IsGroundNet, m.IsRailNet, "+
						"check.NetHasRole), not the process-wide vocabulary", rel, bad)
				}
			}
		}
	}
}

// C12: render style is injectable data, so the render layer carries no bare colour or font literal.
//
// The sweep is narrower than the constraint's prose in two ways, both deliberate. Test files are
// excluded, because a test asserting that a custom style reaches the output has to name a colour to
// assert on. And it matches `font-family=` rather than the bare word: the renderers pass
// "font-family" to svg.A as an ATTRIBUTE NAME with style.Font as the value, which is the rule being
// obeyed rather than broken. The Verify in CONSTRAINTS.md read on the bare word until the audit, and
// returned two dozen hits on a clean tree.
func TestC12RenderLayerHasNoStyleLiterals(t *testing.T) {
	root := repoRoot(t)
	for _, f := range nonTestSources(t, filepath.Join(root, "core", "render")) {
		// style.go IS the one source of truth, so it is where the literals belong.
		if filepath.Base(f) == "style.go" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		rel, _ := filepath.Rel(root, f)
		for i, line := range strings.Split(string(b), "\n") {
			if styleLiteral.MatchString(line) {
				t.Errorf("%s:%d carries a colour or font literal (C12): %q. Resolve it from "+
					"render.Style so the SVG and WebGL paths agree by construction.",
					rel, i+1, strings.TrimSpace(line))
			}
		}
	}
}

var styleLiteral = regexp.MustCompile(`"#[0-9a-fA-F]{3,8}"|font-family=`)

// C24: a datasheet parameter is compared in SI base units, converted in one place, so the RAW unit a
// vendor printed is read only inside datasheet/param.
//
// This is a ratchet rather than a clean sweep, the shape hack/ir_model_baseline.txt uses for C19,
// because the invariant a grep can express ("nothing reads p.Unit") is not the invariant that
// matters ("nothing COMPARES on p.Unit"). Both sites below read the printed unit to PUBLISH it,
// which is the whole point of the param.unit relation and of what `agni params` prints, and neither
// compares anything.
//
// A new site failing here is one of two things. If it compares, it is the bug the constraint
// exists for: convert through datasheet/param first. If it displays, add it here, and that addition
// is the review moment the constraint is asking for.
func TestC24RawUnitIsReadOnlyToDisplay(t *testing.T) {
	allowed := map[string]bool{
		// the param.unit relation: what the vendor actually printed, beside the converted number
		"stdlib/relations/facts.go": true,
		// the parameters table `agni params <mpn>` prints
		"cmd/agni/params.go": true,
	}
	root := repoRoot(t)
	for _, tier := range []string{"core", "stdlib", "readers", "internal", "cmd"} {
		for _, f := range nonTestSources(t, filepath.Join(root, tier)) {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			rel, _ := filepath.Rel(root, f)
			if allowed[filepath.ToSlash(rel)] || !rawUnitRead.Match(b) {
				continue
			}
			t.Errorf("%s reads a parameter's printed unit (C24). A COMPARISON converts through "+
				"datasheet/param first; a DISPLAY belongs in this test's allowlist.", rel)
		}
	}
}

var rawUnitRead = regexp.MustCompile(`\bp\.(Unit\b|GetUnit\(\))`)

// C13: the service tier is importable, so no part of it sits behind the module boundary.
//
// The transport and filesystem halves of C13 are tested where they belong, in
// service/transport_guard_test.go. What is left over is the one thing that is about the tier's
// LOCATION rather than its imports: service/ and artifact/ moved out of internal/ so an embedder
// could compose against them, and moving them back would still compile, since every in-repo caller
// would move with them.
//
// The check is that these exact import paths resolve, which is what an embedder outside this module
// writes. Listing a pattern is not enough on its own: a tier that moved under internal/ makes the
// pattern match nothing, and a check that only looks at what came back would read that as clean.
func TestC13ServiceTierIsImportable(t *testing.T) {
	want := []string{"github.com/panyam/agni/service", "github.com/panyam/agni/artifact"}
	out, err := exec.Command("go", append([]string{"list"}, want...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("the service tier is not importable at its published paths (C13): %v\n%s", err, out)
	}
	got := strings.Fields(string(out))
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("go list did not name %q (C13): the tier an embedder composes against cannot "+
				"drift back behind the module boundary", w)
		}
	}
	for _, pkg := range got {
		if strings.Contains(pkg, "/internal/") {
			t.Errorf("%q sits behind the module boundary (C13)", pkg)
		}
	}
}

// C22: an artifact is named by a single URI, so the web API carries no locator the callee resolves.
//
// A `mount` + `path` pair, or a bare `*_path` / `*_ref` field, is a locator the caller half-resolves
// and the callee finishes, which is the shape that let a config mean one thing to the CLI and
// another to the service. `ref_des` and `cell_refs` are domain names rather than locators and do not
// match: the rule is about the exact `_path` / `_ref` suffix.
func TestC22WebAPICarriesNoLocatorPair(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "protos", "agni", "v1", "webapi")
	files, err := filepath.Glob(filepath.Join(dir, "*.proto"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no protos under %s (%v), so this check proves nothing", dir, err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		rel, _ := filepath.Rel(root, f)
		for i, line := range strings.Split(string(b), "\n") {
			code, _, _ := strings.Cut(line, "//")
			if locatorField.MatchString(code) {
				t.Errorf("%s:%d declares a locator field (C22): %q. An artifact travels as one "+
					"`uri` (or `*_uri` where a message names more than one), never as a mount plus a "+
					"path the callee resolves.", rel, i+1, strings.TrimSpace(line))
			}
		}
	}
}

var locatorField = regexp.MustCompile(`^\s*(repeated\s+|optional\s+)?[A-Za-z0-9_.]+\s+[a-z0-9_]*_(path|ref)\s*=\s*[0-9]+`)

// C25: a run's recorded provenance is derived from the resolved overlay, never from the caller's
// flags, so exactly one non-test site builds the RunConfig a results document records.
//
// Test files are excluded because a fixture legitimately builds a document to render
// (core/results/results_test.go); the rule is about who WRITES a run's record.
func TestC25RunConfigHasOneWriter(t *testing.T) {
	assertSoleWriter(t, "checkspb.RunConfig{", "service/projectoverlay.go",
		"the RunConfig a results document records is derived from the resolved overlay (C25), not "+
			"assembled from whatever flags the caller passed")
}

// C28: a recorded locator names the artifact, never the machine that produced it, so the read-time
// rename has one implementation and one call site per read entry point.
//
// Acceptance for the behaviour is TestCheckProvenanceIsMountRelative (cmd/agni), which asserts the
// shape on both the printed and the STORED document. This guards the structure that makes it hold:
// a second implementation elsewhere is how a new output format or a new host starts publishing host
// paths again.
func TestC28SourceRenameHasOneImplementation(t *testing.T) {
	assertSoleWriter(t, "relocateSources", "readers/formats/",
		"the read-time locator rename has one implementation (C28), so a new output format inherits "+
			"it rather than reimplementing it")
}

// assertSoleWriter fails on any non-test file outside prefix that names needle.
func assertSoleWriter(t *testing.T, needle, prefix, why string) {
	t.Helper()
	root := repoRoot(t)
	found := 0
	for _, tier := range []string{"core", "stdlib", "readers", "internal", "cmd", "service", "datasheet", "intake", "artifact", "census"} {
		for _, f := range nonTestSources(t, filepath.Join(root, tier)) {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			if !strings.Contains(string(b), needle) {
				continue
			}
			rel := filepath.ToSlash(mustRel(t, root, f))
			if strings.HasPrefix(rel, prefix) {
				found++
				continue
			}
			t.Errorf("%s names %s: %s", rel, needle, why)
		}
	}
	// The positive control: a rename or a deletion would otherwise leave this reading as clean.
	if found == 0 {
		t.Errorf("no file under %s names %s, so this check proves nothing", prefix, needle)
	}
}

func mustRel(t *testing.T, base, target string) string {
	t.Helper()
	rel, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatalf("rel(%s, %s): %v", base, target, err)
	}
	return rel
}
