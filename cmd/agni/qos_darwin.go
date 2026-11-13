//go:build darwin

package main

import (
	"net/http"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	libSystem        = "/usr/lib/libSystem.B.dylib"
	qosUserInitiated = 0x19
)

var qosCalls = sync.OnceValue(func() (calls struct {
	self func() uintptr
	get  func(thread uintptr, class unsafe.Pointer, rel unsafe.Pointer) int32
	set  func(class uint32, rel int32) int32
}) {
	lib, err := purego.Dlopen(libSystem, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return calls
	}
	purego.RegisterLibFunc(&calls.self, lib, "pthread_self")
	purego.RegisterLibFunc(&calls.get, lib, "pthread_get_qos_class_np")
	purego.RegisterLibFunc(&calls.set, lib, "pthread_set_qos_class_self_np")
	return calls
})

// requestQoS raises the thread that starts serving each request to the user-initiated QoS class, when
// it sits below that (agni issue 1534). On Apple Silicon, macOS put about half of a fresh `agni
// serve`'s request threads on an efficiency core, and the first board parse then ran about ten times
// slower. A raised thread keeps its class for later requests, so the pool rises as it serves. It
// never lowers a thread, and it passes the request through unchanged when libSystem can't be loaded.
// The calls go through purego, since the release binaries are built without cgo.
func requestQoS(h http.Handler) http.Handler {
	c := qosCalls()
	if c.set == nil {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var class uint32
		var rel int32
		if c.get(c.self(), unsafe.Pointer(&class), unsafe.Pointer(&rel)) != 0 || class < qosUserInitiated {
			c.set(qosUserInitiated, 0)
		}
		h.ServeHTTP(w, r)
	})
}
