// Package web contains the embedded HTML templates and static assets
// served by the web UI. Embedding them into the binary means packages
// (rpm/deb), containers and plain binaries never depend on the process
// working directory or on the assets being installed on disk.
package web

import (
	"embed"
	"io/fs"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// Templates returns the embedded templates, rooted at the repository's web/
// directory (i.e. patterns must be prefixed with "templates/").
func Templates() fs.FS {
	return templatesFS
}

// Static returns the embedded static assets, rooted at web/static.
func Static() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// Only possible on a malformed embed pattern, which is a compile-time constant.
		panic(err)
	}
	return sub
}
