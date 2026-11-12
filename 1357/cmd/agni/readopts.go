package main

import (
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/service"
)

// readerFor is service.LoaderFor, named for the CLI's call sites.
func readerFor(base *formats.Loader, opts ...service.ReadOption) *formats.Loader {
	return service.LoaderFor(base, opts...)
}
