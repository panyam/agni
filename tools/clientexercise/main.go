// Command clientexercise is mission #1625's exercise, `make exercise-clients` (agni issue 1628). It
// starts one `agni serve` as a shared server would run: with a store, --auth tokens, and TLS from a
// certificate a test CA signed. It makes a token with `agni token create`, and runs each language's
// client example against it with the token in AGNI_TOKEN and the CA's file in AGNI_CA (agni issues
// 1627 and 1765). Each example prints one line of JSON summing up its answers,
// and each line must match the CLI's answers on the same design. It prints a step table, one row per
// language, and writes it to -out. A language whose example does not exist yet waits on its ticket.
//
// Each example is clients/examples/<language>/run, taking -server and -design.
// docsite/content/build/client-contract.md says what the line holds.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// language is one client the mission builds, and the ticket that builds it.
type language struct {
	name, ticket string
}

var languages = []language{
	{"go", "#1628"},
	{"ts", "#1630"},
	{"rust", "#1631"},
	{"java", "#1632"},
	{"cpp", "#1638"},
}

// summary is the line each example prints.
type summary struct {
	Version    string   `json:"version"`
	API        string   `json:"api"`
	Features   []string `json:"features"`
	Components int      `json:"components"`
	Nets       int      `json:"nets"`
	Sheets     int      `json:"sheets"`
	Findings   int      `json:"findings"`
	QueryRows  int      `json:"query_rows"`
	// The board's layers, placed parts and routed track segments, from GetBoardGeometry's pages
	// (agni issue 1634).
	BoardLayers int `json:"board_layers"`
	BoardParts  int `json:"board_parts"`
	BoardTracks int `json:"board_tracks"`
}

const resistorsOf = `component.class(?c, "resistor") => ?c`

func main() {
	agni := flag.String("agni", "bin/agni", "the agni binary to serve and to ask for the CLI's answers")
	mount := flag.String("mount", "probe=examples/cross-probe-project", "the mount the server and the CLI read, name=path")
	design := flag.String("design", "mount://probe/boards/jetson-cut", "the design every example ingests")
	out := flag.String("out", "", "a directory to write clients.md into")
	flag.Parse()
	rows, err := exercise(*agni, *mount, *design)
	if err != nil {
		fmt.Fprintln(os.Stderr, "clientexercise:", err)
		os.Exit(1)
	}
	report := "## Clients exercise, " + *design + "\n\n" + table(rows) + "\n"
	fmt.Print(report)
	if *out != "" {
		if err := os.MkdirAll(*out, 0o750); err == nil {
			_ = os.WriteFile(filepath.Join(*out, "clients.md"), []byte(report), 0o600)
		}
	}
	for _, r := range rows {
		if r.status == "fail" {
			os.Exit(1)
		}
	}
}

type row struct{ step, status, detail string }

func table(rows []row) string {
	var b strings.Builder
	b.WriteString("| Step | Status | Detail |\n| --- | --- | --- |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", r.step, r.status, strings.ReplaceAll(r.detail, "|", "\\|"))
	}
	return b.String()
}

