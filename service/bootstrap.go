package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// BootstrapPlan is what a server should hold, as `agni serve --bootstrap` reads it from a file
// (agni issue 1808). Each list is applied in the order connections, mounts, packs, designs,
// projects, so an entry can name what an earlier list declares.
type BootstrapPlan struct {
	// Prune deletes the stored designs, packs and projects the plan doesn't name. Without it they
	// are kept and reported. Connections and mounts are never pruned.
	Prune       bool                  `yaml:"prune"`
	Connections []BootstrapConnection `yaml:"connections"`
	Mounts      []BootstrapMount      `yaml:"mounts"`
	Packs       []BootstrapPack       `yaml:"packs"`
	Designs     []BootstrapDesign     `yaml:"designs"`
	Projects    []BootstrapProject    `yaml:"projects"`
}

// BootstrapConnection is a server connection. It names where each secret is, as env:NAME or
// file:/abs/path, and never holds one.
type BootstrapConnection struct {
	ID string `yaml:"id"`
	// Kind is github-app, github-token, aws, gcp or entra.
	Kind           string `yaml:"kind"`
	Title          string `yaml:"title"`
	BaseURL        string `yaml:"base_url"`
	AppID          string `yaml:"app_id"`
	InstallationID int64  `yaml:"installation_id"`
	Key            string `yaml:"key"`
	Token          string `yaml:"token"`
	AccessKeyID    string `yaml:"access_key_id"`
	RoleARN        string `yaml:"role_arn"`
	Region         string `yaml:"region"`
	TenantID       string `yaml:"tenant_id"`
	ClientID       string `yaml:"client_id"`
	// EntraAuth is secret, certificate, managed-identity or workload-identity.
	EntraAuth string `yaml:"entra_auth"`
	Cert      string `yaml:"cert"`
}

// BootstrapMount is a runtime mount, as `agni mounts add` makes one.
type BootstrapMount struct {
	ID         string `yaml:"id"`
	Source     string `yaml:"source"`
	Ref        string `yaml:"ref"`
	Connection string `yaml:"connection"`
}

// BootstrapPack is a pack read from a project folder in a mount, as `agni packs link` reads one.
// Link is <mount>[/<path>][@<ref>].
type BootstrapPack struct {
	ID    string `yaml:"id"`
	Title string `yaml:"title"`
	Link  string `yaml:"link"`
}

// BootstrapDesign is a stored design whose artifacts are linked from mounts, as `agni designs
// link` links each one. Link maps an artifact id to <mount>[/<path>][@<ref>].
type BootstrapDesign struct {
	ID     string            `yaml:"id"`
	Title  string            `yaml:"title"`
	Series string            `yaml:"series"`
	Packs  []string          `yaml:"packs"`
	Link   map[string]string `yaml:"link"`
}

// BootstrapProject is a stored project grouping designs, with the packs its designs are read under.
type BootstrapProject struct {
	ID      string   `yaml:"id"`
	Title   string   `yaml:"title"`
	Designs []string `yaml:"designs"`
	Packs   []string `yaml:"packs"`
}

// ParseBootstrap reads a plan from YAML. It refuses a key the plan doesn't have, an entry with no
// id, an id given twice in one list, and a link that names no mount, so a typo fails the server's
// start rather than leaving it without something.
func ParseBootstrap(b []byte) (*BootstrapPlan, error) {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var p BootstrapPlan
	if err := dec.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	var errs []error
	ids := func(kind string, n int, id func(int) string) {
		seen := map[string]bool{}
		for i := range n {
			switch v := id(i); {
			case v == "":
				errs = append(errs, fmt.Errorf("%s entry %d has no id", kind, i+1))
			case !newIDPattern.MatchString(v):
				errs = append(errs, fmt.Errorf("%s/%s: the id must be lowercase letters, digits, '.', '_' or '-', starting with a letter or digit", kind, v))
			case seen[v]:
				errs = append(errs, fmt.Errorf("%s/%s is declared twice", kind, v))
			default:
				seen[v] = true
			}
		}
	}
	ids("connections", len(p.Connections), func(i int) string { return p.Connections[i].ID })
	ids("mounts", len(p.Mounts), func(i int) string { return p.Mounts[i].ID })
	ids("packs", len(p.Packs), func(i int) string { return p.Packs[i].ID })
	ids("designs", len(p.Designs), func(i int) string { return p.Designs[i].ID })
	ids("projects", len(p.Projects), func(i int) string { return p.Projects[i].ID })
	for _, c := range p.Connections {
		if _, err := c.proto(); err != nil {
			errs = append(errs, fmt.Errorf("connections/%s: %v", c.ID, err))
		}
	}
	for _, m := range p.Mounts {
		if m.Source == "" {
			errs = append(errs, fmt.Errorf("mounts/%s names no source", m.ID))
		}
	}
	for _, k := range p.Packs {
		if _, err := ParseLinkSource(k.Link); err != nil {
			errs = append(errs, fmt.Errorf("packs/%s: %v", k.ID, err))
		}
	}
	for _, d := range p.Designs {
		if len(d.Link) == 0 {
			errs = append(errs, fmt.Errorf("designs/%s links no artifact", d.ID))
		}
		for _, a := range sortedKeys(d.Link) {
			if !newIDPattern.MatchString(a) {
				errs = append(errs, fmt.Errorf("designs/%s: artifact id %q must be lowercase letters, digits, '.', '_' or '-'", d.ID, a))
			}
			if _, err := ParseLinkSource(d.Link[a]); err != nil {
				errs = append(errs, fmt.Errorf("designs/%s/artifacts/%s: %v", d.ID, a, err))
			}
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArgument, errors.Join(errs...))
	}
	return &p, nil
}

