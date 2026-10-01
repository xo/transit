package cgrammar

import (
	"bytes"
	"context"
	"io/fs"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/usql/usqlpostgres"
	"github.com/xo/transit/inject"
)

// usqlPlaceholder gives the placeholder of a variable of the usql grammar,
// with the length of the variable, so that the SQL grammar parses it (D101).
// :name becomes the identifier _name, :'name' the string '_name', :"name"
// the quoted identifier "_name", and :{?name} the expression TRUE with
// spaces after it. psql gives TRUE or FALSE for :{?name}.
func usqlPlaceholder(_ string, n transit.Node, src []byte) ([]byte, bool) {
	if n.Kind() != "variable" {
		return nil, false
	}
	name, ok := n.ChildByFieldName("name")
	if !ok {
		return nil, false
	}
	text := src[n.StartByte():n.EndByte()]
	id := "_" + string(src[name.StartByte():name.EndByte()])
	switch {
	case bytes.HasPrefix(text, []byte(":{?")):
		return append([]byte("TRUE"), bytes.Repeat([]byte(" "), len(text)-4)...), true
	case bytes.HasPrefix(text, []byte(":'")):
		return []byte("'" + id + "'"), true
	case bytes.HasPrefix(text, []byte(`:"`)):
		return []byte(`"` + id + `"`), true
	}
	return []byte(id), true
}

// TestInjectReplacesUsqlVariables parses the input of usql with the usql
// grammar of the family postgres and the SQL grammar of DerekStride, with
// and without inject.WithReplacer. Without it, each variable is an error in
// the SQL layer. With it, the SQL layer parses the placeholders, and its
// offsets are the offsets of the input.
func TestInjectReplacesUsqlVariables(t *testing.T) {
	t.Parallel()
	_, root, cache := oracleSetup(t)
	configs := goInjectConfigs(t, oracleLanguages(t, root, cache, "sql"), false)
	query, err := fs.ReadFile(usqlpostgres.Queries, "queries/injections.scm")
	if err != nil {
		t.Fatal(err)
	}
	usql, err := inject.NewConfig(usqlpostgres.Language(), "usql", string(query))
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(name string) (*inject.Config, bool) {
		c, ok := configs[name]
		return c, ok
	}
	for _, src := range []string{
		"select * from :tbl where id = :id",
		"select :'s', :\"c\" from t where :{?flag};",
	} {
		for _, test := range []struct {
			name     string
			opts     []inject.Option
			hasError bool
		}{
			{"without", nil, true},
			{"with", []inject.Option{inject.WithReplacer(usqlPlaceholder)}, false},
		} {
			layers, err := usql.Layers(context.Background(), transit.NewParser(), []byte(src), lookup, test.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if len(layers) != 2 || layers[1].Name != "sql" {
				t.Fatalf("%q: expected the root layer and a sql layer, got:\n%s", src, describeGoLayers(layers))
			}
			sql := layers[1].Tree.RootNode()
			if sql.HasError() != test.hasError {
				t.Errorf("%q %s the replacer: the sql layer has an error: %t, and want %t: %s", src, test.name, sql.HasError(), test.hasError, sql)
			}
			if sql.StartByte() != 0 || sql.EndByte() != len(src) {
				t.Errorf("%q %s the replacer: the sql layer runs from %d to %d, and want 0 to %d", src, test.name, sql.StartByte(), sql.EndByte(), len(src))
			}
		}
	}
}
