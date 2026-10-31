package cgrammar

import (
	"context"
	"io/fs"
	"slices"
	"testing"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/postgres/postgres"
	"github.com/xo/transit/grammars/usql"
	"github.com/xo/transit/inject"
)

// injectConfig returns the configuration of inject of a grammar package,
// with its queries/injections.scm.
func injectConfig(t *testing.T, language *transit.Language, queries fs.FS, name string) *inject.Config {
	t.Helper()
	query, err := fs.ReadFile(queries, "queries/injections.scm")
	if err != nil {
		t.Fatal(err)
	}
	c, err := inject.NewConfig(language, name, string(query))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestInjectUsqlIntoPostgres parses the input of usql with the usql grammar
// with the default options, and injects each statement into the Go package
// postgres, with inject.WithReplacer and usqlPlaceholder. The name sql of
// the injection query of usql gives postgres, as a host that maps sql to the
// dialect of the user does (D102). The package postgres has no injection
// query, so the host gives it an empty one, as usql does. The usql layer and
// the layer of each statement have no error, also with dollar-quoted
// strings.
func TestInjectUsqlIntoPostgres(t *testing.T) {
	t.Parallel()
	usqlConfig := injectConfig(t, usql.Language(), usql.Queries, "usql")
	pg, err := inject.NewConfig(postgres.Language(), "postgres", "")
	if err != nil {
		t.Fatal(err)
	}
	configs := map[string]*inject.Config{"sql": pg, "postgres": pg}
	lookup := func(name string) (*inject.Config, bool) {
		c, ok := configs[name]
		return c, ok
	}
	for _, c := range []struct {
		src string
		// names are the names of the layers, in the order of Layers.
		names []string
		// errors is true for each layer of names that has an error.
		errors []bool
	}{
		{"select * from :tbl where id = :id;", []string{"usql", "sql"}, []bool{false, false}},
		{
			"select $$a$$, $t$b; c$t$ from :tbl;\nselect :'name', :\"col\" from t;",
			[]string{"usql", "sql", "sql"},
			[]bool{false, false, false},
		},
	} {
		layers, err := usqlConfig.Layers(context.Background(), transit.NewParser(), []byte(c.src), lookup, inject.WithReplacer(usqlPlaceholder))
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, 0, len(layers))
		for _, l := range layers {
			names = append(names, l.Name)
		}
		if !slices.Equal(names, c.names) {
			t.Errorf("%q: the layers are %q, and want %q:\n%s", c.src, names, c.names, describeGoLayers(layers))
			continue
		}
		for i, l := range layers {
			if root := l.Tree.RootNode(); root.HasError() != c.errors[i] {
				t.Errorf("%q: the layer %d, %s, has an error: %t, and want %t: %s", c.src, i, l.Name, root.HasError(), c.errors[i], root)
			}
		}
	}
}
