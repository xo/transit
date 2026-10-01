package cgrammar

import (
	"strings"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/postgres/plpgsql"
	"github.com/xo/transit/grammars/postgres/postgres"
)

func init() {
	goPackages = append(goPackages, postgresPackages...)
}

// postgresPackages are the two packages of the module grammars/postgres, one
// for each grammar of tree-sitter-postgres. Each has its own scanner.
var postgresPackages = []goPackage{
	{"postgres", postgres.Language},
	{"plpgsql", plpgsql.Language},
}

// postgresScannerTexts are texts that reach each branch of the scanner of
// postgres, which reads a dollar-quoted string: the empty tag, a tag with
// each kind of character, a tag of 63 characters and one of 64, a closing
// tag that matches only in part, characters outside ASCII, and a string with
// no end.
var postgresScannerTexts = []string{
	"$$a$$ $$$$ $a$b$a$ $_1$c$_1$",
	" \t\r\n$tag$x $ta$ $tagx$ $tag$",
	"$" + strings.Repeat("t", 63) + "$x$" + strings.Repeat("t", 63) + "$",
	"$" + strings.Repeat("t", 64) + "$x$" + strings.Repeat("t", 64) + "$",
	"$1$ $ $a $a1$x$a1$ $é$x$é$ $ā$x$ā$ $ÿ$x$ÿ$",
	"$a$x$a",
	"$$\xff$$",
	"x$$",
}

// plpgsqlScannerTexts are texts that reach each branch of the scanner of
// plpgsql: each terminator, the nesting of brackets, the strings, the
// quoted identifiers, the dollar quotes and the comments that the scanner
// reads past, the first words that it refuses, and words outside ASCII.
var plpgsqlScannerTexts = []string{
	"a := b + c; x = y; select x into y from z;",
	"x > 1 then y; a when b; a loop b; a by b loop c; a .. b; a.b..c",
	"f(a, b) into c using d, e; f[1] using x loop y; (a; b) ; a) b] c",
	"a, b; a from b into c; 'a''b;' \"c\"\"d;\" $$e;$$ $t$f;$t$ $1; $x",
	"a -- b; c\nd; a - b; -- x\n; a /* b /* c */ ; */ d; a / b; /* x */ y; /* z",
	"<<label>> a; < b; a << b; a::int; a:b; a :=",
	"begin; declare; end; return next x; return query select 1; open c for execute x;",
	"execute x; reverse 1 .. 2 loop; next 1; query; select 1 = 2; update t set a = 1;",
	"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop then; _x$y é ā; xéy then",
	"'unterminated",
	"\"unterminated",
	"$tag$unterminated",
	"$" + strings.Repeat("t", 64) + "$x",
	"",
	"   ",
}

// postgresScannerSets returns the sets of valid symbols that a test of the
// scanner of a grammar gives: the sets of validSymbolSets, and for each
// external token a set with only that token valid.
func postgresScannerSets(l *transit.Language) [][]bool {
	sets := validSymbolSets(l)
	n := int(tablesOf(l).ExternalTokenCount)
	for i := range n {
		set := make([]bool, n)
		set[i] = true
		sets = append(sets, set)
	}
	return sets
}

// TestPostgresScannersMatchC parses each corpus input of postgres and of
// plpgsql and the texts of the scanner of each, with the Go runtime, once
// with the Go scanner and once with the C scanner, and compares each call of
// the two scanners.
func TestPostgresScannersMatchC(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		gp    goPackage
		texts []string
	}{
		{postgresPackages[0], postgresScannerTexts},
		{postgresPackages[1], plpgsqlScannerTexts},
	} {
		t.Run(c.gp.name, func(t *testing.T) {
			t.Parallel()
			g, examples := loadGoPackage(t, c.gp)
			inputs := make([][]byte, 0, len(examples)+len(c.texts))
			for _, e := range examples {
				inputs = append(inputs, e.Input)
			}
			for _, s := range c.texts {
				inputs = append(inputs, []byte(s))
			}
			n, calls := compareScanners(t, g, c.gp.language(), inputs)
			if calls == 0 {
				t.Errorf("the scanner was not called on %d inputs", n)
			}
			t.Logf("%d inputs, %d scanner calls", n, calls)
		})
	}
}

// TestPostgresScannerScans makes a Go scanner and a C scanner of postgres
// and of plpgsql scan each of their texts from each position, with each set
// of postgresScannerSets, and compares the calls, the results and the
// states after. The scanners keep no state, so each scan starts from the
// empty state and from a state of random bytes.
func TestPostgresScannerScans(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		gp    goPackage
		texts []string
	}{
		{postgresPackages[0], postgresScannerTexts},
		{postgresPackages[1], plpgsqlScannerTexts},
	} {
		t.Run(c.gp.name, func(t *testing.T) {
			t.Parallel()
			g, _ := loadGoPackage(t, c.gp)
			texts := make([][]byte, 0, len(c.texts))
			for _, s := range c.texts {
				texts = append(texts, []byte(s))
			}
			states := [][]byte{nil, {0xff, 1, 2}}
			scans := compareScannerScans(t, g, c.gp.language(), states, texts, postgresScannerSets(c.gp.language()))
			t.Logf("%d scans", scans)
		})
	}
}

// TestPostgresScannerDeserialize gives a Go scanner and a C scanner of
// postgres and of plpgsql random states, and compares the states that they
// serialize after them. Both write no bytes.
func TestPostgresScannerDeserialize(t *testing.T) {
	t.Parallel()
	for _, gp := range postgresPackages {
		t.Run(gp.name, func(t *testing.T) {
			t.Parallel()
			g, _ := loadGoPackage(t, gp)
			compareScannerStates(t, g, gp.language(), randomStates(4, 200, 64))
		})
	}
}
