package main

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/xo/transit/_example/internal/dialect"
)

// escapes matches the escapes that paint writes.
var escapes = regexp.MustCompile("\x1b\\[[0-9;]*m")

// TestRun runs the program for each dialect at both depths of color. The
// output without its escapes must be the text, and then one line for each
// key and a line of totals.
func TestRun(t *testing.T) {
	t.Parallel()
	const text = "\\set id 1\nselect name from users where id = :id; -- c\n"
	for _, name := range dialect.Names() {
		for _, colors := range []string{"256", "16"} {
			var out bytes.Buffer
			if err := run(context.Background(), []string{"-dialect", name, "-colors", colors, "-text", text}, &out); err != nil {
				t.Fatalf("%s, %s colors: %v", name, colors, err)
			}
			plain := escapes.ReplaceAllString(out.String(), "")
			got, report, ok := strings.Cut(plain, "edit 1,")
			if !ok || got != text {
				t.Fatalf("%s, %s colors: the text of the output is %q, and want %q", name, colors, got, text)
			}
			if n, want := strings.Count(report, "\n"), utf8.RuneCountInString(text)+1; n != want {
				t.Errorf("%s, %s colors: the report has %d lines, and want %d", name, colors, n, want)
			}
		}
	}
}

// TestRunColors makes sure that a keyword of a statement and a comment of
// usql get the colors of the style monokai.
func TestRunColors(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		colors string
		want   []string
	}{
		{"256", []string{"\x1b[38;5;81mselect", "\x1b[38;5;242m-- c"}},
		{"16", []string{"\x1b[36mselect", "\x1b[90m-- c"}},
	} {
		var out bytes.Buffer
		if err := run(context.Background(), []string{"-colors", test.colors, "-text", "select 1; -- c"}, &out); err != nil {
			t.Fatal(err)
		}
		for _, want := range test.want {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s colors: the output has no %q:\n%q", test.colors, want, out.String())
			}
		}
	}
}

// TestRunDialectColors makes sure that the highlight query of the SQL
// grammar of each of sqlserver, oracle and cql colors a keyword of a
// statement, which the usql layer does not color.
func TestRunDialectColors(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"sqlserver", "oracle", "cql"} {
		var out bytes.Buffer
		if err := run(context.Background(), []string{"-dialect", name, "-text", "select name from users;"}, &out); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := "\x1b[38;5;81mselect"; !strings.Contains(out.String(), want) {
			t.Errorf("%s: the output has no %q:\n%q", name, want, out.String())
		}
	}
}

// TestRunFlags makes sure that a dialect, a style or a depth of color that
// the program does not know is an error.
func TestRunFlags(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"-dialect", "nosuch"},
		{"-style", "nosuch"},
		{"-colors", "8"},
	} {
		if err := run(context.Background(), args, &bytes.Buffer{}); err == nil {
			t.Errorf("%q: no error", args)
		}
	}
}
