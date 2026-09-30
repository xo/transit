package cgrammar

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/xo/transit/grammars/ruby"
	"github.com/xo/transit/internal/abi"
)

func init() {
	goPackages = append(goPackages, goPackage{"ruby", ruby.Language})
}

// rubyScannerInputs are inputs that reach the branches of the scanner of
// ruby that the corpus reaches seldom or never.
//
//nolint:dupword // a heredoc repeats its word
var rubyScannerInputs = []string{
	// scan_whitespace
	"a\r\nb", "a\n\nb", "a \\\nb", "a \\\r\nb", "a \\x", "a\\", "a\n.b", "a\n..b", "a\n...b",
	"a\n&.b", "a\n# c\nb", "a\n.", "a\n&", "a\n", "\t a \t\n",
	// scan_operator and scan_symbol_identifier
	":<", ":<=", ":<<", ":<=>", ":>", ":>=", ":>>", ":==", ":===", ":=~", ":=", ":+", ":-",
	":~", ":+@", ":-@", ":~@", ":..", ":.", ":&", ":^", ":|", ":/", ":%", ":`", ":!", ":!=",
	":!~", ":*", ":**", ":[]", ":[]=", ":[x", ":?", ":@a", ":@@a", ":$a", ":$0", ":a?", ":a!",
	":a=", "{:a=>1}", ":foo= 1", ":\"a\"", ":'a'", ":\"a#{b}\"", "::A", "a ? b :c", "a ?b : c",
	// The low byte of U+013A is ':', which is_iden_char sees.
	":", ":\u013a", ":é",
	// scan_open_delimiter
	"\"s\"", "'s'", "`x`", "/re/", "a / b", "a /b/", "a /=b", "a / =b", "a /\tb/", "foo /x/",
	"%s(x)", "%r{x}", "%x[x]", "%q<x>", "%Q|x|", "%w(a b)", "%i[a b]", "%W(a #{b})", "%I(a b)",
	"%(x)", "%[x]", "%{x}", "%<x>", "a % b", "a %b", "puts % x ", "puts %\nx\n", "x = %\rx\r",
	"%|x|", "%!x!", "%#x#", "%/x/", "%\\x\\", "%@x@", "%$x$", "%%x%", "%^x^", "%&x&", "%*x*",
	"%)x)", "%]x]", "%}x}", "%>x>", "%+x+", "%-x-", "%~x~", "%`x`", "%,x,", "%.x.", "%?x?",
	"%:x:", "%;x;", "%_x_", "%\"x\"", "%'x'", "%=x=", "%a", "%", "a %= 1", "%w", "%s",
	// scan_heredoc_word and scan_heredoc_content
	"<<A\nx\nA\n", "<<~A\n  x\n  A\n", "<<-A\n  x\n  A\n", "<<A\n  A\nA\n", "<<'A'\n#{x}\\n\nA\n",
	"<<\"A\"\n#{x}\nA\n", "<<`A`\nls\nA\n", "<<'A\n", "<<A\r\nx\r\nA\r\n", "<<A\rx\rA\r",
	"<<A\nx", "<<A\n", "<<A", "<<>", "a <<b", "a << b", "<a", "<<AB\nA\nAB\n", "<<A\nA  \n",
	"<<A\nA \t\nx\n", "<<A\nx\\n\nA\n", "<<A\n\\\nA\n", "<<A\nx#{y}z\nA\n", "<<A\n#{y}\nA\n",
	"<<A\n#@a\nA\n", "<<A\nx#@a\nA\n", "<<A\n#@@a\n#@1\n#$a\n#$-w\n#$-1\n#$!\n#$1\n#x\nA\n",
	"<<A\n#$", "foo(<<A, <<B)\na\nA\nb\nB\n", "foo(<<A, <<~B, <<-'C')\na\nA\n  b\n  B\n c\n C\n",
	"<<A.b\nx\nA\n", "x = <<A + y\nx\nA\n", "<<ÄÖ\nx\nÄÖ\n", "<<'é'\nx\né\n", "<<'\xff'\n\xff\nx\n",
	"<<A\n\t  A\nA\n", "<<-A\n\t  A\n", "<<~A\n\r\n  A\n",
	// scan_short_interpolation and scan_literal_content. The low byte of
	// U+0100 is 0, which strchr finds, and the low byte of U+0140 is '@'.
	"\"#@a\"", "\"#@@a\"", "\"#@1\"", "\"#@\"", "\"#$a\"", "\"#$-w\"", "\"#$-1\"", "\"#$!\"",
	"\"#$1\"", "\"#$\"", "\"#$", "\"#x\"", "\"#\"", "\"x#@a\"", "\"x#$a\"", "\"#$\u0100\"",
	"\"#\u0140a\"", "\"#\u0140\"", "\"a\\nb\"", "'a\\'b'", "'a\\\\'", "\"a#{b}c\"", "\"#{}\"",
	"\"abc", "'abc", "%(a(b)c)", "%[a[b]c]", "%w(a  b\tc\nd)", "%i[a  b]", "%W(a#{b} c)",
	"/a/imx", "/a/iX", "%r(a(b))ix", "`a#{b}`", "\"\x00\"", "'\x00'", "\"\xff\"", "%w(\xff)",
	"%(" + strings.Repeat("(", 300) + strings.Repeat(")", 300) + ")",
	strings.Repeat("\"#{", 210) + strings.Repeat("}\"", 210),
	// scan
	"foo(&blk)", "foo &blk", "a && b", "a &. b", "a &= b", "a & b", "foo(&)", "foo & b",
	"class << self\nend", "class <<self\nend", "class < A\nend", "*a = b", "a * b", "a *b",
	"a*b", "**h", "a ** b", "a**b", "a **b", "a *= b", "a **= b", "foo(**h)", "foo(*)",
	"foo(**)", "foo *a", "foo **a", "def f(*, **); end", "-1", "a -1", "a - 1", "a-1", "-a",
	"a -b", "a -= 1", "->(x){}", "foo -1", "foo - 1", "a -\n1", "x = -1", "x = - 1", "a[1]",
	"a [1]", "[1]", "foo [1]", "a[1] = 2", "{a: 1}", "{a::b}", "foo a: 1", "a!", "foo!", "A!",
	"a != b", "a!= b", "Foo!", "{A: 1}", "{Foo::Bar => 1}", "a?b:c", "{_a: 1}", "_a!",
	"äö!", "{äö: 1}", "Äö!", "{Äö: 1}", "a\u00a0b", "a\u0085b", "%w(a\u00a0b)",
	"x = <<A\n", "", "\n", "\\", "#", "__END__\nx\n",
	// inputs that start with whitespace, for the direct calls of
	// TestGoPackageScannerCallsMatchCRuby
	" **a", " ** a", " *a", " * a", " -1", " -a", " - a", " [1]", " &a", " /a/", " %w(a)",
	" :a", " a: 1", " <<A\nA\n", "\n **a", "\n\n-1",
}

