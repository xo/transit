package cgrammar

import (
	"context"
	"io/fs"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/sql"
	"github.com/xo/transit/grammars/usql"
	"github.com/xo/transit/inject"
)

func init() {
	goPackages = append(goPackages, goPackage{"sql", sql.Language})
}

// TestInjectUsqlIntoSQLPackage parses statements of usql with the usql
// grammar with the default options, and injects them into the Go package of
// the generic SQL grammar with inject.WithReplacer. The SQL layer
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
