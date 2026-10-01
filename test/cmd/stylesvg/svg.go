package main

import (
	"fmt"
	"html"
	"strings"
	"unicode/utf8"

	"github.com/xo/transit/styles"
)

// The sizes of a sheet, in pixels. A character of the monospace font takes
// charWidth, so each segment of text starts at the column that it holds.
const (
	fontSize   = 13
	charWidth  = 7.8
	lineHeight = 17
	pad        = 12
	titleSize  = 22
	gap        = 16
	tabWidth   = 4
)

// tile is one sample drawn in one style.
type tile struct {
	title  string
	style  *styles.Style
	sample *sample
}

// segment is text of one line that has one entry and one error mark.
type segment struct {
	col  int
	text string
	name string
	err  bool
}

// lines splits the sample into lines of segments. A tab becomes spaces up to
// the next stop of tabWidth.
func (s *sample) lines() [][]segment {
	var lines [][]segment
	var cur []segment
	col := 0
	for i := 0; i < len(s.src); {
		r, size := utf8.DecodeRune(s.src[i:])
		if r == '\n' {
			lines = append(lines, cur)
			cur, col = nil, 0
			i += size
			continue
		}
		text := string(r)
		if r == '\t' {
			text = strings.Repeat(" ", tabWidth-col%tabWidth)
		}
		name, err := s.names[i], s.errs[i]
		if n := len(cur); n > 0 && cur[n-1].name == name && cur[n-1].err == err {
			cur[n-1].text += text
		} else {
			cur = append(cur, segment{col: col, text: text, name: name, err: err})
		}
		col += utf8.RuneCountInString(text)
		i += size
	}
	if len(cur) > 0 {
		lines = append(lines, cur)
	}
	return lines
}

// width returns the number of columns of the widest line.
func width(lines [][]segment) int {
	w := 0
	for _, l := range lines {
		if n := len(l); n > 0 {
			w = max(w, l[n-1].col+utf8.RuneCountInString(l[n-1].text))
		}
	}
	return w
}

// hex returns the color as #rrggbb, or def when it is not set.
func hex(c styles.Color, def string) string {
	if !c.IsSet() {
		return def
	}
	r, g, b := c.RGB()
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// drawSheet draws the tiles in a grid of cols columns. Every tile of a sheet
// has the size of the largest one.
func drawSheet(tiles []tile, cols int) []byte {
	cols = max(1, min(cols, len(tiles)))
	all := make([][][]segment, len(tiles))
	maxCols, maxLines := 0, 0
	for i, t := range tiles {
		all[i] = t.sample.lines()
		maxCols = max(maxCols, width(all[i]), len(t.title)+len(t.style.Name)+3)
		maxLines = max(maxLines, len(all[i]))
	}
	tw := float64(maxCols)*charWidth + 2*pad
	th := float64(titleSize + maxLines*lineHeight + 2*pad)
	rows := (len(tiles) + cols - 1) / cols
	w := float64(cols)*tw + float64(cols+1)*gap
	h := float64(rows)*th + float64(rows+1)*gap
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f">`+"\n", w, h, w, h)
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="#808080"/>`+"\n")
	fmt.Fprintf(&b, `<style>text{font-family:"DejaVu Sans Mono",Menlo,Consolas,monospace;font-size:%dpx;white-space:pre}</style>`+"\n", fontSize)
	for i, t := range tiles {
		x := gap + float64(i%cols)*(tw+gap)
		y := gap + float64(i/cols)*(th+gap)
		drawTile(&b, t, all[i], x, y, tw, th)
	}
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// drawTile draws one tile with its top left corner at x, y.
func drawTile(b *strings.Builder, t tile, lines [][]segment, x, y, w, h float64) {
	bg := hex(t.style.Background(), "#ffffff")
	fg := hex(t.style.Default.Fg, "#000000")
	fmt.Fprintf(b, `<g><rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="4" fill="%s"/>`+"\n", x, y, w, h, bg)
	title := t.title
	if title != t.style.Name {
		title += " · " + t.style.Name
	}
	fmt.Fprintf(b, `<text x="%.1f" y="%.1f" fill="%s" opacity="0.6" xml:space="preserve">%s</text>`+"\n", x+pad, y+pad+fontSize, fg, html.EscapeString(title))
	top := y + pad + titleSize
	for li, line := range lines {
		base := top + float64(li*lineHeight) + fontSize
		fmt.Fprintf(b, `<text y="%.1f" xml:space="preserve">`, base)
		for _, r := range line {
			e := t.style.Default
			if r.name != "" {
				e = t.style.Lookup(r.name)
			}
			attrs := fmt.Sprintf(`fill="%s"`, hex(e.Fg, fg))
			if e.Bold {
				attrs += ` font-weight="bold"`
			}
			if e.Italic {
				attrs += ` font-style="italic"`
			}
			if e.Underline {
				attrs += ` text-decoration="underline"`
			}
			fmt.Fprintf(b, `<tspan x="%.1f" %s>%s</tspan>`, x+pad+float64(r.col)*charWidth, attrs, html.EscapeString(r.text))
		}
		b.WriteString("</text>\n")
		for _, r := range line {
			if r.err {
				x0 := x + pad + float64(r.col)*charWidth
				x1 := x0 + float64(utf8.RuneCountInString(r.text))*charWidth
				fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#ff0000" stroke-width="2"/>`+"\n", x0, base+3, x1, base+3)
			}
		}
	}
	b.WriteString("</g>\n")
}
