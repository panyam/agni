// Package deploy holds no code. Its test reads compose.yaml beside the Dockerfile, so a change to
// either that would lose a customer's uploads fails the gate, which never runs Docker (agni issue
// 1698). hack/compose_smoke.sh brings the bundle up for real in the images workflow.
package deploy

import (
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Services map[string]struct {
		Command     []string          `yaml:"command"`
		Volumes     []string          `yaml:"volumes"`
		Secrets     []string          `yaml:"secrets"`
		Environment map[string]string `yaml:"environment"`
	} `yaml:"services"`
	Volumes map[string]any `yaml:"volumes"`
	Secrets map[string]struct {
		File string `yaml:"file"`
	} `yaml:"secrets"`
}

// bundles are the compose files a release attaches: the trial bundle, and the enterprise bundle
// with TLS, tokens and GitHub sign-in (agni issue 1802).
var bundles = []string{"compose.yaml", "enterprise/compose.yaml"}

// writableFlags are the serve flags that name a folder the server writes into, as a file:// URL.
var writableFlags = []string{"--store", "--uploads"}

func readComposeAt(t *testing.T, file string) composeFile {
	t.Helper()
	b, err := os.ReadFile(file) // #nosec G304 -- one of bundles
	if err != nil {
		t.Fatal(err)
	}
	var c composeFile
	if err := yaml.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// namedTargets returns where the service mounts named volumes, as against host folders.
func namedTargets(t *testing.T, c composeFile, service string) []string {
	t.Helper()
	var out []string
	for _, v := range c.Services[service].Volumes {
		parts := strings.Split(v, ":")
		if len(parts) < 2 {
			t.Fatalf("volume %q has no target", v)
		}
		if _, named := c.Volumes[parts[0]]; named {
			out = append(out, parts[1])
		}
	}
	return out
}

func under(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, strings.TrimSuffix(dir, "/")+"/")
}

// A design a customer uploads, a review run and a snapshot's definition each live in a folder a
// serve flag names. Each folder must sit on a named volume, or recreating the container loses it.
func TestServeKeepsItsWritesOnANamedVolume(t *testing.T) {
	for _, file := range bundles {
		c := readComposeAt(t, file)
		targets := namedTargets(t, c, "agni")
		cmd := c.Services["agni"].Command
		for _, flag := range writableFlags {
			v, ok := flagValue(cmd, flag)
			if !ok {
				t.Errorf("%s: the agni service's command has no %s, so the server keeps that state in memory or not at all", file, flag)
				continue
			}
			dir, isFile := strings.CutPrefix(v, "file://")
			if !isFile {
				t.Errorf("%s: %s %s is no file:// URL", file, flag, v)
				continue
			}
			onVolume := false
			for _, target := range targets {
				onVolume = onVolume || under(dir, target)
			}
			if !onVolume {
				t.Errorf("%s: %s %s is on no named volume (the volumes are at %v), so recreating the container loses it", file, flag, dir, targets)
			}
		}
	}
}

// flagValue is the value cmd gives flag, written as "--flag value" or "--flag=value".
func flagValue(cmd []string, flag string) (string, bool) {
	for i, a := range cmd {
		if v, ok := strings.CutPrefix(a, flag+"="); ok {
			return v, true
		}
		if a == flag && i+1 < len(cmd) {
			return cmd[i+1], true
		}
	}
	return "", false
}

