package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xo/transit/styles"
)

// styleFile is the JSON form of a style file of the module styles.
type styleFile struct {
	Name     string            `json:"name"`
	Source   string            `json:"source"`
	Default  string            `json:"default"`
	Captures map[string]string `json:"captures"`
}

// convert converts each XML style in src, and returns the files to write by
// their path in the module styles: captures.txt, one file in chroma/ for each
// style, and the license of chroma.
func convert(src, version string, captures []string) (map[string][]byte, error) {
	types := make(map[string]tokenType, len(captures))
	var unknown []string
	for _, c := range captures {
		t, ok := captureType(c)
		if !ok {
			unknown = append(unknown, c)
			continue
		}
		types[c] = t
	}
	if len(unknown) != 0 {
		return nil, fmt.Errorf("no token type of chroma for the capture names %s. Add their prefixes to captureTypes", strings.Join(unknown, ", "))
	}
	files := map[string][]byte{
		"captures.txt": []byte(strings.Join(captures, "\n") + "\n"),
	}
	lic, err := os.ReadFile(filepath.Join(src, "..", "COPYING"))
	if err != nil {
		return nil, fmt.Errorf("reading the license of chroma: %w", err)
	}
	files["licenses/chroma/COPYING"] = lic
	paths, err := filepath.Glob(filepath.Join(src, "*.xml"))
	if err != nil {
		return nil, fmt.Errorf("listing the styles of chroma: %w", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("listing the styles of chroma: %s holds no XML file", src)
	}
	for _, p := range paths {
		base := strings.TrimSuffix(filepath.Base(p), ".xml")
		b, err := convertStyle(p, version, captures, types)
		if err != nil {
			return nil, fmt.Errorf("%s.xml: %w", base, err)
		}
		files["chroma/"+base+".json"] = b
	}
	return files, nil
}

// convertStyle converts the XML style at path into a style file.
func convertStyle(path, version string, captures []string, types map[string]tokenType) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the style: %w", err)
	}
	s, err := parseStyle(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(filepath.Base(path), ".xml")
	name := strings.ToLower(s.name)
	if name != base {
		return nil, fmt.Errorf("the style is named %s, not %s", s.name, base)
	}
	source := fmt.Sprintf("chroma %s styles/%s.xml (MIT)", version, base)
	if fromPygments[name] {
		source += ", taken from Pygments (BSD-2-Clause)"
	}
	cleared := withoutBackground(s)
	out := styleFile{
		Name:     name,
		Source:   source,
		Default:  toEntry(s.resolve(background)).String(),
		Captures: map[string]string{},
	}
	want := make(map[string]styles.Entry, len(captures))
	written := map[string]styles.Entry{}
	// A prefix sorts before each name that it starts, so the entry of a
	// prefix is written before the names that can leave out their entry.
	for _, c := range captures {
		e := toEntry(cleared.resolve(types[c]))
		want[c] = e
		if p, ok := prefixEntry(written, c); ok && p == e {
			continue
		}
		written[c] = e
		out.Captures[c] = e.String()
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, fmt.Errorf("encoding the style file: %w", err)
	}
	// The module styles must read the file back to the entries.
	back, err := styles.Parse(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("reading back the style file: %w", err)
	}
	for _, c := range captures {
		if got := back.Lookup(c); got != want[c] {
			return nil, fmt.Errorf("reading back the style file: the capture %s gives %q, not %q", c, got, want[c])
		}
	}
	return buf.Bytes(), nil
}

// prefixEntry returns the entry that the lookup of the module styles gives
// for a capture name through its shorter prefixes, without the default.
func prefixEntry(written map[string]styles.Entry, capture string) (styles.Entry, bool) {
	name := capture
	for {
		i := strings.LastIndexByte(name, '.')
		if i < 0 {
			return styles.Entry{}, false
		}
		name = name[:i]
		if e, ok := written[name]; ok {
			return e, true
		}
	}
}

// toEntry returns a resolved entry of chroma as an entry of the module
// styles. A font that is off and a font that the entry passes on are the
// same in a whole entry. The module has no border, so the border is left
// out.
func toEntry(e entry) styles.Entry {
	var out styles.Entry
	if e.colour.isSet() {
		out.Fg = styles.Color{Kind: styles.ColorRGB, Value: e.colour.rgb()}
	}
	if e.background.isSet() {
		out.Bg = styles.Color{Kind: styles.ColorRGB, Value: e.background.rgb()}
	}
	out.Bold = e.bold == yes
	out.Italic = e.italic == yes
	out.Underline = e.underline == yes
	return out
}

// sortedKeys returns the keys of m, sorted.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
