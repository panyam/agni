package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/internal/artifact"
	"github.com/panyam/agni/internal/mounts"
)

// openCmd serves ONE design and prints the URL that shows it.
//
// It exists because the CLI and the viewer disagreed about where you are standing. Every other
// command assumes the working directory is where your netlist lives (`agni check board.edn`), and
// the viewer assumed the opposite: start a server, give it a mount, then navigate a browse tree to
// find the file you were already pointing at. `agni serve` from a design directory did not even
// start, because its assets are not there (agni issue 462).
//
// It composes the same server `serve` does, through runViewer, so the two cannot drift into
// answering differently about the same design.
func openCmd() *cobra.Command {
	var addr, webDir string
	c := &cobra.Command{
		Use:   "open <design>",
		Short: "Serve one design and print the URL that shows it",
		Long: "open starts the viewer for a single design and prints its URL, so a board can be\n" +
			"looked at from the directory it lives in. It binds loopback on a free port unless\n" +
			"--addr says otherwise, and it serves ONLY the design named (and its project, where it\n" +
			"has one). Ctrl-C stops it.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve the design the way every other command does, so a declared COMPANION opens as its
			// design's entry rather than as itself. The URL is built from what was READ, not from what
			// was typed, or the page would show a different design from the one named.
			ws, err := workspace()
			if err != nil {
				return err
			}
			src, err := newDesignResolver(ws).Resolve(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			u, err := artifact.Parse(src.NetlistURI)
			if err != nil {
				return err
			}
			m, ok := mounts.Find(ws.Mounts(), u.Mount)
			if !ok {
				return fmt.Errorf("%s resolved to mount %q, which this run has no root for", args[0], u.Mount)
			}
			if err := refuseTooBroadRoot(m.Root); err != nil {
				return err
			}
			if addr == "" {
				port, err := freePort()
				if err != nil {
					return err
				}
				addr = net.JoinHostPort("127.0.0.1", port)
			}

			view := "/designs/" + u.Mount + "/" + u.Path + "/view"
			return runViewer(cmd, viewerOpts{
				addr:        addr,
				webDir:      webDir,
				extraMounts: []mounts.Mount{m},
				banner: func(urls []string, _ int) {
					openBanner(cmd.ErrOrStderr(), urls[0], m, src.NetlistURI, view)
				},
			})
		},
	}
	c.Flags().StringVar(&addr, "addr", "", "address to listen on; empty picks a free port on loopback, which is what keeps two open runs from colliding and keeps a design directory off the network")
	c.Flags().StringVar(&webDir, "web-dir", "", "directory holding the viewer's own assets; see `agni serve --help`. Defaults to "+defaultWebDir+", then web_dir in the nearest agni.yaml, then "+envWebDir)
	return c
}

// refuseTooBroadRoot rejects serving a mount rooted at the home directory or a filesystem root.
//
// `open` mints a mount for the design it was given, rooted at the enclosing project or at the file's
// own directory (cliWorkspace.mint), so the root is bounded for every ordinary argument. `agni open .`
// in a directory that is neither is the exception, and from $HOME it would put a home directory on an
// HTTP listener.
//
// It refuses rather than warning, and offers no override. A warning is read after the fact, and a
// --force flag is the thing someone pastes from a forum without reading the sentence above it. Naming
// a design, or a narrower directory, is not a hardship.
func refuseTooBroadRoot(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil // not resolvable is not this check's business; the reader will fail on its own terms
	}
	abs = filepath.Clean(abs)
	if abs == filepath.Dir(abs) {
		return fmt.Errorf("refusing to serve %s: that is a filesystem root. Name a design, or a directory that holds one", abs)
	}
	if home, err := os.UserHomeDir(); err == nil && filepath.Clean(home) == abs {
		return fmt.Errorf("refusing to serve %s: that is your home directory, and serving it would put all of it on an HTTP listener. Name a design, or a directory that holds one", abs)
	}
	return nil
}

// freePort asks the kernel for a port nobody is using.
//
// Binding :0 and reading back the assignment races with anything else doing the same, and the race is
// worth taking here: the alternative is a fixed default, and 8080 is exactly the port a developer
// already has a server on. A collision at this scale is a retry, where a fixed port is a daily
// annoyance.
func freePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("finding a free port: %w", err)
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(port), nil
}

// openBanner prints what `open` exists to give you: the design's own URL, and a check command that
// works when pasted.
//
// serveURLs ends its addresses with a slash, and both the view path and --url-base want a bare
// origin. A doubled slash is cosmetic in a browser and is NOT cosmetic in --url-base, which
// concatenates rather than joins.
//
// The check line carries --mount for a reason that is easy to miss. Mounts are minted PER PROCESS, so
// the mount this server invented means nothing to a second agni, which refuses the URI outright and
// says to pass --mount. Printing it is what makes the line paste-able rather than illustrative.
func openBanner(w io.Writer, rawURL string, m mounts.Mount, designURI, viewPath string) {
	base := strings.TrimSuffix(rawURL, "/")
	fmt.Fprintf(w, "%s%s\n", base, viewPath)
	fmt.Fprintf(w, "\ncheck it against this server:\n  agni check --mount %s=%s %s --verdicts --url-base %s\n",
		m.Name, m.Root, designURI, base)
	fmt.Fprintf(w, "\nCtrl-C to stop.\n")
}
