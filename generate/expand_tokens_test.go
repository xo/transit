package generate

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"unicode/utf8"
)

// tokenMatch is the token that simulateNfa matches: the index of the
// variable and the text of the token. It is Some((id, text)) in upstream,
// and a nil *tokenMatch is None.
type tokenMatch struct {
	id   int
	text string
}

// matched returns the tokenMatch of the variable id and the text.
func matched(id int, text string) *tokenMatch {
	return &tokenMatch{id: id, text: text}
}

// String returns the text of the match, for the message of a failed test.
func (m *tokenMatch) String() string {
	if m == nil {
		return "None"
	}
	return fmt.Sprintf("Some((%d, %q))", m.id, m.text)
}

// nfaExample is an input of check and the match that it expects.
type nfaExample struct {
	input    string
	expected *tokenMatch
}

// simulateNfa is simulate_nfa in expand_tokens.rs.
func simulateNfa(grammar *LexicalGrammar, s string) *tokenMatch {
	startStates := make([]uint32, 0, len(grammar.Variables))
	for _, v := range grammar.Variables {
		startStates = append(startStates, v.StartState)
	}
	cursor := NewNfaCursor(&grammar.Nfa, startStates)

	var result *tokenMatch
	resultPrecedence := int32(math.MinInt32)
	startChar := 0
	endChar := 0
	for _, c := range s {
		for id, precedence := range cursor.Completions() {
			if result == nil || resultPrecedence <= precedence {
				result = matched(id, s[startChar:endChar])
				resultPrecedence = precedence
			}
		}
		var next *NfaTransition
		for _, tr := range cursor.Transitions() {
			if tr.Characters.Contains(c) && tr.Precedence >= resultPrecedence {
				next = &tr
				break
			}
		}
		if next == nil {
			break
		}
		cursor.Reset(next.States)
		endChar += utf8.RuneLen(c)
		if next.IsSeparator {
			startChar = endChar
		}
	}

	for id, precedence := range cursor.Completions() {
		if result == nil || resultPrecedence <= precedence {
			result = matched(id, s[startChar:endChar])
			resultPrecedence = precedence
		}
	}

	return result
}

// check builds the lexical grammar of the tokens that build returns, with
// the separators that it returns, and runs simulateNfa on each example.
//
// check is check in expand_tokens.rs.
func check(t *testing.T, build func(p *RulePool) (roots, separators []RuleID), examples []nfaExample) {
	t.Helper()
	pool := NewRulePool()
	roots, separators := build(pool)
	vars := make([]LexicalToken, 0, len(roots))
	for i, root := range roots {
		vars = append(vars, LexicalToken{
			Name: pool.Intern(fmt.Sprintf("tok%d", i)),
			Kind: VariableAnonymous,
			Root: root,
		})
	}
	grammar, err := expandTokens(pool, vars, separators)
	if err != nil {
		t.Fatalf("expandTokens: %v", err)
	}
	for _, example := range examples {
		actual := simulateNfa(grammar, example.input)
		if actual.String() != example.expected.String() {
			t.Errorf("input %s: expected %v, actual %v", example.input, example.expected, actual)
		}
	}
}

