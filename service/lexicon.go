package service

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/ident"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

// LexiconService answers how a snapshot's names were read (agni issue 1266). It is built from the
// CheckService of the same host, since comparing two lexicons runs that service's catalog over the
// same loader, corpus and projects, and a host cannot then serve the two over different ones.
type LexiconService struct {
	checks *CheckService
}

// NewLexiconService returns the lexicon questions over checks' loader, catalog and projects.
func NewLexiconService(checks *CheckService) *LexiconService { return &LexiconService{checks: checks} }

// conventionOf is the convention snap's lexicon was built from and the tier it came from. A snapshot
// whose overlay carries none was read with the process-level vocabulary, which is the server's
// startup convention when one was installed and the built-ins otherwise.
func conventionOf(snap Snap) (*configpb.NamingConvention, webapi.LexiconSource) {
	if snap.Overlay.Convention != nil {
		return snap.Overlay.Convention, snap.Overlay.ConventionSource
	}
	if c := naming.ActiveConvention(); c != nil && c.GetLexicon() != nil {
		return c, webapi.LexiconSource_LEXICON_SOURCE_SERVER
	}
	return nil, webapi.LexiconSource_LEXICON_SOURCE_UNSPECIFIED
}

func lexiconEntries(conv *configpb.NamingConvention, src webapi.LexiconSource) []*webapi.LexiconEntry {
	es := classify.EffectiveEntries(conv.GetLexicon())
	out := make([]*webapi.LexiconEntry, len(es))
	for i, e := range es {
		p := &webapi.LexiconEntry{Vocabulary: e.Vocabulary, Kind: e.Kind, Value: e.Value, Replaced: e.Replaced,
			Source: webapi.LexiconSource_LEXICON_SOURCE_BUILTIN}
		if e.FromConvention {
			p.Source, p.Convention = src, conv.GetName()
		}
		out[i] = p
	}
	return out
}

// GetLexicon lists the snapshot's effective lexicon with each entry's source.
func (s *LexiconService) GetLexicon(ctx context.Context, req *webapi.GetLexiconRequest) (*webapi.GetLexiconResponse, error) {
	_, snap, err := s.checks.projects.Snapshot(ctx, req.GetSnapshot())
	if err != nil {
		return nil, err
	}
	conv, src := conventionOf(snap)
	if cand := req.GetCandidate(); cand != nil {
		if _, err := naming.BuildLexicon(cand); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
		}
		conv, src = cand, webapi.LexiconSource_LEXICON_SOURCE_REQUEST
	}
	return &webapi.GetLexiconResponse{
		Entries:    lexiconEntries(conv, src),
		Effective:  classify.EffectiveLexicon(conv.GetLexicon()),
		Convention: conv,
	}, nil
}

// RenderConvention writes a convention as a conventions file.
func (s *LexiconService) RenderConvention(_ context.Context, req *webapi.RenderConventionRequest) (*webapi.RenderConventionResponse, error) {
	conv := req.GetConventions()
	if conv == nil {
		return nil, fmt.Errorf("%w: RenderConvention needs a convention", ErrInvalidArgument)
	}
	b, err := ConventionYAML(conv)
	if err != nil {
		return nil, err
	}
	name := conv.GetName()
	if name == "" {
		name = "conventions"
	}
	return &webapi.RenderConventionResponse{Content: b, ContentType: "application/yaml", Filename: name + ".yaml"}, nil
}

