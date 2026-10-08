package main

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The sizes of a diagram, in pixels.
const (
	radius     = 12   // the radius of a curve of a track
	boxHeight  = 22   // the height of a box
	charWidth  = 8.4  // the width of a character of the 14px monospace font
	boxPad     = 10   // the space between the text of a box and its sides
	gap        = 10   // the space between two items of a row or a choice
	labelWidth = 7.2  // the width of a character of the 12px label of a field
	labelRise  = 15   // the space that the label of a field takes over a box
	groupPad   = 8    // the space between a group of a field and its items
	margin     = 12   // the space around a diagram
	markerSize = 20   // the width of the start and the end markers
	arrowSize  = 4.5  // the half height of an arrow
	minWidth   = 200. // the narrowest width that a row wraps to
	// columnMin is the number of branches of a choice of plain boxes over
	// which the choice wraps into columns.
	columnMin = 12
)

// box is a node that the layout placed. The track enters it on the left and
// leaves it on the right, at the height of the rail. The box takes up above
// the rail and down below it.
type box struct {
	w, up, down float64
	draw        func(c *canvas, x, y float64)
}

// canvas collects the elements of an SVG diagram.
type canvas struct {
	b    strings.Builder
	link func(name string) string
}

// num formats v with at most one decimal.
func num(v float64) string {
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64)
}

// path builds the d attribute of a path. It knows the direction of the
// track, so that each curve turns the right way.
type path struct {
	b      strings.Builder
	x, y   float64
	dx, dy float64 // the direction of the track, a unit vector
}

// newPath starts a path at x, y, with the track going right.
func newPath(x, y float64) *path {
	p := &path{x: x, y: y, dx: 1}
	fmt.Fprintf(&p.b, "M%s %s", num(x), num(y))
	return p
}

// to draws a straight track to x, y, which must be on the same row or
// column.
func (p *path) to(x, y float64) *path {
	if x == p.x && y == p.y {
		return p
	}
	if y == p.y {
		fmt.Fprintf(&p.b, "H%s", num(x))
		p.dx, p.dy = math.Copysign(1, x-p.x), 0
	} else {
		fmt.Fprintf(&p.b, "V%s", num(y))
		p.dx, p.dy = 0, math.Copysign(1, y-p.y)
	}
	p.x, p.y = x, y
	return p
}

// turn draws a quarter circle that moves the track by one radius along its
// direction and one radius to the side sx, sy, which is a unit vector.
func (p *path) turn(sx, sy float64) *path {
	ex, ey := p.dx*radius+sx*radius, p.dy*radius+sy*radius
	sweep := 0
	if p.dx*sy-p.dy*sx > 0 {
		sweep = 1
	}
	fmt.Fprintf(&p.b, "a%d %d 0 0 %d %s %s", radius, radius, sweep, num(ex), num(ey))
	p.x, p.y = p.x+ex, p.y+ey
	p.dx, p.dy = sx, sy
	return p
}

// draw writes the path to c.
func (p *path) draw(c *canvas) {
	fmt.Fprintf(&c.b, "<path d=%q/>\n", p.b.String())
}

// line draws a straight track from x0, y0 to x1, y1.
func (c *canvas) line(x0, y0, x1, y1 float64) {
	if x0 != x1 || y0 != y1 {
		newPath(x0, y0).to(x1, y1).draw(c)
	}
}

// arrow draws an arrow at x, y that points along the track, in the
// direction dir: 1 for right and -1 for left.
func (c *canvas) arrow(x, y, dir float64) {
	x = math.Round(x)
	fmt.Fprintf(&c.b, "<path class=\"arrow\" d=\"M%s %sL%s %sL%s %s\"/>\n",
		num(x-dir*arrowSize), num(y-arrowSize), num(x+dir*arrowSize*0.4), num(y), num(x-dir*arrowSize), num(y+arrowSize))
}

// textWidth returns the width of s in characters of width w.
func textWidth(s string, w float64) float64 {
	return float64(utf8.RuneCountInString(s)) * w
}

