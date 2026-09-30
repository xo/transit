package cgrammar

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/internal/abi"
)

// This file holds the helpers that compare a Go scanner with its C scanner
// outside a parse: from a state that the test chooses, at each position of
// a text, with each set of valid symbols. A parse reaches only the states
// and the valid symbols that the grammar makes, so these helpers reach the
// branches of a scanner that the corpus does not reach.

// scannerPair is a Go scanner and a C scanner of the same grammar, each with
// a trace of its calls.
type scannerPair struct {
	goScanner, cScanner *traceScanner
	goTrace, cTrace     strings.Builder
}

// newScannerPair makes a new Go scanner and a new C scanner.
func newScannerPair(g *Grammar, goLang *transit.Language) *scannerPair {
	p := &scannerPair{}
	p.goScanner = &traceScanner{inner: tablesOf(goLang).ExternalScanner.Create(), trace: &p.goTrace}
	p.cScanner = &traceScanner{inner: tablesOf(g.Language).ExternalScanner.Create(), trace: &p.cTrace}
	return p
}

// each calls fn with the Go scanner and with the C scanner.
func (p *scannerPair) each(fn func(s *traceScanner)) {
	fn(p.goScanner)
	fn(p.cScanner)
}

// deserialize gives both scanners the state. The C scanner gets the bytes
// of a buffer that holds zeros after the state, because a C scanner can
// read past the length of a state that its serialize did not write.
func (p *scannerPair) deserialize(state []byte) {
	padded := make([]byte, len(state), len(state)+abi.SerializationBufferSize)
	copy(padded, state)
	p.each(func(s *traceScanner) { s.Deserialize(padded) })
}

// serialize makes both scanners write their state to the trace.
func (p *scannerPair) serialize() {
	p.each(func(s *traceScanner) {
		buf := make([]byte, abi.SerializationBufferSize)
		s.Serialize(buf)
	})
}

// scan makes both scanners scan text from pos with the valid symbols, and
// writes the calls of the lexer and the position of each mark_end to the
// trace.
func (p *scannerPair) scan(text []byte, pos int, valid []bool) {
	p.each(func(s *traceScanner) {
		l := &lexLog{text: text}
		l.Funcs = l
		l.reset(pos)
		s.Scan(&l.Lexer, valid)
		fmt.Fprintf(s.trace, "lexer %v\n", l.log)
	})
}

// differs returns the first line of the traces that differs, or "" when the
// traces are equal, and resets the traces.
func (p *scannerPair) differs() string {
	var d string
	if p.goTrace.String() != p.cTrace.String() {
		d = firstDifference(p.cTrace.String(), p.goTrace.String())
	}
	p.goTrace.Reset()
	p.cTrace.Reset()
	return d
}

// validSymbolSets returns each row of the external lex states of a
// language, and a row with every token valid, as the parser gives it in
// error recovery.
func validSymbolSets(l *transit.Language) [][]bool {
	tables := tablesOf(l)
	n := int(tables.ExternalTokenCount)
	var sets [][]bool
	for i := 0; i+n <= len(tables.ExternalScanner.States); i += n {
		sets = append(sets, tables.ExternalScanner.States[i:i+n])
	}
	all := make([]bool, n)
	for i := range all {
		all[i] = true
	}
	return append(sets, all)
}

// compareScannerScans gives a new Go scanner and a new C scanner each state,
// makes them scan each text from each position with each set of valid
// symbols, and compares the calls, the results and the states after. It
// returns the number of scans.
func compareScannerScans(t *testing.T, g *Grammar, goLang *transit.Language, states, texts [][]byte, sets [][]bool) int {
	t.Helper()
	scans, differ := 0, 0
	for si, state := range states {
		for ti, text := range texts {
			for pos := 0; pos <= len(text); pos++ {
				for vi, valid := range sets {
					p := newScannerPair(g, goLang)
					p.deserialize(state)
					p.scan(text, pos, valid)
					p.serialize()
					scans++
					if d := p.differs(); d != "" {
						differ++
						if differ <= 3 {
							t.Errorf("state %d, text %d at %d, valid symbols %d: the scanners differ:\n%s", si, ti, pos, vi, d)
						}
					}
				}
			}
		}
	}
	if differ > 0 {
		t.Errorf("%d of %d scans differ", differ, scans)
	}
	return scans
}

// compareScannerStates gives a new Go scanner and a new C scanner each
// state, and compares the states that they serialize after it.
func compareScannerStates(t *testing.T, g *Grammar, goLang *transit.Language, states [][]byte) {
	t.Helper()
	differ := 0
	for i, state := range states {
		p := newScannerPair(g, goLang)
		p.deserialize(state)
		p.serialize()
		if d := p.differs(); d != "" {
			differ++
			if differ <= 3 {
				t.Errorf("state %d (%x): the scanners differ:\n%s", i, state, d)
			}
		}
	}
	if differ > 0 {
		t.Errorf("%d of %d states differ", differ, len(states))
	}
}

// randomStates returns n states of random bytes from the seed, each with a
// random length up to maxLength, and the states of each length up to 4 with
// every byte 0 or 0xff.
func randomStates(seed uint64, n, maxLength int) [][]byte {
	r := rand.New(rand.NewPCG(seed, 1))
	states := [][]byte{nil}
	for length := 1; length <= 4; length++ {
		states = append(states, make([]byte, length), []byte(strings.Repeat("\xff", length)))
	}
	for range n {
		state := make([]byte, r.IntN(maxLength+1))
		for i := range state {
			state[i] = byte(r.Uint32())
		}
		states = append(states, state)
	}
	return states
}
