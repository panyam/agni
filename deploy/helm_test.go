package deploy

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/internal/flagfile"
	"github.com/panyam/agni/service"
)

// The Helm chart's tests render it with the helm CLI, as `helm install` would, and read the
// manifests back (agni issue 1803). The images workflow installs it on a kind cluster and runs the
// enterprise exercise against it.

const chart = "helm/agni"

// required are the values the chart refuses to render without.
var required = []string{
	"publicURL=https://agni.example.com",
	"ingress.host=agni.example.com",
	"github.clientID=Iv1.example",
	"github.allow={org:acme}",
	"trustedProxies={10.244.0.0/16}",
}

type manifest struct {
	Kind     string            `yaml:"kind"`
	Data     map[string]string `yaml:"data"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Replicas *int `yaml:"replicas"`
		Strategy struct {
			Type string `yaml:"type"`
		} `yaml:"strategy"`
		Template struct {
			Metadata struct {
				Annotations map[string]string `yaml:"annotations"`
			} `yaml:"metadata"`
			Spec podSpec `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type podSpec struct {
	Affinity *struct {
		PodAffinity any `yaml:"podAffinity"`
	} `yaml:"affinity"`
	Containers []struct {
		Name  string   `yaml:"name"`
		Args  []string `yaml:"args"`
		Ports []struct {
			ContainerPort int `yaml:"containerPort"`
		} `yaml:"ports"`
		Env []struct {
			Name  string `yaml:"name"`
			Value string `yaml:"value"`
		} `yaml:"env"`
		ReadinessProbe probe `yaml:"readinessProbe"`
		LivenessProbe  probe `yaml:"livenessProbe"`
		VolumeMounts   []struct {
			Name      string `yaml:"name"`
			MountPath string `yaml:"mountPath"`
			SubPath   string `yaml:"subPath"`
			ReadOnly  bool   `yaml:"readOnly"`
		} `yaml:"volumeMounts"`
	} `yaml:"containers"`
	Volumes []struct {
		Name   string `yaml:"name"`
		Secret *struct {
			SecretName string `yaml:"secretName"`
			Items      []struct {
				Key string `yaml:"key"`
			} `yaml:"items"`
		} `yaml:"secret"`
		PersistentVolumeClaim *struct {
			ClaimName string `yaml:"claimName"`
		} `yaml:"persistentVolumeClaim"`
		ConfigMap *struct {
			Name string `yaml:"name"`
		} `yaml:"configMap"`
	} `yaml:"volumes"`
}

type probe struct {
	HTTPGet struct {
		Path string `yaml:"path"`
		Port any    `yaml:"port"`
	} `yaml:"httpGet"`
}

// render runs helm template over the chart with the required values and sets, and returns the
// manifests, or helm's error output.
func render(t *testing.T, sets ...string) ([]manifest, error) {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Fatal("the chart's tests need the helm CLI on PATH: brew install helm, or https://helm.sh/docs/intro/install/")
	}
	args := []string{"template", "agni", filepath.FromSlash(chart)}
	for _, s := range sets {
		args = append(args, "--set", s)
	}
	cmd := exec.Command("helm", args...) // #nosec G204 -- fixed chart path and the test's own values
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.New(stderr.String())
	}
	var ms []manifest
	dec := yaml.NewDecoder(bytes.NewReader(out))
	for {
		var m manifest
		err := dec.Decode(&m)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if m.Kind != "" {
			ms = append(ms, m)
		}
	}
	return ms, nil
}

func mustRender(t *testing.T, sets ...string) []manifest {
	t.Helper()
	ms, err := render(t, append(slices.Clone(required), sets...)...)
	if err != nil {
		t.Fatalf("helm template: %v", err)
	}
	return ms
}

func find(t *testing.T, ms []manifest, kind, name string) manifest {
	t.Helper()
	for _, m := range ms {
		if m.Kind == kind && m.Metadata.Name == name {
			return m
		}
	}
	t.Fatalf("the chart renders no %s %s", kind, name)
	return manifest{}
}

