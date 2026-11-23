package main

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/panyam/agni/service"
)

// configured parses args as serve's flags, applies the --config file holding yml, and returns the
// command for its flags to be read back, with the inline bootstrap plan and the error.
func configured(t *testing.T, yml string, args ...string) (*cobra.Command, *service.BootstrapPlan, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "serve.yaml")
	if err := os.WriteFile(path, []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	// serve under the root, so the flags every agni command reads are there too.
	cmd, _, err := rootCmd().Find([]string{"serve"})
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.ParseFlags(append(args, "--config", path)); err != nil {
		t.Fatal(err)
	}
	plan, err := applyServeConfig(cmd, path)
	return cmd, plan, err
}

func flagString(t *testing.T, cmd *cobra.Command, name string) string {
	t.Helper()
	return cmd.Flags().Lookup(name).Value.String()
}

// A --config file sets serve's flags, in nested sections or flat, with lists for list flags, and
// leaves a secret as the reference it names (agni issue 1853).
func TestServeConfigSetsTheFlagsItNames(t *testing.T) {
	cmd, _, err := configured(t, `
addr: ":9090"
store: file:///srv/agni/meta
poll: 5m
mcp: true
github:
  client_id: Iv1.nested
  client_secret: file:/run/secrets/agni/gh
  allow: [org:acme, user:alice]
oidc_issuer: https://login.example.com
`)
	if err != nil {
		t.Fatal(err)
	}
	for flag, want := range map[string]string{
		"addr": ":9090", "store": "file:///srv/agni/meta", "poll": "5m0s", "mcp": "true",
		"github-client-id": "Iv1.nested", "github-client-secret": "file:/run/secrets/agni/gh",
		"github-allow": "[org:acme,user:alice]", "oidc-issuer": "https://login.example.com",
	} {
		if got := flagString(t, cmd, flag); got != want {
			t.Errorf("--%s = %q; want %q", flag, got, want)
		}
	}
}

// A flag typed on the command line wins over the file outright, a list flag included.
func TestServeConfigLosesToATypedFlag(t *testing.T) {
	cmd, _, err := configured(t, "addr: \":9090\"\ngithub_allow: [org:acme]\nstore: file:///from/file\n",
		"--addr", ":7070", "--github-allow", "user:bob")
	if err != nil {
		t.Fatal(err)
	}
	if got := flagString(t, cmd, "addr"); got != ":7070" {
		t.Errorf("--addr = %q; want the typed :7070", got)
	}
	if got := flagString(t, cmd, "github-allow"); got != "[user:bob]" {
		t.Errorf("--github-allow = %q; want the typed [user:bob] alone", got)
	}
	if got := flagString(t, cmd, "store"); got != "file:///from/file" {
		t.Errorf("--store = %q; want the file's, which no flag typed", got)
	}
}

// The file refuses a key naming no flag, two spellings of one flag, a value the flag refuses, a list
// for a flag that takes one value, and a flag every agni command reads before serve does. Each
// error names the key.
func TestServeConfigRefusesWhatItCannotApply(t *testing.T) {
	for _, c := range []struct{ yml, want string }{
		{"github:\n  clientid: x\n", "github.clientid: there is no flag --github-clientid"},
		{"github_client_id: a\ngithub:\n  client_id: b\n", "github_client_id and github.client_id both set --github-client-id"},
		{"poll: often\n", "poll: invalid argument"},
		{"addr: [a, b]\n", "addr: --addr takes one value, not a list"},
		{"mount: [board=/srv]\n", "mount: --mount is read before the file"},
		{"config: other.yaml\n", "config: there is no flag --config"},
		{"bootstrap:\n  mounts: [{id: x}]\n", "bootstrap:"},
		{"- addr\n", "want a mapping"},
	} {
		if _, _, err := configured(t, c.yml); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: err = %v; want one saying %q", c.yml, err, c.want)
		}
	}
}

