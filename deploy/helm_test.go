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
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Replicas *int `yaml:"replicas"`
		Strategy struct {
			Type string `yaml:"type"`
		} `yaml:"strategy"`
		Template struct {
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
			ReadOnly  bool   `yaml:"readOnly"`
		} `yaml:"volumeMounts"`
	} `yaml:"containers"`
	Volumes []struct {
		Name   string `yaml:"name"`
		Secret *struct {
			SecretName string `yaml:"secretName"`
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
			v, ok := argValue(c.Args, f)
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
	c := find(t, ms, "Deployment", "agni").Spec.Template.Spec.Containers[0]
	if v, _ := argValue(c.Args, "--trusted-proxy"); v != "10.244.0.0/16" {
		t.Errorf("--trusted-proxy=%q, want the values' 10.244.0.0/16", v)
	}
	if _, ok := argValue(c.Args, "--tls-cert"); ok {
		t.Error("agni ends TLS itself, though the ingress ends it")
	}
	if v, _ := argValue(c.Args, "--auth"); v != "tokens" {
		t.Errorf("--auth=%q, want tokens", v)
	}
	if v, _ := argValue(c.Args, "--public-url"); v != "https://agni.example.com" {
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
		addr, _ := argValue(c.Args, "--addr")
		if len(c.Ports) != 1 || ":"+strconv.Itoa(c.Ports[0].ContainerPort) != addr {
			t.Errorf("%s: --addr=%s but the container port is %v", name, addr, c.Ports)
		}
		for which, p := range map[string]probe{"readiness": c.ReadinessProbe, "liveness": c.LivenessProbe} {
			if p.HTTPGet.Path != "/healthz" || p.HTTPGet.Port != "http" {
				t.Errorf("%s: the %s probe asks %v %v, want /healthz on the http port", name, which, p.HTTPGet.Path, p.HTTPGet.Port)
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
				v, ok := argValue(pod.Containers[0].Args, f)
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
		a, _ := argValue(agni.Containers[0].Args, f)
		d, _ := argValue(ds.Containers[0].Args, f)
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
