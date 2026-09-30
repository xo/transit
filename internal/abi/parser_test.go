package abi

import (
	"fmt"
	"slices"
	"testing"
)

// recorder is a LexerFuncs that records each call.
type recorder struct {
	calls []string
}

func (r *recorder) Advance(skip bool) { r.calls = append(r.calls, fmt.Sprintf("advance %t", skip)) }

func (r *recorder) MarkEnd() { r.calls = append(r.calls, "mark_end") }

func (r *recorder) GetColumn() uint32 {
	r.calls = append(r.calls, "get_column")
	return 7
}

func (r *recorder) IsAtIncludedRangeStart() bool {
	r.calls = append(r.calls, "is_at_included_range_start")
	return true
}

func (r *recorder) EOF() bool {
	r.calls = append(r.calls, "eof")
	return true
}

func (r *recorder) Logf(format string, args ...any) {
	r.calls = append(r.calls, "log "+fmt.Sprintf(format, args...))
}

func TestLexerCallsItsFuncs(t *testing.T) {
	r := &recorder{}
	l := &Lexer{Funcs: r}
	l.Advance(true)
	l.MarkEnd()
	if got := l.GetColumn(); got != 7 {
		t.Errorf("GetColumn() = %d, want 7", got)
	}
	if !l.IsAtIncludedRangeStart() {
		t.Error("IsAtIncludedRangeStart() = false")
	}
	if !l.EOF() {
		t.Error("EOF() = false")
	}
	l.Logf("%s %d", "x", 1)
	want := []string{"advance true", "mark_end", "get_column", "is_at_included_range_start", "eof", "log x 1"}
	if !slices.Equal(r.calls, want) {
		t.Errorf("the calls are %q, want %q", r.calls, want)
	}
}

func TestParseActionTypeString(t *testing.T) {
	for typ, want := range map[ParseActionType]string{
		ParseActionTypeShift:   "shift",
		ParseActionTypeReduce:  "reduce",
		ParseActionTypeAccept:  "accept",
		ParseActionTypeRecover: "recover",
		9:                      "unknown",
	} {
		if got := typ.String(); got != want {
			t.Errorf("ParseActionType(%d).String() = %q, want %q", typ, got, want)
		}
	}
}

// textLexer is a LexerFuncs over a text of ASCII characters that records
// each call, with the position of the lexer.
type textLexer struct {
	lexer *Lexer
	text  string
	pos   int
	calls []string
}

// newTextLexer returns a lexer at the start of text.
func newTextLexer(text string) *textLexer {
	tl := &textLexer{text: text}
	tl.lexer = &Lexer{Funcs: tl}
	tl.setLookahead()
	return tl
}

func (tl *textLexer) setLookahead() {
	tl.lexer.Lookahead = 0
	if tl.pos < len(tl.text) {
		tl.lexer.Lookahead = int32(tl.text[tl.pos])
	}
}

func (tl *textLexer) Advance(skip bool) {
	tl.calls = append(tl.calls, fmt.Sprintf("advance %t", skip))
	if tl.pos < len(tl.text) {
		tl.pos++
	}
	tl.setLookahead()
}

func (tl *textLexer) MarkEnd() { tl.calls = append(tl.calls, fmt.Sprintf("mark_end %d", tl.pos)) }

func (tl *textLexer) GetColumn() uint32 { return 0 }

func (tl *textLexer) IsAtIncludedRangeStart() bool { return false }

func (tl *textLexer) EOF() bool {
	tl.calls = append(tl.calls, "eof")
	return tl.pos >= len(tl.text)
}

func (tl *textLexer) Logf(string, ...any) {}

// wordTable is a lex table of words of the letters a to z, symbol 1, with
// the spaces between them skipped, and the end of the input, symbol 0.
var wordTable = LexTable{
	States: []LexState{
		{Start: 0, Count: 3, EOFState: 2, HasEOF: true},
		{Start: 3, Count: 1, Accept: 1, HasAccept: true},
		{HasAccept: true},
	},
	Ranges: []LexRange{
		{Lo: '\t', Hi: '\n', State: 0, Skip: true},
		{Lo: ' ', Hi: ' ', State: 0, Skip: true},
		{Lo: 'a', Hi: 'z', State: 1},
		{Lo: 'a', Hi: 'z', State: 1},
	},
}

func TestLexTableLex(t *testing.T) {
	for _, c := range []struct {
		text   string
		state  uint16
		found  bool
		symbol uint16
		calls  []string
	}{
		{" ab c", 0, true, 1, []string{
			"eof", "advance true",
			"eof", "advance false",
			"eof", "mark_end 2", "advance false",
			"eof", "mark_end 3",
		}},
		{"\n", 0, true, 0, []string{
			"eof", "advance true",
			"eof", "advance false",
			"eof", "mark_end 1",
		}},
		{"!", 0, false, 0, []string{"eof"}},
		{"a", 7, false, 0, []string{"eof"}},
	} {
		tl := newTextLexer(c.text)
		found := wordTable.Lex(tl.lexer, c.state)
		if found != c.found || tl.lexer.ResultSymbol != c.symbol || !slices.Equal(tl.calls, c.calls) {
			t.Errorf("%q in state %d: found %t, symbol %d, calls %q; want %t, %d, %q",
				c.text, c.state, found, tl.lexer.ResultSymbol, tl.calls, c.found, c.symbol, c.calls)
		}
	}
}
