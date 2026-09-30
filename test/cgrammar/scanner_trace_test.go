package cgrammar

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/internal/abi"
)

// This file compares a Go external scanner with the C scanner of the same
// grammar, as docs/GRAMMAR.md asks: the same tokens and the same serialized
// bytes for every corpus input. The Go runtime parses each input twice, once
// with each scanner, and each call of each scanner is written to a trace.
// Both parses run the same runtime, so the traces are equal only when the two
// scanners do the same thing at each call.

// traceScanner wraps a scanner and writes each call to a trace.
type traceScanner struct {
	inner abi.Scanner
	trace *strings.Builder
}

// countingFuncs wraps the functions of the lexer, and counts the calls of
// Advance, the characters that it skips and the calls of MarkEnd.
type countingFuncs struct {
	inner    abi.LexerFuncs
	lexer    *abi.Lexer
	advances []string
}

func (f *countingFuncs) Advance(skip bool) {
	if skip {
		f.advances = append(f.advances, fmt.Sprintf("s%d", f.lexer.Lookahead))
	} else {
		f.advances = append(f.advances, fmt.Sprintf("a%d", f.lexer.Lookahead))
	}
	f.inner.Advance(skip)
}

func (f *countingFuncs) MarkEnd() {
	f.advances = append(f.advances, "m")
	f.inner.MarkEnd()
}

func (f *countingFuncs) GetColumn() uint32 {
	c := f.inner.GetColumn()
	f.advances = append(f.advances, fmt.Sprintf("c%d", c))
	return c
}

func (f *countingFuncs) IsAtIncludedRangeStart() bool {
	return f.inner.IsAtIncludedRangeStart()
}

func (f *countingFuncs) EOF() bool {
	return f.inner.EOF()
}

func (f *countingFuncs) Logf(format string, args ...any) {
	f.inner.Logf(format, args...)
}

// Scan writes the valid symbols, the calls of the lexer, the result and the
// symbol of the call.
func (s *traceScanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	valid := make([]byte, len(validSymbols))
	for i, v := range validSymbols {
		valid[i] = '0'
		if v {
			valid[i] = '1'
		}
	}
	funcs := &countingFuncs{inner: lexer.Funcs, lexer: lexer}
	lexer.Funcs = funcs
	found := s.inner.Scan(lexer, validSymbols)
	lexer.Funcs = funcs.inner
	fmt.Fprintf(s.trace, "scan %s at %d: %v", valid, lexer.Lookahead, found)
	if found {
		fmt.Fprintf(s.trace, " symbol %d", lexer.ResultSymbol)
	}
	fmt.Fprintf(s.trace, " %s\n", strings.Join(funcs.advances, " "))
	return found
}

// Serialize writes the bytes that the scanner serializes.
func (s *traceScanner) Serialize(buf []byte) int {
	n := s.inner.Serialize(buf)
	fmt.Fprintf(s.trace, "serialize %x\n", buf[:n])
	return n
}

// Deserialize writes the bytes that the scanner is given.
func (s *traceScanner) Deserialize(buf []byte) {
	fmt.Fprintf(s.trace, "deserialize %x\n", buf)
	s.inner.Deserialize(buf)
}

// withTrace returns a copy of a language whose scanner is create, wrapped so
// that each call is written to trace.
func withTrace(l *transit.Language, create func() abi.Scanner, trace *strings.Builder) *transit.Language {
	tables := *tablesOf(l)
	tables.ExternalScanner.Create = func() abi.Scanner {
		return &traceScanner{inner: create(), trace: trace}
	}
	return transit.NewLanguage(&tables)
}

// compareScanners parses each input with the Go runtime and the tables of
// goLang, once with the scanner of goLang and once with the scanner of the
// C grammar g, and fails the test when the traces of the two scanners
// differ. It returns the number of inputs and the number of scanner calls.
func compareScanners(t *testing.T, g *Grammar, goLang *transit.Language, inputs [][]byte) (int, int) {
	t.Helper()
	goCreate := tablesOf(goLang).ExternalScanner.Create
	cCreate := tablesOf(g.Language).ExternalScanner.Create
	if goCreate == nil || cCreate == nil {
		t.Fatalf("the Go grammar or the C grammar %s has no external scanner", g.Name)
	}
	var goTrace, cTrace strings.Builder
	goTraced := withTrace(goLang, goCreate, &goTrace)
	cTraced := withTrace(goLang, cCreate, &cTrace)
	calls, differ := 0, 0
	for i, src := range inputs {
		goTrace.Reset()
		cTrace.Reset()
		for _, l := range []*transit.Language{goTraced, cTraced} {
			p := transit.NewParser()
			if err := p.SetLanguage(l); err != nil {
				t.Fatal(err)
			}
			if _, err := p.Parse(context.Background(), src, nil); err != nil {
				t.Fatal(err)
			}
		}
		calls += strings.Count(cTrace.String(), "\n")
		if goTrace.String() != cTrace.String() {
			differ++
			if differ <= 3 {
				t.Errorf("input %d: the scanners differ at the first line that differs:\n%s", i, firstDifference(cTrace.String(), goTrace.String()))
			}
		}
	}
	return len(inputs), calls
}

// firstDifference returns the first line of two traces that differs, with
// the line before it.
func firstDifference(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := range max(len(w), len(g)) {
		var a, b string
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			b = g[i]
		}
		if a != b {
			before := ""
			if i > 0 {
				before = w[i-1]
			}
			return fmt.Sprintf("  before: %s\n  C:  %s\n  Go: %s", before, a, b)
		}
	}
	return ""
}

// TestCompareScannersSelf checks compareScanners with the C scanner of
// python on both sides, so that the trace is written and compared.
func TestCompareScannersSelf(t *testing.T) {
	t.Parallel()
	root, cache := setup(t)
	for _, f := range fixtures(t, root) {
		if f.Name != "python" {
			continue
		}
		g, examples := loadFixture(t, cache, f)
		var inputs [][]byte
		for _, e := range examples[:min(len(examples), 20)] {
			inputs = append(inputs, e.Input)
		}
		n, calls := compareScanners(t, g, g.Language, inputs)
		if calls == 0 {
			t.Errorf("the scanner of python was not called on %d inputs", n)
		}
		t.Logf("%d inputs, %d calls", n, calls)
	}
}