// layout places the node n in a width of avail or less, where it can.
func layout(n *node, avail float64) *box {
	switch n.kind {
	case kTerm, kNonterm, kExternal:
		return layoutBox(n)
	case kLabel:
		return layoutLabel(n)
	case kSeq:
		return layoutSeq(n, avail)
	case kChoice:
		if len(n.items) > columnMin && plain(n.items) {
			return layoutColumns(n, avail)
		}
		return layoutChoice(n, avail)
	case kOptional:
		return layoutOptional(n, avail)
	case kLoop:
		return layoutLoop(n, avail)
	case kField:
		return layoutField(n, avail)
	}
	return &box{draw: func(*canvas, float64, float64) {}}
}

// plain reports whether each item is one box.
func plain(items []*node) bool {
	for _, it := range items {
		switch it.kind {
		case kTerm, kNonterm, kExternal:
		default:
			return false
		}
	}
	return true
}

// layoutBox places a terminal, the name of a rule or an external token.
func layoutBox(n *node) *box {
	w := math.Ceil(textWidth(n.text, charWidth) + 2*boxPad)
	return &box{w: w, up: boxHeight / 2, down: boxHeight / 2, draw: func(c *canvas, x, y float64) {
		class, rx := n.class, 0.
		switch n.kind {
		case kTerm:
			rx = boxHeight / 2
		case kNonterm:
			class = "nonterminal"
		case kExternal:
			class, rx = "external", 4
		}
		href := ""
		if n.href != "" && c.link != nil {
			href = c.link(n.href)
		}
		if href != "" {
			fmt.Fprintf(&c.b, "<a href=%q xlink:href=%q>", href, href)
		}
		fmt.Fprintf(&c.b, "<g class=%q>", class)
		if n.title != "" {
			fmt.Fprintf(&c.b, "<title>%s</title>", html.EscapeString(n.title))
		}
		fmt.Fprintf(&c.b, "<rect x=\"%s\" y=\"%s\" width=\"%s\" height=\"%d\" rx=\"%s\"/>", num(x), num(y-boxHeight/2), num(w), boxHeight, num(rx))
		fmt.Fprintf(&c.b, "<text x=\"%s\" y=\"%s\">%s</text></g>", num(x+w/2), num(y+5), html.EscapeString(n.text))
		if href != "" {
			c.b.WriteString("</a>")
		}
		c.b.WriteByte('\n')
	}}
}

// layoutLabel places the name of the rule at the start of a diagram.
func layoutLabel(n *node) *box {
	w := math.Ceil(textWidth(n.text, charWidth) + 8)
	return &box{w: w, up: boxHeight / 2, down: boxHeight / 2, draw: func(c *canvas, x, y float64) {
		fmt.Fprintf(&c.b, "<text class=\"comment\" x=\"%s\" y=\"%s\">%s</text>\n", num(x+w/2), num(y+5), html.EscapeString(n.text))
	}}
}

// row places boxes in a row, with a track of gap between them.
func row(boxes []*box) *box {
	r := &box{}
	for i, b := range boxes {
		if i > 0 {
			r.w += gap
		}
		r.w += b.w
		r.up, r.down = max(r.up, b.up), max(r.down, b.down)
	}
	r.draw = func(c *canvas, x, y float64) {
		for i, b := range boxes {
			if i > 0 {
				c.line(x, y, x+gap, y)
				x += gap
			}
			b.draw(c, x, y)
			x += b.w
		}
	}
	return r
}

