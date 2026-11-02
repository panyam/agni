// Package plugins holds the agent skills agni ships, packaged so that Claude Code and Cursor install
// them as a plugin and GitHub's `gh skill install` finds them for any agent. The files are data, so
// this package is its test alone, which holds the layout every one of those installers reads.
package plugins

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const pluginName = "agni"

var skillName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("go.mod not found at %s: %v", root, err)
	}
	return root
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

type manifest struct {
	Name    string          `json:"name"`
	Version json.RawMessage `json:"version"`
}

type marketplace struct {
	Name    string                `json:"name"`
	Owner   struct{ Name string } `json:"owner"`
	Plugins []struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	} `json:"plugins"`
}

// Claude Code and Cursor each read their own marketplace file at the repo root and their own
// manifest in the plugin. All four have to name the same plugin at the same folder, or one tool
// installs it and the other reports it missing. No manifest carries a version, because the git tag
// is agni's only version and a second one would drift from it.
func TestTheMarketplacesAndManifestsAgree(t *testing.T) {
	root := repoRoot(t)
	for _, tool := range []string{".claude-plugin", ".cursor-plugin"} {
		var m marketplace
		readJSON(t, filepath.Join(root, tool, "marketplace.json"), &m)
		if m.Name != pluginName || m.Owner.Name == "" {
			t.Errorf("%s/marketplace.json: name %q, owner %q", tool, m.Name, m.Owner.Name)
		}
		if len(m.Plugins) != 1 {
			t.Fatalf("%s/marketplace.json lists %d plugins, want 1", tool, len(m.Plugins))
		}
		p := m.Plugins[0]
		if p.Name != pluginName {
			t.Errorf("%s/marketplace.json: plugin %q, want %q", tool, p.Name, pluginName)
		}
		if got := filepath.Clean(p.Source); got != filepath.Join("plugins", pluginName) {
			t.Errorf("%s/marketplace.json: source %q, want the plugins/%s folder", tool, p.Source, pluginName)
		}

		var pm manifest
		readJSON(t, filepath.Join(root, "plugins", pluginName, tool, "plugin.json"), &pm)
		if pm.Name != pluginName {
			t.Errorf("plugins/%s/%s/plugin.json: name %q, want %q", pluginName, tool, pm.Name, pluginName)
		}
		if pm.Version != nil {
			t.Errorf("plugins/%s/%s/plugin.json carries a version; the git tag is the only one", pluginName, tool)
		}
	}
}

// Every folder under the plugin's skills/ is one skill, named by its folder. That is the
// skills/*/SKILL.md convention all three installers discover, so a skill added there needs no
// other edit to reach Claude Code, Cursor and Copilot.
func TestEverySkillIsWellFormed(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "plugins", pluginName, "skills")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range entries {
		if !e.IsDir() {
			t.Errorf("skills/%s is a file; each skill is a folder holding SKILL.md", e.Name())
			continue
		}
		seen++
		path := filepath.Join(dir, e.Name(), "SKILL.md")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("skills/%s: %v", e.Name(), err)
			continue
		}
		front, ok := frontmatter(b)
		if !ok {
			t.Errorf("skills/%s/SKILL.md opens with no --- frontmatter", e.Name())
			continue
		}
		var meta struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		}
		if err := yaml.Unmarshal(front, &meta); err != nil {
			t.Errorf("skills/%s/SKILL.md frontmatter: %v", e.Name(), err)
			continue
		}
		if meta.Name != e.Name() {
			t.Errorf("skills/%s/SKILL.md is named %q; a skill's name is its folder's", e.Name(), meta.Name)
		}
		if !skillName.MatchString(meta.Name) || len(meta.Name) > 64 {
			t.Errorf("skill name %q is not lowercase kebab-case of at most 64 characters", meta.Name)
		}
		if d := strings.TrimSpace(meta.Description); d == "" || len(d) > 1024 {
			t.Errorf("skills/%s/SKILL.md description is %d characters; it needs 1 to 1024", e.Name(), len(d))
		}
	}
	if seen == 0 {
		t.Fatal("no skills found; the walk read nothing")
	}
}

// A SKILL.md anywhere else in the tree reaches no installer, so it would work only for someone
// with a checkout, which is the case this layout exists to avoid.
func TestNoSkillLivesOutsideThePlugin(t *testing.T) {
	root := repoRoot(t)
	plugin := filepath.Join(root, "plugins", pluginName, "skills")
	skip := map[string]bool{".git": true, "node_modules": true, "samples": true, ".venv": true, "worktrees": true}
	found := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skip[d.Name()] {
			return filepath.SkipDir
		}
		if d.Name() != "SKILL.md" {
			return nil
		}
		found++
		if !strings.HasPrefix(path, plugin+string(filepath.Separator)) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s is outside plugins/%s/skills, where no installer finds it", rel, pluginName)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == 0 {
		t.Fatal("the walk found no SKILL.md at all, not even the plugin's own")
	}
}

func frontmatter(b []byte) ([]byte, bool) {
	rest, ok := bytes.CutPrefix(b, []byte("---\n"))
	if !ok {
		return nil, false
	}
	front, _, ok := bytes.Cut(rest, []byte("\n---\n"))
	return front, ok
}
