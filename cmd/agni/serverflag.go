package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// selfServer is the --server value that means "this process". Everything else is an address.
const selfServer = "self"

// serverSpec is a parsed --server value, saying where the links this run mints should point.
//
// Three states. Empty is the default and mints no links, which is what a pipeline wants. A URL names a
// server someone else is running, so the run asks it whether it serves the same mounts from the same
// roots (verifyServerMount) before trusting a link. self is this process, which reads the design and
// serves it from one mount table, so that question cannot arise.
type serverSpec struct {
	self bool
	// ln is self's listener, BOUND at parse time so nothing can take the port between the flag being
	// read and the serve, and `self` learns its own port from ln.Addr(). servicekit serves on it
	// through WithListener.
	ln net.Listener
	// url is a remote server's base address.
	url string
}

// active reports whether this run mints links at all.
func (s serverSpec) active() bool { return s.self || s.url != "" }

// base is the address links are built from.
func (s serverSpec) base() string {
	if s.self {
		return "http://" + s.ln.Addr().String()
	}
	return s.url
}

// parseServerSpec reads a --server value.
//
// `self` binds port 0 and reads back what it got, because two runs in one directory must not collide.
// `self:PORT` binds that port and FAILS if it is in use rather than falling back to a free one, since
// binding elsewhere would mint links that resolve on whatever else is listening on the named port.
//
// Both hold the listener, so the caller must Close a spec it does not go on to serve.
func parseServerSpec(v string) (serverSpec, error) {
	v = strings.TrimSpace(v)
	switch {
	case v == "":
		return serverSpec{}, nil
	case v == selfServer:
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return serverSpec{}, fmt.Errorf("--server self: %w", err)
		}
		return serverSpec{self: true, ln: ln}, nil
	case strings.HasPrefix(v, selfServer+":"):
		port := strings.TrimPrefix(v, selfServer+":")
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return serverSpec{}, fmt.Errorf("--server %q: %q is not a port", v, port)
		}
		ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
		if err != nil {
			return serverSpec{}, fmt.Errorf("--server %s: port %s is in use; pass `self` for a free one", v, port)
		}
		return serverSpec{self: true, ln: ln}, nil
	default:
		u, err := url.Parse(v)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return serverSpec{}, fmt.Errorf("--server %q: want `self`, `self:PORT`, or a URL like http://localhost:8080", v)
		}
		return serverSpec{url: strings.TrimRight(v, "/")}, nil
	}
}

// serverFlag registers --server on a command that mints viewer links.
//
// --url-base, which this replaced, is GONE rather than kept as a hidden alias, because a working alias
// let stale docs keep teaching it with nothing failing (agni issue 636). Do not reintroduce one.
func serverFlag(cmd *cobra.Command, server *string) {
	cmd.Flags().StringVar(server, "server", "",
		"where the links this run mints should point. Empty (the default) mints none, which is what a "+
			"pipeline wants. `self` starts a viewer on a free port, serves this run's own mount table, "+
			"and blocks until Ctrl-C, so the links cannot disagree with what was read. `self:PORT` does "+
			"the same on that port and fails if it is taken. A URL names a server someone else is "+
			"running, which is asked whether it serves the same mounts from the same roots")
}

// resolveServer turns the flag into a spec, and refuses a `self` spec this process could not serve.
//
// The asset check belongs HERE rather than in serveSelf, like the port check. self mints links into an
// artifact and then serves them, so both preconditions must hold before the wrapped command writes
// anything, or the report fills with links to a server that never bound (agni issue 637).
//
// A failed check CLOSES the listener parseServerSpec bound, per that function's contract, so a failed
// run does not leak the port it reserved.
func resolveServer(server string) (serverSpec, error) {
	spec, err := parseServerSpec(server)
	if err != nil || !spec.self {
		return spec, err
	}
	// Commands that mint links carry no --web-dir of their own, so the flag is empty here and the
	// value comes from the agni.yaml/environment chain resolveWebDir applies.
	if _, _, err := resolveWebAssets("", os.Getenv); err != nil {
		spec.Close()
		return serverSpec{}, fmt.Errorf("--server %s cannot serve: %w", server, err)
	}
	return spec, nil
}

// Close releases the listener a `self` spec is holding. Safe on a spec that holds none, so a caller
// can close unconditionally on any path that does not go on to serve.
func (s serverSpec) Close() error {
	if s.ln == nil {
		return nil
	}
	return s.ln.Close()
}

// serveSelf runs the viewer over the mount table THIS RUN built, then blocks.
//
// It reads the workspace after the command body has run, because the CLI mints a mount lazily while
// resolving its argument, so a table read any earlier would be missing the design the links name.
func serveSelf(cmd *cobra.Command, spec serverSpec) error {
	ws, err := workspace()
	if err != nil {
		return err
	}
	table := ws.Mounts()
	if len(table) == 0 {
		return fmt.Errorf("--server self: this run resolved no mount, so there is nothing to serve")
	}
	for _, m := range table {
		if err := refuseTooBroadRoot(m.Root); err != nil {
			return err
		}
	}
	return runViewer(cmd, viewerOpts{
		addr:        spec.ln.Addr().String(),
		listener:    spec.ln,
		extraMounts: table,
		banner: func(urls []string, n int) {
			fmt.Fprintf(cmd.ErrOrStderr(), "serving %d mount(s) at %s so the links above resolve (Ctrl-C to stop)\n", n, urls[0])
		},
	})
}

// withSelfServer wraps a command so `--server self` serves after the command's own work is done.
//
// Wrapping rather than a PostRunE, because PostRunE does not run when RunE returns an error, and
// `check --fail-on error` returns one whenever it finds something. The server starts whether the run
// passed or not, and the command's own error is reported before it blocks.
func withSelfServer(cmd *cobra.Command, spec *serverSpec) {
	inner := cmd.RunE
	cmd.RunE = func(c *cobra.Command, args []string) error {
		err := inner(c, args)
		if !spec.self {
			return err
		}
		if err != nil {
			fmt.Fprintf(c.ErrOrStderr(), "note: the run reported %v; serving anyway so the links can be followed\n", err)
		}
		return serveSelf(c, *spec)
	}
}