// TestRuleExpansion is test_rule_expansion in expand_tokens.rs.
func TestRuleExpansion(t *testing.T) {
	t.Parallel()
	// This is a regex with sequences and alternatives.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, "(a|b|c)d(e|f|g)h?")}, nil
	}, []nfaExample{
		{"ade1", matched(0, "ade")},
		{"bdf1", matched(0, "bdf")},
		{"bdfh1", matched(0, "bdfh")},
		{"ad1", nil},
	})
	// This is a regex with repeats.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, "a*")}, nil
	}, []nfaExample{
		{"aaa1", matched(0, "aaa")},
		{"b", matched(0, "")},
	})
	// This is a regex with repeats in sequences.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, "a((bc)+|(de)*)f")}, nil
	}, []nfaExample{
		{"af1", matched(0, "af")},
		{"adedef1", matched(0, "adedef")},
		{"abcbcbcf1", matched(0, "abcbcbcf")},
		{"a", nil},
	})
	// This is a regex with ranges of characters.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, "[a-fA-F0-9]+")}, nil
	}, []nfaExample{
		{"A1ff0.", matched(0, "A1ff0")},
	})
	// This is a regex with the character classes of Perl.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, `\w\d\s`)}, nil
	}, []nfaExample{
		{"_0  ", matched(0, "_0 ")},
	})
	// This is a string.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{str(p, "abc")}, nil
	}, []nfaExample{
		{"abcd", matched(0, "abc")},
		{"ab", nil},
	})
	// This is a complex rule that holds strings and regexes.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		reg, empty := p.Intern("[a-f]+"), p.Intern("")
		lb, pattern, rb := p.Intern("{"), p.Pattern(reg, empty), p.Intern("}")
		left, right := p.String(lb), p.String(rb)
		seq := p.Seq([]RuleID{left, pattern, right})
		return []RuleID{p.Repeat(seq)}, nil
	}, []nfaExample{
		{"{a}{", matched(0, "{a}")},
		{"{a}{d", matched(0, "{a}")},
		{"ab", nil},
	})
	// This is the rule of the longest match.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		empty := p.Intern("")
		x, y, z := p.Intern("a|bc"), p.Intern("aa"), p.Intern("bcd")
		return []RuleID{
			p.Pattern(x, empty),
			p.Pattern(y, empty),
			p.Pattern(z, empty),
		}, nil
	}, []nfaExample{
		{"a.", matched(0, "a")},
		{"bc.", matched(0, "bc")},
		{"aa.", matched(1, "aa")},
		{"bcd?", matched(2, "bcd")},
		{"b.", nil},
		{"c.", nil},
	})
	// This is a regex with an alternative that holds the empty string.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, "a(b|)+c")}, nil
	}, []nfaExample{
		{"ac.", matched(0, "ac")},
		{"abc.", matched(0, "abc")},
		{"abbc.", matched(0, "abbc")},
	})
	// These are separators.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern("[a-f]+"), p.Intern("")
		token := p.Pattern(v, f)
		escapedNewline := p.String(p.Intern("\\\n"))
		whitespace := p.Pattern(p.Intern(`\s`), f)
		return []RuleID{token}, []RuleID{escapedNewline, whitespace}
	}, []nfaExample{
		{"  a", matched(0, "a")},
		{"  \nb", matched(0, "b")},
		{`  \a`, nil},
		{"  \\\na", matched(0, "a")},
	})
	// These are shorter tokens with a higher precedence.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		f := p.Intern("")
		v1, v2, v3 := p.Intern("abc"), p.Intern("ab[cd]e"), p.Intern("[a-e]+")
		pat1, pat2, pat3 := p.Pattern(v1, f), p.Pattern(v2, f), p.Pattern(v3, f)
		return []RuleID{
			p.Prec(precInt(2), pat1),
			p.Prec(precInt(1), pat2),
			pat3,
		}, nil
	}, []nfaExample{
		{"abceef", matched(0, "abc")},
		{"abdeef", matched(1, "abde")},
		{"aeeeef", matched(2, "aeeee")},
	})
	// These are immediate tokens with a higher precedence.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		f := p.Intern("")
		v1, v2 := p.Intern("[^a]+"), p.Intern("[^ab]+")
		pat1, pat2 := p.Pattern(v1, f), p.Pattern(v2, f)
		r1 := p.Prec(precInt(1), pat1)
		r2 := p.ImmediateToken(p.Prec(precInt(2), pat2))
		sep := p.Pattern(p.Intern(`\s`), f)
		return []RuleID{r1, r2}, []RuleID{sep}
	}, []nfaExample{
		{"cccb", matched(1, "ccc")},
	})
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		a, b, c, d := p.Intern("a"), p.Intern("b"), p.Intern("c"), p.Intern("d")
		r1 := p.String(a)
		inner1, inner2 := p.String(b), p.String(c)
		r2 := p.Choice([]RuleID{inner1, inner2})
		r3 := p.String(d)
		return []RuleID{p.Seq([]RuleID{r1, r2, r3})}, nil
	}, []nfaExample{
		{"abd", matched(0, "abd")},
		{"acd", matched(0, "acd")},
		{"abc", nil},
		{"ad", nil},
		{"d", nil},
		{"a", nil},
	})
	// These are choices in choices in sequences.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		r1 := pat(p, "[0-9]+")
		blank := p.Blank()
		e1, e2 := str(p, "e"), str(p, "E")
		ch1 := p.Choice([]RuleID{e1, e2})
		innerBlank := p.Blank()
		plus, minus := str(p, "+"), str(p, "-")
		innerCh := p.Choice([]RuleID{plus, minus})
		ch2 := p.Choice([]RuleID{innerBlank, innerCh})
		exponent := pat(p, "[0-9]+")
		sq := p.Seq([]RuleID{ch1, ch2, exponent})
		r2 := p.Choice([]RuleID{blank, p.Choice([]RuleID{sq})})
		return []RuleID{p.Seq([]RuleID{r1, r2})}, nil
	}, []nfaExample{
		{"12", matched(0, "12")},
		{"12e", matched(0, "12")},
		{"12g", matched(0, "12")},
		{"12e3", matched(0, "12e3")},
		{"12e+", matched(0, "12")},
		{"12E+34 +", matched(0, "12E+34")},
		{"12e34", matched(0, "12e34")},
	})
	// These are groups in groups.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{p.Seq([]RuleID{pat(p, `([^x\\]|\\(.|\n))+`)})}, nil
	}, []nfaExample{
		{"abcx", matched(0, "abc")},
		{`abc\0x`, matched(0, `abc\0`)},
	})
	// The regex can hold escape sequences that it does not know.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		f := p.Intern("")
		// This is an escaped forward slash. JavaScript uses it, because '/'
		// ends a regex there.
		v1 := p.Intern(`\/`)
		// These are escaped quotes.
		v2 := p.Intern(`\"\'`)
		// This is a quote after a literal backslash.
		v3 := p.Intern(`[\\']+`)
		return []RuleID{p.Pattern(v1, f), p.Pattern(v2, f), p.Pattern(v3, f)}, nil
	}, []nfaExample{
		{"/", matched(0, "/")},
		{`"'`, matched(1, `"'`)},
		{`'\'a`, matched(2, `'\'`)},
	})
	// These are escapes of Unicode properties.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		f := p.Intern("")
		v1 := p.Intern(`\p{L}+\P{L}+`)
		v2 := p.Intern(`\p{White_Space}+\P{White_Space}+[\p{White_Space}]*`)
		return []RuleID{p.Pattern(v1, f), p.Pattern(v2, f)}, nil
	}, []nfaExample{
		{"  123   abc", matched(1, "  123   ")},
		{"ბΨƁ___ƀƔ", matched(0, "ბΨƁ___")},
	})
	// These are escapes of Unicode properties in bracketed sets.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, `[\p{L}\p{Nd}]+`)}, nil
	}, []nfaExample{
		{"abΨ12٣٣, ok", matched(0, "abΨ12٣٣")},
	})
	// These are escapes of Unicode characters.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		f := p.Intern("")
		v1 := p.Intern(`\u{00dc}`)
		v2 := p.Intern(`\U{000000dd}`)
		v3 := p.Intern(`Þ`)
		v4 := p.Intern(`\U000000df`)
		return []RuleID{
			p.Pattern(v1, f),
			p.Pattern(v2, f),
			p.Pattern(v3, f),
			p.Pattern(v4, f),
		}, nil
	}, []nfaExample{
		{"Ü", matched(0, "Ü")},
		{"Ý", matched(1, "Ý")},
		{"Þ", matched(2, "Þ")},
		{"ß", matched(3, "ß")},
	})
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		f := p.Intern("")
		v1 := p.Intern(`u\{[0-9a-fA-F]+\}`)
		// These curly braces are escaped already.
		v2 := p.Intern(`\{[ab]{3}\}`)
		// This is a Unicode code point.
		v3 := p.Intern(`\u{1000A}`)
		// This is a Unicode code point in lowercase.
		v4 := p.Intern(`\u{1000b}`)
		return []RuleID{
			p.Pattern(v1, f),
			p.Pattern(v2, f),
			p.Pattern(v3, f),
			p.Pattern(v4, f),
		}, nil
	}, []nfaExample{
		{"u{1234} ok", matched(0, "u{1234}")},
		{"{aba}}", matched(1, "{aba}")},
		{"\U0001000A", matched(2, "\U0001000A")},
		{"\U0001000b", matched(3, "\U0001000b")},
	})
	// A pattern without case must not fold in the two code points outside
	// ASCII that the simple case folding of Unicode maps onto ASCII letters.
	// These are `ſ` (U+017F), which folds onto `s`, and the Kelvin sign `K`
	// (U+212A), which folds onto `k`.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern("[sk]+"), p.Intern("i")
		return []RuleID{p.Pattern(v, f)}, nil
	}, []nfaExample{
		{"sSkK.", matched(0, "sSkK")},
		// `s` does not match the long s.
		{"ſ", nil},
		// `k` does not match the Kelvin sign.
		{"K", nil},
		// The code point that folds ends the token.
		{"skK", matched(0, "sk")},
	})
	// A broad class with `/i`, such as a negated class or `\p{L}`, keeps `ſ`
	// and `K`. Folding does not add them here, because the class holds them
	// already. Thus there is nothing to drop.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern(`[^"]`), p.Intern("i")
		return []RuleID{p.Pattern(v, f)}, nil
	}, []nfaExample{
		// The long s stays under /i.
		{"ſ", matched(0, "ſ")},
		// The Kelvin sign stays under /i.
		{"K", matched(0, "K")},
		{"s", matched(0, "s")},
	})
	// A `ſ` or a `K` that the grammar writes stays. The code above drops them
	// only when the ASCII letter that they fold with is in the class too.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern("[ſK]+"), p.Intern("")
		return []RuleID{p.Pattern(v, f)}, nil
	}, []nfaExample{
		{"ſK.", matched(0, "ſK")},
	})
	// Without the flag `i`, nothing folds. Thus a broad class such as `[^"]`
	// must keep `ſ` and `K`, and must not drop them as the result of a fold.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern(`[^"]`), p.Intern("")
		return []RuleID{p.Pattern(v, f)}, nil
	}, []nfaExample{
		{"a", matched(0, "a")},
		// This is the long s.
		{"ſ", matched(0, "ſ")},
		// This is the Kelvin sign.
		{"K", matched(0, "K")},
		// This is the one character that the class does not hold.
		{`"`, nil},
	})
	// `ſ` and `K` fold with nothing. Thus an explicit one adds no ASCII
	// letter.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern("[ſK]+"), p.Intern("i")
		return []RuleID{p.Pattern(v, f)}, nil
	}, []nfaExample{
		{"ſK.", matched(0, "ſK")},
		{"s", nil},
		{"k", nil},
	})
	// `\p{L}` holds `ſ` and `K` already, so folding adds nothing to drop.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern(`\p{L}+`), p.Intern("i")
		return []RuleID{p.Pattern(v, f)}, nil
	}, []nfaExample{
		{"ſ", matched(0, "ſ")},
		{"K", matched(0, "K")},
		{"aA", matched(0, "aA")},
		{"1", nil},
	})
	// Folding happens at the leaves, before the negation. Under `/i`,
	// `[^a-z]` must leave out `A-Z`, and must not add it again.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern("[^a-z]"), p.Intern("i")
		return []RuleID{p.Pattern(v, f)}, nil
	}, []nfaExample{
		{"!", matched(0, "!")},
		{"a", nil},
		{"A", nil},
		{"ſ", matched(0, "ſ")},
		{"K", matched(0, "K")},
	})
	// The order is the same in a nested class and in the operations on sets
	// of a class.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern("[^[a-c]]"), p.Intern("i")
		return []RuleID{p.Pattern(v, f)}, nil
	}, []nfaExample{
		{"d", matched(0, "d")},
		{"a", nil},
		{"A", nil},
		{"C", nil},
	})
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern("[[a-z]--[b-d]]"), p.Intern("i")
		return []RuleID{p.Pattern(v, f)}, nil
	}, []nfaExample{
		{"a", matched(0, "a")},
		{"A", matched(0, "A")},
		{"b", nil},
		{"B", nil},
	})
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern("[[a-z]&&[b-d]]"), p.Intern("i")
		return []RuleID{p.Pattern(v, f)}, nil
	}, []nfaExample{
		{"b", matched(0, "b")},
		{"B", matched(0, "B")},
		{"e", nil},
		{"E", nil},
	})
	// A scoped flag applies only where it is in effect.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern("(?i)a(?-i)b"), p.Intern("")
		return []RuleID{p.Pattern(v, f)}, nil
	}, []nfaExample{
		{"ab", matched(0, "ab")},
		{"Ab", matched(0, "Ab")},
		{"aB", nil},
	})
	// The flag `i` of one token does not apply to the next token.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v1, f1 := p.Intern("ab"), p.Intern("i")
		v2, f2 := p.Intern("cd"), p.Intern("")
		s := p.Intern("ef")
		return []RuleID{p.Pattern(v1, f1), p.Pattern(v2, f2), p.String(s)}, nil
	}, []nfaExample{
		{"AB", matched(0, "AB")},
		{"cd", matched(1, "cd")},
		{"CD", nil},
		{"ef", matched(2, "ef")},
		{"EF", nil},
	})
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		v, f := p.Intern(`\$\{[a-z0-9_\.]*[^a-z0-9_\.\}]`), p.Intern("i")
		return []RuleID{p.Pattern(v, f)}, nil
	}, []nfaExample{
		{"${a}", nil},
		{"${A}", nil},
		{"${a!", matched(0, "${a!")},
		{"${!", matched(0, "${!")},
	})
	// `(?-u:...)` matches bytes. A byte below `0x80` is its own code point,
	// so a class of ASCII bytes converts exactly.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, `(?-u:[a-z])+`)}, nil
	}, []nfaExample{
		{"abc.", matched(0, "abc")},
		{"ABC", nil},
	})
	// The folding of bytes is ASCII only, so it cannot add `K`.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, `(?-u:(?i)k)+`)}, nil
	}, []nfaExample{
		{"kK.", matched(0, "kK")},
		{"K", nil},
	})
	// These are emojis.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, `\p{Emoji}+`)}, nil
	}, []nfaExample{
		{"🐎", matched(0, "🐎")},
		{"🐴🐴", matched(0, "🐴🐴")},
		// Unicode names these characters emojis too.
		{"#0", matched(0, "#0")},
		{"\u2ee2", nil},
		{"♞", nil},
		{"horse", nil},
	})
	// This is an intersection.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, `[[0-7]&&[4-9]]+`)}, nil
	}, []nfaExample{
		{"456", matched(0, "456")},
		{"64", matched(0, "64")},
		{"452", matched(0, "45")},
		{"91", nil},
		{"8", nil},
		{"3", nil},
	})
	// This is a difference.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, `[[0-9]--[4-7]]+`)}, nil
	}, []nfaExample{
		{"123", matched(0, "123")},
		{"83", matched(0, "83")},
		{"9", matched(0, "9")},
		{"124", matched(0, "12")},
		{"67", nil},
		{"4", nil},
	})
	// This is a symmetric difference.
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, `[[0-7]~~[4-9]]+`)}, nil
	}, []nfaExample{
		{"123", matched(0, "123")},
		{"83", matched(0, "83")},
		{"9", matched(0, "9")},
		{"124", matched(0, "12")},
		{"67", nil},
		{"4", nil},
	})
	// These are set operations in set operations. The table shows the digits
	// that each part matches:
	//
	//	               0 1 2 3 4 5 6 7 8 9
	//	[0-5]:         0 1 2 3 4 5
	//	[2-4]:             2 3 4
	//	[0-5]--[2-4]:  0 1       5
	//	[3-9]:               3 4 5 6 7 8 9
	//	[6-7]:                     6 7
	//	[3-9]--[6-7]:        3 4 5     8 9
	//	final regex:   0 1   3 4       8 9
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		return []RuleID{pat(p, `[[[0-5]--[2-4]]~~[[3-9]--[6-7]]]+`)}, nil
	}, []nfaExample{
		{"01", matched(0, "01")},
		{"432", matched(0, "43")},
		{"8", matched(0, "8")},
		{"9", matched(0, "9")},
		{"2", nil},
		{"567", nil},
	})
}