// layoutSeq places the items of a row. When the row is wider than avail, it
// wraps into several rows, which a track joins from the end of one row to
// the start of the next, as a line of text wraps.
func layoutSeq(n *node, avail float64) *box {
	inner := max(avail-4*radius-gap, minWidth)
	boxes := make([]*box, len(n.items))
	for i, it := range n.items {
		a := inner
		if i == 1 && boxes[0] != nil && n.items[0].kind == kLabel {
			// The first item stays in the row of the name of the rule.
			a = min(a, max(avail-boxes[0].w-gap, minWidth))
		}
		boxes[i] = layout(it, a)
	}
	whole := row(boxes)
	if whole.w <= avail || len(boxes) < 2 {
		return whole
	}
	var rows []*box
	var cur []*box
	w := 0.
	for i, b := range boxes {
		alone := i == 1 && n.items[0].kind == kLabel
		if len(cur) > 0 && w+gap+b.w > inner && !alone {
			rows = append(rows, row(cur))
			cur, w = nil, 0
		}
		if len(cur) > 0 {
			w += gap
		}
		cur = append(cur, b)
		w += b.w
	}
	rows = append(rows, row(cur))
	if len(rows) == 1 {
		return whole
	}
	widest := 0.
	for _, r := range rows {
		widest = max(widest, r.w)
	}
	// The rails of the rows, as offsets from the rail of the first row, and
	// the tracks that go back to the left between them.
	rails := []float64{0}
	backs := make([]float64, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		backs[i-1] = rails[i-1] + max(rows[i-1].down+gap, 2*radius)
		rails = append(rails, backs[i-1]+max(gap+rows[i].up, 2*radius))
	}
	last := len(rows) - 1
	total := widest + 4*radius + gap
	return &box{w: total, up: rows[0].up, down: rails[last] + rows[last].down, draw: func(c *canvas, x, y float64) {
		left := x + 2*radius
		c.line(x, y, left, y)
		for i, r := range rows {
			ry := y + rails[i]
			r.draw(c, left, ry)
			end := left + r.w
			if i < last {
				by := y + backs[i]
				newPath(end, ry).turn(0, 1).to(end+radius, by-radius).turn(-1, 0).
					to(x+2*radius, by).turn(0, 1).to(x+radius, y+rails[i+1]-radius).turn(1, 0).draw(c)
				c.arrow((x+2*radius+end)/2, by, -1)
				continue
			}
			p := newPath(end, ry).to(x+total-2*radius, ry)
			if ry > y {
				p.turn(0, -1).to(x+total-radius, y+radius).turn(1, 0)
			} else {
				p.to(x+total, y)
			}
			p.draw(c)
		}
	}}
}

// layoutChoice places the branches of a choice under each other. The first
// branch is on the rail.
func layoutChoice(n *node, avail float64) *box {
	boxes := make([]*box, len(n.items))
	widest := 0.
	for i, it := range n.items {
		boxes[i] = layout(it, avail-4*radius)
		widest = max(widest, boxes[i].w)
	}
	rails := make([]float64, len(boxes))
	for i := 1; i < len(boxes); i++ {
		rails[i] = rails[i-1] + max(boxes[i-1].down+gap+boxes[i].up, 2*radius)
	}
	last := len(boxes) - 1
	w := widest + 4*radius
	return &box{w: w, up: boxes[0].up, down: rails[last] + boxes[last].down, draw: func(c *canvas, x, y float64) {
		for i, b := range boxes {
			by := y + rails[i]
			if i == 0 {
				c.line(x, y, x+2*radius, y)
				b.draw(c, x+2*radius, y)
				c.line(x+2*radius+b.w, y, x+w, y)
				continue
			}
			newPath(x, y).turn(0, 1).to(x+radius, by-radius).turn(1, 0).draw(c)
			b.draw(c, x+2*radius, by)
			newPath(x+2*radius+b.w, by).to(x+w-2*radius, by).turn(0, -1).to(x+w-radius, y+radius).turn(1, 0).draw(c)
		}
	}}
}

