package mcpserver

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/stdlib/lib"
	"github.com/panyam/jaala/frontends/nl"
	"github.com/panyam/mcpkit/core"
	mcpkit "github.com/panyam/mcpkit/server"
)

// registerResources adds the catalogs an agent reads before asking: the relations a query can call,
// the shipped library's derived relations, the rules a check runs and the designs to ask about. Each
// is read from the service that answers it, so a resource and the tools never disagree.
func (s *srv) registerResources(srv *mcpkit.Server) {
	srv.RegisterResource(core.ResourceDef{
		URI: "agni://relations", Name: "relations", MimeType: "text/plain",
		Description: "Every relation, predicate and library member a query can call, one line each with its signature and what it holds. The schema tool says the same, and with a design names that design's project library too.",
	}, func(ctx core.ResourceContext, req core.ResourceRequest) (core.ResourceResult, error) {
		v, err := s.api.Query.Vocabulary(ctx, "")
		if err != nil {
			return core.ResourceResult{}, err
		}
		card, err := nl.Card(v)
		if err != nil {
			return core.ResourceResult{}, err
		}
		return text("agni://relations", "text/plain", card), nil
	})
	srv.RegisterResource(core.ResourceDef{
		URI: "agni://library", Name: "library", MimeType: "text/plain",
		Description: "The shipped library of derived relations (component.probed_both, net.has_test_point, ...), as the Datalog that defines them.",
	}, func(core.ResourceContext, core.ResourceRequest) (core.ResourceResult, error) {
		var b strings.Builder
		for _, m := range lib.Modules() {
			fmt.Fprintf(&b, "# module %s (%s)\n%s\n", m.Path, m.File, strings.TrimRight(m.Text, "\n"))
			b.WriteString("\n")
		}
		return text("agni://library", "text/plain", b.String()), nil
	})
	srv.RegisterResource(core.ResourceDef{
		URI: "agni://rules", Name: "rules", MimeType: "text/plain",
		Description: "Every rule the check tool runs, with its severity and what it checks.",
	}, func(ctx core.ResourceContext, req core.ResourceRequest) (core.ResourceResult, error) {
		if s.api.Check == nil {
			return core.ResourceResult{}, fmt.Errorf("this server runs no checks")
		}
		resp, err := s.api.Check.ListRules(ctx, &webapi.ListRulesRequest{})
		if err != nil {
			return core.ResourceResult{}, err
		}
		var b strings.Builder
		for _, r := range resp.GetRules() {
			fmt.Fprintf(&b, "%s [%s]: %s\n", r.GetName(), r.GetSeverity(), strings.Join(strings.Fields(r.GetSummary()), " "))
		}
		return text("agni://rules", "text/plain", b.String()), nil
	})
	srv.RegisterResource(core.ResourceDef{
		URI: "agni://designs", Name: "designs", MimeType: "application/json",
		Description: "The designs this server answers about, by the name the tools take, with each one's declared revisions.",
	}, func(ctx core.ResourceContext, req core.ResourceRequest) (core.ResourceResult, error) {
		list, err := s.listDesigns(ctx)
		if err != nil {
			return core.ResourceResult{}, err
		}
		data, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			return core.ResourceResult{}, err
		}
		return text("agni://designs", "application/json", string(data)), nil
	})
}

func text(uri, mime, body string) core.ResourceResult {
	return core.ResourceResult{Contents: []core.ResourceReadContent{{URI: uri, MimeType: mime, Text: body}}}
}
