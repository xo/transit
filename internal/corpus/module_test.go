package corpus

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadModule makes sure that the folder of a package in its module comes
// from the working folder, or from the name of its language when no entry
// of tree-sitter.json has the working folder, and that a case runs in the
// package that D83 names.
func TestReadModule(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"grammars": [{"name": "typescript", "path": "typescript"}, {"name": "tsx", "path": "tsx"}, {"name": "flow", "path": "tsx"}]}`
	if err := os.WriteFile(filepath.Join(dir, "tree-sitter.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"typescript", "tsx", "other"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct{ wd, own string }{
		{"typescript", "typescript"},
		{"tsx", "tsx"},
		{"other", "tsx"},
		{".", "tsx"},
	} {
		t.Chdir(filepath.Join(dir, c.wd))
		m, err := ReadModule("tsx")
		if err != nil {
			t.Fatal(err)
		}
		if m.Own != c.own || len(m.Entries) != 3 {
			t.Errorf("in %s: own %q with %d entries, want %q with 3", c.wd, m.Own, len(m.Entries), c.own)
		}
	}
	t.Chdir(filepath.Join(dir, "tsx"))
	m, err := ReadModule("tsx")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		languages []string
		owner     string
		holds     bool
	}{
		{[]string{""}, "typescript", true},
		{[]string{"tsx"}, "tsx", true},
		{[]string{"flow"}, "tsx", true},
		{[]string{"typescript"}, "typescript", false},
		{[]string{"css"}, "tsx", false},
	} {
		if got := m.Owner(Attributes{Languages: c.languages}); got != c.owner {
			t.Errorf("owner of %q = %q, want %q", c.languages, got, c.owner)
		}
		if got := m.Holds("tsx", c.languages[0]); got != c.holds {
			t.Errorf("Holds(%q) = %t, want %t", c.languages[0], got, c.holds)
		}
	}
	// A package whose folder no entry has runs each case that names no
	// grammar.
	t.Chdir(filepath.Join(dir, "other"))
	if m, err = ReadModule("other"); err != nil {
		t.Fatal(err)
	}
	if got := m.Owner(Attributes{Languages: []string{""}}); got != "other" {
		t.Errorf("in a folder that no entry has, the owner of a case = %q, want %q", got, "other")
	}
	t.Chdir(t.TempDir())
	if m, err := ReadModule("tsx"); err != nil || m.Own != "." || len(m.Entries) != 0 {
		t.Errorf("with no tree-sitter.json, ReadModule = %+v, %v", m, err)
	}
}
