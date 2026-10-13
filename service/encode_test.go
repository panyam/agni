package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// The tests here cover agni issue 956: Encode writes a table as the csv `--format csv` writes.

func TestEncodeGuardsAFormulaCell(t *testing.T) {
	var ts TableService
	resp, err := ts.Encode(context.Background(), &webapi.EncodeTablesRequest{
		Encoding: webapi.TableEncoding_TABLE_ENCODING_CSV,
		Stem:     "gateway",
		Tables: []*webapi.Table{{
			Name:    "findings",
			Columns: []*webapi.TableColumn{{Name: "subject"}, {Name: "rule"}},
			Rows:    []*webapi.TableRow{{Cells: []string{"=HYPERLINK(\"x\")", "r"}}, {Cells: []string{"U1", "r"}}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	f := resp.GetFiles()[0]
	if f.GetFilename() != "gateway.findings.csv" || !strings.HasPrefix(f.GetContentType(), "text/csv") {
		t.Errorf("file = %q (%s), want gateway.findings.csv as text/csv", f.GetFilename(), f.GetContentType())
	}
	lines := strings.Split(strings.TrimSpace(string(f.GetContent())), "\n")
	if len(lines) != 3 || lines[0] != "subject,rule" {
		t.Fatalf("csv = %q, want the header and two rows", f.GetContent())
	}
	if strings.HasPrefix(lines[1], "=") || strings.HasPrefix(lines[1], `"=`) {
		t.Errorf("a cell starting = was written as a formula: %q", lines[1])
	}
	if lines[2] != "U1,r" {
		t.Errorf("an ordinary row changed: %q", lines[2])
	}
}

func TestEncodeRefusesNoEncodingAndANamelessTable(t *testing.T) {
	var ts TableService
	tab := &webapi.Table{Name: "t", Columns: []*webapi.TableColumn{{Name: "a"}}}
	if _, err := ts.Encode(context.Background(), &webapi.EncodeTablesRequest{Tables: []*webapi.Table{tab}}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("no encoding: err = %v, want an invalid argument", err)
	}
	if _, err := ts.Encode(context.Background(), &webapi.EncodeTablesRequest{Tables: []*webapi.Table{{}}, Encoding: webapi.TableEncoding_TABLE_ENCODING_CSV}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a nameless table: err = %v, want an invalid argument", err)
	}
}
