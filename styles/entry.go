package styles

import (
	"fmt"
	"strings"
)

// Error is an error of the package.
type Error string

// Error returns the text of the error.
func (e Error) Error() string {
	return string(e)
}

// ErrSyntax is the error of a style string or a style file that the package
// cannot read. The error that comes back wraps it, and says what is wrong.
const ErrSyntax Error = "invalid style"

// Entry is the style of one capture name: a foreground color, a background
// color, and the font. An entry is whole. A field that it leaves out does
// not come from another entry.
type Entry struct {
	Fg        Color
	Bg        Color
	Bold      bool
	Italic    bool
	Underline bool
}

// ParseEntry reads a style string. The words of the string are separated by
// spaces, and a later word wins over an earlier one. A word is one of these:
//
//   - bold, italic or underline, which turns the font on
//   - nobold, noitalic or nounderline, which turns it off
//   - a color, which is the foreground color
//   - bg: and a color, which is the background color
//
// A color is #rgb or #rrggbb, #ansi and the name of one of the 16 ANSI
// colors of chroma, such as #ansired or #ansidarkblue, or #ansi and the
// number of one of the 256 colors of a terminal, such as #ansi208. The
// empty string is the zero Entry.
func ParseEntry(s string) (Entry, error) {
	var e Entry
	for word := range strings.FieldsSeq(s) {
		switch word {
		case "bold":
			e.Bold = true
		case "nobold":
			e.Bold = false
		case "italic":
			e.Italic = true
		case "noitalic":
			e.Italic = false
		case "underline":
			e.Underline = true
		case "nounderline":
			e.Underline = false
		default:
			text, isBg := strings.CutPrefix(word, "bg:")
			if !isBg && !strings.HasPrefix(word, "#") {
				return Entry{}, fmt.Errorf("%w: unknown word %q in %q", ErrSyntax, word, s)
			}
			c, err := parseColor(text)
			if err != nil {
				return Entry{}, fmt.Errorf("reading %q: %w", s, err)
			}
			if isBg {
				e.Bg = c
			} else {
				e.Fg = c
			}
		}
	}
	return e, nil
}

// String returns the entry as a style string, in the order of chroma: the
// font, the foreground color, and the background color. ParseEntry reads it
// back to the same entry.
func (e Entry) String() string {
	var words []string
	if e.Bold {
		words = append(words, "bold")
	}
	if e.Italic {
		words = append(words, "italic")
	}
	if e.Underline {
		words = append(words, "underline")
	}
	if e.Fg.IsSet() {
		words = append(words, e.Fg.String())
	}
	if e.Bg.IsSet() {
		words = append(words, "bg:"+e.Bg.String())
	}
	return strings.Join(words, " ")
}
