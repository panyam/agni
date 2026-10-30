//go:build embedweb

package web

import (
	"embed"
	"io/fs"
)

// The built bundle is gitignored, so this file only compiles after `pnpm build`, which is what makes
// a release build without one fail rather than ship a binary with no viewer. The wasm engine is left
// out: it is 37 MB, and a viewer an installed binary serves runs on the server engine.
//
//go:embed templates static/*.js static/*.css
var files embed.FS

// Bundle returns the viewer's templates/ and static/ when the binary was built with the embedweb tag.
func Bundle() (fs.FS, bool) { return files, true }
