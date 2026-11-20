package authgate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// loginClient has no timeout of its own. The server holds each poll for up to devicePoll, and the
// caller's context ends the wait.
var loginClient = &http.Client{}

// DeviceLogin signs in to the agni server at server through GitHub's device flow, which the server
// runs (agni issue 1772). It calls show with the code the person types at GitHub, then waits until
// the server answers with a token, the code expires, or ctx ends.
func DeviceLogin(ctx context.Context, server string, show func(DeviceStart)) (DeviceToken, error) {
	server = strings.TrimRight(server, "/")
	var start DeviceStart
	if err := postJSON(ctx, server+DevicePath, nil, &start); err != nil {
		return DeviceToken{}, err
	}
	show(start)
	deadline := time.Now().Add(time.Duration(start.ExpiresIn) * time.Second)
	for {
		var got DeviceToken
		if err := postJSON(ctx, server+DeviceTokenPath, map[string]string{"handle": start.Handle}, &got); err != nil {
			return DeviceToken{}, err
		}
		if !got.Pending {
			return got, nil
		}
		if start.ExpiresIn > 0 && time.Now().After(deadline) {
			return DeviceToken{}, fmt.Errorf("the code %s expired before it was entered; run agni login again", start.UserCode)
		}
	}
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
	resp, err := loginClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach %s: %w", target, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode == http.StatusNotFound && !bytes.Contains(raw, []byte("handle")) {
		return fmt.Errorf("%s doesn't sign people in with GitHub; its operator starts it with --github-client-id", target)
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s answered %d: %s", target, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return json.Unmarshal(raw, out)
}
