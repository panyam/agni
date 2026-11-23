package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	gproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/service"
)

func runPacks(t *testing.T, url string, args ...string) (string, error) {
	t.Helper()
	cmd := packsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append(args, "--remote", url))
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

// gatewayIntent is a design.yaml intent section the gateway fixture reads cleanly, with one net in
// the wrong domain so an intent rule has something to report.
const gatewayIntent = `intent:
  modules:
    - {name: regulators, class: regulator, count: 2}
  nets:
    PMIC_MAIN_12V0: {nominal: 12.0, domain: main}
    PMIC_CORE_3V3: {nominal: 3.3, domain: io}
    PMIC_IO_1V8: {nominal: 3.3, domain: core}
`

// tutorialProject copies the tutorial project's shared config into a folder of its own, with the
// gateway's netlist and board as the design gw. The design extends the project and declares
// gatewayIntent, as a laptop folder would.
func tutorialProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	src := "../../examples/tutorial-project"
	if err := os.CopyFS(root, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "boards")); err != nil {
		t.Fatal(err)
	}
	gw := filepath.Join(root, "boards", "gw")
	if err := os.MkdirAll(gw, 0o750); err != nil {
		t.Fatal(err)
	}
	for from, to := range map[string]string{gatewayRevA: "gateway.edn", "../../examples/tutorial-project/boards/gateway-rev-b/gateway-rev-b.kicad_pcb": "gateway.kicad_pcb"} {
		b, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gw, to), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	descriptor := "name: gw\ntitle: Sample Board\nextends: ../../project.yaml\nentry: gateway.edn\ncompanions: [gateway.kicad_pcb]\n" + gatewayIntent
	if err := os.WriteFile(filepath.Join(gw, "design.yaml"), []byte(descriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// findingKeys runs agni check with --format json and returns each finding as rule|kind:ref|severity,
// sorted.
func findingKeys(t *testing.T, args ...string) []string {
	t.Helper()
	out, err := runAgni(t, append(args, "--format", "json")...)
	if err != nil {
		t.Fatalf("agni %v: %v\n%s", args, err, out)
	}
	var doc struct {
		Findings []struct {
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
			Subject  struct {
				Kind string `json:"kind"`
				Ref  string `json:"ref"`
			} `json:"subject"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("agni %v printed no JSON: %v\n%s", args, err, out)
	}
	var keys []string
	for _, f := range doc.Findings {
		keys = append(keys, f.Rule+"|"+f.Subject.Kind+":"+f.Subject.Ref+"|"+f.Severity)
	}
	slices.Sort(keys)
	return keys
}

func hasRulePrefix(keys []string, prefix string) bool {
	return slices.ContainsFunc(keys, func(k string) bool { return strings.HasPrefix(k, prefix) })
}

// A stored design checked under a pack pushed from its project.yaml gives the findings agni check
// gives on its folder (agni issue 1704). The project's convention, profiles and params travel in the
// pack, and the design's intent travels with the design. The pack takes the project's name, so its
// profile rules are named as the folder's are.
func TestACheckUnderAPackMatchesTheFolder(t *testing.T) {
	srv := designServer(t, t.TempDir(), t.TempDir())
	project := tutorialProject(t)
	if out, err := runPacks(t, srv.URL, "push", project); err != nil {
		t.Fatalf("packs push: %v\n%s", err, out)
	} else if !strings.Contains(out, "profiles: profiles") || !strings.Contains(out, "params: params") {
		t.Fatalf("packs push printed %q; want its profiles and params tiers", out)
	}
	design := filepath.Join(project, "boards", "gw")
	if out, err := runDesigns(t, srv.URL, "push", design, "--id", "gw"); err != nil {
		t.Fatalf("designs push: %v\n%s", err, out)
	}

	local := findingKeys(t, "check", design)
	underPack := findingKeys(t, "check", "designs/gw", "--pack", "gateway", "--remote", srv.URL)
	bare := findingKeys(t, "check", "designs/gw", "--remote", srv.URL)

	if !hasRulePrefix(local, "gateway/") {
		t.Fatalf("check on the folder reports no gateway/ convention finding, so this test proves nothing about the pack: %v", local)
	}
	if !slices.Equal(underPack, local) {
		t.Errorf("check designs/gw --pack gateway = %d findings, check on the folder = %d; want the same set\nonly under the pack: %v\nonly on the folder: %v",
			len(underPack), len(local), minus(underPack, local), minus(local, underPack))
	}
	if slices.Equal(bare, local) || hasRulePrefix(bare, "gateway/") {
		t.Errorf("check designs/gw with no pack = the folder's findings; want the pack's convention missing, so the comparison above shows the pack applied")
	}
}

func minus(a, b []string) []string {
	var out []string
	for _, k := range a {
		if !slices.Contains(b, k) {
			out = append(out, k)
		}
	}
	return out
}

// A design keeps the intent its design.yaml declares, and a push of a design.yaml with no intent
// leaves it in place (agni issue 1704).
func TestDesignsPushKeepsTheIntent(t *testing.T) {
	ctx := context.Background()
	srv := designServer(t, t.TempDir(), t.TempDir())
	design := filepath.Join(tutorialProject(t), "boards", "gw")
	if out, err := runDesigns(t, srv.URL, "push", design, "--id", "gw"); err != nil {
		t.Fatalf("push: %v\n%s", err, out)
	}
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	get := func() *webapi.Design {
		t.Helper()
		resp, err := designs.GetDesign(ctx, connect.NewRequest(&webapi.GetDesignRequest{Name: "designs/gw"}))
		if err != nil {
			t.Fatal(err)
		}
		return resp.Msg
	}
	nets := get().GetConfig().GetIntent().GetNets()
	if len(nets) != 3 || nets["PMIC_IO_1V8"].GetDomain() != "core" {
		t.Fatalf("stored intent nets = %v; want the three gatewayIntent declares", nets)
	}
	if err := os.WriteFile(filepath.Join(design, "design.yaml"), []byte("name: gw\nentry: gateway.edn\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := runDesigns(t, srv.URL, "push", design, "--id", "gw"); err != nil {
		t.Fatalf("second push: %v\n%s", err, out)
	}
	if n := len(get().GetConfig().GetIntent().GetNets()); n != 3 {
		t.Errorf("after a push with no intent, the stored intent has %d nets; want the 3 kept", n)
	}
	if _, err := designs.UpdateDesign(ctx, connect.NewRequest(&webapi.UpdateDesignRequest{
		Design:     &webapi.Design{Name: "designs/gw", Config: &webapi.AnalysisConfig{ProfileUris: []string{"mount://x/p"}}},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"config"}},
	})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("UpdateDesign with mask config = %v; want INVALID_ARGUMENT, as a stored design keeps its intent alone", err)
	}
}

// A snapshot named for a design and packs looks its packs up again on a repeat ingest, so one deleted
// since is NOT_FOUND rather than the old read. Its id differs from the design's own (agni issue
// 1704).
func TestASnapshotRereadsWhenItsPackChanges(t *testing.T) {
	ctx := context.Background()
	srv := designServer(t, t.TempDir(), t.TempDir())
	if out, err := runDesigns(t, srv.URL, "push", gatewayFolder(t, gatewayRevA), "--id", "gw"); err != nil {
		t.Fatalf("push: %v\n%s", err, out)
	}
	packs := webapiconnect.NewPackServiceClient(http.DefaultClient, srv.URL)
	house := &webapi.Pack{Config: &webapi.AnalysisConfig{Checklists: []*webapi.NamedChecklist{{Name: "a"}}}}
	if _, err := packs.CreatePack(ctx, connect.NewRequest(&webapi.CreatePackRequest{PackId: "house", Pack: house})); err != nil {
		t.Fatal(err)
	}
	snaps := webapiconnect.NewSnapshotServiceClient(http.DefaultClient, srv.URL)
	id := service.DesignSnapshotID("designs/gw", "packs/house")
	if id == service.DesignSnapshotID("designs/gw") {
		t.Fatal("a design read under a pack has the design's own snapshot id")
	}
	ingest := func() (*webapi.Snapshot, error) {
		resp, err := snaps.Ingest(ctx, connect.NewRequest(&webapi.IngestRequest{SnapshotId: id, Snapshot: &webapi.Snapshot{Design: "designs/gw", Packs: []string{"packs/house"}}}))
		if err != nil {
			return nil, err
		}
		return resp.Msg, nil
	}
	first, err := ingest()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(first.GetPacks(), []string{"packs/house"}) {
		t.Fatalf("snapshot packs = %v; want the pack named", first.GetPacks())
	}
	if again, err := ingest(); err != nil || again.GetName() != first.GetName() {
		t.Fatalf("a repeat ingest = %v, %v; want the same snapshot", again, err)
	}
	if _, err := packs.DeletePack(ctx, connect.NewRequest(&webapi.DeletePackRequest{Name: "packs/house"})); err != nil {
		t.Fatal(err)
	}
	if _, err := ingest(); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("ingest under a deleted pack = %v; want NOT_FOUND naming the pack", err)
	}
}

// check --verdicts under a pack writes text and csv, and they match the same report on the folder
// (agni issue 1746). The verdicts carry what each section shows, so the CLI needs none of the pack's
// rules.
func TestVerdictsUnderAPackMatchTheFolder(t *testing.T) {
	srv := designServer(t, t.TempDir(), t.TempDir())
	project := tutorialProject(t)
	if out, err := runPacks(t, srv.URL, "push", project); err != nil {
		t.Fatalf("packs push: %v\n%s", err, out)
	}
	design := filepath.Join(project, "boards", "gw")
	if out, err := runDesigns(t, srv.URL, "push", design, "--id", "gw"); err != nil {
		t.Fatalf("designs push: %v\n%s", err, out)
	}
	for _, format := range []string{"text", "csv"} {
		underPack, err := runAgni(t, "check", "designs/gw", "--pack", "gateway", "--verdicts", "--format", format, "--remote", srv.URL)
		if err != nil {
			t.Fatalf("check --pack --verdicts --format %s: %v\n%s", format, err, underPack)
		}
		folder, err := runAgni(t, "check", design, "--verdicts", "--format", format)
		if err != nil {
			t.Fatalf("check --verdicts --format %s on the folder: %v\n%s", format, err, folder)
		}
		if !strings.Contains(folder, "gateway-profiles/") {
			t.Fatalf("the folder's %s verdicts name no gateway-profiles rule, so this test proves nothing about the pack", format)
		}
		if underPack != folder {
			t.Errorf("--verdicts --format %s under the pack differs from the folder's; want the same report", format)
		}
	}
}

// review designs/<d> --pack runs the pack's checklist, its first by default or one by name, and each
// item reads as review on the folder reads it (agni issue 1746).
func TestAReviewUnderAPackMatchesTheFolder(t *testing.T) {
	srv := designServer(t, t.TempDir(), t.TempDir())
	project := tutorialProject(t)
	if out, err := runPacks(t, srv.URL, "push", project); err != nil {
		t.Fatalf("packs push: %v\n%s", err, out)
	}
	design := filepath.Join(project, "boards", "gw")
	if out, err := runDesigns(t, srv.URL, "push", design, "--id", "gw"); err != nil {
		t.Fatalf("designs push: %v\n%s", err, out)
	}
	for _, named := range [][]string{nil, {"--checklist", "house"}} {
		underPack := reviewOutcomes(t, append([]string{"review", "designs/gw", "--pack", "gateway", "--remote", srv.URL}, named...)...)
		folder := reviewOutcomes(t, append([]string{"review", design}, named...)...)
		if len(folder) == 0 || !slices.ContainsFunc(folder, func(k string) bool { return strings.HasSuffix(k, "|pass") || strings.HasSuffix(k, "|fail") }) {
			t.Fatalf("review %v on the folder answered no item, so this test proves nothing: %v", named, folder)
		}
		if !slices.Equal(underPack, folder) {
			t.Errorf("review %v under the pack differs from the folder\nonly under the pack: %v\nonly on the folder: %v", named, minus(underPack, folder), minus(folder, underPack))
		}
	}
	if out, err := runAgni(t, "review", design, "--pack", "gateway", "--remote", srv.URL); err == nil || !strings.Contains(err.Error(), "designs/<id>") {
		t.Errorf("review <folder> --pack = %v\n%s; want it refused, naming designs/<id>", err, out)
	}
}

// splitChecklistProject is tutorialProject with a second checklist, split, kept as a folder of two
// item files under checklist/ (agni issue 1812).
func splitChecklistProject(t *testing.T) string {
	t.Helper()
	project := tutorialProject(t)
	descriptor := filepath.Join(project, "project.yaml")
	b, err := os.ReadFile(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	split := "checklists:\n  split:\n    name: Power, one file per item\n    items_dir: checklist\n"
	if err := os.WriteFile(descriptor, []byte(strings.Replace(string(b), "checklists:\n", split, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"01-power/p1.md": "---\nid: P1\ntitle: every rail carries a bulk capacitor\nrule: bulk-cap\n---\n",
		"01-power/p2.md": "---\nid: P2\ntitle: every rail carries decoupling\nrule: decoupling-present\n---\n",
	} {
		full := filepath.Join(project, "checklist", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return project
}

// A checklist written as a folder of item files reviews the same under a pushed pack as on the
// folder, so the push carries the item files with the pack (agni issue 1812).
func TestAFolderChecklistReviewsTheSameUnderAPack(t *testing.T) {
	srv := designServer(t, t.TempDir(), t.TempDir())
	project := splitChecklistProject(t)
	if out, err := runPacks(t, srv.URL, "push", project); err != nil {
		t.Fatalf("packs push: %v\n%s", err, out)
	}
	design := filepath.Join(project, "boards", "gw")
	if out, err := runDesigns(t, srv.URL, "push", design, "--id", "gw"); err != nil {
		t.Fatalf("designs push: %v\n%s", err, out)
	}
	folder := reviewOutcomes(t, "review", design, "--checklist", "split")
	if len(folder) != 2 || !slices.ContainsFunc(folder, func(k string) bool { return strings.HasSuffix(k, "|pass") || strings.HasSuffix(k, "|fail") }) {
		t.Fatalf("review --checklist split on the folder = %v; want P1 and P2 decided", folder)
	}
	if underPack := reviewOutcomes(t, "review", "designs/gw", "--pack", "gateway", "--checklist", "split", "--remote", srv.URL); !slices.Equal(underPack, folder) {
		t.Errorf("review --checklist split under the pack = %v; want the folder's %v", underPack, folder)
	}
}

// agni checklists packs/<id> --remote lists a pack's checklists in order, the default first, and
// refuses a design with --remote or a pack without it, saying which form to use (agni issue 1812).
func TestChecklistsListsAPacksChecklists(t *testing.T) {
	srv := designServer(t, t.TempDir(), t.TempDir())
	project := splitChecklistProject(t)
	if out, err := runPacks(t, srv.URL, "push", project); err != nil {
		t.Fatalf("packs push: %v\n%s", err, out)
	}
	out, err := runAgni(t, "checklists", "packs/gateway", "--remote", srv.URL)
	if err != nil {
		t.Fatalf("checklists packs/gateway: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 || lines[0] != "split  Power, one file per item  (default)" {
		t.Errorf("checklists packs/gateway =\n%s\nwant split first, marked the default, then the project's own", out)
	}
	js, err := runAgni(t, "checklists", "packs/gateway", "--remote", srv.URL, "--format", "json")
	var resp webapi.ListChecklistsResponse
	if err == nil {
		err = protojson.Unmarshal([]byte(js), &resp)
	}
	if err != nil || len(resp.GetChecklists()) < 2 || resp.GetChecklists()[0].GetName() != "split" || len(resp.GetChecklists()[0].GetManifest().GetAreas()[0].GetItems()) != 2 {
		t.Errorf("checklists packs/gateway --format json = %v\n%s; want split first, with its two items", err, js)
	}
	for args, want := range map[string]string{
		"designs/gw --remote": "name it as packs/<id>",
		"packs/gateway":       "add --remote <url>",
	} {
		a := strings.Fields(args)
		if strings.HasSuffix(args, "--remote") {
			a = append(a, srv.URL)
		}
		if out, err := runAgni(t, append([]string{"checklists"}, a...)...); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("checklists %s = %v\n%s; want an error saying %q", args, err, out, want)
		}
	}
}

// Importing a pack whose checklist is a folder asks for every item file's contents in one round, not
// one round per file, so a checklist of hundreds of items pushes in three rounds (agni issue 1812).
func TestImportPackFilesAsksForEveryItemFileAtOnce(t *testing.T) {
	ctx := context.Background()
	srv := designServer(t, t.TempDir(), t.TempDir())
	local, files, _, err := localPack(splitChecklistProject(t), "", "")
	if err != nil {
		t.Fatal(err)
	}
	packs := webapiconnect.NewPackServiceClient(http.DefaultClient, srv.URL)
	snaps := webapiconnect.NewSnapshotServiceClient(http.DefaultClient, srv.URL)
	var rounds [][]string
	for range 5 {
		_, err := packs.ImportPackFiles(ctx, connect.NewRequest(&webapi.ImportPackFilesRequest{Files: files.files}))
		if err == nil {
			break
		}
		var ce *connect.Error
		if !errors.As(err, &ce) {
			t.Fatal(err)
		}
		var missing []string
		for _, d := range ce.Details() {
			if v, derr := d.Value(); derr == nil {
				if bm, ok := v.(*webapi.BlobsMissing); ok {
					missing = append(missing, bm.GetSha256()...)
				}
			}
		}
		if len(missing) == 0 {
			t.Fatalf("ImportPackFiles = %v; want BlobsMissing or success", err)
		}
		rounds = append(rounds, missing)
		var contents [][]byte
		for _, h := range missing {
			contents = append(contents, files.byHash[h])
		}
		if _, err := snaps.UploadBlobs(ctx, connect.NewRequest(&webapi.UploadBlobsRequest{Contents: contents})); err != nil {
			t.Fatal(err)
		}
	}
	if len(rounds) != 3 || len(rounds[1]) != 2 {
		t.Fatalf("ImportPackFiles asked for contents in rounds %v; want project.yaml, then both item files together, then the tiers' files", rounds)
	}
	if n := len(local.GetConfig().GetChecklists()); n < 2 {
		t.Errorf("the pack holds %d checklists; want split beside the project's own", n)
	}
}

// reviewOutcomes runs agni with args and --format json, and returns each item as "id|outcome", sorted.
func reviewOutcomes(t *testing.T, args ...string) []string {
	t.Helper()
	out, err := runAgni(t, append(args, "--format", "json")...)
	if err != nil {
		t.Fatalf("agni %v: %v\n%s", args, err, out)
	}
	var doc struct {
		Results struct {
			Areas []struct {
				Items []struct {
					ID      string `json:"id"`
					Outcome string `json:"outcome"`
				} `json:"items"`
			} `json:"areas"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("agni %v printed no JSON: %v\n%s", args, err, out)
	}
	var keys []string
	for _, a := range doc.Results.Areas {
		for _, it := range a.Items {
			keys = append(keys, it.ID+"|"+it.Outcome)
		}
	}
	slices.Sort(keys)
	return keys
}

// designs push keeps the project.yaml a design extends as a pack, writes it again only when its hash
// moves, and leaves it alone under --no-pack (agni issue 1746).
func TestDesignsPushKeepsTheProjectItExtends(t *testing.T) {
	ctx := context.Background()
	srv := designServer(t, t.TempDir(), t.TempDir())
	project := tutorialProject(t)
	design := filepath.Join(project, "boards", "gw")
	packs := webapiconnect.NewPackServiceClient(http.DefaultClient, srv.URL)
	hash := func() string {
		t.Helper()
		p, err := packs.GetPack(ctx, connect.NewRequest(&webapi.GetPackRequest{Name: "packs/gateway"}))
		if err != nil {
			t.Fatalf("GetPack(packs/gateway): %v", err)
		}
		return p.Msg.GetHash()
	}
	if out, err := runDesigns(t, srv.URL, "push", design, "--id", "plain", "--no-pack"); err != nil {
		t.Fatalf("designs push --no-pack: %v\n%s", err, out)
	}
	if _, err := packs.GetPack(ctx, connect.NewRequest(&webapi.GetPackRequest{Name: "packs/gateway"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("after push --no-pack, GetPack(packs/gateway) = %v; want NOT_FOUND", err)
	}
	if out, err := runDesigns(t, srv.URL, "push", design, "--id", "gw"); err != nil || !strings.Contains(out, "packs/gateway\t") || !strings.Contains(out, "--pack gateway") {
		t.Fatalf("designs push = %q, %v; want packs/gateway kept and named for --pack", out, err)
	}
	first := hash()
	if out, err := runDesigns(t, srv.URL, "push", design, "--id", "gw"); err != nil || !strings.Contains(out, "packs/gateway\tunchanged") {
		t.Errorf("a second push = %q, %v; want packs/gateway unchanged", out, err)
	}
	profiles, err := filepath.Glob(filepath.Join(project, "profiles", "*.yaml"))
	if err != nil || len(profiles) == 0 {
		t.Fatalf("no profile to change under %s: %v", project, err)
	}
	f, err := os.OpenFile(profiles[0], os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n# changed\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if out, err := runDesigns(t, srv.URL, "push", design, "--id", "gw"); err != nil || strings.Contains(out, "unchanged") {
		t.Fatalf("a push after a profile changed = %q, %v; want the pack written again", out, err)
	}
	if hash() == first {
		t.Error("after a profile changed and the design was pushed, the pack's hash did not move")
	}
	if under, local := findingKeys(t, "check", "designs/gw", "--pack", "gateway", "--remote", srv.URL), findingKeys(t, "check", design); !slices.Equal(under, local) {
		t.Errorf("check under the pushed pack = %d findings, on the folder = %d; want the same set", len(under), len(local))
	}
}

// ImportPackFiles reads a whole project folder into a pack. It asks for project.yaml's contents
// first, then for the files its tiers name, and never for a design's files beside them (agni issue
// 1697).
func TestImportPackFilesAsksOnlyForWhatThePackHolds(t *testing.T) {
	ctx := context.Background()
	srv := designServer(t, t.TempDir(), t.TempDir())
	project := tutorialProject(t)
	all, err := localFiles(project)
	if err != nil {
		t.Fatal(err)
	}
	hashOf := map[string]string{}
	for _, f := range all.files {
		hashOf[f.GetPath()] = f.GetSha256()
	}
	if hashOf["boards/gw/gateway.edn"] == "" || hashOf["project.yaml"] == "" {
		t.Fatalf("the folder lacks the files this test reads: %v", hashOf)
	}
	packs := webapiconnect.NewPackServiceClient(http.DefaultClient, srv.URL)
	snaps := webapiconnect.NewSnapshotServiceClient(http.DefaultClient, srv.URL)
	var rounds [][]string
	var made *webapi.Pack
	for range 4 {
		resp, err := packs.ImportPackFiles(ctx, connect.NewRequest(&webapi.ImportPackFilesRequest{Files: all.files}))
		if err == nil {
			made = resp.Msg
			break
		}
		var ce *connect.Error
		if !errors.As(err, &ce) {
			t.Fatal(err)
		}
		var missing []string
		for _, d := range ce.Details() {
			if v, derr := d.Value(); derr == nil {
				if bm, ok := v.(*webapi.BlobsMissing); ok {
					missing = append(missing, bm.GetSha256()...)
				}
			}
		}
		if len(missing) == 0 {
			t.Fatalf("ImportPackFiles = %v; want BlobsMissing or success", err)
		}
		rounds = append(rounds, missing)
		var contents [][]byte
		for _, h := range missing {
			contents = append(contents, all.byHash[h])
		}
		if _, err := snaps.UploadBlobs(ctx, connect.NewRequest(&webapi.UploadBlobsRequest{Contents: contents})); err != nil {
			t.Fatal(err)
		}
	}
	if made == nil || len(rounds) != 2 {
		t.Fatalf("ImportPackFiles took %d rounds of missing contents and made %q; want project.yaml, then its files, then the pack", len(rounds), made.GetName())
	}
	if !slices.Equal(rounds[0], []string{hashOf["project.yaml"]}) {
		t.Errorf("the first round asked for %v; want project.yaml's contents alone", rounds[0])
	}
	for _, h := range rounds[1] {
		if h == hashOf["boards/gw/gateway.edn"] || h == hashOf["boards/gw/gateway.kicad_pcb"] {
			t.Errorf("the second round asked for a design's file (%s); want the pack's files alone", h)
		}
	}
	if made.GetName() != "packs/gateway" || made.GetTitle() != "Sample Board review project" || len(made.GetFiles()) == 0 {
		t.Errorf("pack = %s %q with %d files; want packs/gateway titled from project.yaml, with its tiers' files", made.GetName(), made.GetTitle(), len(made.GetFiles()))
	}
	for _, f := range made.GetFiles() {
		if strings.HasPrefix(f.GetPath(), "boards/") || f.GetPath() == "project.yaml" {
			t.Errorf("the pack keeps %s; want only the files its tiers name", f.GetPath())
		}
	}
	local, _, _, err := localPack(project, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if service.PackHash(local) != made.GetHash() {
		t.Errorf("the CLI's hash of the folder %s differs from the server's %s; want one reading of a folder", service.PackHash(local), made.GetHash())
	}
}

// A folder dropped in the viewer goes up through ImportDesignFiles alone, and the design keeps the
// intent its design.yaml declares, as `designs push` keeps it (agni issue 1697).
func TestImportDesignFilesKeepsTheDeclaredIntent(t *testing.T) {
	ctx := context.Background()
	srv := designServer(t, t.TempDir(), t.TempDir())
	folder, err := localFiles(filepath.Join(tutorialProject(t), "boards", "gw"))
	if err != nil {
		t.Fatal(err)
	}
	var contents [][]byte
	for _, f := range folder.files {
		contents = append(contents, folder.byHash[f.GetSha256()])
	}
	if _, err := webapiconnect.NewSnapshotServiceClient(http.DefaultClient, srv.URL).UploadBlobs(ctx, connect.NewRequest(&webapi.UploadBlobsRequest{Contents: contents})); err != nil {
		t.Fatal(err)
	}
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	if _, err := designs.CreateDesign(ctx, connect.NewRequest(&webapi.CreateDesignRequest{DesignId: "gw"})); err != nil {
		t.Fatal(err)
	}
	if _, err := designs.ImportDesignFiles(ctx, connect.NewRequest(&webapi.ImportDesignFilesRequest{Parent: "designs/gw", Files: folder.files})); err != nil {
		t.Fatal(err)
	}
	d, err := designs.GetDesign(ctx, connect.NewRequest(&webapi.GetDesignRequest{Name: "designs/gw"}))
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Msg.GetConfig().GetIntent(); got == nil || gproto.Size(got) == 0 {
		t.Fatalf("after ImportDesignFiles, intent = %v; want design.yaml's", got)
	}
}
