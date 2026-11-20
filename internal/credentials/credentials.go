// Package credentials keeps the token `agni login` gets from each server, by the server's URL, in
// the user's config folder (agni issue 1772). The file holds one line per server, its URL, a space
// and the token, and is readable by its owner alone.
package credentials

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// File is where the tokens are kept. AGNI_CONFIG_DIR moves it, for a test.
func File() (string, error) {
	dir := os.Getenv("AGNI_CONFIG_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "agni")
	}
	return filepath.Join(dir, "credentials"), nil
}

func read() (map[string]string, error) {
	p, err := File()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p) // #nosec G304 -- the path is the user's own config folder
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	creds := map[string]string{}
	for i, line := range strings.Split(string(b), "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		server, token, ok := strings.Cut(line, " ")
		if !ok {
			return nil, fmt.Errorf("%s:%d: want a server URL, a space and a token", p, i+1)
		}
		creds[server] = strings.TrimSpace(token)
	}
	return creds, nil
}

// Token is the token kept for server, or "" for none or an unreadable file.
func Token(server string) string {
	creds, err := read()
	if err != nil {
		return ""
	}
	return creds[strings.TrimRight(server, "/")]
}

// Save keeps token for server, and returns the file it is in.
func Save(server, token string) (string, error) {
	creds, err := read()
	if err != nil {
		return "", err
	}
	creds[strings.TrimRight(server, "/")] = token
	p, err := File()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, server := range slices.Sorted(maps.Keys(creds)) {
		fmt.Fprintf(&b, "%s %s\n", server, creds[server])
	}
	return p, os.WriteFile(p, []byte(b.String()), 0o600)
}