// ParseLinkSource reads <mount>[/<path>][@<ref>], the source `agni designs link` and `agni packs
// link` take.
func ParseLinkSource(s string) (*webapi.ArtifactSource, error) {
	src := &webapi.ArtifactSource{}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s, src.Ref = s[:i], s[i+1:]
		if src.Ref == "" {
			return nil, fmt.Errorf("%q names an empty ref after '@'", s+"@")
		}
	}
	src.Mount, src.Path, _ = strings.Cut(s, "/")
	if src.Mount == "" {
		return nil, fmt.Errorf("%q names no mount; write <mount>[/<path>][@<ref>]", s)
	}
	return src, nil
}

// proto is c as CreateConnection takes it.
func (c BootstrapConnection) proto() (*webapi.Connection, error) {
	kind, ok := webapi.ConnectionKind_value["CONNECTION_KIND_"+strings.ToUpper(strings.ReplaceAll(c.Kind, "-", "_"))]
	if !ok || c.Kind == "" || c.Kind == "github-user" {
		return nil, fmt.Errorf("kind %q must be github-app, github-token, aws, gcp or entra", c.Kind)
	}
	out := &webapi.Connection{
		Kind: webapi.ConnectionKind(kind), DisplayName: c.Title, BaseUrl: c.BaseURL, AppId: c.AppID, InstallationId: c.InstallationID,
		KeySource: c.Key, TokenSource: c.Token, AccessKeyId: c.AccessKeyID, RoleArn: c.RoleARN, Region: c.Region,
		TenantId: c.TenantID, ClientId: c.ClientID, CertSource: c.Cert, Scope: webapi.ConnectionScope_CONNECTION_SCOPE_SERVER,
	}
	if c.EntraAuth != "" {
		a, ok := webapi.EntraAuth_value["ENTRA_AUTH_"+strings.ToUpper(strings.ReplaceAll(c.EntraAuth, "-", "_"))]
		if !ok {
			return nil, fmt.Errorf("entra_auth %q must be secret, certificate, managed-identity or workload-identity", c.EntraAuth)
		}
		out.EntraAuth = webapi.EntraAuth(a)
	}
	return out, nil
}

// Bootstrap applies a plan through the server's own services, as the server rather than as any
// caller, since the operator who wrote the plan chose every source in it.
type Bootstrap struct {
	Connections *ConnectionService
	Workspace   *WorkspaceService
	Packs       *PackService
	Designs     *DesignService
	Projects    *ProjectService
}

// BootstrapResult says what one Apply did. Every entry the plan names is in exactly one of
// Applied, Unchanged and Failed, by resource name.
type BootstrapResult struct {
	Applied   []string
	Unchanged []string
	Failed    []BootstrapFailure
	// Unnamed are stored designs, packs and projects the plan doesn't name, and Pruned the ones
	// of those Apply deleted.
	Unnamed []string
	Pruned  []string
}

// BootstrapFailure is one entry Apply could not apply, and why.
type BootstrapFailure struct {
	Name string
	Err  error
}

// OK reports whether every entry applied.
func (r *BootstrapResult) OK() bool { return len(r.Failed) == 0 }

