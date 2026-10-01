package main

import "github.com/panyam/agni/artifact"

// mustURI builds a mount-relative URI a test names directly, failing the test rather than the
// assertion when the literal is malformed.
func mustURI(mount, p string) artifact.URI {
	u, err := artifact.New(mount, p)
	if err != nil {
		panic(err)
	}
	return u
}
