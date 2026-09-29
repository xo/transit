package cgrammar

import "testing"

// This file ports crates/cli/src/tests/pathological_test.rs of upstream
// (D35). allocations::record is dropped, because Go has no allocator to
// count.

func TestPathologicalExample1(t *testing.T) {
	language := "cpp"
	source := `*ss<s"ss<sqXqss<s._<s<sq<(qqX<sqss<s.ss<sqsssq<(qss<qssqXqss<s._<s<sq<(qqX<sqss<s.ss<sqsssq<(qss<sqss<sqss<s._<s<sq>(qqX<sqss<s.ss<sqsssq<(qss<sq&=ss<s<sqss<s._<s<sq<(qqX<sqss<s.ss<sqs`

	parser := urParser(t, fixtureGrammar(t, language).Language)
	urParse(t, parser, source, nil)
}
