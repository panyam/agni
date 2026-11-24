package authgate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/panyam/agni/internal/teamca"
)

// loginClient has no timeout of its own. The server holds each poll for up to devicePoll, and the
// caller's context ends the wait. It trusts the CA in AGNI_CA_FILE too (agni issue 1800).
func loginClient() (*http.Client, error) {
	tr, err := teamca.Transport()
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: tr}, nil
}

// AnyDevicePath and AnyDeviceTokenPath run the device flow of the server's only sign-in provider, so
// `agni login` needs no --with when the server offers one (agni issue 1811).
const (
	AnyDevicePath      = "/signin/device"
	AnyDeviceTokenPath = "/signin/device/token" // #nosec G101 -- a URL path, not a credential
)

// errNoSignIn says a server signs nobody in through a provider. postJSON returns it for a 404 that
// isn't about a handle.
var errNoSignIn = errors.New("no sign-in")

// DeviceLogin signs in to the agni server at server through a provider's device flow, which the
// server runs (agni issues 1772, 1811). with is "github", "oidc", or empty for the server's only
// provider. It calls show with the code the person types at the provider, then waits until the
// server answers with a token, the code expires, or ctx ends.
func DeviceLogin(ctx context.Context, server, with string, show func(DeviceStart)) (DeviceToken, error) {
	server = strings.TrimRight(server, "/")
	var start, poll string
	switch with {
	case "":
		start, poll = AnyDevicePath, AnyDeviceTokenPath
	case "github":
		start, poll = DevicePath, DeviceTokenPath
	case "oidc":
		start, poll = oidcDevice, oidcDeviceToken
	default:
		return DeviceToken{}, fmt.Errorf("--with %q: want github, oidc or local", with)
	}
	var st DeviceStart
	err := postJSON(ctx, server+start, nil, &st)
	var se *statusError
	if with == "" && errors.As(err, &se) && se.code == http.StatusUnauthorized {
		// A server from before agni issue 1811 has no AnyDevicePath and guards it like any other
		// path. It signs in with GitHub alone.
		start, poll = DevicePath, DeviceTokenPath
		err = postJSON(ctx, server+start, nil, &st)
	}
	if errors.Is(err, errNoSignIn) {
		if with == "" {
			return DeviceToken{}, fmt.Errorf("%s signs nobody in through a provider; its operator starts it with --github-client-id or --oidc-issuer, and a local account signs in with agni login --with local", server)
		}
		return DeviceToken{}, fmt.Errorf("%s doesn't sign people in with %s", server, with)
	}
	if err != nil {
		return DeviceToken{}, err
	}
	show(st)
	deadline := time.Now().Add(time.Duration(st.ExpiresIn) * time.Second)
	for {
		var got DeviceToken
		if err := postJSON(ctx, server+poll, map[string]string{"handle": st.Handle}, &got); err != nil {
			return DeviceToken{}, err
		}
		if !got.Pending {
			return got, nil
		}
		if st.ExpiresIn > 0 && time.Now().After(deadline) {
			return DeviceToken{}, fmt.Errorf("the code %s expired before it was entered; run agni login again", st.UserCode)
		}
	}
}

// statusError is a non-2xx answer, with the server's text.
type statusError struct {
	target string
	code   int
	text   string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("%s answered %d: %s", e.target, e.code, e.text)
}

// postJSON posts in, or no body when in is nil, and decodes a 2xx answer into out. Any other
// answer is an error carrying the server's text.
func postJSON(ctx context.Context, target string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client, err := loginClient()
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach %s: %w", target, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode == http.StatusNotFound && !bytes.Contains(raw, []byte("handle")) {
		return errNoSignIn
	}
	if resp.StatusCode/100 != 2 {
		return &statusError{target: target, code: resp.StatusCode, text: strings.TrimSpace(string(raw))}
	}
	return json.Unmarshal(raw, out)
}

// anyDevice runs the device route of the gate's only sign-in provider. With none it answers 404, so
// DeviceLogin says the server signs nobody in, and with two it answers 409 naming both.
func (g *Gate) anyDevice(w http.ResponseWriter, r *http.Request) {
	var names []string
	if g.github != nil {
		names = append(names, "github")
	}
	if g.oidc != nil {
		names = append(names, "oidc")
	}
	switch {
	case len(names) == 0:
		http.Error(w, "this server signs nobody in from a terminal", http.StatusNotFound)
	case len(names) > 1:
		http.Error(w, "this server signs people in with "+strings.Join(names, " and ")+"; run agni login --with github or --with oidc", http.StatusConflict)
	case g.github != nil && r.URL.Path == AnyDevicePath:
		g.github.deviceStart(w, r)
	case g.github != nil:
		g.github.deviceToken(w, r)
	case r.URL.Path == AnyDevicePath:
		g.oidc.deviceStart(w, r)
	default:
		g.oidc.deviceToken(w, r)
	}
}