// layoutColumns places a long choice of plain boxes in columns. The track
// enters along the top, goes down into a column, and leaves along a track
// under the columns, so that a path goes through one box only.
func layoutColumns(n *node, avail float64) *box {
	boxes := make([]*box, len(n.items))
	for i, it := range n.items {
		boxes[i] = layout(it, avail)
	}
	widths := func(cols int) ([]float64, int) {
		per := (len(boxes) + cols - 1) / cols
		ws := make([]float64, cols)
		for i, b := range boxes {
			ws[i/per] = max(ws[i/per], b.w)
		}
		return ws, per
	}
	total := func(ws []float64) float64 {
		t := 0.
		for _, cw := range ws {
			t += cw + 3*radius + gap
		}
		return t + 3*radius
	}
	cols := (len(boxes) + columnMin - 1) / columnMin
	ws, per := widths(cols)
	for cols > 1 && total(ws) > avail {
		cols--
		ws, per = widths(cols)
	}
	cols = (len(boxes) + per - 1) / per
	ws = ws[:cols]
	step := float64(boxHeight + gap)
	first := float64(2*radius + boxHeight/2)
	bottom := first + float64(per-1)*step + max(boxHeight/2+gap, 2*radius)
	w := total(ws)
	return &box{w: w, up: radius, down: bottom, draw: func(c *canvas, x, y float64) {
		xc := x
		ye := y + bottom
		for col := range cols {
			for i := col * per; i < min((col+1)*per, len(boxes)); i++ {
				b := boxes[i]
				by := y + first + float64(i-col*per)*step
				newPath(xc, y).turn(0, 1).to(xc+radius, by-radius).turn(1, 0).draw(c)
				b.draw(c, xc+2*radius, by)
				right := xc + 2*radius + ws[col]
				newPath(xc+2*radius+b.w, by).to(right, by).turn(0, 1).to(right+radius, ye-radius).turn(1, 0).draw(c)
			}
			if col < cols-1 {
				next := xc + ws[col] + 3*radius + gap
				c.line(xc, y, next, y)
				c.arrow((xc+next)/2+radius, y, 1)
				xc = next
			}
		}
		start := x + ws[0] + 4*radius
		newPath(start, ye).to(x+w-2*radius, ye).turn(0, -1).to(x+w-radius, y+radius).turn(1, 0).draw(c)
		c.arrow((start+x+w-2*radius)/2, ye, 1)
	}}
}

// layoutOptional places an item on the rail, with a track over it that
// goes around it.
func layoutOptional(n *node, avail float64) *box {
	b := layout(n.items[0], avail-4*radius)
	over := max(b.up+gap, 2*radius)
	w := b.w + 4*radius
	return &box{w: w, up: over + arrowSize, down: b.down, draw: func(c *canvas, x, y float64) {
		c.line(x, y, x+2*radius, y)
		b.draw(c, x+2*radius, y)
		c.line(x+2*radius+b.w, y, x+w, y)
		top := y - over
		newPath(x, y).turn(0, -1).to(x+radius, top+radius).turn(1, 0).to(x+w-2*radius, top).
			turn(0, 1).to(x+w-radius, y-radius).turn(1, 0).draw(c)
		c.arrow(x+w/2, top, 1)
	}}
}

// layoutLoop places an item on the rail, with a track under it that goes
// back to its start. The separator is on that track.
func layoutLoop(n *node, avail float64) *box {
	b := layout(n.items[0], avail-2*radius)
	var sep *box
	sepUp, sepDown, sepW := 0., 0., 0.
	if n.sep != nil {
		sep = layout(n.sep, avail-2*radius)
		sepUp, sepDown, sepW = sep.up, sep.down, sep.w
	}
	inner := max(b.w, sepW)
	if sep == nil {
		// Leave room for the arrow on the track back.
		inner = max(inner, 2*gap)
	}
	back := max(b.down+gap+sepUp, 2*radius)
	w := inner + 2*radius
	return &box{w: w, up: b.up, down: back + max(sepDown, arrowSize), draw: func(c *canvas, x, y float64) {
		c.line(x, y, x+radius, y)
		b.draw(c, x+radius, y)
		c.line(x+radius+b.w, y, x+w, y)
		by := y + back
		p := newPath(x+w-radius, y)
		p.turn(0, 1).to(x+w, by-radius).turn(-1, 0)
		if sep == nil {
			p.to(x+radius, by).turn(0, -1).to(x, y+radius).turn(1, 0).draw(c)
			c.arrow(x+w/2, by, -1)
			return
		}
		sx := x + radius + (inner-sep.w)/2
		p.to(sx+sep.w, by).draw(c)
		sep.draw(c, sx, by)
		newPath(sx, by).to(x+radius, by).turn(0, -1).to(x, y+radius).turn(1, 0).draw(c)
		if sx-(x+radius) >= 3*arrowSize {
			c.arrow((sx+x+radius)/2, by, -1)
		}
	}}
}