func argValue(args []string, flag string) (string, bool) {
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, flag+"="); ok {
			return v, true
		}
	}
	return "", false
}

// flagsOf is the flags a Deployment's server runs with. That is its args, plus, for one started
// with --config, the flags its serve.yaml sets, read by the flagfile package agni serve reads it
// with (agni issue 1853). An inline bootstrap reads as --bootstrap=<inline>.
func flagsOf(t *testing.T, ms []manifest, name string) []string {
	t.Helper()
	args := slices.Clone(find(t, ms, "Deployment", name).Spec.Template.Spec.Containers[0].Args)
	if _, ok := argValue(args, "--config"); !ok {
		return args
	}
	entries, err := flagfile.Parse([]byte(find(t, ms, "ConfigMap", name+"-serve").Data["serve.yaml"]), "bootstrap")
	if err != nil {
		t.Fatalf("%s's serve.yaml: %v", name, err)
	}
	for _, e := range entries {
		switch {
		case e.Raw != nil && e.Raw.Kind == yaml.ScalarNode:
			args = append(args, "--"+e.Flag+"="+e.Raw.Value)
		case e.Raw != nil:
			args = append(args, "--"+e.Flag+"=<inline>")
		default:
			args = append(args, "--"+e.Flag+"="+strings.Join(e.Values, ","))
		}
	}
	return args
}

func TestTheChartReadsEverySecretFromASecretItNeverMakes(t *testing.T) {
	ms := mustRender(t, "datasheets.enabled=true", "datasheets.host=ds.example.com")
	for _, m := range ms {
		if m.Kind == "Secret" {
			t.Errorf("the chart makes Secret %s, so a secret would sit in a values file", m.Metadata.Name)
		}
	}
	for _, name := range []string{"agni", "agni-datasheets"} {
		pod := find(t, ms, "Deployment", name).Spec.Template.Spec
		secretAt := map[string]string{}
		for _, v := range pod.Volumes {
			if v.Secret != nil {
				secretAt[v.Name] = v.Secret.SecretName
			}
		}
		c := pod.Containers[0]
		mounted := map[string]string{}
		for _, vm := range c.VolumeMounts {
			if s, ok := secretAt[vm.Name]; ok {
				mounted[vm.MountPath] = s
			}
		}
		flags := []string{"--connections-key"}
		if name == "agni" {
			flags = append(flags, "--github-client-secret", "--github-webhook-secret")
		}
		for _, f := range flags {
			v, ok := argValue(flagsOf(t, ms, name), f)
			if !ok {
				t.Errorf("%s: no %s", name, f)
				continue
			}
			p, ok := strings.CutPrefix(v, "file:")
			if !ok {
				t.Errorf("%s: %s=%s reads no file", name, f, v)
				continue
			}
			if mounted[filepath.ToSlash(filepath.Dir(p))] != "agni" {
				t.Errorf("%s: %s reads %s, which is not in the Secret agni (mounted: %v)", name, f, p, mounted)
			}
		}
	}
}

func TestTheChartServesBehindTheIngressThroughATrustedProxy(t *testing.T) {
	ms := mustRender(t)
	args := flagsOf(t, ms, "agni")
	if v, _ := argValue(args, "--trusted-proxy"); v != "10.244.0.0/16" {
		t.Errorf("--trusted-proxy=%q, want the values' 10.244.0.0/16", v)
	}
	if _, ok := argValue(args, "--tls-cert"); ok {
		t.Error("agni ends TLS itself, though the ingress ends it")
	}
	if v, _ := argValue(args, "--auth"); v != "tokens" {
		t.Errorf("--auth=%q, want tokens", v)
	}
	if v, _ := argValue(args, "--public-url"); v != "https://agni.example.com" {
		t.Errorf("--public-url=%q", v)
	}
	find(t, ms, "Ingress", "agni")

	_, err := render(t, slices.DeleteFunc(slices.Clone(required), func(s string) bool { return strings.HasPrefix(s, "trustedProxies=") })...)
	if err == nil || !strings.Contains(err.Error(), "set trustedProxies") {
		t.Errorf("the chart renders with no trustedProxies, where agni would refuse to start; err=%v", err)
	}
}

