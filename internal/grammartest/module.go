package grammartest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	golang "github.com/xo/transit/generate/backend/go"
)

// This file reads the tree-sitter.json of the module of a grammar package,
// which the corpus test (D83) and the highlight test (D80) read. It ports no
// upstream file.

// module is the tree-sitter.json of the module of a grammar package.
type module struct {
	// entries holds the grammars of tree-sitter.json, in its order. It is
	// empty when no folder above the package holds a tree-sitter.json.
	entries []treeSitterGrammar
	// own is the folder of the package in the module, as
	// golang.GrammarFolder gives it.
	own string
}

// treeSitterGrammar is an entry of grammars in tree-sitter.json, with the
// fields that the tests read.
//
// treeSitterGrammar is Grammar of the loader.
type treeSitterGrammar struct {
	Name           string    `json:"name"`
	Path           string    `json:"path"`
	Highlights     pathsJSON `json:"highlights"`
	Injections     pathsJSON `json:"injections"`
	Locals         pathsJSON `json:"locals"`
	InjectionRegex string    `json:"injection-regex"`
}

// folder returns the folder in the module of the package of the grammar of
// the entry.
func (e treeSitterGrammar) folder() string {
	return golang.GrammarFolder(e.Path)
}

// readModule reads the nearest tree-sitter.json, in the working folder or in
// a folder above it, for the package of the language name. The folder of
// the package is the working folder, relative to the folder of
// tree-sitter.json, when an entry has that folder. Else it is the folder
// that the name gives, as for a checkout of a repository with more than one
// grammar, whose tree-sitter.json is in the working folder.
func readModule(name string) (*module, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("finding the working folder: %w", err)
	}
	return readModuleIn(wd, name)
}

// readModuleIn is readModule with the folder wd in place of the working
// folder.
func readModuleIn(wd, name string) (*module, error) {
	m := &module{own: golang.GrammarFolder(name)}
	for dir := wd; ; {
		file := filepath.Join(dir, "tree-sitter.json")
		b, err := os.ReadFile(file)
		switch {
		case err == nil:
			var cfg struct {
				Grammars []treeSitterGrammar `json:"grammars"`
			}
			if err := json.Unmarshal(b, &cfg); err != nil {
				return nil, fmt.Errorf("reading %s: %w", file, err)
			}
			m.entries = cfg.Grammars
			rel, err := filepath.Rel(dir, wd)
			if err != nil {
				return nil, fmt.Errorf("finding the folder of the package: %w", err)
			}
			if rel = golang.GrammarFolder(filepath.ToSlash(rel)); m.hasFolder(rel) {
				m.own = rel
			}
			return m, nil
		case !errors.Is(err, fs.ErrNotExist):
			return nil, fmt.Errorf("reading %s: %w", file, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			m.own = "."
			return m, nil
		}
		dir = parent
	}
}

// hasFolder reports whether an entry of the module is in the folder f.
func (m *module) hasFolder(f string) bool {
	for _, e := range m.entries {
		if e.folder() == f {
			return true
		}
	}
	return false
}

// entry returns the first entry with the name, and false when the module
// has none.
func (m *module) entry(name string) (treeSitterGrammar, bool) {
	for _, e := range m.entries {
		if e.Name == name {
			return e, true
		}
	}
	return treeSitterGrammar{}, false
}