// Apply brings the server to plan, entry by entry. An entry that already matches the plan is left
// alone, so applying a plan twice writes nothing the second time. A failed entry doesn't stop the
// others, and Apply looks for unnamed resources only once every entry applied, so a retry after a
// failure never prunes what the failed entry would have kept.
func (b *Bootstrap) Apply(ctx context.Context, plan *BootstrapPlan) *BootstrapResult {
	ctx = unguarded(ctx)
	r := &BootstrapResult{}
	record := func(name string, changed bool, err error) {
		switch {
		case err != nil:
			r.Failed = append(r.Failed, BootstrapFailure{Name: name, Err: err})
		case changed:
			r.Applied = append(r.Applied, name)
		default:
			r.Unchanged = append(r.Unchanged, name)
		}
	}
	for _, c := range plan.Connections {
		changed, err := b.connection(ctx, c)
		record(ConnectionName(c.ID), changed, err)
	}
	for _, m := range plan.Mounts {
		changed, err := b.mount(ctx, m)
		record("mounts/"+m.ID, changed, err)
	}
	for _, k := range plan.Packs {
		changed, err := b.pack(ctx, k)
		record(PackName(k.ID), changed, err)
	}
	for _, d := range plan.Designs {
		changed, err := b.design(ctx, d)
		record(StandaloneDesignName(d.ID), changed, err)
	}
	for _, p := range plan.Projects {
		changed, err := b.project(ctx, p)
		record(ProjectName(p.ID), changed, err)
	}
	if !r.OK() {
		return r
	}
	unnamed, err := b.unnamed(ctx, plan)
	if err != nil {
		r.Failed = append(r.Failed, BootstrapFailure{Name: "unnamed", Err: err})
		return r
	}
	r.Unnamed = unnamed
	if plan.Prune {
		for _, name := range unnamed {
			if err := b.remove(ctx, name); err != nil {
				r.Failed = append(r.Failed, BootstrapFailure{Name: name, Err: fmt.Errorf("pruning: %w", err)})
				continue
			}
			r.Pruned = append(r.Pruned, name)
		}
	}
	return r
}

// connection makes c, or makes it again when the stored record says something else.
func (b *Bootstrap) connection(ctx context.Context, c BootstrapConnection) (bool, error) {
	if b.Connections == nil || b.Connections.store == nil {
		return false, fmt.Errorf("%w: %s", ErrFailedPrecondition, noConnections)
	}
	want, err := c.proto()
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	req := &webapi.CreateConnectionRequest{ConnectionId: c.ID, Connection: want}
	have, err := b.Connections.GetConnection(ctx, &webapi.GetConnectionRequest{Name: ConnectionName(c.ID)})
	switch {
	case errors.Is(err, ErrNotFound):
		_, err := b.Connections.CreateConnection(ctx, req)
		return err == nil, err
	case err != nil:
		return false, err
	case have.GetSealed():
		return false, fmt.Errorf("%w: %s keeps a sealed secret; delete it to let the plan name where its secret is", ErrFailedPrecondition, ConnectionName(c.ID))
	}
	if sameConnection(have, want) {
		return false, nil
	}
	// The record is replaced under its id, so the mounts reading through it keep reading.
	old, err := b.Connections.store.GetConnection(ctx, "", c.ID)
	if err != nil {
		return false, ConnectionErr(err)
	}
	if err := b.Connections.store.DeleteConnection(ctx, "", c.ID); err != nil {
		return false, ConnectionErr(err)
	}
	if _, err := b.Connections.CreateConnection(ctx, req); err != nil {
		_ = b.Connections.store.PutConnection(ctx, old)
		return false, err
	}
	return true, nil
}

// sameConnection reports whether have already says what want says. An empty base URL takes the
// server's GitHub API, so it matches whatever have holds.
func sameConnection(have, want *webapi.Connection) bool {
	return have.GetKind() == want.GetKind() && have.GetDisplayName() == want.GetDisplayName() &&
		(want.GetBaseUrl() == "" || have.GetBaseUrl() == want.GetBaseUrl()) &&
		have.GetAppId() == want.GetAppId() && have.GetInstallationId() == want.GetInstallationId() &&
		have.GetKeySource() == want.GetKeySource() && have.GetTokenSource() == want.GetTokenSource() &&
		have.GetAccessKeyId() == want.GetAccessKeyId() && have.GetRoleArn() == want.GetRoleArn() &&
		have.GetRegion() == want.GetRegion() && have.GetTenantId() == want.GetTenantId() &&
		have.GetClientId() == want.GetClientId() && have.GetCertSource() == want.GetCertSource() &&
		have.GetEntraAuth() == want.GetEntraAuth()
}