// TestGoPackageScannerMatchesCRuby compares the scanner of the package ruby
// with its C scanner on every corpus input, on the error corpus of upstream
// and on rubyScannerInputs.
func TestGoPackageScannerMatchesCRuby(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"ruby", ruby.Language})
	inputs := make([][]byte, 0, len(examples)+len(rubyScannerInputs))
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range rubyScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, ruby.Language(), inputs)
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// TestGoPackageScannerDeserializeRuby gives the Go scanner of ruby and its C
// scanner the same random states, and compares what Serialize writes after
// Deserialize. The C function reads past the end of a buffer that is too
// short, and it fails an assertion when the state ends before the buffer, so
// each state that both get has the form that serialize writes, with random
// bytes in each field. The Go scanner also gets random bytes of any form,
// which must not make it panic.
func TestGoPackageScannerDeserializeRuby(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"ruby", ruby.Language})
	goScanner := tablesOf(ruby.Language()).ExternalScanner.Create()
	cScanner := tablesOf(g.Language).ExternalScanner.Create()
	rng := rand.New(rand.NewPCG(1, 2))

	// The buffer is longer than the buffer of the runtime, so that a state
	// that C writes 1 byte past the end of the buffer for is compared too.
	serialize := func(s abi.Scanner, buf []byte) []byte {
		out := make([]byte, 2*abi.SerializationBufferSize)
		s.Deserialize(buf)
		return out[:s.Serialize(out)]
	}
	for i := range 20000 {
		var buf []byte
		literals := rng.IntN(8)
		heredocs := rng.IntN(5)
		wordMax := 40
		switch i % 100 {
		case 0:
			literals = 200 + rng.IntN(56)
		case 1:
			heredocs, wordMax = 5+rng.IntN(10), 255
		}
		buf = append(buf, byte(literals))
		for range literals * 5 {
			buf = append(buf, byte(rng.IntN(256)))
		}
		buf = append(buf, byte(heredocs))
		for range heredocs {
			buf = append(buf, byte(rng.IntN(256)), byte(rng.IntN(256)), byte(rng.IntN(256)))
			word := rng.IntN(wordMax + 1)
			buf = append(buf, byte(word))
			for range word {
				buf = append(buf, byte(rng.IntN(256)))
			}
		}
		want, got := serialize(cScanner, buf), serialize(goScanner, buf)
		if !bytes.Equal(want, got) {
			t.Fatalf("state %x: C serializes %x, Go %x", buf, want, got)
		}
	}

	// A state of four heredocs that fills the buffer: the test of room in
	// serialize passes for the last heredoc, and C writes 1025 bytes, 1 past
	// the end of the buffer of the runtime. The Go scanner writes no state
	// there (D81).
	full := []byte{0, 4}
	for _, word := range []int{255, 255, 255, 242} {
		full = append(full, 0, 0, 0, byte(word))
		full = append(full, bytes.Repeat([]byte{'A'}, word)...)
	}
	want, got := serialize(cScanner, full), serialize(goScanner, full)
	if len(want) != abi.SerializationBufferSize+1 {
		t.Errorf("C serializes %d bytes of a state that fills the buffer, want %d", len(want), abi.SerializationBufferSize+1)
	}
	if len(got) != 0 {
		t.Errorf("Go serializes %d bytes of a state that does not fit, want 0 (D81)", len(got))
	}

	// A state 1 byte shorter fits, and the two write the same 1024 bytes.
	fits := []byte{0, 4}
	for _, word := range []int{255, 255, 255, 241} {
		fits = append(fits, 0, 0, 0, byte(word))
		fits = append(fits, bytes.Repeat([]byte{'A'}, word)...)
	}
	want, got = serialize(cScanner, fits), serialize(goScanner, fits)
	if !bytes.Equal(want, got) || len(got) != abi.SerializationBufferSize {
		t.Errorf("state %x: C serializes %d bytes, Go %d, want %d from both", fits, len(want), len(got), abi.SerializationBufferSize)
	}

	for range 20000 {
		buf := make([]byte, rng.IntN(64))
		for j := range buf {
			buf[j] = byte(rng.IntN(256))
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("state %x: Deserialize and Serialize panic: %v", buf, r)
				}
			}()
			out := make([]byte, abi.SerializationBufferSize)
			goScanner.Deserialize(buf)
			goScanner.Serialize(out)
		}()
	}
}

