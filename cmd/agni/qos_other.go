//go:build !darwin

package main

import "net/http"

// requestQoS passes requests through unchanged. Only macOS needs a request's thread raised to run on
// a performance core (agni issue 1534).
func requestQoS(h http.Handler) http.Handler { return h }
