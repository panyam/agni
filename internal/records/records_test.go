package records_test

import (
	"testing"

	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/internal/records/recordstest"
)

func TestDirContract(t *testing.T) {
	recordstest.Run(t, func(t *testing.T) records.Store {
		d, err := records.NewDir(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return d
	})
}