func TestTheChartsProbesAskThePortAgniListensOn(t *testing.T) {
	ms := mustRender(t, "datasheets.enabled=true", "datasheets.host=ds.example.com")
	for _, name := range []string{"agni", "agni-datasheets"} {
		c := find(t, ms, "Deployment", name).Spec.Template.Spec.Containers[0]
		addr, _ := argValue(flagsOf(t, ms, name), "--addr")
		if len(c.Ports) != 1 || ":"+strconv.Itoa(c.Ports[0].ContainerPort) != addr {
			t.Errorf("%s: --addr=%s but the container port is %v", name, addr, c.Ports)
		}
		// agni is ready once its bootstrap applied (agni issue 1808), and the workbench has none.
		ready := map[string]string{"agni": "/readyz", "agni-datasheets": "/healthz"}[name]
		for which, p := range map[string]probe{"readiness": c.ReadinessProbe, "liveness": c.LivenessProbe} {
			want := "/healthz"
			if which == "readiness" {
				want = ready
			}
			if p.HTTPGet.Path != want || p.HTTPGet.Port != "http" {
				t.Errorf("%s: the %s probe asks %v %v, want %s on the http port", name, which, p.HTTPGet.Path, p.HTTPGet.Port, want)
			}
		}
	}
}

func TestTheChartKeepsItsWritesOnTheClaimWithOneWriter(t *testing.T) {
	cases := map[string][]string{
		"on the claim": nil,
		"in a bucket":  {"uploads=s3://bucket/agni"},
	}
	for name, sets := range cases {
		t.Run(name, func(t *testing.T) {
			ms := mustRender(t, sets...)
			d := find(t, ms, "Deployment", "agni")
			replicas := -1
			if d.Spec.Replicas != nil {
				replicas = *d.Spec.Replicas
			}
			if replicas != 1 || d.Spec.Strategy.Type != "Recreate" {
				t.Errorf("replicas %d, strategy %q: two pods would write one store", replicas, d.Spec.Strategy.Type)
			}
			pod := d.Spec.Template.Spec
			claimed := map[string]bool{}
			for _, v := range pod.Volumes {
				if v.PersistentVolumeClaim != nil {
					claimed[v.Name] = true
				}
			}
			var onClaim []string
			for _, vm := range pod.Containers[0].VolumeMounts {
				if claimed[vm.Name] && !vm.ReadOnly {
					onClaim = append(onClaim, vm.MountPath)
				}
			}
			flags := slices.Clone(writableFlags)
			if sets != nil {
				flags = []string{"--store", "--uploads-cache"}
			}
			checked := 0
			for _, f := range flags {
				v, ok := argValue(flagsOf(t, ms, "agni"), f)
				if !ok {
					t.Errorf("no %s", f)
					continue
				}
				p := strings.TrimPrefix(v, "file://")
				if !slices.ContainsFunc(onClaim, func(dir string) bool { return under(p, dir) }) {
					t.Errorf("%s=%s is not on the claim (claim mounts: %v)", f, v, onClaim)
				}
				checked++
			}
			if checked != len(flags) {
				t.Errorf("checked %d of %d flags", checked, len(flags))
			}
			find(t, ms, "PersistentVolumeClaim", "agni-data")
		})
	}
}