func exercise(agni, mount, design string) ([]row, error) {
	name, dir, ok := strings.Cut(mount, "=")
	if !ok {
		return nil, fmt.Errorf("-mount %q is not name=path", mount)
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	mount = name + "=" + root
	if agni, err = filepath.Abs(agni); err != nil {
		return nil, err
	}
	want, err := cliAnswers(agni, mount, design)
	if err != nil {
		return nil, err
	}
	server, token, ca, stop, err := serve(agni, mount)
	if err != nil {
		return nil, err
	}
	defer stop()
	rows := []row{{"serve", "ok", fmt.Sprintf("%s with --store, --auth tokens and TLS; the CLI reads %d components, %d nets, %d sheets, %d findings, %d query rows, and a board of %d layers, %d parts and %d tracks", server, want.Components, want.Nets, want.Sheets, want.Findings, want.QueryRows, want.BoardLayers, want.BoardParts, want.BoardTracks)}}
	rows = append(rows, refusedWithoutToken(server, ca))
	for _, l := range languages {
		run := filepath.Join("clients", "examples", l.name, "run")
		if _, err := os.Stat(run); err != nil {
			rows = append(rows, row{l.name, "not available", "waits on " + l.ticket + ": no " + run})
			continue
		}
		got, err := runExample(run, server, design, token, ca)
		if err != nil {
			rows = append(rows, row{l.name, "fail", err.Error()})
			continue
		}
		if diff := compare(got, want); diff != "" {
			rows = append(rows, row{l.name, "fail", diff})
			continue
		}
		rows = append(rows, row{l.name, "ok", fmt.Sprintf("%s, features %s, and the CLI's answers", got.Version, strings.Join(got.Features, " "))})
	}
	return rows, nil
}

// compare says how an example's line differs from what it should hold, or "".
func compare(got, want *summary) string {
	var d []string
	if got.API != "agni.v1" {
		d = append(d, fmt.Sprintf("api %q, want agni.v1", got.API))
	}
	if got.Version != want.Version {
		d = append(d, fmt.Sprintf("version %q, want the server's %q", got.Version, want.Version))
	}
	for _, f := range []string{"grpc", "snapshot_recovery", "snapshot_store", "auth"} {
		if !slices.Contains(got.Features, f) {
			d = append(d, "no feature "+f)
		}
	}
	for _, c := range []struct {
		what      string
		got, want int
	}{{"components", got.Components, want.Components}, {"nets", got.Nets, want.Nets}, {"sheets", got.Sheets, want.Sheets}, {"findings", got.Findings, want.Findings}, {"query rows", got.QueryRows, want.QueryRows},
		{"board layers", got.BoardLayers, want.BoardLayers}, {"board parts", got.BoardParts, want.BoardParts}, {"board tracks", got.BoardTracks, want.BoardTracks}} {
		if c.got != c.want {
			d = append(d, fmt.Sprintf("%d %s, the CLI reads %d", c.got, c.what, c.want))
		}
	}
	return strings.Join(d, "; ")
}

// cliAnswers is what the CLI answers on the design, the reference every example is held to.
func cliAnswers(agni, mount, design string) (*summary, error) {
	json1 := func(args ...string) (map[string]any, error) {
		cmd := exec.Command(agni, append(args, "--mount", mount, "--format", "json")...) // #nosec G204 -- the agni binary this exercise was pointed at
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("agni %s: %w", strings.Join(args, " "), err)
		}
		var m map[string]any
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			return nil, fmt.Errorf("agni %s: %w", strings.Join(args, " "), err)
		}
		return m, nil
	}
	count := func(m map[string]any, key string) int {
		l, _ := m[key].([]any)
		return len(l)
	}
	number := func(m map[string]any, key string) int {
		n, _ := m[key].(float64)
		return int(n)
	}
	stats, err := json1("stats", design)
	if err != nil {
		return nil, err
	}
	check, err := json1("check", design)
	if err != nil {
		return nil, err
	}
	query, err := json1("query", design, resistorsOf)
	if err != nil {
		return nil, err
	}
	board, err := json1("board", design)
	if err != nil {
		return nil, err
	}
	tracks := 0
	for _, n := range board["nets"].([]any) {
		segs, _ := n.(map[string]any)["segments"].([]any)
		tracks += len(segs)
	}
	v, err := exec.Command(agni, "version").Output() // #nosec G204 -- as above
	if err != nil {
		return nil, err
	}
	version := strings.TrimPrefix(strings.SplitN(string(v), "\n", 2)[0], "agni ")
	return &summary{Version: version, Components: number(stats, "componentCount"), Nets: number(stats, "netCount"), Sheets: count(stats, "sheets"), Findings: count(check, "findings"), QueryRows: count(query, "rows"),
		BoardLayers: count(board, "layers"), BoardParts: count(board, "placements"), BoardTracks: tracks}, nil
}

