package main

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/transit/_example/internal/dialect"
)

// TestRun runs the program at cursors in a meta command, in a variable, in
// a statement and after a statement, and makes sure that the output names
// each place and some of its candidates. The cursor is at the end of
// before, and after is the text after the cursor.
func TestRun(t *testing.T) {
	t.Parallel()
	const meta = "\\set tbl users\n\\dt+ public.* x\n"
	for _, test := range []struct {
		dialect, before, after string
		want                   []string
	}{
		{"postgres", "\\se", "t tbl users", []string{"in the name of the meta command \\set"}},
		{"postgres", "\\dt+ ", "", []string{"in argument 1 of the meta command \\dt"}},
		{"postgres", "\\dt+ public.* x", "", []string{"in argument 2 of the meta command \\dt"}},
		{"postgres", meta + "select id from :t", "bl", []string{"in the variable tbl", "variables: tbl\n"}},
		{"postgres", meta + "select id from :tbl where id = 1 ", "\n;", []string{
			"in a SQL statement of postgres",
			"context: source_file > toplevel_stmt > stmt > SelectStmt",
			"keywords: ", " AND ", " OR ",
		}},
		{"postgres", meta + "select 1; sel", "", []string{"in a SQL statement of postgres", "keywords: SELECT\n"}},
		{"postgres", meta + "select 1; ", "", []string{"at the start of a SQL statement of postgres", " SELECT "}},
		{"mysql", meta + "select id from ", "", []string{"in a SQL statement of mysql", "nodes: ", " table_reference "}},
	} {
		var out bytes.Buffer
		text := test.before + test.after
		args := []string{"-dialect", test.dialect, "-text", text, strconv.Itoa(len(test.before))}
		if err := run(context.Background(), args, &out); err != nil {
			t.Fatalf("%q: %v", test.before, err)
		}
		for _, want := range test.want {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s, %q: the output has no %q:\n%s", test.dialect, test.before, want, out.String())
			}
		}
	}
}

// TestRunDialects runs the program at every offset of a text in each
// dialect, so that no cursor makes it fail.
func TestRunDialects(t *testing.T) {
	t.Parallel()
	const text = "\\echo 'a' b\nselect a, b from t where x = :'v' and y = 1; -- c\ninsert into t values (1)"
	args := []string{"-text", text}
	for i := range len(text) + 1 {
		args = append(args, strconv.Itoa(i))
	}
	for _, name := range dialect.Names() {
		var out bytes.Buffer
		if err := run(context.Background(), append([]string{"-dialect", name}, args...), &out); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if n := strings.Count(out.String(), "cursor "); n != len(text)+1 {
			t.Errorf("%s: the output has %d cursors, and want %d", name, n, len(text)+1)
		}
	}
}

// TestRunFlags makes sure that a dialect or a cursor that the program does
// not know is an error.
func TestRunFlags(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"-dialect", "nosuch"},
		{"-text", "select", "7"},
		{"-text", "select", "x"},
	} {
		if err := run(context.Background(), args, &bytes.Buffer{}); err == nil {
			t.Errorf("%q: no error", args)
		}
	}
}
