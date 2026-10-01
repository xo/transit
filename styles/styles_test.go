package styles

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
)

// TestParseEntry reads each word of a style string.
func TestParseEntry(t *testing.T) {
	rgb := func(v uint32) Color { return Color{Kind: ColorRGB, Value: v} }
	for _, c := range []struct {
		s    string
		want Entry
	}{
		{"", Entry{}},
		{"   ", Entry{}},
		{"bold", Entry{Bold: true}},
		{"italic underline", Entry{Italic: true, Underline: true}},
		{"bold nobold", Entry{}},
		{"italic noitalic", Entry{}},
		{"underline nounderline", Entry{}},
		{"nobold bold", Entry{Bold: true}},
		{"#abc", Entry{Fg: rgb(0xaabbcc)}},
		{"#A0B1C2", Entry{Fg: rgb(0xa0b1c2)}},
		{"bg:#272822", Entry{Bg: rgb(0x272822)}},
		{"#f00 #0f0", Entry{Fg: rgb(0x00ff00)}},
		{"bold #f8f8f2 bg:#272822", Entry{Fg: rgb(0xf8f8f2), Bg: rgb(0x272822), Bold: true}},
		{"#ansired", Entry{Fg: Color{Kind: ColorANSI, Value: 9}}},
		{"#ansiblack bg:#ansiwhite", Entry{Fg: Color{Kind: ColorANSI}, Bg: Color{Kind: ColorANSI, Value: 15}}},
		{"#ansidarkred", Entry{Fg: Color{Kind: ColorANSI, Value: 1}}},
		{"#ansi0", Entry{Fg: Color{Kind: Color256}}},
		{"#ansi208", Entry{Fg: Color{Kind: Color256, Value: 208}}},
		{"bg:#ansi255", Entry{Bg: Color{Kind: Color256, Value: 255}}},
	} {
		got, err := ParseEntry(c.s)
		if err != nil {
			t.Errorf("%q: expected no error, got: %v", c.s, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: expected %+v, got: %+v", c.s, c.want, got)
		}
	}
}

// TestParseEntryErrors makes sure that a word outside the subset is an
// error, such as the words of chroma that only the converter reads.
func TestParseEntryErrors(t *testing.T) {
	for _, s := range []string{
		"noinherit",
		"inherit",
		"border:#000000",
		"bg:",
		"bg:red",
		"red",
		"#",
		"#12",
		"#1234",
		"#12345g",
		"#ansi",
		"#ansi256",
		"#ansi-1",
		"#ansi007",
		"#ansipink",
		"#red",
		"bold #ff00zz",
	} {
		if _, err := ParseEntry(s); !errors.Is(err, ErrSyntax) {
			t.Errorf("%q: expected ErrSyntax, got: %v", s, err)
		}
	}
}

// TestEntryString makes sure that String writes the words in the order of
// chroma, and that ParseEntry reads them back.
func TestEntryString(t *testing.T) {
	for _, c := range []struct {
		e    Entry
		want string
	}{
		{Entry{}, ""},
		{Entry{Bold: true, Italic: true, Underline: true}, "bold italic underline"},
		{Entry{Fg: Color{Kind: ColorRGB, Value: 0xabc}, Bg: Color{Kind: ColorRGB, Value: 0x272822}, Italic: true}, "italic #000abc bg:#272822"},
		{Entry{Fg: Color{Kind: ColorANSI, Value: 4}}, "#ansidarkblue"},
		{Entry{Bg: Color{Kind: Color256, Value: 17}}, "bg:#ansi17"},
	} {
		got := c.e.String()
		if got != c.want {
			t.Errorf("%+v: expected %q, got: %q", c.e, c.want, got)
		}
		back, err := ParseEntry(got)
		if err != nil || back != c.e {
			t.Errorf("%q: expected to read back %+v, got: %+v, %v", got, c.e, back, err)
		}
	}
}

// TestLookup makes sure that a lookup tries the whole name, then each
// shorter prefix, then the default, and that an entry is whole.
func TestLookup(t *testing.T) {
	s, err := Parse([]byte(`{
  "name": "test",
  "source": "the test",
  "default": "#111111 bg:#000000",
  "captures": {
    "keyword": "bold #ff0000",
    "keyword.function": "#00ff00",
    "string.special.url": "underline"
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "test" || s.Source != "the test" {
		t.Errorf("expected the name and the source of the file, got: %q, %q", s.Name, s.Source)
	}
	if want := (Color{Kind: ColorRGB, Value: 0}); s.Background() != want {
		t.Errorf("expected the background %v, got: %v", want, s.Background())
	}
	for _, c := range []struct {
		capture, want string
	}{
		{"keyword", "bold #ff0000"},
		{"keyword.function", "#00ff00"},
		{"keyword.function.builtin", "#00ff00"},
		{"keyword.return", "bold #ff0000"},
		{"string.special.url", "underline"},
		{"string.special", "#111111"},
		{"string", "#111111"},
		{"variable", "#111111"},
		{"", "#111111"},
		{"keywords", "#111111"},
	} {
		if got := s.Lookup(c.capture).String(); got != c.want {
			t.Errorf("%q: expected %q, got: %q", c.capture, c.want, got)
		}
	}
}

// TestParseErrors makes sure that Parse rejects a file that is not a style.
func TestParseErrors(t *testing.T) {
	for _, data := range []string{
		``,
		`[]`,
		`{"source": "no name"}`,
		`{"name": "x", "colors": {}}`,
		`{"name": "x", "default": "noinherit"}`,
		`{"name": "x", "captures": {"keyword": "#ansipink"}}`,
		`{"name": "x"} {"name": "y"}`,
	} {
		if _, err := Parse([]byte(data)); !errors.Is(err, ErrSyntax) {
			t.Errorf("%q: expected ErrSyntax, got: %v", data, err)
		}
	}
}

// TestColorRGB makes sure that each kind of color gives the values of the
// default palette of xterm.
func TestColorRGB(t *testing.T) {
	for _, c := range []struct {
		c       Color
		r, g, b uint8
	}{
		{Color{}, 0, 0, 0},
		{Color{Kind: ColorRGB, Value: 0x123456}, 0x12, 0x34, 0x56},
		{Color{Kind: ColorANSI, Value: 1}, 0xcd, 0, 0},
		{Color{Kind: ColorANSI, Value: 12}, 0x5c, 0x5c, 0xff},
		{Color{Kind: Color256, Value: 9}, 0xff, 0, 0},
		{Color{Kind: Color256, Value: 16}, 0, 0, 0},
		{Color{Kind: Color256, Value: 196}, 0xff, 0, 0},
		{Color{Kind: Color256, Value: 67}, 95, 135, 175},
		{Color{Kind: Color256, Value: 231}, 0xff, 0xff, 0xff},
		{Color{Kind: Color256, Value: 232}, 8, 8, 8},
		{Color{Kind: Color256, Value: 255}, 238, 238, 238},
	} {
		r, g, b := c.c.RGB()
		if r != c.r || g != c.g || b != c.b {
			t.Errorf("%v: expected %d %d %d, got: %d %d %d", c.c, c.r, c.g, c.b, r, g, b)
		}
	}
}

// TestReduce makes sure that To256 and To16 reduce a color to the nearest
// color of the palette, and keep a color that already fits.
func TestReduce(t *testing.T) {
	rgb := func(v uint32) Color { return Color{Kind: ColorRGB, Value: v} }
	c256 := func(v uint32) Color { return Color{Kind: Color256, Value: v} }
	ansi := func(v uint32) Color { return Color{Kind: ColorANSI, Value: v} }
	for _, c := range []struct {
		in, to256, to16 Color
	}{
		{Color{}, Color{}, Color{}},
		{rgb(0xff0000), c256(196), ansi(9)},
		{rgb(0x000000), c256(16), ansi(0)},
		{rgb(0xffffff), c256(231), ansi(15)},
		{rgb(0x5f87af), c256(67), ansi(8)},
		{rgb(0x808080), c256(244), ansi(8)},
		{rgb(0x272822), c256(235), ansi(0)},
		{ansi(3), ansi(3), ansi(3)},
		{c256(5), c256(5), ansi(5)},
		{c256(196), c256(196), ansi(9)},
		{c256(232), c256(232), ansi(0)},
	} {
		if got := c.in.To256(); got != c.to256 {
			t.Errorf("%v.To256(): expected %v, got: %v", c.in, c.to256, got)
		}
		if got := c.in.To16(); got != c.to16 {
			t.Errorf("%v.To16(): expected %v, got: %v", c.in, c.to16, got)
		}
	}
}

// TestBundledStyles reads every bundled style. Each file is in chroma/ or in
// themes/, its name is the name of its file, and no two files have the same
// name.
func TestBundledStyles(t *testing.T) {
	seen := map[string]string{}
	err := fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		dir, base := path.Split(p)
		if !slices.Contains(folders, strings.TrimSuffix(dir, "/")) {
			t.Errorf("%s: expected a style file in chroma/ or themes/", p)
		}
		name := strings.TrimSuffix(base, ".json")
		if other, ok := seen[name]; ok {
			t.Errorf("%s: expected one style named %s, got also %s", p, name, other)
		}
		seen[name] = p
		data, err := files.ReadFile(p)
		if err != nil {
			return err
		}
		s, err := Parse(data)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			return nil
		}
		if s.Name != name {
			t.Errorf("%s: expected the name %s, got: %s", p, name, s.Name)
		}
		if s.Source == "" {
			t.Errorf("%s: expected a source", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	names := Names()
	if len(names) != len(seen) {
		t.Errorf("expected %d names, got: %d", len(seen), len(names))
	}
	for _, n := range names {
		s, ok := Get(n)
		if !ok || s.Name != n {
			t.Errorf("Get(%q): expected the style, got: %v, %v", n, s, ok)
		}
	}
	for _, n := range []string{"", "nosuchstyle", "chroma/monokai", "../monokai", "monokai.json"} {
		if _, ok := Get(n); ok {
			t.Errorf("Get(%q): expected no style", n)
		}
	}
	if s, ok := Get("monokai"); !ok || s.Lookup("keyword").String() != "#66d9ef" {
		t.Errorf("expected the keyword of monokai to be #66d9ef, got: %v", s)
	}
}

// TestCaptures is the test of D65: each name in captures.txt reaches an
// entry of each bundled style through its prefixes, without the default.
func TestCaptures(t *testing.T) {
	b, err := os.ReadFile("captures.txt")
	if err != nil {
		t.Fatal(err)
	}
	captures := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(captures) == 0 || !slices.IsSorted(captures) {
		t.Fatalf("expected captures.txt to hold names, sorted")
	}
	for _, n := range Names() {
		s, _ := Get(n)
		for _, c := range captures {
			if !reaches(s, c) {
				t.Errorf("%s: expected an entry for %s or one of its prefixes", n, c)
			}
		}
	}
}

// reaches reports whether a capture name or one of its prefixes has an entry
// in the style.
func reaches(s *Style, capture string) bool {
	name := capture
	for {
		if _, ok := s.Captures[name]; ok {
			return true
		}
		i := strings.LastIndexByte(name, '.')
		if i < 0 {
			return false
		}
		name = name[:i]
	}
}
