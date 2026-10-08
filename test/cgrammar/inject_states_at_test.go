package cgrammar

import (
	"context"
	"slices"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/postgres/postgres"
	"github.com/xo/transit/grammars/usql"
	"github.com/xo/transit/inject"
)

// TestInjectStatesAtUsql completes in a SQL statement of the input of usql,
// as usql does (D111). It parses the input with the usql grammar with the
// options of PostgreSQL and injects each statement into the Go package
// postgres, with usqlPlaceholder. A meta command comes before the
// statement, so the layer of the statement does not cover the whole input.
// Its argument starts with a quote, which starts a string in SQL.
// At each cursor, the test finds the layer that holds it, and
// Layer.StatesAt gives the states there. They must be the states of a plain
// parse of the statement with the placeholders written out, and they must
// accept the symbols of what comes next. A parse of the text of the layer
// without the ranges of the layer gives other states.
func TestInjectStatesAtUsql(t *testing.T) {
	t.Parallel()
	language := usql.LanguageFor(usql.Options{DollarQuotes: true, BlockComments: true})
	usqlConfig := injectConfig(t, language, usql.Queries, "usql")
	pg := injectConfig(t, postgres.Language(), postgres.Queries, "postgres")
	lookup := func(name string) (*inject.Config, bool) {
		return pg, name == "sql" || name == "postgres"
	}
	const (
		meta      = "\\echo 'it\n"
		statement = "select * from :tbl where id = :id;" //nolint:unqueryvet // the text is an input to parse
		plain     = "select * from _tbl where id = _id;" //nolint:unqueryvet // the text is an input to parse
	)
	src := []byte(meta + statement + "\n") //nolint:unqueryvet // the text is an input to parse
	p := transit.NewParser()
	layers, err := usqlConfig.Layers(context.Background(), p, src, lookup, inject.WithReplacer(usqlPlaceholder))
	if err != nil {
		t.Fatal(err)
	}
	if root := layers[0].Tree.RootNode(); root.HasError() {
		t.Errorf("the usql layer has an error: %s", root)
	}
	for _, c := range []struct {
		// before is the text of the statement before the cursor.
		before string
		// symbols are some of the symbols that the states must accept.
		symbols []string
	}{
		{"select * from ", []string{"table_ref", "relation_expr", "select_with_parens"}},
		{"select * from :tbl where ", []string{"a_expr", "columnref", "kw_not"}},
	} {
		cursor := len(meta) + len(c.before)
		i := slices.IndexFunc(layers, func(l inject.Layer) bool {
			return l.Name == "sql" && slices.ContainsFunc(l.Ranges, func(r transit.Range) bool {
				return r.StartByte <= cursor && cursor <= r.EndByte
			})
		})
		if i < 0 {
			t.Fatalf("%q: no sql layer holds the cursor:\n%s", c.before, describeGoLayers(layers))
		}
		l := layers[i]
		if l.Ranges[0].StartByte != len(meta) {
			t.Errorf("%q: the sql layer starts at %d, and want %d", c.before, l.Ranges[0].StartByte, len(meta))
		}
		if got := string(l.Text[len(meta) : len(meta)+len(plain)]); got != plain {
			t.Errorf("%q: the text of the layer is %q, and want %q", c.before, got, plain)
		}
		states, err := l.StatesAt(context.Background(), p, cursor)
		if err != nil {
			t.Fatal(err)
		}
		want, err := p.StatesAt(context.Background(), []byte(plain), len(c.before), nil)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(states, want) {
			t.Errorf("%q: the states of the layer are %v, and the states of a plain parse are %v", c.before, states, want)
		}
		// Without the ranges of the layer, the quote of the meta command starts
		// a string that holds the statement, so the states are not the same.
		if err := p.SetLanguage(postgres.Language()); err != nil {
			t.Fatal(err)
		}
		whole, err := p.StatesAt(context.Background(), l.Text, cursor, nil)
		if err != nil {
			t.Fatal(err)
		}
		if slices.Equal(whole, want) {
			t.Errorf("%q: a parse of the text of the layer without its ranges gives the same states %v", c.before, whole)
		}
		var names []string
		for _, state := range states {
			lookahead, ok := postgres.Language().LookaheadIterator(state)
			if !ok {
				t.Fatalf("%q: LookaheadIterator(%d) returned false", c.before, state)
			}
			names = append(names, slices.Collect(lookahead.Names())...)
		}
		for _, s := range c.symbols {
			if !slices.Contains(names, s) {
				t.Errorf("%q: the states %v do not accept %s, only %q", c.before, states, s, names)
			}
		}
	}
}
