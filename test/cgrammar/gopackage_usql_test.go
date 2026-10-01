package cgrammar

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/xo/transit/grammars/usql/usqlcql"
	"github.com/xo/transit/grammars/usql/usqlmysql"
	"github.com/xo/transit/grammars/usql/usqlplain"
	"github.com/xo/transit/grammars/usql/usqlpostgres"
	"github.com/xo/transit/grammars/usql/usqlsqlite"
	"github.com/xo/transit/grammars/usql/usqlstandard"
	"github.com/xo/transit/internal/abi"
)

func init() {
	goPackages = append(goPackages, usqlPackages...)
}

// usqlPackages are the six packages of the module grammars/usql, one for
// each family of dialects (D101). They share the scanner of
// common/scanner.h.
var usqlPackages = []goPackage{
	{"usql_postgres", usqlpostgres.Language},
	{"usql_mysql", usqlmysql.Language},
	{"usql_sqlite", usqlsqlite.Language},
	{"usql_standard", usqlstandard.Language},
	{"usql_cql", usqlcql.Language},
	{"usql_plain", usqlplain.Language},
}

// usqlScannerInputs are inputs that reach each branch of the scanner of the
// usql grammars: each kind of token, the end of the input in each loop, the
// options of each family, and characters that are not ASCII.
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

// TestUsqlScannersMatchC compares the Go scanner of each package of
// grammars/usql with its C scanner on each corpus input and on each input
// of usqlScannerInputs.
func TestUsqlScannersMatchC(t *testing.T) {
	t.Parallel()
	for _, gp := range usqlPackages {
		t.Run(gp.name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadGoPackage(t, gp)
			inputs := make([][]byte, 0, len(examples)+len(usqlScannerInputs))
			for _, e := range examples {
				inputs = append(inputs, e.Input)
			}
			for _, s := range usqlScannerInputs {
				inputs = append(inputs, []byte(s))
			}
			n, calls := compareScanners(t, g, gp.language(), inputs)
			if calls == 0 {
				t.Errorf("the scanner was not called on %d inputs", n)
			}
			t.Logf("%d inputs, %d scanner calls", n, calls)
		})
	}
}

// TestUsqlScannerDeserializeMatchesC gives random bytes to Deserialize of
// the Go scanner and of the C scanner of each package, and compares what
// Serialize writes after it. Every fifth run gives 6 bytes, the size of the
// state.
func TestUsqlScannerDeserializeMatchesC(t *testing.T) {
	t.Parallel()
	for _, gp := range usqlPackages {
		t.Run(gp.name, func(t *testing.T) {
			t.Parallel()
			g, _ := loadGoPackage(t, gp)
			goCreate := tablesOf(gp.language()).ExternalScanner.Create
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
		})
	}
}
