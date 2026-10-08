// Package corpus reads the corpus of a grammar package, the tree-sitter.json
// of its module and its testdata/failing.txt. The corpus test of the package
// internal/grammartest reads them, and so does the Go backend, which writes
// the example of a grammar package from the first corpus case that passes
// (D115). The package imports only the Go standard library.
//
// test.go ports the parts of crates/cli/src/test.rs that read a corpus.
// The other files port no upstream file.
package corpus

import "iter"

// Cases yields each corpus case of a group and of the groups in it, in the
// order of the corpus, with its path: the names of its groups under the root
// and its own name, joined by "/", such as "expressions/Binary operators".
// A group with no case is left out. The path names a case in
// testdata/failing.txt.
func Cases(root Entry) iter.Seq2[string, Entry] {
	return func(yield func(string, Entry) bool) {
		walk(root, "", yield)
	}
}

// walk yields each case of a group with the path prefix, and reports
// whether the walk goes on.
func walk(group Entry, prefix string, yield func(string, Entry) bool) bool {
	for _, child := range group.Children {
		name := child.Name
		if prefix != "" {
			name = prefix + "/" + child.Name
		}
		if child.IsGroup {
			if !walk(child, name, yield) {
				return false
			}
			continue
		}
		if !yield(name, child) {
			return false
		}
	}
	return true
}
