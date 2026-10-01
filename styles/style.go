package styles

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// Style is a style: an entry for each capture name that it gives, and the
// default entry.
type Style struct {
	// Name is the name of the style, such as "monokai".
	Name string
	// Source says where the colors of the style come from, and under which
	// license.
	Source string
	// Default is the entry of text that no capture names. Its background is
	// the background of the style.
	Default Entry
	// Captures holds the entry of each capture name that the style gives.
	Captures map[string]Entry
}

// file is the JSON form of a style file.
type file struct {
	Name     string            `json:"name"`
	Source   string            `json:"source"`
	Default  string            `json:"default"`
	Captures map[string]string `json:"captures"`
}

// Parse reads a style file. The file is a JSON object with the keys name,
// source, default and captures. The name must not be empty, and each value
// of default and captures is a style string that ParseEntry reads.
func Parse(data []byte) (*Style, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f file
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%w: decoding the style file: %w", ErrSyntax, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: the style file holds more than one JSON value", ErrSyntax)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("%w: the style file has no name", ErrSyntax)
	}
	def, err := ParseEntry(f.Default)
	if err != nil {
		return nil, fmt.Errorf("style %s, the default: %w", f.Name, err)
	}
	s := &Style{
		Name:     f.Name,
		Source:   f.Source,
		Default:  def,
		Captures: make(map[string]Entry, len(f.Captures)),
	}
	for name, v := range f.Captures {
		e, err := ParseEntry(v)
		if err != nil {
			return nil, fmt.Errorf("style %s, the capture %s: %w", f.Name, name, err)
		}
		s.Captures[name] = e
	}
	return s, nil
}

// Lookup returns the entry of a capture name, such as "keyword.function". It
// tries the whole name, then each shorter prefix by dots, such as "keyword",
// and then the default entry. It leaves out the background of the default
// entry, which is the background of the style.
func (s *Style) Lookup(capture string) Entry {
	name := capture
	for {
		if e, ok := s.Captures[name]; ok {
			return e
		}
		i := strings.LastIndexByte(name, '.')
		if i < 0 {
			break
		}
		name = name[:i]
	}
	e := s.Default
	e.Bg = Color{}
	return e
}

// Background returns the background of the style, which is the background
// of the default entry.
func (s *Style) Background() Color {
	return s.Default.Bg
}

// files holds the bundled styles: the converted styles of chroma in chroma/,
// and the styles that a person makes in themes/.
//
//go:embed */*.json
var files embed.FS

// folders are the folders of files, in the order that Get looks in them.
var folders = []string{"chroma", "themes"}

// Get returns the bundled style of a name, such as "monokai". It returns
// false when the package has no style of that name. Each call reads the file
// again, so the caller can change the style that comes back.
func Get(name string) (*Style, bool) {
	if name == "" || strings.ContainsAny(name, "/\\.") {
		return nil, false
	}
	for _, dir := range folders {
		data, err := files.ReadFile(path.Join(dir, name+".json"))
		if err != nil {
			continue
		}
		s, err := Parse(data)
		if err != nil {
			// The test of the package reads every bundled style.
			panic(fmt.Sprintf("styles: reading the bundled style %s: %v", name, err))
		}
		return s, true
	}
	return nil, false
}

// Names returns the names of the bundled styles, sorted.
func Names() []string {
	var names []string
	for _, dir := range folders {
		entries, err := fs.ReadDir(files, dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if n, ok := strings.CutSuffix(e.Name(), ".json"); ok && !e.IsDir() {
				names = append(names, n)
			}
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}
