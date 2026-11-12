//go:build darwin

package main

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
)

// qosOfThisThread reads the QoS class of the calling thread.
func qosOfThisThread(t *testing.T) uint32 {
	t.Helper()
	lib, err := purego.Dlopen(libSystem, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	var self func() uintptr
	var get func(thread uintptr, class unsafe.Pointer, rel unsafe.Pointer) int32
	purego.RegisterLibFunc(&self, lib, "pthread_self")
	purego.RegisterLibFunc(&get, lib, "pthread_get_qos_class_np")
	var class uint32
	var rel int32
	if rc := get(self(), unsafe.Pointer(&class), unsafe.Pointer(&rel)); rc != 0 {
		t.Fatalf("pthread_get_qos_class_np: %d", rc)
	}
	return class
}

// classDuringRequest starts a thread at class, runs one request through requestQoS on it, and
// returns the class the handler ran at.
func classDuringRequest(t *testing.T, class uint32) uint32 {
	t.Helper()
	lib, err := purego.Dlopen(libSystem, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	var set func(class uint32, rel int32) int32
	purego.RegisterLibFunc(&set, lib, "pthread_set_qos_class_self_np")
	done := make(chan uint32)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if rc := set(class, 0); rc != 0 {
			t.Errorf("pthread_set_qos_class_self_np(%#x): %d", class, rc)
		}
		var seen uint32
		h := requestQoS(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { seen = qosOfThisThread(t) }))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		done <- seen
	}()
	return <-done
}

// On Apple Silicon, macOS left about half of a fresh server's request threads below user-initiated,
// on an efficiency core, and the first board parse ran about ten times slower (agni issue 1534). A
// request through requestQoS runs at user-initiated at least, and a thread already above it keeps
// its class.
func TestRequestQoSRaisesALowRequestThread(t *testing.T) {
	const utility = 0x11
	if got := classDuringRequest(t, utility); got != qosUserInitiated {
		t.Errorf("a utility thread served the request at QoS class %#x, want user-initiated (%#x)", got, qosUserInitiated)
	}
}

func TestRequestQoSKeepsAHigherClass(t *testing.T) {
	const userInteractive = 0x21
	if got := classDuringRequest(t, userInteractive); got != userInteractive {
		t.Errorf("a user-interactive thread served the request at QoS class %#x, want it kept at %#x", got, userInteractive)
	}
}