// bootstrap: holds the resources inline, or names their file as --bootstrap does. A typed
// --bootstrap wins over either.
func TestServeConfigTakesTheBootstrapInlineOrByPath(t *testing.T) {
	inline := "bootstrap:\n  prune: true\n  mounts:\n    - {id: board, source: github://acme/board, ref: main}\n"
	_, plan, err := configured(t, inline)
	if err != nil || plan == nil || !plan.Prune || len(plan.Mounts) != 1 || plan.Mounts[0].ID != "board" {
		t.Fatalf("inline bootstrap = %+v, %v; want the board mount, pruning", plan, err)
	}
	cmd, plan, err := configured(t, "bootstrap: /etc/agni/bootstrap.yaml\n")
	if err != nil || plan != nil || flagString(t, cmd, "bootstrap") != "/etc/agni/bootstrap.yaml" {
		t.Errorf("bootstrap by path: plan %+v, --bootstrap %q, %v; want the path in --bootstrap", plan, flagString(t, cmd, "bootstrap"), err)
	}
	cmd, plan, err = configured(t, inline, "--bootstrap", "typed.yaml")
	if err != nil || plan != nil || flagString(t, cmd, "bootstrap") != "typed.yaml" {
		t.Errorf("typed --bootstrap: plan %+v, --bootstrap %q, %v; want the typed file alone", plan, flagString(t, cmd, "bootstrap"), err)
	}
}

// agni serve --config reads the file before it checks anything, so a setting only the file holds
// reaches serve's own refusals, and a bad file stops it before it listens.
func TestServeWithConfigChecksTheFilesSettingsBeforeListening(t *testing.T) {
	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()
	run := func(yml string) error {
		path := filepath.Join(dir, "serve.yaml")
		if err := os.WriteFile(path, []byte(yml), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := rootCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"serve", "--config", path})
		return cmd.Execute()
	}
	store := "file:///" + strings.TrimPrefix(filepath.ToSlash(filepath.Join(dir, "store")), "/")
	if err := run("addr: 192.0.2.1:0\nstore: " + store + "\nauth: tokens\n"); err == nil || !strings.Contains(err.Error(), "in the clear") {
		t.Errorf("serve --config with --auth tokens and no TLS = %v; want serve's own refusal, so the file's settings applied", err)
	}
	if err := run("addr: 192.0.2.1:0\nbootstrap:\n  mounts: [{id: board, source: github://acme/board}]\n"); err == nil || !strings.Contains(err.Error(), "needs --store and --uploads") {
		t.Errorf("serve --config with an inline bootstrap and no store = %v; want it refused", err)
	}
	if err := run("addr: 192.0.2.1:0\nstoer: x\n"); err == nil || !strings.Contains(err.Error(), "stoer: there is no flag --stoer") {
		t.Errorf("serve --config with a misspelt key = %v; want it refused by name", err)
	}
}

// Every flag serve takes can be set from the file, but --config itself, so a new flag needs no
// code to get a file form.
func TestEveryServeFlagHasAFileForm(t *testing.T) {
	cmd := serveCmd()
	var names []string
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) { names = append(names, f.Name) })
	if len(names) < 40 {
		t.Fatalf("serve has %d flags; the count guard wants 40 or more", len(names))
	}
	for _, n := range names {
		if n == "config" || n == "help" {
			continue
		}
		yml := strings.ReplaceAll(n, "-", "_") + ": " + sample(cmd.LocalFlags().Lookup(n).Value.Type()) + "\n"
		got, _, err := configured(t, yml)
		if err != nil || !got.Flags().Lookup(n).Changed {
			t.Errorf("%q didn't set --%s: %v", strings.TrimSpace(yml), n, err)
		}
	}
	if !slices.Contains(names, "github-client-id") {
		t.Error("serve has no --github-client-id, so this test reads the wrong flags")
	}
}

// sample is a value a flag of type typ takes.
func sample(typ string) string {
	switch typ {
	case "bool":
		return "true"
	case "int", "int64":
		return "1"
	case "duration":
		return "1s"
	case "stringSlice", "stringArray":
		return "[a]"
	}
	return "x"
}

// The Deploying guide's serve.yaml applies to serve's own flags, so a renamed flag fails here rather
// than in an admin's start (agni issue 1853).
func TestTheGuidesServeYAMLApplies(t *testing.T) {
	page, err := os.ReadFile("../../docsite/content/guide/deploying.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(page), "## Keep the server's settings in one file\n")
	if !ok {
		t.Fatal("the guide has no section \"Keep the server's settings in one file\"")
	}
	_, block, ok := strings.Cut(section, "```yaml\n")
	block, _, closed := strings.Cut(block, "```")
	if !ok || !closed {
		t.Fatal("the section has no yaml block")
	}
	cmd, _, err := configured(t, block)
	if err != nil {
		t.Fatalf("the guide's serve.yaml: %v", err)
	}
	for flag, want := range map[string]string{"github-client-id": "Iv1.0123456789abcdef", "tls-key": "/srv/agni/tls.key", "bootstrap": "/srv/agni/bootstrap.yaml"} {
		if got := flagString(t, cmd, flag); got != want {
			t.Errorf("the guide's serve.yaml sets --%s = %q; want %q", flag, got, want)
		}
	}
}