// serve starts agni serve on a free port with a fresh store, --auth tokens and TLS, from an empty
// working directory so it serves the API alone. It returns the server's URL, a token `agni token
// create` made for it, the file of the CA that signed its certificate, and a function that stops it.
func serve(agni, mount string) (string, string, string, func(), error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", "", nil, err
	}
	addr := l.Addr().String()
	l.Close()
	work, err := os.MkdirTemp("", "agni-clients-")
	if err != nil {
		return "", "", "", nil, err
	}
	ca, cert, key, err := testCertificates(work)
	if err != nil {
		_ = os.RemoveAll(work)
		return "", "", "", nil, err
	}
	store := "file:///" + strings.TrimPrefix(filepath.ToSlash(filepath.Join(work, "store")), "/")
	made, err := exec.Command(agni, "token", "create", "clients-exercise", "--store", store).Output() // #nosec G204 -- as above
	if err != nil {
		_ = os.RemoveAll(work)
		return "", "", "", nil, fmt.Errorf("agni token create: %w", err)
	}
	token := strings.TrimSpace(string(made))
	cmd := exec.Command(agni, "serve", "--addr", addr, "--mount", mount, "--store", store, "--auth", "tokens", "--tls-cert", cert, "--tls-key", key) // #nosec G204 -- as above
	cmd.Dir = work
	var log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Start(); err != nil {
		return "", "", "", nil, err
	}
	stop := func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = os.RemoveAll(work)
	}
	url := "https://" + addr
	client, err := trusting(ca)
	if err != nil {
		stop()
		return "", "", "", nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for {
		if r, err := client.Get(url + "/healthz"); err == nil {
			r.Body.Close()
			return url, token, ca, stop, nil
		}
		select {
		case <-ctx.Done():
			stop()
			return "", "", "", nil, fmt.Errorf("agni serve did not answer at %s: %s", url, log.String())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// refusedWithoutToken is the step that asks the server for its info with no token, as Connect's JSON,
// and passes when the server refuses it as unauthenticated.
func refusedWithoutToken(server, ca string) row {
	client, err := trusting(ca)
	if err != nil {
		return row{"no token", "fail", err.Error()}
	}
	r, err := client.Post(server+"/agni.v1.webapi.ServerService/GetServerInfo", "application/json", strings.NewReader("{}"))
	if err != nil {
		return row{"no token", "fail", err.Error()}
	}
	defer r.Body.Close()
	var body struct{ Code, Message string }
	_ = json.NewDecoder(r.Body).Decode(&body)
	if r.StatusCode != http.StatusUnauthorized || body.Code != "unauthenticated" {
		return row{"no token", "fail", fmt.Sprintf("a call with no token answered %d %q, want 401 unauthenticated", r.StatusCode, body.Code)}
	}
	return row{"no token", "ok", "refused as unauthenticated: " + body.Message}
}

// runExample runs one language's example with token in AGNI_TOKEN and the CA's file in AGNI_CA, and
// reads the last line it printed.
func runExample(run, server, design, token, ca string) (*summary, error) {
	cmd := exec.Command("./"+run, "-server", server, "-design", design) // #nosec G204 -- an example committed in this repo
	cmd.Env = append(os.Environ(), "AGNI_TOKEN="+token, "AGNI_CA="+ca)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %v: %s", run, err, strings.TrimSpace(errOut.String()))
	}
	var last string
	sc := bufio.NewScanner(&out)
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" {
			last = t
		}
	}
	s := &summary{}
	if err := json.Unmarshal([]byte(last), s); err != nil {
		return nil, fmt.Errorf("%s printed %q, not a summary line: %v", run, last, err)
	}
	return s, nil
}

// testCertificates writes a test CA and a server certificate it signed for 127.0.0.1 and localhost
// into dir, and returns the three files' paths.
func testCertificates(dir string) (ca, cert, key string, err error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", "", err
	}
	now := time.Now()
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "agni clients exercise CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return "", "", "", err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return "", "", "", err
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", "", err
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return "", "", "", err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		return "", "", "", err
	}
	ca, cert, key = filepath.Join(dir, "ca.pem"), filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	for path, block := range map[string]*pem.Block{
		ca:   {Type: "CERTIFICATE", Bytes: caDER},
		cert: {Type: "CERTIFICATE", Bytes: leafDER},
		key:  {Type: "PRIVATE KEY", Bytes: keyDER},
	} {
		if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
			return "", "", "", err
		}
	}
	return ca, cert, key, nil
}

// trusting is an HTTP client that trusts the CA in the file ca.
func trusting(ca string) (*http.Client, error) {
	pemBytes, err := os.ReadFile(ca) // #nosec G304 -- the CA this exercise wrote
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("%s holds no certificate", ca)
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}, nil
}
