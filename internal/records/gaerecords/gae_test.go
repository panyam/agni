package gaerecords_test

import (
	"os"
	"testing"

	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/internal/records/recordstest"
)

// The contract on the Datastore emulator, when DATASTORE_EMULATOR_HOST names one (make test-stores
// starts it).
func TestGAEContract(t *testing.T) {
	if os.Getenv("DATASTORE_EMULATOR_HOST") == "" {
		t.Skip("DATASTORE_EMULATOR_HOST names no emulator; make test-stores runs this")
	}
	t.Setenv(recordstest.BackendEnv, "datastore")
	recordstest.Run(t, func(t *testing.T) records.Store { return recordstest.New(t) })
}
