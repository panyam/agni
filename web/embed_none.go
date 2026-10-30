//go:build !embedweb

// Package web carries the viewer's built assets into a release binary (agni WS16-003). Only a build
// with the embedweb tag holds them; `go install` and `make agni` builds hold none, and
// internal/webbundle fetches the matching release's bundle for them instead.
package web

import "io/fs"

// Bundle returns the viewer's templates/ and static/ when the binary was built with the embedweb tag,
// and false otherwise.
func Bundle() (fs.FS, bool) { return nil, false }
