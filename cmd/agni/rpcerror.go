package main

import (
	"errors"
	"io"
	"net/http/httptest"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/panyam/agni/internal/server"
)

// rpcError marks an error a service returned for the command's request, as distinct from a flag
// mistake before any request or a status the command reports after writing its answer (a query
// set's "2 of 5 could not be answered"). Only a marked error is written as JSON, so a failure
// after the answer was written never puts a second document on stdout.
type rpcError struct{ err error }

func (e *rpcError) Error() string { return e.err.Error() }

func (e *rpcError) Unwrap() error { return e.err }

// rpcFailed marks err as the failure of the command's request, and is nil for nil. Every service
// call in a command whose --format json writes a wire message returns its error through it.
func rpcFailed(err error) error {
	if err == nil {
		return nil
	}
	return &rpcError{err: err}
}

// reportError prints a failed command's error on stderr, as every command always has. When the
// command was asked for --format json and its request failed, it also writes the failure as the
// Connect protocol encodes it, `{"code": ..., "message": ..., "details": [...]}`, where the answer
// would have gone (stdout, or -o's file). C31 makes a command's json the wire message its rpc
// returns, and a failed rpc's wire message is this error, so a script reading the CLI and a client
// reading the API parse one shape for a failure as for an answer. A query that would not parse
// carries its position as a TextPosition detail, whose `debug` field spells it as JSON.
func reportError(cmd *cobra.Command, err error, stderr io.Writer) {
	_, _ = io.WriteString(stderr, "error: "+err.Error()+"\n")
	var re *rpcError
	if cmd == nil || !errors.As(err, &re) || flagValue(cmd, "format") != "json" {
		return
	}
	body, werr := wireError(re.err)
	if werr != nil {
		return
	}
	switch out := flagValue(cmd, "out"); out {
	case "", "-":
		_, _ = cmd.OutOrStdout().Write(body)
	default:
		_ = os.WriteFile(out, body, 0o644) // #nosec G306 -- a report the user asked for, meant to be read by others
	}
}

// wireError encodes err exactly as a Connect server writes it, through connect's own ErrorWriter,
// so the CLI cannot drift from what the server sends. An error a --remote server returned is
// already a Connect error and is written as it arrived.
func wireError(err error) ([]byte, error) {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		ce = server.ConnectError(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Content-Type", "application/json")
	if err := connect.NewErrorWriter().Write(rec, req, ce); err != nil {
		return nil, err
	}
	return append(rec.Body.Bytes(), '\n'), nil
}

// flagValue is a flag's value on cmd, or "" when cmd has no such flag.
func flagValue(cmd *cobra.Command, name string) string {
	if f := cmd.Flags().Lookup(name); f != nil {
		return f.Value.String()
	}
	return ""
}