// ConventionYAML writes conv with field names as the YAML binding reads them, in the order the schema
// declares them, so naming.Parse reads it back to the same message.
func ConventionYAML(conv *configpb.NamingConvention) ([]byte, error) {
	b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(conv)
	if err != nil {
		return nil, err
	}
	// JSON is YAML, and a yaml.Node keeps the keys in the order protojson wrote them.
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	plainStyle(&doc)
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// plainStyle clears the JSON spelling off a node tree, so the file reads as a person writes one. The
// encoder quotes a string again where plain YAML would read it as something else.
func plainStyle(n *yaml.Node) {
	n.Style &^= yaml.FlowStyle | yaml.DoubleQuotedStyle | yaml.SingleQuotedStyle
	for _, c := range n.Content {
		plainStyle(c)
	}
}

// ExplainNet returns one net's stamped roles and the net-name entries its name matches.
func (s *LexiconService) ExplainNet(ctx context.Context, req *webapi.ExplainNetRequest) (*webapi.ExplainNetResponse, error) {
	ctx, snap, err := s.checks.projects.Snapshot(ctx, req.GetSnapshot())
	if err != nil {
		return nil, err
	}
	if req.GetNet() == "" {
		return nil, fmt.Errorf("%w: ExplainNet needs a net", ErrInvalidArgument)
	}
	// Through the model, as a rule reads the net, so a role a datasheet backs shows that evidence. No
	// role reads the copper, so the board stays unread.
	b, err := buildModelEntry(ctx, LoaderOr(snap, s.checks.loader), snap.Netlist, snap.Board, snap.Overlay, s.checks.specs, SkipBoard)
	if err != nil {
		return nil, err
	}
	n, err := findNet(b.model.Nets(), req.GetNet())
	if err != nil {
		return nil, err
	}
	conv, src := conventionOf(snap)
	resp := &webapi.ExplainNetResponse{Net: n.GetName()}
	for _, r := range n.GetRoles() {
		resp.Roles = append(resp.Roles, proto.Clone(r).(*ir.NetRole))
	}
	leaf := classify.RoleLeaf(n.GetName())
	for _, e := range lexiconEntries(conv, src) {
		if !strings.HasPrefix(e.GetVocabulary(), "net.") || e.GetReplaced() || (e.GetKind() != "pattern" && e.GetKind() != "exclude") {
			continue
		}
		// The effective lexicon compiled once already, so a pattern that fails here cannot occur.
		if re, err := regexp.Compile("(?i)" + e.GetValue()); err == nil && re.MatchString(leaf) {
			resp.Matches = append(resp.Matches, e)
		}
	}
	return resp, nil
}

// findNet resolves a net by its exact name, then by a hierarchy leaf exactly one net carries. A name
// matching neither is NOT_FOUND, naming a few nets whose names contain it.
func findNet(nets []*ir.Net, name string) (*ir.Net, error) {
	var byLeaf []*ir.Net
	for _, n := range nets {
		if n.GetName() == name {
			return n, nil
		}
		if classify.RoleLeaf(n.GetName()) == name {
			byLeaf = append(byLeaf, n)
		}
	}
	if len(byLeaf) == 1 {
		return byLeaf[0], nil
	}
	var near []string
	for _, n := range byLeaf {
		near = append(near, n.GetName())
	}
	if len(near) == 0 {
		for _, n := range nets {
			if strings.Contains(strings.ToUpper(n.GetName()), strings.ToUpper(name)) {
				near = append(near, n.GetName())
			}
		}
	}
	sort.Slice(near, func(i, j int) bool { return ident.CompareNatural(near[i], near[j]) < 0 })
	if len(byLeaf) > 1 {
		return nil, fmt.Errorf("%w: net %q names %d nets by their leaf: %s", ErrInvalidArgument, name, len(byLeaf), strings.Join(near, ", "))
	}
	if len(near) > 5 {
		near = near[:5]
	}
	if len(near) == 0 {
		return nil, fmt.Errorf("%w: no net named %q", ErrNotFound, name)
	}
	return nil, fmt.Errorf("%w: no net named %q; nets containing it: %s", ErrNotFound, name, strings.Join(near, ", "))
}

// CompareLexicons reads the snapshot with the candidate convention in place of its own and diffs the
// roles and the findings of the two reads.
func (s *LexiconService) CompareLexicons(ctx context.Context, req *webapi.CompareLexiconsRequest) (*webapi.CompareLexiconsResponse, error) {
	ctx, snap, err := s.checks.projects.Snapshot(ctx, req.GetSnapshot())
	if err != nil {
		return nil, err
	}
	if req.GetConventions() == nil {
		return nil, fmt.Errorf("%w: CompareLexicons needs a candidate convention", ErrInvalidArgument)
	}
	cfg := &webapi.OverlayConfig{}
	if snap.Config != nil {
		cfg = proto.Clone(snap.Config).(*webapi.OverlayConfig)
	}
	if cfg.Config == nil {
		cfg.Config = &webapi.AnalysisConfig{}
	}
	cand := req.GetConventions()
	// A candidate stating no lexicon reads names with the built-ins alone. A request's convention with
	// no lexicon otherwise leaves the vocabulary it replaces in effect, which would compare the current
	// names against themselves.
	if cand.GetLexicon() == nil {
		cand = proto.Clone(cand).(*configpb.NamingConvention)
		cand.Lexicon = &configpb.NamingLexicon{}
	}
	cfg.Config.Conventions = cand
	after, err := snap.ProjectsOr(s.checks.projects).Overlay(ctx, snap.Design, cfg, s.checks.baseConvention)
	if err != nil {
		return nil, err
	}
	after = after.WithFlattenHierarchy(snap.Overlay.FlattenHierarchy)
	m0, f0, err := s.run(ctx, snap, snap.Overlay)
	if err != nil {
		return nil, err
	}
	// Taken before the second read, which may stamp roles onto the same nets.
	r0 := rolesByNet(m0)
	m1, f1, err := s.run(ctx, snap, after)
	if err != nil {
		return nil, err
	}
	return &webapi.CompareLexiconsResponse{Tables: []*webapi.Table{roleChanges(r0, rolesByNet(m1)), findingChanges(f0, f1)}}, nil
}

// run builds snap's model under ov and runs every rule of ov's catalog that model can answer, as
// CheckDesign does for a request naming no rules.
func (s *LexiconService) run(ctx context.Context, snap Snap, ov Overlay) (check.Model, []check.Finding, error) {
	b, err := buildModelEntry(ctx, LoaderOr(snap, s.checks.loader), snap.Netlist, snap.Board, ov, s.checks.specs, ReadBoard)
	if err != nil {
		return nil, nil, err
	}
	cat, err := ov.Catalog(s.checks.catalog)
	if err != nil {
		return nil, nil, err
	}
	runnable, _ := partitionAvailable(cat.Filter(check.Facets{}), b.model)
	findings, _, err := b.runKept(ctx, s.checks.catalog, runnable)
	if err != nil {
		return nil, nil, err
	}
	return b.model, findings, nil
}

// roleSet spells a net's roles with the evidence for each, "rail (convention), ground (declared)", so
// a role whose evidence moved from a name to a datasheet reads as a change.
func roleSet(n *ir.Net) string {
	var toks []string
	for _, r := range n.GetRoles() {
		toks = append(toks, classify.RoleToken(r.GetRoleKind())+" ("+strings.ToLower(strings.TrimPrefix(r.GetSource().String(), "ROLE_SOURCE_"))+")")
	}
	sort.Strings(toks)
	return strings.Join(toks, ", ")
}

func rolesByNet(m check.Model) map[string]string {
	out := map[string]string{}
	for _, n := range m.Nets() {
		out[n.GetName()] = roleSet(n)
	}
	return out
}

// roleChanges is one row per net whose roles differ between the two reads, in name order.
func roleChanges(was, now map[string]string) *webapi.Table {
	t := &webapi.Table{Name: "role_changes", Columns: []*webapi.TableColumn{
		{Name: "net", Type: webapi.ColumnType_COLUMN_TYPE_NAME, Kind: check.KindNet},
		{Name: "roles_before"}, {Name: "roles_after"},
	}}
	var names []string
	for n, r := range now {
		if r != was[n] {
			names = append(names, n)
		}
	}
	sort.Slice(names, func(i, j int) bool { return ident.CompareNatural(names[i], names[j]) < 0 })
	for _, n := range names {
		t.Rows = append(t.Rows, &webapi.TableRow{Cells: []string{n, was[n], now[n]}})
	}
	return t
}

// findingChanges is one row per finding one run reports and the other does not, keyed on its rule,
// subject and message, gone ones first and then by rule and subject.
func findingChanges(before, after []check.Finding) *webapi.Table {
	t := &webapi.Table{Name: "finding_changes", Columns: []*webapi.TableColumn{
		{Name: "change"}, {Name: "rule"}, {Name: "subject_kind"},
		{Name: "subject", Type: webapi.ColumnType_COLUMN_TYPE_NAME}, {Name: "message"},
	}}
	key := func(f *checkspb.Finding) string {
		s := f.GetSubject()
		return strings.Join([]string{f.GetRule(), s.GetKind(), s.GetRef(), s.GetPin(), f.GetMessage()}, "\x00")
	}
	count := func(fs []*checkspb.Finding) map[string]int {
		m := map[string]int{}
		for _, f := range fs {
			m[key(f)]++
		}
		return m
	}
	b, a := FindingProtos(before), FindingProtos(after)
	nb, na := count(b), count(a)
	type row struct {
		change string
		f      *checkspb.Finding
	}
	var rows []row
	diff := func(change string, fs []*checkspb.Finding, mine, other map[string]int) {
		for _, f := range fs {
			k := key(f)
			if mine[k] > other[k] {
				mine[k]--
				rows = append(rows, row{change, f})
			}
		}
	}
	diff("gone", b, nb, na)
	diff("appeared", a, na, nb)
	sort.SliceStable(rows, func(i, j int) bool {
		x, y := rows[i], rows[j]
		if x.change != y.change {
			return x.change == "gone"
		}
		if x.f.GetRule() != y.f.GetRule() {
			return x.f.GetRule() < y.f.GetRule()
		}
		return ident.CompareNatural(subjectName(x.f.GetSubject()), subjectName(y.f.GetSubject())) < 0
	})
	for _, r := range rows {
		s := r.f.GetSubject()
		t.Rows = append(t.Rows, &webapi.TableRow{
			Cells:     []string{r.change, r.f.GetRule(), s.GetKind(), subjectName(s), r.f.GetMessage()},
			CellKinds: subjectKinds(5, 3, s.GetKind()),
		})
	}
	return t
}

func subjectName(s *checkspb.Subject) string {
	if s.GetPin() != "" {
		return s.GetRef() + "." + s.GetPin()
	}
	return s.GetRef()
}
