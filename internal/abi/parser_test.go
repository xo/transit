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
