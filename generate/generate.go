// Package generate is the parser generator of transit, a port of the crate
// crates/generate of upstream tree-sitter (D7). It reads a grammar from
// grammar.json (D17), builds the parse table and the lexer tables, and gives
// them to a backend, which writes a parser (D8).
//
// The package holds only the first modules of the port yet: the string pool,
// the bit vectors, the rules, the grammars, the reader of grammar.json and the
// NFA of the tokens.
package generate

import (
	"fmt"
	"strings"
)

// This file ports crates/generate/src/generate.rs. It holds only Diagnostic
// yet. The functions that run the whole generator come when the modules that
// they call are ported.

// DiagnosticKind is the kind of a diagnostic.
type DiagnosticKind uint8

// The kinds of diagnostic, in the order of upstream.
const (
	DiagnosticUnnecessaryConflicts DiagnosticKind = iota
	DiagnosticUnaryChoice
	DiagnosticUnarySeq
	DiagnosticEmptyStringMatch
	DiagnosticUnsupportedRegexFlag
	DiagnosticSupertypeInlined
)

// Diagnostic is a warning that the generator reports and that does not stop
// it.
//
// Diagnostic is Diagnostic, an enum with data upstream. The fields that a kind
// uses are:
//
//   - DiagnosticUnnecessaryConflicts: Conflicts.
//   - DiagnosticUnaryChoice and DiagnosticUnarySeq: Name, which is empty for
//     a rule with no name.
//   - DiagnosticEmptyStringMatch and DiagnosticSupertypeInlined: Name.
//   - DiagnosticUnsupportedRegexFlag: Flag and Pattern.
type Diagnostic struct {
	Kind      DiagnosticKind
	Conflicts [][]string
	Name      string
	Flag      rune
	Pattern   string
}

// String returns the text of the diagnostic, as upstream writes it.
//
// String is the Display of Diagnostic.
func (d Diagnostic) String() string {
	var b strings.Builder
	switch d.Kind {
	case DiagnosticUnnecessaryConflicts:
		b.WriteString("unnecessary conflicts:\n")
		for i, conflict := range d.Conflicts {
			b.WriteString("  ")
			for j, symbol := range conflict {
				fmt.Fprintf(&b, "`%s`", symbol)
				if j < len(conflict)-1 {
					b.WriteString(", ")
				}
			}
			if i < len(d.Conflicts)-1 {
				b.WriteString("\n")
			}
		}
	case DiagnosticUnaryChoice:
		fmt.Fprintf(&b, "rule %s contains a `choice` rule with a single element. this is unnecessary.", nameOrAnonymous(d.Name))
	case DiagnosticUnarySeq:
		fmt.Fprintf(&b, "rule %s contains a `seq` rule with a single element. this is unnecessary.", nameOrAnonymous(d.Name))
	case DiagnosticEmptyStringMatch:
		fmt.Fprintf(&b, "named extra rule `%s` matches the empty string. inline this to avoid infinite loops while parsing.", d.Name)
	case DiagnosticUnsupportedRegexFlag:
		fmt.Fprintf(&b, "unsupported regex flag `%c` in pattern `%s`", d.Flag, d.Pattern)
	case DiagnosticSupertypeInlined:
		fmt.Fprintf(&b, "rule `%s` is both a supertype and inlined. the supertype is ignored.", d.Name)
	}
	return b.String()
}

// nameOrAnonymous returns the name, or <ANONYMOUS> when it is empty.
func nameOrAnonymous(name string) string {
	if name == "" {
		return "<ANONYMOUS>"
	}
	return name
}
