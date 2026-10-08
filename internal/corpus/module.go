package corpus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// This file reads the tree-sitter.json of the module of a grammar package,
// which the corpus test (D83), the highlight test (D80) and the example of a
// grammar package (D115) read. It ports no upstream file.

// Module is the tree-sitter.json of the module of a grammar package.
type Module struct {
	// Entries holds the grammars of tree-sitter.json, in its order. It is
	// empty when no folder above the package holds a tree-sitter.json.
	Entries []Grammar
	// Own is the folder of the package in the module, as GrammarFolder
	// gives it.
	Own string
}

// Grammar is an entry of grammars in tree-sitter.json, with the fields that
// the tests read.
//
// Grammar is Grammar of the loader.
type Grammar struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Highlights     Paths  `json:"highlights"`
	Injections     Paths  `json:"injections"`
	Locals         Paths  `json:"locals"`
	InjectionRegex string `json:"injection-regex"`
}

// Folder returns the folder in the module of the package of the grammar of
// the entry.
func (g Grammar) Folder() string {
	return GrammarFolder(g.Path)
}

// Paths is a list of paths in tree-sitter.json: a path, a list of paths,
// or nothing.
//
// Paths is PathsJSON.
type Paths struct {
	// Paths holds the paths. Set is true when tree-sitter.json gives the
	// field.
	Paths []string
	Set   bool
}

// UnmarshalJSON reads a path or a list of paths.
func (p *Paths) UnmarshalJSON(b []byte) error {
	if string(bytes.TrimSpace(b)) == "null" {
		return nil
	}
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		*p = Paths{Paths: []string{single}, Set: true}
		return nil
	}
	var multiple []string
	if err := json.Unmarshal(b, &multiple); err != nil {
		return fmt.Errorf("reading a list of paths: %w", err)
	}
	*p = Paths{Paths: multiple, Set: true}
	return nil
}

// GrammarFolder returns the folder in its module of the package of a
// grammar whose path in tree-sitter.json is p: "." for the folder of the
// module, and else the last element of p with each "_" and "-" removed, as
// docs/GRAMMAR.md lays it out. The path php_only gives the folder phponly.
func GrammarFolder(p string) string {
	p = path.Clean(filepath.ToSlash(p))
	if p == "." || p == "" {
		return "."
	}
	return strings.NewReplacer("_", "", "-", "").Replace(path.Base(p))
}

// ReadModule reads the nearest tree-sitter.json, in the working folder or in
// a folder above it, for the package of the language name. The folder of
// the package is the working folder, relative to the folder of
// tree-sitter.json, when an entry has that folder. Else it is the folder
// that the name gives, as for a checkout of a repository with more than one
// grammar, whose tree-sitter.json is in the working folder.
func ReadModule(name string) (*Module, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("finding the working folder: %w", err)
	}
	return ReadModuleIn(wd, name)
}

// ReadModuleIn is ReadModule with the folder wd in place of the working
// folder.
func ReadModuleIn(wd, name string) (*Module, error) {
	m := &Module{Own: GrammarFolder(name)}
	for dir := wd; ; {
		file := filepath.Join(dir, "tree-sitter.json")
		b, err := os.ReadFile(file)
		switch {
		case err == nil:
			var cfg struct {
				Grammars []Grammar `json:"grammars"`
			}
			if err := json.Unmarshal(b, &cfg); err != nil {
				return nil, fmt.Errorf("reading %s: %w", file, err)
			}
			m.Entries = cfg.Grammars
			rel, err := filepath.Rel(dir, wd)
			if err != nil {
				return nil, fmt.Errorf("finding the folder of the package: %w", err)
			}
			if rel = GrammarFolder(filepath.ToSlash(rel)); m.HasFolder(rel) {
				m.Own = rel
			}
			return m, nil
		case !errors.Is(err, fs.ErrNotExist):
			return nil, fmt.Errorf("reading %s: %w", file, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			m.Own = "."
			return m, nil
		}
		dir = parent
	}
}

// HasFolder reports whether an entry of the module is in the folder f.
func (m *Module) HasFolder(f string) bool {
	for _, e := range m.Entries {
		if e.Folder() == f {
			return true
		}
	}
	return false
}

// Entry returns the first entry with the name, and false when the module
// has none.
func (m *Module) Entry(name string) (Grammar, bool) {
	for _, e := range m.Entries {
		if e.Name == name {
			return e, true
		}
	}
	return Grammar{}, false
}

// Owner returns the folder of the package that runs a case: the package of
// the grammar of its first :language, or of the first grammar of the module
// when it names none. A case for a grammar that the module does not hold
// runs in the package, and fails with "Language not found". A case that
// names no grammar runs in the package when no entry of tree-sitter.json
// has the folder of the package, such as plpgsql of tree-sitter-postgres.
// Upstream runs the corpus of such a grammar in its own folder, where the
// grammar of the folder is the only language.
func (m *Module) Owner(a Attributes) string {
	name := ""
	if len(a.Languages) > 0 {
		name = a.Languages[0]
	}
	if name == "" {
		if len(m.Entries) == 0 || !m.HasFolder(m.Own) {
			return m.Own
		}
		return m.Entries[0].Folder()
	}
	if e, ok := m.Entry(name); ok {
		return e.Folder()
	}
	return m.Own
}

// Holds reports whether the package of the language whose name is
// language can parse a case for the grammar name of an attribute
// :language. A package has one language. A grammar of the module in the
// folder of the package, such as flow of tree-sitter-typescript, is that
// language, and an empty name names it too. A case for any other grammar
// fails with "Language not found".
func (m *Module) Holds(language, name string) bool {
	if name == "" || name == language {
		return true
	}
	e, ok := m.Entry(name)
	return ok && e.Folder() == m.Own
}
