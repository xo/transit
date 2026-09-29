package generate

import (
	"slices"
	"testing"
)

// TestChoiceFlattensAndDedupes makes sure that Choice flattens a nested
// choice, keeps the order of the members, and drops a member that is the same
// as one before it.
func TestChoiceFlattensAndDedupes(t *testing.T) {
	t.Parallel()
	p := NewRulePool()
	a, b, c := p.String(p.Intern("a")), p.String(p.Intern("b")), p.String(p.Intern("c"))
	a2 := p.String(p.Intern("a"))
	inner := p.Choice([]RuleID{b, c})
	outer := p.Choice([]RuleID{a, inner, a2})
	got := p.ChildSlice(p.Node(outer).Children)
	if want := []RuleID{a, b, c}; !slices.Equal(got, want) {
		t.Errorf("expected %v, got: %v", want, got)
	}
}

// TestTokenSet makes sure of the order in which a set gives its symbols, and
// that Remove shortens the bits.
func TestTokenSet(t *testing.T) {
	t.Parallel()
	var s TokenSet
	s.Insert(SymbolEndValue)
	s.Insert(ExternalSymbol(1))
	s.Insert(TerminalSymbol(70))
	s.Insert(TerminalSymbol(3))
	var got []Symbol
	for sym := range s.All() {
		got = append(got, sym)
	}
	want := []Symbol{TerminalSymbol(3), TerminalSymbol(70), ExternalSymbol(1), SymbolEndValue}
	if !slices.Equal(got, want) {
		t.Errorf("expected %v, got: %v", want, got)
	}
	if s.Len() != 4 || !s.Contains(TerminalSymbol(70)) || s.Contains(TerminalSymbol(4)) {
		t.Errorf("expected four symbols with terminal 70 and without terminal 4, got: %v", got)
	}
	if !s.Remove(TerminalSymbol(70)) || s.terminalBits.Len() != 4 {
		t.Errorf("expected Remove to shorten the bits to 4, got: %d", s.terminalBits.Len())
	}
}

// TestBitVecCompare makes sure that the vector with the lowest differing bit
// set is the greater, as the Ord of upstream says, and that equality ignores
// zero words at the end.
func TestBitVecCompare(t *testing.T) {
	t.Parallel()
	var a, b BitVec
	a.Resize(130, false)
	b.Resize(10, false)
	a.Set(5, true)
	b.Set(5, true)
	if !a.Equal(&b) || a.Compare(&b) != 0 {
		t.Errorf("expected equal vectors that differ only in zero words")
	}
	b.Set(2, true)
	a.Set(129, true)
	if a.Compare(&b) != -1 || b.Compare(&a) != 1 {
		t.Errorf("expected b, with bit 2, to be the greater")
	}
	if !a.InsertAll(&b) || a.InsertAll(&b) {
		t.Errorf("expected the first InsertAll to add a bit, and the second none")
	}
}