// rubyScannerStates are serialized states of the scanner of ruby: none, a
// string, a word array at depth 2, a regex, two heredocs, and a string with
// a heredoc. Each heredoc has a word, because the C function
// scan_heredoc_content fails an assertion for an empty word.
var rubyScannerStates = [][]byte{
	nil,
	{1, 3, '(', ')', 1, 1, 0},
	{1, 7, '[', ']', 2, 0, 0},
	{1, 6, '/', '/', 1, 1, 0},
	{0, 1, 1, 1, 0, 1, 'A'},
	{0, 1, 0, 0, 0, 3, 'E', 'O', 'S'},
	{1, 3, '"', '"', 1, 1, 1, 0, 1, 1, 1, 'A'},
}

// TestGoPackageScannerCallsMatchCRuby calls the Go scanner of ruby and its C
// scanner directly, from the start of each input of rubyScannerInputs, in
// each state of rubyScannerStates, with valid symbols that the parse tables
// do not give: none, all, each token alone and random sets. It compares what
// each call finds, each call of the lexer and the state after the call.
func TestGoPackageScannerCallsMatchCRuby(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"ruby", ruby.Language})
	goScanner := tablesOf(ruby.Language()).ExternalScanner.Create()
	cScanner := tablesOf(g.Language).ExternalScanner.Create()
	count := int(tablesOf(ruby.Language()).ExternalTokenCount)
	rng := rand.New(rand.NewPCG(3, 4))

	valids := [][]bool{make([]bool, count), make([]bool, count)}
	for i := range valids[1] {
		valids[1][i] = true
	}
	for i := range count {
		v := make([]bool, count)
		v[i] = true
		valids = append(valids, v)
	}
	for range 40 {
		v := make([]bool, count)
		for i := range v {
			v[i] = rng.IntN(2) == 0
		}
		valids = append(valids, v)
	}

	call := func(s abi.Scanner, l *lexLog, state []byte, valid []bool) string {
		l.reset(0)
		s.Deserialize(state)
		found := s.Scan(&l.Lexer, valid)
		out := make([]byte, 2*abi.SerializationBufferSize)
		n := s.Serialize(out)
		return fmt.Sprintf("%v %d %v %x", found, l.ResultSymbol, l.log, out[:n])
	}
	runs := 0
	for _, input := range rubyScannerInputs {
		l := &lexLog{text: []byte(input)}
		l.Funcs = l
		for _, state := range rubyScannerStates {
			for _, valid := range valids {
				runs++
				want := call(cScanner, l, state, valid)
				got := call(goScanner, l, state, valid)
				if want != got {
					t.Fatalf("input %q, state %x, valid %v: C %s, Go %s", input, state, valid, want, got)
				}
			}
		}
	}
	t.Logf("%d calls", runs)
}
