package main

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sync"
)

// renderPath is one path of the C code that upstream render.rs writes, and the
// text that shows that a parser.c reaches it. docs/CANDIDATES.md lists the
// paths, under "The paths of the C template".
type renderPath struct {
	name   string
	marker *regexp.Regexp
}

// renderPaths holds the paths of render.rs that the harness looks for.
var renderPaths = []renderPath{
	{"field map", regexp.MustCompile(`static const TSFieldMapEntry ts_field_map_entries`)},
	{"inherited field", regexp.MustCompile(`\.inherited = true`)},
	{"alias sequences", regexp.MustCompile(`static const TSSymbol ts_alias_sequences`)},
	{"named unique alias", regexp.MustCompile(`\balias_sym_\w+ = `)},
	{"anonymous unique alias", regexp.MustCompile(`\banon_alias_sym_\w+ = `)},
	{"non-terminal alias map", regexp.MustCompile(`ts_non_terminal_alias_map\[\]`)},
	{"supertype map", regexp.MustCompile(`static const TSSymbol ts_supertype_map_entries`)},
	{"keyword lexer", regexp.MustCompile(`\.keyword_lex_fn = ts_lex_keywords`)},
	{"large character set", regexp.MustCompile(`static const TSCharacterRange \w+\[\]`)},
	{"reserved words", regexp.MustCompile(`static const TSSymbol ts_reserved_words`)},
	{"reserved word set of a state", regexp.MustCompile(`\.reserved_word_set_id = `)},
	{"external scanner", regexp.MustCompile(`static const bool ts_external_scanner_states`)},
	{"external lex state", regexp.MustCompile(`\.external_lex_state = `)},
	{"pragma for a large lexer", regexp.MustCompile(`#pragma GCC optimize \("O0", "jump-tables"\)`)},
	{"ADVANCE_MAP", regexp.MustCompile(`ADVANCE_MAP\(`)},
	{"SKIP", regexp.MustCompile(`\bSKIP\(`)},
	{"eof action", regexp.MustCompile(`if \(eof\) ADVANCE\(`)},
	{"small parse table", regexp.MustCompile(`static const uint16_t ts_small_parse_table\[\]`)},
	{"end of a non-terminal extra", regexp.MustCompile(`\(TSStateId\)\(-1\)`)},
	{"SHIFT_REPEAT", regexp.MustCompile(`\bSHIFT_REPEAT\(`)},
	{"SHIFT_EXTRA", regexp.MustCompile(`\bSHIFT_EXTRA\(\)`)},
	{"RECOVER", regexp.MustCompile(`\bRECOVER\(\)`)},
	{"dynamic precedence", regexp.MustCompile(`\bREDUCE\([^,]+, \d+, -?[1-9]\d*, `)},
	{"ABI 14 lex modes", regexp.MustCompile(`static const TSLexMode ts_lex_modes`)},
	{"ABI 15 lex modes", regexp.MustCompile(`static const TSLexerMode ts_lex_modes`)},
	// ABI 15 always writes the metadata, with zeros when tree-sitter.json
	// gives no version
	{"grammar version", regexp.MustCompile(`\.(?:major|minor|patch)_version = [1-9]`)},
}

// reachedPaths returns the names of the paths that a parser.c reaches.
func reachedPaths(parserC []byte) []string {
	var names []string
	for _, p := range renderPaths {
		if p.marker.Match(parserC) {
			names = append(names, p.name)
		}
	}
	return names
}

// coverage counts, for each path, the outputs that reach it.
type coverage struct {
	mu      sync.Mutex
	outputs int
	reached map[string][]string // path to the grammars that reach it
}

// add records the paths that one output reaches.
func (c *coverage) add(grammar string, parserC []byte) {
	names := reachedPaths(parserC)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.reached == nil {
		c.reached = map[string][]string{}
	}
	c.outputs++
	for _, n := range names {
		if !slices.Contains(c.reached[n], grammar) {
			c.reached[n] = append(c.reached[n], grammar)
		}
	}
}

// print writes the coverage, and names each path that no output reaches.
func (c *coverage) print(w io.Writer) {
	// a run of the set corpus makes no output of the generator, and has
	// nothing to report
	if c.outputs == 0 {
		return
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "\npaths of render.rs, over %d outputs\n", c.outputs)
	var gaps []string
	for _, p := range renderPaths {
		g := c.reached[p.name]
		if len(g) == 0 {
			gaps = append(gaps, p.name)
		}
		slices.Sort(g)
		sample := g[:min(4, len(g))]
		fmt.Fprintf(&b, "  %-30s %4d grammars  %v\n", p.name, len(g), sample)
	}
	if len(gaps) > 0 {
		fmt.Fprintf(&b, "no output reaches: %v\n", gaps)
	}
	_, _ = w.Write(b.Bytes())
}