// mount makes m, or makes it again when the mount by its name reads something else.
func (b *Bootstrap) mount(ctx context.Context, m BootstrapMount) (bool, error) {
	if b.Workspace == nil || b.Workspace.maker == nil {
		return false, fmt.Errorf("%w: %s", ErrFailedPrecondition, noMaker)
	}
	req := &webapi.CreateMountRequest{Name: m.ID, Source: m.Source, Ref: m.Ref, Connection: m.Connection}
	i := slices.IndexFunc(b.Workspace.ws.Mounts(), func(have MountInfo) bool { return have.Name == m.ID })
	if i < 0 {
		_, err := b.Workspace.CreateMount(ctx, req)
		return err == nil, err
	}
	have := b.Workspace.ws.Mounts()[i]
	if have.Source == m.Source && have.Ref == m.Ref && ConnectionID(have.Connection) == ConnectionID(m.Connection) {
		return false, nil
	}
	if have.Fixed {
		return false, fmt.Errorf("%w: the server's --mount flag already serves a mount named %q", ErrAlreadyExists, m.ID)
	}
	if _, err := b.Workspace.DeleteMount(ctx, &webapi.DeleteMountRequest{Name: m.ID}); err != nil {
		return false, err
	}
	_, err := b.Workspace.CreateMount(ctx, req)
	return err == nil, err
}

// pack links k, or links it again when the pack reads from somewhere else.
func (b *Bootstrap) pack(ctx context.Context, k BootstrapPack) (bool, error) {
	if b.Packs == nil {
		return false, errNoPackStore
	}
	src, err := ParseLinkSource(k.Link)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	have, err := b.Packs.GetPack(ctx, &webapi.GetPackRequest{Name: PackName(k.ID)})
	switch {
	case errors.Is(err, ErrNotFound):
	case err != nil:
		return false, err
	case packLinked(have) && sameSource(have.GetSource(), src) && (k.Title == "" || have.GetTitle() == k.Title):
		return false, nil
	}
	_, err = b.Packs.LinkPack(ctx, &webapi.LinkPackRequest{PackId: k.ID, Source: src, Title: k.Title})
	return err == nil, err
}

// sameSource reports whether have reads the mount, path and ref want names.
func sameSource(have, want *webapi.ArtifactSource) bool {
	p, err := cleanSourcePath(want.GetPath())
	return err == nil && have.GetMount() == want.GetMount() && have.GetPath() == p && have.GetRef() == want.GetRef()
}

// design makes d with its title, series and packs, and links each of its artifacts that reads from
// somewhere else.
func (b *Bootstrap) design(ctx context.Context, d BootstrapDesign) (bool, error) {
	if b.Designs == nil {
		return false, fmt.Errorf("%w: this server keeps no designs; start it with --store and --uploads", ErrFailedPrecondition)
	}
	name := StandaloneDesignName(d.ID)
	packs, err := packNames(prefixed(packsNamePrefix, d.Packs))
	if err != nil {
		return false, err
	}
	changed := false
	have, err := b.Designs.GetDesign(ctx, &webapi.GetDesignRequest{Name: name})
	switch {
	case errors.Is(err, ErrNotFound):
		if _, err := b.Designs.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: d.ID, Design: &webapi.Design{Title: d.Title, Series: d.Series, Packs: packs}}); err != nil {
			return false, err
		}
		changed = true
	case err != nil:
		return false, err
	case have.GetReadOnly():
		return false, fmt.Errorf("%w: a mounted folder already declares %s", ErrAlreadyExists, name)
	default:
		var mask []string
		if d.Title != "" && have.GetTitle() != d.Title {
			mask = append(mask, "title")
		}
		if have.GetSeries() != d.Series {
			mask = append(mask, "series")
		}
		if !slices.Equal(have.GetPacks(), packs) {
			mask = append(mask, "packs")
		}
		if len(mask) > 0 {
			in := &webapi.Design{Name: name, Title: d.Title, Series: d.Series, Packs: packs}
			if _, err := b.Designs.UpdateDesign(ctx, &webapi.UpdateDesignRequest{Design: in, UpdateMask: &fieldmaskpb.FieldMask{Paths: mask}}); err != nil {
				return false, err
			}
			changed = true
		}
	}
	for _, id := range sortedKeys(d.Link) {
		src, err := ParseLinkSource(d.Link[id])
		if err != nil {
			return changed, fmt.Errorf("%w: artifact %s: %v", ErrInvalidArgument, id, err)
		}
		a, err := b.Designs.GetArtifact(ctx, &webapi.GetArtifactRequest{Name: ArtifactName(d.ID, id)})
		switch {
		case errors.Is(err, ErrNotFound):
		case err != nil:
			return changed, err
		case b.Designs.linked(a) && sameSource(a.GetSource(), src):
			continue
		}
		if _, err := b.Designs.LinkArtifact(ctx, &webapi.LinkArtifactRequest{Name: ArtifactName(d.ID, id), Source: src, AllowMissing: true}); err != nil {
			return changed, fmt.Errorf("artifact %s: %w", id, err)
		}
		changed = true
	}
	return changed, nil
}