func TestTheChartsWorkbenchReadsAgnisClaimBesideIt(t *testing.T) {
	ms := mustRender(t, "datasheets.enabled=true", "datasheets.host=ds.example.com", "caConfigMap=team-ca")
	agni := find(t, ms, "Deployment", "agni").Spec.Template.Spec
	ds := find(t, ms, "Deployment", "agni-datasheets").Spec.Template.Spec
	for _, f := range writableFlags {
		a, _ := argValue(flagsOf(t, ms, "agni"), f)
		d, _ := argValue(flagsOf(t, ms, "agni-datasheets"), f)
		if a != d {
			t.Errorf("%s: agni %q, the workbench %q", f, a, d)
		}
	}
	var dataRO bool
	for _, vm := range ds.Containers[0].VolumeMounts {
		if vm.MountPath == "/var/lib/agni" {
			dataRO = vm.ReadOnly
		}
	}
	if !dataRO {
		t.Error("the workbench mounts agni's claim writable, or not at all")
	}
	if ds.Affinity == nil || ds.Affinity.PodAffinity == nil {
		t.Error("the workbench can land on another node than agni's ReadWriteOnce claim")
	}
	for name, pod := range map[string]podSpec{"agni": agni, "agni-datasheets": ds} {
		var dir string
		for _, e := range pod.Containers[0].Env {
			if e.Name == "SSL_CERT_DIR" {
				dir = e.Value
			}
		}
		if dir != "/etc/ssl/certs:/etc/agni/ca" {
			t.Errorf("%s: SSL_CERT_DIR=%q, which drops the system's roots or the team's CA", name, dir)
		}
		var fromMap bool
		for _, v := range pod.Volumes {
			if v.Name == "ca" && v.ConfigMap != nil && v.ConfigMap.Name == "team-ca" {
				fromMap = true
			}
		}
		if !fromMap {
			t.Errorf("%s: the CA folder doesn't come from caConfigMap", name)
		}
	}
}

// With oidc.issuer set, the chart passes the OIDC sign-in flags and mounts oidc-client-secret from
// the same Secret. A public client mounts no secret, and an issuer with no allow list fails to
// render, as agni would refuse to start (agni issue 1811).
func TestTheChartSignsInThroughOIDCWhenAsked(t *testing.T) {
	args := func(ms []manifest) []string { return flagsOf(t, ms, "agni") }
	keys := func(ms []manifest) []string {
		var out []string
		for _, v := range find(t, ms, "Deployment", "agni").Spec.Template.Spec.Volumes {
			if v.Secret != nil {
				for _, it := range v.Secret.Items {
					out = append(out, it.Key)
				}
			}
		}
		return out
	}
	off := mustRender(t)
	if _, ok := argValue(args(off), "--oidc-issuer"); ok || slices.Contains(keys(off), "oidc-client-secret") {
		t.Errorf("with no oidc.issuer the chart passes %v and mounts %v; want no OIDC sign-in", args(off), keys(off))
	}
	oidc := []string{"oidc.issuer=https://login.example.com/t/v2.0", "oidc.clientID=agni", "oidc.allow={domain:example.com,group:g-eng}", "oidc.label=Okta"}
	on := mustRender(t, oidc...)
	for flag, want := range map[string]string{
		"--oidc-issuer": "https://login.example.com/t/v2.0", "--oidc-client-id": "agni", "--oidc-allow": "domain:example.com,group:g-eng",
		"--oidc-label": "Okta", "--oidc-client-secret": "file:/run/secrets/agni/oidc-client-secret",
	} {
		if got, _ := argValue(args(on), flag); got != want {
			t.Errorf("with oidc set, %s = %q; want %q", flag, got, want)
		}
	}
	if !slices.Contains(keys(on), "oidc-client-secret") {
		t.Errorf("with oidc set the chart mounts %v; want oidc-client-secret from the Secret", keys(on))
	}
	public := mustRender(t, append(oidc, "oidc.clientSecret=false")...)
	if _, ok := argValue(args(public), "--oidc-client-secret"); ok || slices.Contains(keys(public), "oidc-client-secret") {
		t.Errorf("a public client passes %v and mounts %v; want no client secret", args(public), keys(public))
	}
	if _, err := render(t, append(slices.Clone(required), "oidc.issuer=https://login.example.com", "oidc.clientID=agni")...); err == nil || !strings.Contains(err.Error(), "set oidc.allow") {
		t.Errorf("an issuer with no allow list rendered: %v; want it refused, asking for oidc.allow", err)
	}
}

