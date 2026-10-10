package cgrammar

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xo/transit/grammars/usql"
	"github.com/xo/transit/internal/abi"
)

func init() {
	goPackages = append(goPackages, goPackage{"usql", usql.Language})
}

// usqlOptionSets are the options of the families of dialects of D101, which
// had a grammar each before D108, sets with BeginEndBlocks (D112), and sets
// with Batches. The test builds the C scanner once for each of them (D108). The options of
// SQL Server and Oracle with BeginEndBlocks are those of standard with
// BeginEndBlocks, and postgres with BeginEndBlocks reaches the branches of
// dollar quotes.
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
	{"mysql_blocks", usql.Options{BlockComments: true, HashComments: true, Backticks: true, BeginEndBlocks: true}},
	{"standard_blocks", usql.Options{BlockComments: true, BeginEndBlocks: true}},
	{"postgres_blocks", usql.Options{DollarQuotes: true, BlockComments: true, BeginEndBlocks: true}},
	{"cql_batches", usql.Options{DollarQuotes: true, BlockComments: true, SlashComments: true, Batches: true}},
	{"mysql_blocks_batches", usql.Options{BlockComments: true, HashComments: true, Backticks: true, BeginEndBlocks: true, Batches: true}},
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
	"select :'a b' :\"c d\" :'a' :\"b\" :'' :\"\" :' :\" :'a :\"b",
	"select x:'a b', (:'c')",
	"\\echo :'a b' :\"c d\" :' :\"",
	"\\g (a=:'b c' d=:\"e f\" g=:'h' :'i j'=k l=:'m",
	"\\g (:'a'=b)",
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
	"\\echo `a\nselect 1; \\echo ` b\n\\echo `\\p",
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
	// the blocks of stored programs (D112)
	"create procedure p() begin select 1; end; select 2",
	"CREATE PROCEDURE p() BEGIN; END; x",
	"CREATE PROCEDURE p() BEGIN(1); x; END; y",
	"CREATE PROCEDURE p() BEGIN WORK; x; END; y",
	"CREATE PROCEDURE p() BEGIN /* a */ BEGIN TRANSACTION; END; x; END; y",
	"CREATE PROCEDURE p() BEGIN x; END) y; END END; z",
	"CREATE PROCEDURE p() BEGIN a.end; @end; $end; #end; [end]; b$end; END; x",
	"CREATE PROCEDURE p() BEGIN case x when 1 then end case; END CASE; END; y",
	"CREATE PROCEDURE p() BEGIN IF a THEN END IF; LOOP END LOOP; WHILE b DO END WHILE; REPEAT UNTIL c END REPEAT; END; x",
	"CREATE PROCEDURE p() BEGIN -- a\nBEGIN TRY x; END TRY BEGIN CATCH y; END CATCH END; z",
	"CREATE PROCEDURE p() BEGIN x; END 'a'; y",
	"CREATE PROCEDURE p() BEGIN x; END",
	"CREATE PROCEDURE p() BEGIN",
	"CREATE PROCEDURE p() BEGIN x; \\ y; END; z",
	"CREATE PROCEDURE p() BEGIN x \\; y; END; z",
	"CREATE PROCEDURE p() BEGIN :a; :'b'; END; c",
	"CREATE PROCEDURE p() BEGIN `end`; \"end\"; 'end'; -- end\n /* end */ # end\n END; x",
	"CREATE PROCEDURE p() BEGIN averyveryverylongidentifier; END averyveryverylongidentifier; x",
	"CREATE PROCEDURE p() BEGIN \u00e9nd; END \u00e9; x",
	"CREATE PROCEDURE p() BEGIN $$ end; $$; $t$ x $t$; END; y",
	"CREATE PROCEDURE p() " + strings.Repeat("BEGIN x ", 300) + strings.Repeat("END; ", 300) + "y; z",
	"CREATE DEFINER = root@localhost PROCEDURE p() BEGIN x; END; y",
	"CREATE DEFINER=`a`@`b` FUNCTION f() BEGIN x; END; y",
	"CREATE DEFINER = CURRENT_USER EVENT e DO BEGIN x; END; y",
	"CREATE DEFINER = x VIEW v AS SELECT 1; BEGIN; y",
	"CREATE OR REPLACE AGGREGATE FUNCTION f() BEGIN x; END; y",
	"CREATE TEMP TRIGGER t BEGIN x; END; y",
	"CREATE CONSTRAINT TRIGGER t BEGIN x; END; y",
	"CREATE NONEDITIONABLE PROCEDURE p IS BEGIN x; END; y",
	"CREATE TABLE t (begin int); x",
	"SELECT 1; CREATE PROCEDURE p() BEGIN x; END; y",
	"(CREATE PROCEDURE p() BEGIN x; END); y",
	"CREATE PROCEDURE p IS v NUMBER; BEGIN x; END; y",
	"CREATE PROCEDURE p AS v NUMBER; w NUMBER; BEGIN x; END; y",
	"CREATE PROCEDURE p AS BEGIN x; END; y",
	"CREATE PROCEDURE p AS SELECT 1; y",
	"CREATE PROCEDURE p AS RETURN; y",
	"CREATE PROCEDURE p AS PRINT 1; y",
	"CREATE PROCEDURE p AS IF a; y",
	"CREATE PROCEDURE p AS WHILE a; y",
	"CREATE PROCEDURE p AS LANGUAGE C; y",
	"CREATE PROCEDURE p AS EXTERNAL; y",
	"CREATE PROCEDURE p AS 'body'; y",
	"CREATE PROCEDURE p AS $$ body $$; y",
	"CREATE PROCEDURE p AS (x); y",
	"CREATE PROCEDURE p AS; y",
	"CREATE PROCEDURE p (a int AS b) x; y",
	"CREATE PROCEDURE p WITH x AS y; z",
	"CREATE PROCEDURE p CALL q; y",
	"CREATE PROCEDURE p DELETE; INSERT; UPDATE; y",
	"CREATE PROCEDURE p EXEC a; EXECUTE b; MERGE c; REPLACE d; VALUES e; SET f; y",
	"CREATE PROCEDURE p DECLARE x; BEGIN y; END; z",
	"CREATE TRIGGER t DECLARE x; BEGIN y; END; z",
	"CREATE TRIGGER t AS x; y",
	"CREATE FUNCTION f RETURN NUMBER IS BEGIN RETURN 1; END; y",
	"CREATE PROC p AS BEGIN DISTRIBUTED TRANSACTION; TRAN; END; x",
	"create Procedure P() Begin x; End; y",
	// the batches of CQL
	"BEGIN BATCH INSERT x; UPDATE y; APPLY BATCH; z",
	"begin unlogged batch x; apply batch; Begin Counter Batch y; Apply Batch; z",
	"BEGIN BATCH x; APPLY; APPLY y; APPLY -- a\n BATCH; z",
	"BEGIN BATCH x; apply/* a */batch; z",
	"BEGIN BATCH x; APPLY 'BATCH'; y; APPLY BATCH; z",
	"BEGIN BATCH x; (APPLY BATCH; y); APPLY BATCH; z",
	"BEGIN BATCH x; \\g\nselect 1; y",
	"BEGIN BATCH x;",
	"BEGIN TRANSACTION; x; BEGIN; y; BEGIN UNLOGGED; z; BEGIN COUNTER x; y",
	"x; BEGIN BATCH y; APPLY BATCH; BEGIN BATCH",
	"SELECT 1 BEGIN BATCH x; y",
	"CREATE PROCEDURE p() BEGIN BATCH x; END; y; BEGIN BATCH z; APPLY BATCH; w",
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
// Serialize writes after it. Every fifth run gives 10 bytes, the size of the
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
			size = 10
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
