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

// serverSpec is a parsed --server value: where the links this run mints should point.
//
// Three states rather than two. Empty is the default and mints no links at all, which is what a
// pipeline wants. A URL names a server someone else is running, so the run has to ask that server
// whether it serves the same mounts from the same roots (verifyServerMount) before it trusts a link.
// self is this process, where the question cannot arise: one mount table minted once, used to read
// the design and to serve it.
type serverSpec struct {
	self bool
	// ln is self's listener, BOUND at parse time. Binding early rather than probing is what makes
	// "fails on a taken port" exact: the port is held from the moment the flag is read, so nothing can
	// take it in between, and `self` learns its own port from ln.Addr() instead of guessing a free one
	// and hoping. servicekit serves on it through WithListener.
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
// `self:PORT` binds that port and FAILS if it is in use rather than falling back to a free one: an
// explicit port is an assertion about where the links will point, and quietly binding elsewhere would
// mint links that resolve on whatever else is listening there.
//
// Both hold the listener rather than probing and closing it, so the port cannot be taken between the
// check and the serve. That is why the caller must Close a spec it does not go on to serve.
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
// --url-base, which this replaced, is GONE rather than kept as a hidden alias. It survived one
// release that way and the cost showed up immediately: five docsite pages and `agni open`'s own
// printed command went on teaching it, and nothing failed, because a working alias makes stale
// instructions indistinguishable from current ones (agni issue 636). A removed flag errors, which is
// the feedback a rename is supposed to give.
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
// The asset check belongs HERE rather than in serveSelf, for the reason the port check is already at
// parse time. self mints links into an artifact and then serves them, so both of its preconditions
// have to hold before the wrapped command writes anything; checking the assets afterwards left a
// report full of links to a server that never bound (agni issue 637). Binding the port early and
// stat-ing the assets late meant one precondition was exact and the other was a hope.
//
// A failed check CLOSES the listener parseServerSpec bound, per that function's contract that a
// caller must close a spec it does not go on to serve. Without it every failed run would leak the
// port it had just reserved, so the second attempt would fail for a different and more confusing
// reason than the first.
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
// It reads the workspace after the command body has run, which is the whole point: the CLI mints a
// mount lazily while resolving its argument, so a table read any earlier would be missing the design
// the links name. That is the disagreement --url-base could only ever paper over, and serving the
// same table removes it rather than checking for it.
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
// Wrapping rather than a PostRunE, for one reason worth stating: PostRunE does not run when RunE
// returns an error, and `check --fail-on error` returns one whenever it finds something. That is
// exactly when the operator wants the viewer up, so the server starts whether the run passed or not,
// and the command's own exit condition is reported before it blocks.
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
