package cgrammar

import (
	"bytes"
	"context"
	"io/fs"
	"math/rand/v2"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/sql"
	"github.com/xo/transit/grammars/usql"
	"github.com/xo/transit/inject"
	"github.com/xo/transit/internal/abi"
)

func init() {
	goPackages = append(goPackages, goPackage{"sql", sql.Language})
}

// sqlScannerInputs are inputs that reach the branches of the scanner of sql
// that the corpus does not reach: dollar quoted strings with and without a
// tag, tags that do not end, tags that hold a 0 byte, characters that are
// not ASCII, and tags whose characters have the same low byte, which the C
// scanner keeps as one char.
var sqlScannerInputs = []string{
	"select $$a$$;",
	"select $t$ a $ b $t$;",
	"select $a$ $b$ x $b$ $a$;",
	"select $a$ x",
	"select $a b$;",
	"select $$",
	"select $",
	"select $a$x$b$y$a$;",
	"select $a\x00b$ x $a\x00c$;",
	"select $\u00e9$ x $\u00e9$;",
	"select $\u4e2d$ x $\u4f2d$;",
	"select \xff$\xff$ x $\xff$;",
	"create function f() returns int as $$ select 1 $$ language sql;",
	"create function f() returns int as $body$\nbegin\n  return $$x$$;\nend\n$body$ language plpgsql;",
	"create function f() returns int as\n\t $f$ select 1 $f$ language sql;",
	"create function f() returns int as $f$ select 1",
	"do $$ begin perform 1; end $$;",
	"",
}

// sqlScannerTexts are the texts that TestSQLScannerScans scans from each
// position.
var sqlScannerTexts = []string{
	"  $a$ x $a$ $$",
	"$t$\n$u$ $t$",
	"$a\x00$ $\u00e9$ $\u4e2d$",
	"$ab cd$ $",
}

// sqlScannerStates are states of the scanner of sql: no state, a state of
// one byte, which the scanner ignores, and start tags. Each tag ends with a
// 0 byte, because the C scanner reads a tag up to its 0 byte.
var sqlScannerStates = [][]byte{
	nil,
	{'$'},
	[]byte("$$\x00"),
	[]byte("$a$\x00"),
	[]byte("$t$\x00"),
	[]byte("$\xc3\xa9$\x00"),
	[]byte("$\xe9$\x00"),
	[]byte("\x00\x00"),
	[]byte("$a\x00b$\x00"),
}

// TestSQLScannerMatchesC parses each corpus input of sql and the inputs of
// sqlScannerInputs with the Go runtime, once with the Go scanner and once
// with the C scanner, and compares each call of the two scanners.
func TestSQLScannerMatchesC(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"sql", sql.Language})
	var inputs [][]byte
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range sqlScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, sql.Language(), inputs)
	if calls == 0 {
		t.Errorf("the scanner was not called on %d inputs", n)
	}
	t.Logf("%d inputs, %d scanner calls", n, calls)
}

// TestSQLScannerScans makes a Go scanner and a C scanner of sql scan each
// text of sqlScannerTexts from each position, from each state of
// sqlScannerStates and with each set of valid symbols of the grammar, and
// compares the calls, the results and the states after.
func TestSQLScannerScans(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"sql", sql.Language})
	var texts [][]byte
	for _, s := range sqlScannerTexts {
		texts = append(texts, []byte(s))
	}
	scans := compareScannerScans(t, g, sql.Language(), sqlScannerStates, texts, validSymbolSets(sql.Language()))
	t.Logf("%d scans", scans)
}

// TestSQLScannerDeserialize gives a Go scanner and a C scanner of sql
// random states, and compares the states that they serialize after them.
// Each random state holds a 0 byte at a random place, because the C scanner
// reads a tag up to its 0 byte, and past the end of the state when the
// state has none. The longest states make a tag that Serialize does not
// write, because it does not fit in the buffer.
func TestSQLScannerDeserialize(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"sql", sql.Language})
	r := rand.New(rand.NewPCG(3, 4))
	states := append([][]byte{}, sqlScannerStates...)
	for _, state := range randomStates(5, 2000, 40) {
		if len(state) > 0 {
			state[r.IntN(len(state))] = 0
			states = append(states, state)
		}
	}
	for _, n := range []int{abi.SerializationBufferSize - 2, abi.SerializationBufferSize - 1, abi.SerializationBufferSize} {
		states = append(states, append(bytes.Repeat([]byte{'a'}, n-1), 0))
	}
	compareScannerStates(t, g, sql.Language(), states)
}

// TestInjectUsqlIntoSQLPackage parses statements of usql with the usql
// grammar with the default options, and injects them into the Go package of
// the SQL grammar of DerekStride with inject.WithReplacer. The SQL layer
// parses the placeholders of the variables with no error.
func TestInjectUsqlIntoSQLPackage(t *testing.T) {
	t.Parallel()
	query, err := fs.ReadFile(usql.Queries, "queries/injections.scm")
	if err != nil {
		t.Fatal(err)
	}
	usqlConfig, err := inject.NewConfig(usql.Language(), "usql", string(query))
	if err != nil {
		t.Fatal(err)
	}
	sqlConfig, err := inject.NewConfig(sql.Language(), "sql", "")
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(name string) (*inject.Config, bool) {
		return sqlConfig, name == "sql"
	}
	for _, src := range []string{
		"select * from :tbl where id = :id;",
		"select :'s', :\"c\" from t where :{?flag};",
	} {
		layers, err := usqlConfig.Layers(context.Background(), transit.NewParser(), []byte(src), lookup, inject.WithReplacer(usqlPlaceholder))
		if err != nil {
			t.Fatal(err)
		}
		if len(layers) != 2 || layers[1].Name != "sql" || layers[1].Config != sqlConfig {
			t.Fatalf("%q: expected the root layer and a sql layer of the Go package, got:\n%s", src, describeGoLayers(layers))
		}
		root := layers[1].Tree.RootNode()
		if root.HasError() {
			t.Errorf("%q: the sql layer has an error: %s", src, root)
		}
		if root.StartByte() != 0 || root.EndByte() != len(src) {
			t.Errorf("%q: the sql layer runs from %d to %d, and want 0 to %d", src, root.StartByte(), root.EndByte(), len(src))
		}
	}
}
