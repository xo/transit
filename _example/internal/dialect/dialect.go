// Package dialect holds what the two sample programs share: the SQL dialects
// that they know, the options of the usql grammar for each dialect, and the
// placeholders that take the place of the variables of usql in a SQL
// statement.
//
// usql keeps this match of a dialect to a grammar itself, because transit
// knows nothing about the dialects (docs/USQL.md).
package dialect

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"slices"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/cql"
	"github.com/xo/transit/grammars/mysql"
	"github.com/xo/transit/grammars/oracle"
	"github.com/xo/transit/grammars/postgres/postgres"
	"github.com/xo/transit/grammars/sql"
	"github.com/xo/transit/grammars/sqlserver"
	"github.com/xo/transit/grammars/usql"
	"github.com/xo/transit/inject"
)

// Dialect is a SQL dialect: the options of the usql grammar for it, and the
// SQL grammar that parses its statements.
type Dialect struct {
	Name     string
	Options  usql.Options
	Language func() *transit.Language
	Queries  fs.FS
	Keywords func() []string
}

// dialects are the dialects that the flag -dialect chooses from. The
// options are those of the syntax of each dialect in dbmeta. MySQL, Oracle
// and SQL Server also turn on BeginEndBlocks, so that a stored program is
// one statement (D112). CQL also turns on Batches, so that a batch is one
// statement.
var dialects = []Dialect{
	{"cql", usql.Options{DollarQuotes: true, BlockComments: true, SlashComments: true, Batches: true}, cql.Language, cql.Queries, cql.Keywords},
	{"generic", usql.Options{BlockComments: true}, sql.Language, sql.Queries, sql.Keywords},
	{"mysql", usql.Options{BlockComments: true, HashComments: true, Backticks: true, BeginEndBlocks: true}, mysql.Language, mysql.Queries, mysql.Keywords},
	{"oracle", usql.Options{BlockComments: true, BeginEndBlocks: true}, oracle.Language, oracle.Queries, oracle.Keywords},
	{"postgres", usql.Options{DollarQuotes: true, BlockComments: true}, postgres.Language, postgres.Queries, postgres.Keywords},
	{"sqlserver", usql.Options{BlockComments: true, BeginEndBlocks: true}, sqlserver.Language, sqlserver.Queries, sqlserver.Keywords},
}

// Names returns the names of the dialects, sorted.
func Names() []string {
	names := make([]string, len(dialects))
	for i, d := range dialects {
		names[i] = d.Name
	}
	return names
}

// Get returns the dialect of a name, and false for a name that is not one.
func Get(name string) (Dialect, bool) {
	i := slices.IndexFunc(dialects, func(d Dialect) bool { return d.Name == name })
	if i < 0 {
		return Dialect{}, false
	}
	return dialects[i], true
}

// Query returns the text of the query file name of the queries fsys, such as
// "highlights.scm", or "" when the grammar has no such file.
func Query(fsys fs.FS, name string) string {
	b, err := fs.ReadFile(fsys, "queries/"+name)
	if err != nil {
		return ""
	}
	return string(b)
}

// Injector gives the layers of a text of usql: the usql layer, and a layer
// of the SQL grammar of the dialect for each statement.
type Injector struct {
	usql *inject.Config
	sql  *inject.Config
}

// NewInjector compiles the injection query of the usql grammar with the
// options of d. The layer of a statement gets no injections of its own, so
// the SQL grammar gets no injection query.
func NewInjector(d Dialect) (*Injector, error) {
	u, err := inject.NewConfig(usql.LanguageFor(d.Options), "usql", Query(usql.Queries, "injections.scm"))
	if err != nil {
		return nil, fmt.Errorf("compiling the injection query of usql: %w", err)
	}
	s, err := inject.NewConfig(d.Language(), d.Name, "")
	if err != nil {
		return nil, fmt.Errorf("making the configuration of %s: %w", d.Name, err)
	}
	return &Injector{usql: u, sql: s}, nil
}

// Usql returns the language of the usql grammar with the options of the
// dialect.
func (i *Injector) Usql() *transit.Language {
	return i.usql.Language()
}

// Layers parses src with the usql grammar, and parses each statement with
// the SQL grammar of the dialect, with Placeholder. The injection query of
// usql names the language sql for a statement, and the host maps that name
// to its dialect. The usql grammar also names bash for a shell command, and
// the programs skip it.
func (i *Injector) Layers(ctx context.Context, p *transit.Parser, src []byte) ([]inject.Layer, error) {
	lookup := func(name string) (*inject.Config, bool) {
		return i.sql, name == "sql"
	}
	layers, err := i.usql.Layers(ctx, p, src, lookup, inject.WithReplacer(Placeholder))
	if err != nil {
		return nil, fmt.Errorf("finding the layers: %w", err)
	}
	return layers, nil
}

// Placeholder is the inject.Replacer of a variable of usql. It gives a
// placeholder of the same length that a SQL grammar accepts: an identifier
// for :name, a string for :'name', a quoted identifier for :"name", and
// TRUE for :{?name}, as docs/API.md shows.
func Placeholder(_ string, n transit.Node, src []byte) ([]byte, bool) {
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
