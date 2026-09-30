package cgrammar

import (
	"strings"
	"testing"

	"github.com/xo/transit/grammars/python"
)

func init() {
	goPackages = append(goPackages, goPackage{"python", python.Language})
}

// The external tokens of python, as the scanner numbers them.
const (
	pyNewline = iota
	pyIndent
	pyDedent
	pyStringStart
	pyStringContent
	pyEscapeInterpolation
	pyStringEnd
	pyComment
	pyCloseParen
	pyCloseBracket
	pyCloseBrace
	pyExcept
	pyTokenCount
)

// pythonScannerInputs are inputs that reach the branches of the scanner of
// python that the corpus does not reach, or reaches only a few times.
var pythonScannerInputs = []string{
	// the escapes of the braces of a format string
	"f'{{x}}'\n", "f\"}}{{\"\n", "f'{x}}'\n", "f'{'\n", "f'}x'\n", "f'''{{\n}}'''\n",
	// raw strings with escaped quotes, backslashes and line ends
	"r'\\''\n", "r\"\\\"\"\n", "r'\\\\'\n", "r'a\\\nb'\n", "r'a\\\r\nb'\n", "r'a\\\rb'\n", "rb'\\x'\n", "Rb'''\\'''''\n",
	// bytes strings with the escapes that bytes strings do not have
	"b'\\N{DASH}'\n", "b'\\u1234'\n", "b'\\U00012345'\n", "b'\\n'\n", "b'a\\x41'\n", "B\"\\\\\"\n",
	// escapes of other strings
	"'\\n'\n", "u'\\u00e9'\n", "U\"a\\\"b\"\n",
	// triple quoted strings with one and two quotes inside
	"'''a'b''c'''\n", "\"\"\"a\"\"\"\n", "\"\"\"\"\"\"\n", "'''a\n''\n'''\n", "\"\"\"a\"\" \"\"\"\n", "'''a''''\n",
	// a line end inside a string that is not triple quoted
	"'abc\ndef'\n", "'\n'\n", "x = 'abc\n",
	// backquotes of Python 2
	"`x`\n", "`a\\`b`\n",
	// prefixes of strings, and names that start with a prefix
	"rb''\nbr''\nRB''\nFr''\nfR''\nuR''\nub''\n", "f = 1\nfr = 2\nbu = 3\nu\n", "fb'x'\n", "f'''a{b}c'''\n",
	// nested format strings
	"f'{f\"{x}\"}'\n", "f'{a:{b}}'\n", "f'''\n{x}\n'''\n", "f'''\n  {x}\n  '''\nx\n",
	// indents with spaces, tabs, form feeds and carriage returns
	"if x:\n\ty\n\tz\n", "if x:\n  y\r\n  z\r\n", "if x:\n\f  y\n", "if x:\n  \f y\n", "if x:\r  y\r",
	"if a:\n  if b:\n    c\n  d\ne\n", "if a:\n        b\n\tc\n",
	// comments at each indent, and comments after an expression
	"if a:\n  b\n  # c\n# d\n  e\n", "if a:\n  b\n# c\n  # d\ne\n", "if a:\n    b # c\n    # d\n",
	"if a:\n  b\n    # c\nd\n", "# a\n# b\nc\n", "if a:\n  b\n#c", "if a:\n  b\n  #",
	// line continuations
	"x = 1 + \\\n  2\n", "x = 1 + \\\r\n  2\n", "x = \\ 2\n", "x = 1\\", "if a:\n  b\\\n\nc\n", "\\\n\\\nx\n",
	// the end of the input
	"if a:\n  b", "if a:\n  if b:\n    c", "if a:", "x = (", "x = [1,\n", "x = {1:\n", "",
	// an except that follows a comment
	"try:\n  a\n# b\nexcept:\n  c\n",
	// a dedent that waits for a string in brackets
	"x = [\n  'a'\n]\n", "if a:\n  x = (1,\n'a')\n",
	// an indent of more than 255 columns, which the state keeps in one byte
	"if a:\n" + strings.Repeat(" ", 300) + "b\n" + strings.Repeat(" ", 300) + "c\nd\n",
	// bytes that are not UTF-8, and characters that are not ASCII
	"'\xff'\n", "x = '\u00e9\U0001F600'\n", "f'\xc3{x}'\n", "\xff\n",
}

// pythonScannerTexts are the texts that TestPythonScannerScans scans from
// each position.
var pythonScannerTexts = []string{
	"f'{{a}}' r'\\'' b'\\N\\x' '''a''b'''",
	"\"\"\"\"\"\" 'a\nb' `c`",
	"\n  # a\n\t# b\n\\\n\\\r\n\\ x\r\f\n",
	"rb'' Fu'' ub'' fx",
	"'\\\r\n\\\n\\u\\\\'",
	"\xff\u00e9",
}

// pythonScannerStates are states of the scanner of python: no string, and a
// string of each kind that the scanner can be inside, with some indents.
var pythonScannerStates = [][]byte{
	nil,
	{0, 0},
	{0, 0, 2, 4},
	{1, 1, 0x12},
	{1, 1, 0x31},
	{0, 1, 0x09},
	{0, 1, 0x42},
	{0, 1, 0x62},
	{0, 1, 0x04},
	{0, 1, 0x1a, 4},
	{1, 2, 0x11, 0x32, 4, 8},
	{0, 1, 0},
}

// TestPythonScannerMatchesC parses each corpus input of python, the error
// corpus of upstream and the inputs of pythonScannerInputs with the Go
// runtime, once with the Go scanner and once with the C scanner, and
// compares each call of the two scanners.
func TestPythonScannerMatchesC(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"python", python.Language})
	var inputs [][]byte
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range pythonScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, python.Language(), inputs)
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// TestPythonScannerScans makes a Go scanner and a C scanner of python scan
// each text of pythonScannerTexts from each position, from each state of
// pythonScannerStates and with each set of valid symbols of the grammar,
// and compares the calls, the results and the states after.
func TestPythonScannerScans(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"python", python.Language})
	var texts [][]byte
	for _, s := range pythonScannerTexts {
		texts = append(texts, []byte(s))
	}
	scans := compareScannerScans(t, g, python.Language(), pythonScannerStates, texts, validSymbolSets(python.Language()))
	t.Logf("%d scans", scans)
}

// TestPythonScannerLimits makes a Go scanner and a C scanner of python
// enter 300 strings, which is more than the 255 delimiters that the state
// holds, and push 1100 indents, which is more than the state holds, and
// compares the calls and the states.
func TestPythonScannerLimits(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"python", python.Language})
	p := newScannerPair(g, python.Language())
	valid := make([]bool, pyTokenCount)
	valid[pyStringStart] = true
	for i := range 300 {
		p.scan([]byte("f'"), 0, valid)
		if i%50 == 0 {
			p.serialize()
		}
	}
	p.serialize()
	valid = make([]bool, pyTokenCount)
	valid[pyIndent] = true
	for i := 1; i <= 1100; i++ {
		p.scan([]byte("\n"+strings.Repeat(" ", i)+"x"), 0, valid)
		if i%100 == 0 {
			p.serialize()
		}
	}
	p.serialize()
	if d := p.differs(); d != "" {
		t.Errorf("the scanners differ:\n%s", d)
	}
}

// TestPythonScannerDeserialize gives a Go scanner and a C scanner of python
// random states, and compares the states that they serialize after them.
func TestPythonScannerDeserialize(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"python", python.Language})
	states := append(randomStates(1, 2000, 40), randomStates(2, 200, 1024)...)
	states = append(states, pythonScannerStates...)
	compareScannerStates(t, g, python.Language(), states)
}