// The chart hands agni serve one serve.yaml from a ConfigMap, with --config alone on its command
// line (agni issue 1853). The file holds the values' serve section beside the chart's own settings,
// and the chart's win, since the pod is built around them. A changed file restarts the pod.
func TestTheChartHandsTheServerOneServeYAML(t *testing.T) {
	ms := mustRender(t, "serve.poll=5m", "serve.addr=:9999")
	d := find(t, ms, "Deployment", "agni")
	spec := d.Spec.Template.Spec
	if got := spec.Containers[0].Args; !slices.Equal(got, []string{"serve", "--config=/etc/agni/serve.yaml"}) {
		t.Errorf("agni's args = %v; want serve --config=/etc/agni/serve.yaml alone", got)
	}
	var mounted, volume bool
	for _, m := range spec.Containers[0].VolumeMounts {
		mounted = mounted || (m.Name == "serve-config" && m.MountPath == "/etc/agni/serve.yaml" && m.SubPath == "serve.yaml" && m.ReadOnly)
	}
	for _, v := range spec.Volumes {
		volume = volume || (v.Name == "serve-config" && v.ConfigMap != nil && v.ConfigMap.Name == "agni-serve")
	}
	if !mounted || !volume {
		t.Errorf("serve.yaml mounted %v from the ConfigMap %v; want agni-serve's serve.yaml read-only at /etc/agni/serve.yaml", mounted, volume)
	}
	flags := flagsOf(t, ms, "agni")
	if v, _ := argValue(flags, "--poll"); v != "5m" {
		t.Errorf("--poll = %q; want the values' serve.poll 5m", v)
	}
	if v, _ := argValue(flags, "--addr"); v != ":8080" {
		t.Errorf("--addr = %q; want the chart's :8080 over serve.addr, since the port and probes use it", v)
	}
	sum := d.Spec.Template.Metadata.Annotations["agni/serve-sha256"]
	other := find(t, mustRender(t, "serve.poll=10m"), "Deployment", "agni").Spec.Template.Metadata.Annotations["agni/serve-sha256"]
	if sum == "" || sum == other {
		t.Errorf("the pod's serve-sha256 is %q with poll 5m and %q with 10m; want it to change, so a changed file restarts the pod", sum, other)
	}
}

// The chart puts the values' bootstrap in serve.yaml, where it parses as a plan, and puts none
// there without one (agni issue 1808).
func TestTheChartHandsTheServerItsBootstrap(t *testing.T) {
	bootstrapOf := func(ms []manifest) *yaml.Node {
		var doc map[string]yaml.Node
		if err := yaml.Unmarshal([]byte(find(t, ms, "ConfigMap", "agni-serve").Data["serve.yaml"]), &doc); err != nil {
			t.Fatal(err)
		}
		if n, ok := doc["bootstrap"]; ok {
			return &n
		}
		return nil
	}
	if n := bootstrapOf(mustRender(t)); n != nil {
		t.Errorf("with no bootstrap, serve.yaml holds bootstrap: %v; want none", n)
	}
	on := mustRender(t, "bootstrap.mounts[0].id=board", "bootstrap.mounts[0].source=github://hw/baseboard",
		"bootstrap.designs[0].id=bb", "bootstrap.designs[0].link.schematic=board/gateway.kicad_sch@main")
	n := bootstrapOf(on)
	if n == nil {
		t.Fatal("with a bootstrap, serve.yaml holds none")
	}
	b, err := yaml.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.ParseBootstrap(b)
	if err != nil || len(plan.Mounts) != 1 || len(plan.Designs) != 1 || plan.Designs[0].Link["schematic"] != "board/gateway.kicad_sch@main" {
		t.Errorf("serve.yaml's bootstrap = %+v, %v; want the values' mount and design", plan, err)
	}
}