// TestNonASCIIByteClassIsRejected is test_non_ascii_byte_class_is_rejected
// in expand_tokens.rs.
func TestNonASCIIByteClassIsRejected(t *testing.T) {
	t.Parallel()
	pool := NewRulePool()
	root := pat(pool, `(?-u:[\xc3\xa9])`)
	vars := []LexicalToken{{
		Name: pool.Intern("tok"),
		Kind: VariableAnonymous,
		Root: root,
	}}
	_, err := expandTokens(pool, vars, nil)
	tokensErr, ok := errors.AsType[*ExpandTokensError](err)
	if !ok {
		t.Fatalf("expected an *ExpandTokensError, actual %v", err)
	}
	if tokensErr.Kind != ExpandTokensProcessing || tokensErr.Rule != "tok" {
		t.Errorf("expected ExpandTokensProcessing of tok, actual kind %d of %s", tokensErr.Kind, tokensErr.Rule)
	}
	regexErr, ok := errors.AsType[*ExpandRegexError](tokensErr.Err)
	if !ok {
		t.Fatalf("expected an *ExpandRegexError, actual %v", tokensErr.Err)
	}
	expected := ExpandRegexError{Kind: ExpandRegexNonASCIIByteClass, Start: 0xa9, End: 0xa9}
	if *regexErr != expected {
		t.Errorf("expected %+v, actual %+v", expected, *regexErr)
	}
}

// TestRepeatOfEmptyChoiceDoesNotLeaveAnAcceptState is
// test_repeat_of_empty_choice_does_not_leave_an_accept_state in
// expand_tokens.rs.
func TestRepeatOfEmptyChoiceDoesNotLeaveAnAcceptState(t *testing.T) {
	t.Parallel()
	check(t, func(p *RulePool) ([]RuleID, []RuleID) {
		tokenA := str(p, "a")
		left, right := p.Blank(), p.Blank()
		emptyChoice := p.Choice([]RuleID{left, right})
		repeatedEmpty := p.Repeat(emptyChoice)
		tokenBSuffix := str(p, "b")
		tokenB := p.Seq([]RuleID{repeatedEmpty, tokenBSuffix})
		return []RuleID{tokenA, tokenB}, nil
	}, []nfaExample{
		{"b", matched(1, "b")},
	})
}