// layoutField places an item with the name of its field over it. A field
// of more than one box has a dashed frame around its items.
func layoutField(n *node, avail float64) *box {
	item := n.items[0]
	lw := math.Ceil(textWidth(n.text, labelWidth))
	switch item.kind {
	case kTerm, kNonterm, kExternal:
		b := layout(item, avail)
		w := max(b.w, lw+4)
		return &box{w: w, up: b.up + labelRise, down: b.down, draw: func(c *canvas, x, y float64) {
			b.draw(c, x, y)
			c.line(x+b.w, y, x+w, y)
			fmt.Fprintf(&c.b, "<text class=\"field\" x=\"%s\" y=\"%s\">%s</text>\n", num(x+2), num(y-b.up-4), html.EscapeString(n.text))
		}}
	}
	b := layout(item, avail-2*groupPad)
	w := max(b.w, lw+4) + 2*groupPad
	up := b.up + groupPad + labelRise
	down := b.down + groupPad
	return &box{w: w, up: up, down: down, draw: func(c *canvas, x, y float64) {
		fmt.Fprintf(&c.b, "<g class=\"group\"><rect x=\"%s\" y=\"%s\" width=\"%s\" height=\"%s\" rx=\"4\"/>", num(x), num(y-up+4), num(w), num(up+down-4))
		fmt.Fprintf(&c.b, "<text class=\"field\" x=\"%s\" y=\"%s\">%s</text></g>\n", num(x+groupPad), num(y-up+labelRise+1), html.EscapeString(n.text))
		c.line(x, y, x+groupPad, y)
		b.draw(c, x+groupPad, y)
		c.line(x+groupPad+b.w, y, x+w, y)
	}}
}

// diagram draws n as an SVG element of the class railroad, with a start
// marker and an end marker, in a width of width or less where it can. The
// function link returns the URL of a rule, or "" for no link. A non-empty
// css is put in the SVG element as its style sheet.
func diagram(n *node, width float64, link func(string) string, css string) string {
	b := layout(n, width-2*margin-2*markerSize)
	w := b.w + 2*margin + 2*markerSize
	h := b.up + b.down + 2*margin
	c := &canvas{link: link}
	fmt.Fprintf(&c.b, "<svg xmlns=\"http://www.w3.org/2000/svg\" xmlns:xlink=\"http://www.w3.org/1999/xlink\" class=\"railroad\" width=\"%s\" height=\"%s\" viewBox=\"0 0 %s %s\">\n", num(w), num(h), num(w), num(h))
	if css != "" {
		fmt.Fprintf(&c.b, "<style>\n%s</style>\n", css)
	}
	c.b.WriteString("<rect class=\"railroad_canvas\" width=\"100%\" height=\"100%\"/>\n")
	x, y := float64(margin), margin+b.up
	// The start marker is a circle with a short track, and so is the end.
	fmt.Fprintf(&c.b, "<circle class=\"marker\" cx=\"%s\" cy=\"%s\" r=\"5\"/>\n", num(x+5), num(y))
	c.line(x+10, y, x+markerSize, y)
	b.draw(c, x+markerSize, y)
	ex := x + markerSize + b.w
	c.line(ex, y, ex+markerSize-10, y)
	fmt.Fprintf(&c.b, "<circle class=\"marker\" cx=\"%s\" cy=\"%s\" r=\"5\"/>\n", num(ex+markerSize-5), num(y))
	c.b.WriteString("</svg>\n")
	return c.b.String()
}
