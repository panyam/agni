package projects

import (
	"context"
	"fmt"
	"github.com/panyam/agni/artifact"
	"os"
	"path/filepath"
	"testing"
)

// benchDir builds a mount holding `projects` projects, each with `designs` designs and a conventions
// file, plus `noise` unrelated directories at various depths.
//
// The noise matters most. An operator can hand the server any folder as a mount, not a curated
// tree. The walk's cost then depends on what ELSE is in there, such as a vendored library, a build
// output directory or somebody's home folder. The number of projects matters less.
//
// It writes to a REAL directory, since the server runs on a real filesystem. fstest.MapFS actively
// misleads here. Its ReadDir scans the entire map, so a walk over it costs O(all files) per
// directory, and the fake dominates any measurement rather than the code under test.
func benchDir(b *testing.B, projects, designs, noise int) Tree {
	b.Helper()
	root := b.TempDir()
	write := func(rel, body string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	for p := range projects {
		base := fmt.Sprintf("p%02d", p)
		write(base+"/project.yaml", fmt.Sprintf("name: p%02d\ntitle: Project %02d\nconventions:\n  name: p%02d\n  lexicon:\n    net:\n      rail:\n        patterns: [\"^R%d\"]\n", p, p, p, p))
		for d := range designs {
			dd := fmt.Sprintf("%s/designs/d%02d", base, d)
			write(dd+"/design.yaml", fmt.Sprintf("name: d%02d\nextends: ../../project.yaml\nentry: board.edn\n", d))
			write(dd+"/board.edn", "x")
		}
	}
	for n := range noise {
		write(fmt.Sprintf("vendor/lib%03d/src/deep/file.txt", n), "x")
		write(fmt.Sprintf("build/out%03d/artifact.bin", n), "x")
	}
	return Tree{Mount: "m", FS: os.DirFS(root)}
}

// BenchmarkProjectsWalk measures the cost behind this ticket. Every ListProjects does a bounded
// directory walk and parses every descriptor it finds.
func BenchmarkProjectsWalk(b *testing.B) {
	for _, c := range []struct {
		name                     string
		projects, designs, noise int
	}{
		{"small", 2, 1, 0},
		{"typical", 5, 3, 20},
		{"large", 20, 10, 200},
	} {
		b.Run(c.name, func(b *testing.B) {
			s := NewFSStore(benchDir(b, c.projects, c.designs, c.noise))
			ctx := context.Background()
			b.ReportAllocs()
			for b.Loop() {
				if _, err := s.Projects(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkDesignsWalk measures the listing a browse UI asks for after picking a project. It
// resolves the project first and then walks that project's own folder, so it incurs two of the
// three costs.
func BenchmarkDesignsWalk(b *testing.B) {
	for _, c := range []struct {
		name                     string
		projects, designs, noise int
	}{
		{"small", 2, 1, 0},
		{"large", 20, 10, 200},
	} {
		b.Run(c.name, func(b *testing.B) {
			s := NewFSStore(benchDir(b, c.projects, c.designs, c.noise))
			ctx := context.Background()
			b.ReportAllocs()
			for b.Loop() {
				if _, err := s.Designs(ctx, "projects/p00"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkResolveDesign measures the OTHER hot path. A viewer calls it on every file open. It
// walks UP rather than down, so it should be cheap regardless of how big the mount is. This
// benchmark confirms that rather than assuming it.
func BenchmarkResolveDesign(b *testing.B) {
	for _, c := range []struct {
		name                     string
		projects, designs, noise int
	}{
		{"small", 2, 1, 0},
		{"large", 20, 10, 200},
	} {
		b.Run(c.name, func(b *testing.B) {
			s := NewFSStore(benchDir(b, c.projects, c.designs, c.noise))
			ctx := context.Background()
			u, err := artifact.New("m", "p00/designs/d00/board.edn")
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := s.ResolveDesign(ctx, u); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