// A named volume copies the ownership of the folder it is mounted over, and a folder the image lacks
// comes up owned by root. The server runs as the user agni, so every named volume's target must be a
// folder the Dockerfile makes, or the server cannot write to it.
func TestTheImageMakesEveryNamedVolumesFolder(t *testing.T) {
	b, err := os.ReadFile("../Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	var made []string
	for _, m := range regexp.MustCompile(`mkdir -p ([^\\&\n]+)`).FindAllSubmatch(b, -1) {
		made = append(made, strings.Fields(string(m[1]))...)
	}
	if len(made) == 0 {
		t.Fatal("the Dockerfile has no mkdir -p")
	}
	for _, file := range bundles {
		c := readComposeAt(t, file)
		targets := namedTargets(t, c, "agni")
		if len(targets) == 0 {
			t.Fatalf("%s: the agni service mounts no named volume", file)
		}
		for _, target := range targets {
			ok := false
			for _, dir := range made {
				ok = ok || under(path.Clean(dir), path.Clean(target))
			}
			if !ok {
				t.Errorf("%s: the Dockerfile makes none of %s, so a named volume there is owned by root (it makes %v)", file, target, made)
			}
		}
	}
}

// The workbench serves the mounts the agni service made at runtime by reading agni's records and
// files (agni issue 1735). The agnids image's command names agni's --store and --uploads, so the
// datasheets service must mount agni's named volume there, read-only, since agnids only reads them.
func TestTheWorkbenchReadsTheAgniServicesMounts(t *testing.T) {
	for _, file := range bundles {
		workbenchReadsAgni(t, file, readComposeAt(t, file))
	}
}

func workbenchReadsAgni(t *testing.T, file string, c composeFile) {
	t.Helper()
	b, err := os.ReadFile("../Dockerfile.agnids")
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, flag := range []string{"store", "uploads"} {
		m := regexp.MustCompile(`"--` + flag + `", "file://([^"]+)"`).FindSubmatch(b)
		if m == nil {
			t.Fatalf("the agnids image's command names no --%s", flag)
		}
		want = append(want, string(m[1]))
	}
	agni := namedTargets(t, c, "agni")
	for _, dir := range want {
		found := false
		for _, v := range c.Services["datasheets"].Volumes {
			parts := strings.Split(v, ":")
			if _, named := c.Volumes[parts[0]]; !named || len(parts) < 3 || parts[2] != "ro" || !under(dir, parts[1]) {
				continue
			}
			for _, target := range agni {
				found = found || (target == parts[1] && namedVolumeAt(c, "agni", target) == parts[0])
			}
		}
		if !found {
			t.Errorf("%s: the datasheets service mounts no read-only copy of the agni service's volume at %s", file, dir)
		}
	}
}

// namedVolumeAt is the named volume service mounts at target.
func namedVolumeAt(c composeFile, service, target string) string {
	for _, v := range c.Services[service].Volumes {
		parts := strings.Split(v, ":")
		if len(parts) >= 2 && parts[1] == target {
			return parts[0]
		}
	}
	return ""
}

// The enterprise bundle takes every secret from a file the operator's .env names, mounted under
// /run/secrets, and never as a value in compose.yaml or the environment (agni issue 1802). Each flag
// that reads a secret names its file there, and the service mounts that secret.
func TestTheEnterpriseBundleReadsEverySecretFromAFile(t *testing.T) {
	c := readComposeAt(t, "enterprise/compose.yaml")
	secretFlags := map[string][]string{
		"agni":       {"--tls-cert", "--tls-key", "--connections-key", "--github-client-secret", "--github-webhook-secret"},
		"datasheets": {"--tls-cert", "--tls-key", "--connections-key"},
	}
	for service, flags := range secretFlags {
		cmd := c.Services[service].Command
		for _, flag := range flags {
			v, ok := flagValue(cmd, flag)
			if !ok {
				t.Errorf("the %s service's command has no %s", service, flag)
				continue
			}
			name, ok := strings.CutPrefix(strings.TrimPrefix(v, "file:"), "/run/secrets/")
			if !ok {
				t.Errorf("%s %s %s reads no file under /run/secrets", service, flag, v)
				continue
			}
			if !slices.Contains(c.Services[service].Secrets, name) {
				t.Errorf("%s %s reads secret %s, which the service doesn't mount (it mounts %v)", service, flag, name, c.Services[service].Secrets)
			}
			src := c.Secrets[name].File
			if !regexp.MustCompile(`^\$\{AGNI_[A-Z_]+:\?[^}]+\}$`).MatchString(src) {
				t.Errorf("secret %s comes from %q; want a file path the operator's .env must name, as ${AGNI_...:?why}", name, src)
			}
		}
		for _, a := range cmd {
			if strings.Contains(a, "env:") {
				t.Errorf("%s passes %q, a secret from the environment, where compose would hold its value", service, a)
			}
		}
	}
}

// A server connection's token is a file the server reads, so the enterprise bundle mounts a folder of
// them read-only, named by the operator's .env (agni issue 1802).
func TestTheEnterpriseBundleMountsTheServersConnectionSecrets(t *testing.T) {
	c := readComposeAt(t, "enterprise/compose.yaml")
	for _, v := range c.Services["agni"].Volumes {
		// The source can hold a colon of its own, as ${VAR:-default} does, so split from the right.
		rest, mode, _ := cutLast(v, ":")
		src, target, _ := cutLast(rest, ":")
		if target == "/run/agni/connections" {
			if mode != "ro" || !strings.HasPrefix(src, "${AGNI_CONNECTION_SECRETS") {
				t.Errorf("the connection secrets mount is %q; want a read-only folder that AGNI_CONNECTION_SECRETS names", v)
			}
			return
		}
	}
	t.Errorf("the agni service mounts no folder at /run/agni/connections, so a server connection's token has nowhere to live (volumes %v)", c.Services["agni"].Volumes)
}

// cutLast is strings.Cut at the last sep.
func cutLast(s, sep string) (before, after string, found bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}

// A GitHub Enterprise Server behind the team's own CA needs both servers to trust that CA. The bundle
// mounts a folder of CA certificates and adds it to SSL_CERT_DIR, keeping the system's roots, since
// SSL_CERT_FILE would replace them (agni issue 1802).
func TestTheEnterpriseBundleTrustsATeamsOwnCA(t *testing.T) {
	c := readComposeAt(t, "enterprise/compose.yaml")
	for _, service := range []string{"agni", "datasheets"} {
		svc := c.Services[service]
		dirs := strings.Split(svc.Environment["SSL_CERT_DIR"], ":")
		if !slices.Contains(dirs, "/etc/ssl/certs") || !slices.Contains(dirs, "/etc/agni/ca") {
			t.Errorf("%s's SSL_CERT_DIR = %q; want the system's /etc/ssl/certs and the mounted /etc/agni/ca", service, svc.Environment["SSL_CERT_DIR"])
		}
		if _, ok := svc.Environment["SSL_CERT_FILE"]; ok {
			t.Errorf("%s sets SSL_CERT_FILE, which replaces the system's roots", service)
		}
		mounted := false
		for _, v := range svc.Volumes {
			rest, mode, _ := cutLast(v, ":")
			src, target, _ := cutLast(rest, ":")
			mounted = mounted || target == "/etc/agni/ca" && mode == "ro" && strings.HasPrefix(src, "${AGNI_CA_DIR")
		}
		if !mounted {
			t.Errorf("%s mounts no read-only folder that AGNI_CA_DIR names at /etc/agni/ca (volumes %v)", service, svc.Volumes)
		}
	}
}
