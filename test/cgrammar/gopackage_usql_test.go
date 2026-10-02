package cgrammar

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xo/transit/grammars/usql"
	"github.com/xo/transit/internal/abi"
)

func init() {
	goPackages = append(goPackages, goPackage{"usql", usql.Language})
}

// usqlOptionSets are the options of the families of dialects of D101, which
// had a grammar each before D108. The test builds the C scanner once for
// each of them (D108).
var usqlOptionSets = []struct {
	name string
	opts usql.Options
}{
	{"postgres", usql.Options{DollarQuotes: true, BlockComments: true}},
	{"mysql", usql.Options{BlockComments: true, HashComments: true, Backticks: true}},
	{"sqlite", usql.Options{BlockComments: true, Backticks: true}},
	{"standard", usql.Options{BlockComments: true}},
	{"cql", usql.Options{DollarQuotes: true, BlockComments: true, SlashComments: true}},
	{"plain", usql.Options{}},
}

// usqlDefine returns the C macro of src/scanner.c of the usql grammar that
// sets the options opts, such as USQL_OPTIONS=3. Field i of usql.Options is
// the flag 1<<i of src/scanner.c, so a new field needs no change here.
func usqlDefine(opts usql.Options) string {
	v := reflect.ValueOf(opts)
	n := 0
	for i := range v.NumField() {
		if v.Field(i).Bool() {
			n |= 1 << i
		}
	}
	return fmt.Sprintf("USQL_OPTIONS=%d", n)
}

// loadUsql builds and loads the C grammar of the usql grammar with the
// options opts, and returns it with the inputs of the corpus, of the cases
// of testdata/options of the package, and of usqlScannerInputs.
func loadUsql(t *testing.T, opts usql.Options) (*Grammar, [][]byte) {
	t.Helper()
	g, examples := loadGoPackageDefines(t, goPackage{"usql", usql.Language}, usqlDefine(opts))
	root, _ := setup(t)
	more, err := ReadCorpus(filepath.Join(root, "grammars", "usql", "testdata", "options"))
	if err != nil {
		t.Fatal(err)
	}
	examples = append(examples, more...)
	inputs := make([][]byte, 0, len(examples)+len(usqlScannerInputs))
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range usqlScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	return g, inputs
}

// usqlScannerInputs are inputs that reach each branch of the scanner of the
// usql grammar: each kind of token, the end of the input in each loop, each
// option, and characters that are not ASCII.
var usqlScannerInputs = []string{
	// statements, parentheses and escapes
	"select 1; select 2",
	"select (1; (2)); x",
	"select ) ; (",
	"select \\;\\: \\x",
	"\\;a;\\:b",
	"((((",
	";;;",
	"select 1\r\n\\g\r\n",
	// strings, quoted identifiers, backticks and dollar quotes
	"select 'a''b', 'c\\'d', 'e",
	"select 'a\\",
	"select \"a\"\"b\", \"c",
	"select `a``b`, `c",
	"select $$a$$, $t$b$t$, $t$c$u$d$t$, $1, a$b$, $$e",
	"select $t$x$t",
	"select $_x$ y $_x$, $\u00e9$ z $\u00e9$",
	"select $" + string(bytes.Repeat([]byte("a"), 130)) + "$",
	"$$",
	"$a",
	// comments
	"select 1 -- a\n; -- b",
	"select /* a ; */ 1; /* b",
	"select 1 # a; b\n;",
	"select 1 // a; b\n;",
	"select 1 - 2 / 3 -",
	"--",
	"/*",
	"#",
	"//",
	// variables
	"select :a, :'b', :\"c\", :{?d}, ::e, :::f",
	"select :'a b', :\"c d\", :'', :\"\", :'e",
	"select :{x}, :{?}, :{?y, :{?z z}",
	"select a:b, a:'c', a:\"d\", a:{?e}, a::f, a:",
	":",
	":'",
	":{",
	":{?",
	"select :\u540d, :'\u00f1'",
	// meta commands
	"\\q",
	"\\dS+ a \\dtv \\dvt \\dtq \\d+S \\dfnS+",
	"\\lo_list+ \\include_relative x",
	"\\echo'a' \\'b'",
	"\\ \n\\",
	"\\",
	"\\\\ select 1",
	"\\x \\\\ \\\\",
	"\\! ls -l 'a\\b' \"c\\d\" \\p",
	"\\!",
	"\\! \n",
	"\\!x",
	"\\echo a b\tc \\p",
	"\\echo 'a''b' \"c\"\"d\" 'e\\'f' 'g",
	"\\echo 'a\\",
	"\\echo \"a",
	"\\echo a'b'c\"d\"e",
	"\\echo :a :'b' :\"c\" :{?d} x:y x::y x:'y z' x:",
	"\\echo `date` x`y`z `a",
	"\\echo `",
	"\\echo ``",
	"\\o |cat \\x",
	"\\o | ",
	"\\g |",
	"\\g (a=b c 'd'=e f=:g h=\"i\" j='k') file",
	"\\g (a=b",
	"\\g (a=",
	"\\g (",
	"\\g (\n)",
	"\\g ((a)",
	"\\chart bar (x=1)",
	"\\echo (a)",
	"\\copy a b 'select 1' t(a,b)",
	"\\if :a\nselect 1;\n\\elif b\n\\else\n\\endif",
	"\\echo -- a /* b */ # c",
	"\\echo \u00e9\u00e8 \\\u00e9",
	"\\" + string(bytes.Repeat([]byte("d"), 40)),
	"\\d\x01x",
	// the error recovery and mixed inputs
	"select 1 \\g ) (\nselect ';",
	"\\set a `b\nselect :'c",
	"select $$ \\g $$ \\g",
	"(\\g\n)",
	"\\g (=)",
	"\u00a0select\u2028:x",
}

