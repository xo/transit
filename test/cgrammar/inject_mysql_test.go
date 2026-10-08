package cgrammar

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/mysql"
	"github.com/xo/transit/grammars/usql"
	"github.com/xo/transit/inject"
)

// usqlMysqlOptions are the options of the usql grammar for MySQL.
var usqlMysqlOptions = usql.Options{BlockComments: true, HashComments: true, Backticks: true}

// mysqlInjectLayers parses src with the usql grammar with the options opts,
// and injects each statement into the MySQL grammar with usqlPlaceholder.
// The injection query is query, or the query of the usql grammar when query
// is empty.
func mysqlInjectLayers(t *testing.T, opts usql.Options, src, query string) []inject.Layer {
	t.Helper()
	if query == "" {
		b, err := fs.ReadFile(usql.Queries, "queries/injections.scm")
		if err != nil {
			t.Fatal(err)
		}
		query = string(b)
	}
	usqlConfig, err := inject.NewConfig(usql.LanguageFor(opts), "usql", query)
	if err != nil {
		t.Fatal(err)
	}
	mysqlConfig, err := inject.NewConfig(mysql.Language(), "mysql", "")
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(name string) (*inject.Config, bool) {
		return mysqlConfig, name == "sql"
	}
	layers, err := usqlConfig.Layers(context.Background(), transit.NewParser(), []byte(src), lookup, inject.WithReplacer(usqlPlaceholder))
	if err != nil {
		t.Fatal(err)
	}
	return layers
}

// TestInjectMysqlStatements parses texts of usql with the options of MySQL,
// injects each statement into the MySQL grammar, and makes sure that each
// statement layer has no error: statements with the variables of usql and
// of MySQL, backticks, # comments and a stored procedure.
func TestInjectMysqlStatements(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		src        string
		statements int
	}{
		{"select * from `my table` where id = :id and name = @name;", 1},
		{"SET @a = 1; # set a\nSELECT @a := @a + 1, @@session.sql_mode FROM `t`; -- next\nSELECT :'s';", 3},
		{"select `a``b` from t /* c */ where x = :\"col\" # d\n;\n\\g", 1},
		{"CREATE PROCEDURE p (IN x INT) SELECT x * :factor;", 1},
		{"CREATE FUNCTION f (x INT) RETURNS INT DETERMINISTIC RETURN x + 1;", 1},
	} {
		layers := mysqlInjectLayers(t, usqlMysqlOptions, test.src, "")
		if len(layers) != test.statements+1 {
			t.Fatalf("%q: expected the root layer and %d mysql layers, got:\n%s", test.src, test.statements, describeGoLayers(layers))
		}
		for _, l := range layers[1:] {
			if l.Name != "sql" {
				t.Errorf("%q: expected a sql layer, got %s", test.src, l.Name)
			}
			if root := l.Tree.RootNode(); root.HasError() {
				t.Errorf("%q: the layer has an error: %s", test.src, root)
			}
		}
	}
}

// TestInjectMysqlProcedureBody parses a stored procedure whose body is a
// BEGIN ... END block. Without the option BeginEndBlocks, the usql grammar
// ends a statement at each semicolon, so its injection query gives each
// part of the body a layer of its own, and the first part has an error.
// With the option, the procedure is one statement (D112), so the MySQL
// grammar gets the whole procedure in one layer, and the layer has no
// error.
func TestInjectMysqlProcedureBody(t *testing.T) {
	t.Parallel()
	src := "CREATE PROCEDURE p (IN n INT)\nBEGIN\n" +
		"  DECLARE i INT DEFAULT 0; # the counter\n" +
		"  WHILE i < n DO\n" +
		"    INSERT INTO `log` (v) VALUES (:prefix + i);\n" +
		"    SET i = i + 1;\n" +
		"  END WHILE;\n" +
		"END;\n" +
		"CALL p(@n);\n"
	split := mysqlInjectLayers(t, usqlMysqlOptions, src, "")
	if len(split) < 3 || !split[1].Tree.RootNode().HasError() {
		t.Errorf("expected the query of the usql grammar to split the body into layers with an error, got:\n%s", describeGoLayers(split))
	}
	opts := usqlMysqlOptions
	opts.BeginEndBlocks = true
	layers := mysqlInjectLayers(t, opts, src, "")
	if len(layers) != 3 {
		t.Fatalf("expected the root layer, the layer of the procedure and the layer of CALL, got:\n%s", describeGoLayers(layers))
	}
	for _, l := range layers[1:] {
		if root := l.Tree.RootNode(); root.HasError() {
			t.Errorf("the layer has an error: %s", root)
		}
	}
	procedure := layers[1].Tree.RootNode().String()
	for _, kind := range []string{"create_procedure_statement", "block", "declare_variable", "while_statement", "insert_statement"} {
		if !strings.Contains(procedure, "("+kind) {
			t.Errorf("expected a node %s in the layer of the procedure, got %s", kind, procedure)
		}
	}
	if call := layers[2].Tree.RootNode().String(); !strings.Contains(call, "(call_statement") {
		t.Errorf("expected a node call_statement in the layer of CALL, got %s", call)
	}
}
