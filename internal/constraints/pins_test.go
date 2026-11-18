package constraints

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	usesLine  = regexp.MustCompile(`^\s*(?:-\s+)?uses:\s*["']?([^\s"'#]+)`)
	commitSHA = regexp.MustCompile(`@[0-9a-f]{40}$`)
	fromLine  = regexp.MustCompile(`(?i)^FROM\s+(?:--platform=\S+\s+)?(\S+)(?:\s+AS\s+(\S+))?`)
	digest    = regexp.MustCompile(`@sha256:[0-9a-f]{64}$`)
	// pipInstall matches pip, pip3 or a quoted path to either, followed by install.
	pipInstall = regexp.MustCompile(`\bpip3?"?\s+install\b`)
)

// TestC37ActionsArePinnedToACommit fails on a workflow or composite action step whose `uses:` names
// a tag or a branch. A tag can be moved to any commit by whoever controls the action's repo, and the
// moved commit runs with this repo's token, and in the release job with the PyPI credentials too.
func TestC37ActionsArePinnedToACommit(t *testing.T) {
	root := repoRoot(t)
	files, _ := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.y*ml"))
	actions, _ := filepath.Glob(filepath.Join(root, ".github", "actions", "*", "action.y*ml"))
	files = append(files, actions...)
	steps := 0
	for _, f := range files {
		eachLine(t, f, func(n int, line string) {
			m := usesLine.FindStringSubmatch(line)
			if m == nil {
				return
			}
			steps++
			ref := m[1]
			if strings.HasPrefix(ref, "./") || commitSHA.MatchString(ref) {
				return
			}
			rel, _ := filepath.Rel(root, f)
			t.Errorf("%s:%d uses %s, which C37 forbids. Pin it to the full commit SHA its tag points at, "+
				"with the tag in a trailing comment (`uses: owner/repo@<sha> # v1`).", rel, n, ref)
		})
	}
	if len(files) == 0 || steps < 20 {
		t.Fatalf("read %d workflow files and %d uses: steps, so this check proves nothing", len(files), steps)
	}
}

// TestC37BaseImagesArePinnedToADigest fails on a Dockerfile FROM that names an image by tag alone. A
// tag is mutable on the registry, so the same Dockerfile could build on different bytes tomorrow. A
// FROM that names an earlier stage of the same file, or scratch, has nothing to pin.
func TestC37BaseImagesArePinnedToADigest(t *testing.T) {
	root := repoRoot(t)
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" ||
				path == filepath.Join(root, "tools", "samples")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), "Dockerfile") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	froms := 0
	for _, f := range files {
		stages := map[string]bool{"scratch": true}
		eachLine(t, f, func(n int, line string) {
			m := fromLine.FindStringSubmatch(strings.TrimSpace(line))
			if m == nil {
				return
			}
			froms++
			image := m[1]
			if m[2] != "" {
				stages[strings.ToLower(m[2])] = true
			}
			if stages[strings.ToLower(image)] || digest.MatchString(image) {
				return
			}
			rel, _ := filepath.Rel(root, f)
			t.Errorf("%s:%d builds FROM %s, which C37 forbids. Pin it as image:tag@sha256:<digest>, "+
				"taking the multi-platform index digest from `docker buildx imagetools inspect`.", rel, n, image)
		})
	}
	if len(files) == 0 || froms < 5 {
		t.Fatalf("read %d Dockerfiles and %d FROM lines, so this check proves nothing", len(files), froms)
	}
}

func eachLine(t *testing.T, path string, fn func(n int, line string)) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for n := 1; s.Scan(); n++ {
		fn(n, s.Text())
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
}

// TestC37PipInstallsCheckHashes fails on a `pip install` in a Dockerfile or the Makefile that can
// fetch a package without checking its hash. An install passes with --require-hashes and a lock, or
// with --no-deps for the local package whose dependencies a hashed install already put in place.
func TestC37PipInstallsCheckHashes(t *testing.T) {
	root := repoRoot(t)
	files, _ := filepath.Glob(filepath.Join(root, "Dockerfile*"))
	files = append(files, filepath.Join(root, "Makefile"))
	installs := 0
	for _, f := range files {
		eachLine(t, f, func(n int, line string) {
			code := strings.TrimSpace(line)
			if strings.HasPrefix(code, "#") || !pipInstall.MatchString(code) {
				return
			}
			installs++
			if strings.Contains(code, "--require-hashes") || strings.Contains(code, "--no-deps") {
				return
			}
			rel, _ := filepath.Rel(root, f)
			t.Errorf("%s:%d runs %q, which C37 forbids. Install from a lock compiled with "+
				"--generate-hashes, with --require-hashes on the same line.", rel, n, code)
		})
	}
	if installs < 4 {
		t.Fatalf("found %d pip installs in the Dockerfiles and the Makefile, so this check proves nothing", installs)
	}
}

// TestC37GradleBuildsVerifyDependencies fails on a Gradle build with no dependency verification
// beside its settings file. Without gradle/verification-metadata.xml, Gradle takes whatever jar Maven
// Central serves for a version.
func TestC37GradleBuildsVerifyDependencies(t *testing.T) {
	root := repoRoot(t)
	var builds []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" ||
				d.Name() == "build" || path == filepath.Join(root, "tools", "samples")) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "settings.gradle.kts" || d.Name() == "settings.gradle" {
			builds = append(builds, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(builds) == 0 {
		t.Fatal("found no Gradle settings file, so this check proves nothing")
	}
	for _, dir := range builds {
		rel, _ := filepath.Rel(root, dir)
		b, err := os.ReadFile(filepath.Join(dir, "gradle", "verification-metadata.xml"))
		if err != nil {
			t.Errorf("%s has no gradle/verification-metadata.xml, which C37 requires. Write it with "+
				"`./gradlew --write-verification-metadata sha256` and the tasks CI runs.", rel)
			continue
		}
		s := string(b)
		if !strings.Contains(s, "<verify-metadata>true</verify-metadata>") {
			t.Errorf("%s/gradle/verification-metadata.xml does not verify metadata", rel)
		}
		if n := strings.Count(s, "<sha256 "); n < 20 {
			t.Errorf("%s/gradle/verification-metadata.xml holds %d sha256 checksums, too few to cover the build", rel, n)
		}
	}
}