// TestUsqlScannersMatchC compares the Go scanner of the usql grammar with
// its C scanner, built with the same options, for each set of options of
// usqlOptionSets, on each corpus input and on each input of
// usqlScannerInputs.
func TestUsqlScannersMatchC(t *testing.T) {
	t.Parallel()
	for _, set := range usqlOptionSets {
		t.Run(set.name, func(t *testing.T) {
			t.Parallel()
			g, inputs := loadUsql(t, set.opts)
			n, calls := compareScanners(t, g, usql.LanguageFor(set.opts), inputs)
			if calls == 0 {
				t.Errorf("the scanner was not called on %d inputs", n)
			}
			t.Logf("%d inputs, %d scanner calls", n, calls)
		})
	}
}

// TestUsqlScannerOptionsDiffer makes sure that each set of options of
// usqlOptionSets gives the C scanner and the Go scanner other trees than
// the default options on the cases of testdata/options, so that the test
// above compares scanners that read their options.
func TestUsqlScannerOptionsDiffer(t *testing.T) {
	t.Parallel()
	for _, set := range usqlOptionSets[1:] {
		t.Run(set.name, func(t *testing.T) {
			t.Parallel()
			g, inputs := loadUsql(t, set.opts)
			def, _ := loadUsql(t, usql.Options{DollarQuotes: true, BlockComments: true})
			differ := false
			for _, in := range inputs {
				got, _, err := g.CParse(in)
				if err != nil {
					t.Fatal(err)
				}
				want, _, err := def.CParse(in)
				if err != nil {
					t.Fatal(err)
				}
				if Diff(want, got) != "" {
					differ = true
					break
				}
			}
			if !differ {
				t.Error("expected another tree than with the default options for an input, got the same trees")
			}
		})
	}
}

// TestUsqlScannerDeserializeMatchesC gives random bytes to Deserialize of
// the Go scanner and of the C scanner of the usql grammar, and compares what
// Serialize writes after it. Every fifth run gives 6 bytes, the size of the
// state. The options do not change the state, so the test uses the default
// options.
func TestUsqlScannerDeserializeMatchesC(t *testing.T) {
	t.Parallel()
	g, _ := loadUsql(t, usql.Options{DollarQuotes: true, BlockComments: true})
	goCreate := tablesOf(usql.Language()).ExternalScanner.Create
	cCreate := tablesOf(g.Language).ExternalScanner.Create
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 1000 {
		size := r.IntN(abi.SerializationBufferSize + 1)
		if i%5 == 0 {
			size = 6
		}
		in := make([]byte, size)
		for k := range in {
			in[k] = byte(r.Uint32())
		}
		goScanner, cScanner := goCreate(), cCreate()
		goScanner.Deserialize(in)
		cScanner.Deserialize(in)
		goBuf := make([]byte, abi.SerializationBufferSize)
		cBuf := make([]byte, abi.SerializationBufferSize)
		goN, cN := goScanner.Serialize(goBuf), cScanner.Serialize(cBuf)
		if goN != cN || !bytes.Equal(goBuf[:goN], cBuf[:cN]) {
			t.Fatalf("run %d with %d bytes: Serialize writes %x in Go and %x in C", i, len(in), goBuf[:goN], cBuf[:cN])
		}
	}
}
