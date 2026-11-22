package mountset

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panyam/goapplib/storage/connections"
	"github.com/panyam/goapplib/storage/connections/entratest"
	"github.com/panyam/goapplib/storage/drivers/msgraph/graphtest"

	"github.com/panyam/agni/internal/records/recordstest"
	"github.com/panyam/agni/service"
)

// RenewSubscriptions renews a SharePoint mount's Graph subscription near its end, makes it again
// when Graph forgot it, and drops the record of a mount that's gone (agni issue 1729).
func TestRenewSubscriptions(t *testing.T) {
	ets := entratest.New(t)
	ets.AddSecretApp("contoso", "agni-app", "app-secret")
	graph := graphtest.New(t)
	graph.AcceptToken(ets.ValidToken)
	graph.AddToken("admin")
	graph.AddDrive("d1").Put("hw/bb.kicad_sch", "(kicad_sch)")
	t.Setenv("AGNI_TEST_ENTRA_SECRET", "app-secret")
	dir := t.TempDir()
	subs := recordstest.New(t)
	tab, err := Open(Config{Uploads: filepath.Join(dir, "uploads"), Records: filepath.Join(dir, "mounts"), Connections: filepath.Join(dir, "connections"),
		GraphAPI: graph.BaseURL(), Subscriptions: subs})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := tab.Connections().PutConnection(ctx, connections.Connection{ID: "sp", Kind: connections.KindEntra, Scope: connections.ScopeServer,
		BaseURL: ets.URL, TenantID: "contoso", ClientID: "agni-app", EntraAuth: connections.EntraSecret, KeyRef: "env:AGNI_TEST_ENTRA_SECRET", Status: connections.StatusOK}); err != nil {
		t.Fatal(err)
	}
	if _, err := tab.CreateMount(ctx, "sp", "msgraph://drives/d1/root/hw", "", "sp"); err != nil {
		t.Fatal(err)
	}
	state := service.WebhookSecrets{Graph: []byte("client-state")}
	h, err := tab.CreateWebhook(ctx, "sp", "https://agni.example.com/hooks/msgraph", state)
	if err != nil || h.SubscriptionID == "" || !h.Created {
		t.Fatalf("CreateWebhook = %+v, %v; want a new subscription", h, err)
	}
	if err := subs.Put(ctx, "gone.json", []byte(`{"mount":"gone","id":"sub-x","url":"https://agni.example.com/hooks/msgraph","expires":"2026-01-01T00:00:00Z"}`)); err != nil {
		t.Fatal(err)
	}

	// Nothing ends within a day, so nothing is renewed, and the gone mount's record goes only once
	// it falls due.
	if n, err := tab.RenewSubscriptions(ctx, 24*time.Hour, state.Graph, t.Output()); err != nil || n != 0 {
		t.Fatalf("RenewSubscriptions(1 day) = %d, %v; want nothing due but the gone mount", n, err)
	}
	if _, err := subs.Get(ctx, "gone.json"); err == nil {
		t.Errorf("the record of a mount that's gone is still kept after it fell due")
	}
	patches := func() int {
		n := 0
		for _, r := range graph.Requests() {
			if r.Method == http.MethodPatch && strings.HasSuffix(r.Path, "/subscriptions/"+h.SubscriptionID) {
				n++
			}
		}
		return n
	}
	if n, err := tab.RenewSubscriptions(ctx, 30*24*time.Hour, state.Graph, t.Output()); err != nil || n != 1 {
		t.Fatalf("RenewSubscriptions(30 days) = %d, %v; want the one renewed", n, err)
	}
	if p := patches(); p != 1 || len(graph.Subscriptions()) != 1 {
		t.Errorf("after the renewal Graph got %d renewals and holds %d subscriptions; want one of each", p, len(graph.Subscriptions()))
	}

	req, _ := http.NewRequest(http.MethodDelete, graph.BaseURL()+"/subscriptions/"+h.SubscriptionID, nil)
	req.Header.Set("Authorization", "Bearer admin")
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("forgetting the subscription in the fake = %v, %v", resp, err)
	}
	if n, err := tab.RenewSubscriptions(ctx, 30*24*time.Hour, state.Graph, t.Output()); err != nil || n != 1 {
		t.Fatalf("RenewSubscriptions after Graph forgot it = %d, %v; want it made again", n, err)
	}
	if len(graph.Subscriptions()) != 1 {
		t.Fatalf("Graph holds %d subscriptions after it forgot one; want a new one", len(graph.Subscriptions()))
	}
	if _, ok := graph.Subscriptions()[h.SubscriptionID]; ok {
		t.Errorf("the forgotten subscription %s came back; want a new id", h.SubscriptionID)
	}
}