// project makes p, or changes its title, designs and packs to the plan's.
func (b *Bootstrap) project(ctx context.Context, p BootstrapProject) (bool, error) {
	if b.Projects == nil {
		return false, errNoProjectStore
	}
	designs, err := designNames(prefixed(designsNamePrefix, p.Designs))
	if err != nil {
		return false, err
	}
	packs, err := packNames(prefixed(packsNamePrefix, p.Packs))
	if err != nil {
		return false, err
	}
	name := ProjectName(p.ID)
	have, err := b.Projects.GetProject(ctx, &webapi.GetProjectRequest{Name: name})
	switch {
	case errors.Is(err, ErrNotFound):
		_, err := b.Projects.CreateProject(ctx, &webapi.CreateProjectRequest{ProjectId: p.ID, Project: &webapi.Project{Title: p.Title, Designs: designs, DefaultPacks: packs}})
		return err == nil, err
	case err != nil:
		return false, err
	case have.GetUri() != "":
		return false, fmt.Errorf("%w: a mounted folder already declares %s", ErrAlreadyExists, name)
	}
	if (p.Title == "" || have.GetTitle() == p.Title) && slices.Equal(have.GetDesigns(), designs) && slices.Equal(have.GetDefaultPacks(), packs) {
		return false, nil
	}
	in := &webapi.Project{Name: name, Title: orID(p.Title, p.ID), Designs: designs, DefaultPacks: packs}
	_, err = b.Projects.UpdateProject(ctx, &webapi.UpdateProjectRequest{Project: in})
	return err == nil, err
}

// unnamed lists the stored designs, packs and projects plan doesn't name, and the artifacts of a
// named design that its entry doesn't link, by resource name.
func (b *Bootstrap) unnamed(ctx context.Context, plan *BootstrapPlan) ([]string, error) {
	var out []string
	named := map[string]bool{}
	for _, d := range plan.Designs {
		named[StandaloneDesignName(d.ID)] = true
		for id := range d.Link {
			named[ArtifactName(d.ID, id)] = true
		}
	}
	for _, k := range plan.Packs {
		named[PackName(k.ID)] = true
	}
	for _, p := range plan.Projects {
		named[ProjectName(p.ID)] = true
	}
	if b.Projects != nil && b.Projects.records != nil {
		stored, err := b.Projects.records.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range stored {
			if !named[p.GetName()] {
				out = append(out, p.GetName())
			}
		}
	}
	if b.Designs != nil {
		all, _, err := b.Designs.catalog.Designs(ctx)
		if err != nil {
			return nil, err
		}
		for _, d := range all {
			switch {
			case d.GetReadOnly():
			case !named[d.GetName()]:
				out = append(out, d.GetName())
			default:
				for _, a := range d.GetArtifacts() {
					if !named[a] {
						out = append(out, a)
					}
				}
			}
		}
	}
	if b.Packs != nil && b.Packs.store != nil {
		token := ""
		for {
			resp, err := b.Packs.ListPacks(ctx, &webapi.ListPacksRequest{PageToken: token})
			if err != nil {
				return nil, err
			}
			for _, k := range resp.GetPacks() {
				if !named[k.GetName()] {
					out = append(out, k.GetName())
				}
			}
			if token = resp.GetNextPageToken(); token == "" {
				break
			}
		}
	}
	return out, nil
}

// remove deletes one unnamed resource.
func (b *Bootstrap) remove(ctx context.Context, name string) error {
	switch {
	case strings.HasPrefix(name, projectNamePrefix):
		return b.Projects.DeleteProject(ctx, &webapi.DeleteProjectRequest{Name: name})
	case strings.HasPrefix(name, packsNamePrefix):
		return b.Packs.DeletePack(ctx, &webapi.DeletePackRequest{Name: name})
	case strings.Contains(name, "/artifacts/"):
		return b.Designs.DeleteArtifact(ctx, &webapi.DeleteArtifactRequest{Name: name})
	default:
		return b.Designs.DeleteDesign(ctx, &webapi.DeleteDesignRequest{Name: name})
	}
}

// prefixed names each of ids as a resource under prefix. A plan names a pack as the CLI does, as
// house or packs/house.
func prefixed(prefix string, ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = prefix + strings.TrimPrefix(id, prefix)
	}
	return out
}

// sortedKeys is m's keys in order.
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
