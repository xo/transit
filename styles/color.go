package styles

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ColorKind is the kind of a color: none, one of the 16 ANSI colors, one of
// the 256 colors of a terminal, or an RGB color.
type ColorKind uint8

// The kinds of a color.
const (
	// ColorNone is the kind of a color that is not set.
	ColorNone ColorKind = iota
	// ColorANSI is one of the 16 ANSI colors. Its value is 0 to 15.
	ColorANSI
	// Color256 is one of the 256 colors of a terminal. Its value is 0 to
	// 255.
	Color256
	// ColorRGB is an RGB color. Its value is 0xrrggbb.
	ColorRGB
)

// Color is a color of a style entry. The zero Color is not set.
type Color struct {
	Kind  ColorKind
	Value uint32
}

// ansiNames are the names of the 16 ANSI colors in a style string, in the
// order of their numbers. They are the names of chroma. A name with "dark"
// is one of the first 8 colors, and the name without it is the bright color
// of the last 8.
var ansiNames = [16]string{
	"ansiblack",
	"ansidarkred",
	"ansidarkgreen",
	"ansibrown",
	"ansidarkblue",
	"ansipurple",
	"ansiteal",
	"ansilightgray",
	"ansidarkgray",
	"ansired",
	"ansigreen",
	"ansiyellow",
	"ansiblue",
	"ansifuchsia",
	"ansiturquoise",
	"ansiwhite",
}

// parseColor reads a color of a style string: #rgb, #rrggbb, #ansi and the
// name of an ANSI color, or #ansi and the number of one of the 256 colors.
func parseColor(s string) (Color, error) {
	hex, ok := strings.CutPrefix(s, "#")
	if !ok {
		return Color{}, fmt.Errorf("%w: the color %q does not start with #", ErrSyntax, s)
	}
	if name, ok := strings.CutPrefix(hex, "ansi"); ok {
		for i, n := range ansiNames {
			if n == hex {
				return Color{Kind: ColorANSI, Value: uint32(i)}, nil
			}
		}
		if name != "" && strings.Trim(name, "0123456789") == "" && (name == "0" || name[0] != '0') {
			if n, err := strconv.ParseUint(name, 10, 8); err == nil {
				return Color{Kind: Color256, Value: uint32(n)}, nil
			}
		}
		return Color{}, fmt.Errorf("%w: the color %q is not an ANSI color", ErrSyntax, s)
	}
	switch len(hex) {
	case 3:
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	case 6:
	default:
		return Color{}, fmt.Errorf("%w: the color %q does not have 3 or 6 hex digits", ErrSyntax, s)
	}
	if strings.Trim(hex, "0123456789abcdefABCDEF") != "" {
		return Color{}, fmt.Errorf("%w: the color %q has a digit that is not hex", ErrSyntax, s)
	}
	n, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("%w: the color %q: %w", ErrSyntax, s, err)
	}
	return Color{Kind: ColorRGB, Value: uint32(n)}, nil
}

// IsSet reports whether the color is set.
func (c Color) IsSet() bool {
	return c.Kind != ColorNone
}

// String returns the color as a style string writes it: #rrggbb, #ansi and
// the name of an ANSI color, or #ansi and the number of one of the 256
// colors. It returns "" for a color that is not set.
func (c Color) String() string {
	switch c.Kind {
	case ColorANSI:
		return "#" + ansiNames[c.Value&15]
	case Color256:
		return "#ansi" + strconv.FormatUint(uint64(c.Value&255), 10)
	case ColorRGB:
		return fmt.Sprintf("#%06x", c.Value&0xffffff)
	default:
		return ""
	}
}

// RGB returns the red, green and blue parts of the color. An ANSI color and
// one of the 256 colors take the values of the default palette of xterm. A
// color that is not set is black.
func (c Color) RGB() (r, g, b uint8) {
	var v uint32
	switch c.Kind {
	case ColorANSI:
		v = xterm(c.Value & 15)
	case Color256:
		v = xterm(c.Value & 255)
	case ColorRGB:
		v = c.Value
	default:
		return 0, 0, 0
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v)
}

// To256 reduces the color to the 256 colors of a terminal. An RGB color
// becomes the nearest of the colors 16 to 255, because a terminal can change
// the first 16. Any other color comes back as it is.
func (c Color) To256() Color {
	if c.Kind != ColorRGB {
		return c
	}
	return Color{Kind: Color256, Value: nearest(c, 16, 256)}
}

// To16 reduces the color to the 16 ANSI colors. One of the first 16 of the
// 256 colors becomes the ANSI color of the same number. Another of the 256
// colors and an RGB color become the nearest ANSI color in the default
// palette of xterm. An ANSI color and a color that is not set come back as
// they are.
func (c Color) To16() Color {
	switch {
	case c.Kind == Color256 && c.Value < 16:
		return Color{Kind: ColorANSI, Value: c.Value}
	case c.Kind == Color256 || c.Kind == ColorRGB:
		return Color{Kind: ColorANSI, Value: nearest(c, 0, 16)}
	default:
		return c
	}
}

// nearest returns the number of the color of the xterm palette, from lo to
// hi-1, that is nearest to c. A tie goes to the lower number. The distance
// is the one that chroma uses to reduce a color, a weighted Euclidean
// distance from https://www.compuphase.com/cmetric.htm.
func nearest(c Color, lo, hi uint32) uint32 {
	r, g, b := c.RGB()
	best, bestDist := lo, math.Inf(1)
	for i := lo; i < hi; i++ {
		p := xterm(i)
		if d := distance(r, g, b, uint8(p>>16), uint8(p>>8), uint8(p)); d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

// distance is the Distance of a Colour of chroma.
func distance(ar, ag, ab, br, bg, bb uint8) float64 {
	rmean := (int64(ar) + int64(br)) / 2
	r := int64(ar) - int64(br)
	g := int64(ag) - int64(bg)
	b := int64(ab) - int64(bb)
	return math.Sqrt(float64((((512 + rmean) * r * r) >> 8) + 4*g*g + (((767 - rmean) * b * b) >> 8)))
}

// xtermBase is the default palette of xterm for the 16 ANSI colors.
var xtermBase = [16]uint32{
	0x000000, 0xcd0000, 0x00cd00, 0xcdcd00, 0x0000ee, 0xcd00cd, 0x00cdcd, 0xe5e5e5,
	0x7f7f7f, 0xff0000, 0x00ff00, 0xffff00, 0x5c5cff, 0xff00ff, 0x00ffff, 0xffffff,
}

// cubeLevels are the six levels of each part of the color cube of the 256
// colors, the colors 16 to 231.
var cubeLevels = [6]uint32{0, 95, 135, 175, 215, 255}

// xterm returns the RGB value of the color i of the 256 colors of xterm.
func xterm(i uint32) uint32 {
	switch {
	case i < 16:
		return xtermBase[i]
	case i < 232:
		i -= 16
		return cubeLevels[i/36]<<16 | cubeLevels[i/6%6]<<8 | cubeLevels[i%6]
	default:
		v := 8 + 10*(i-232)
		return v<<16 | v<<8 | v
	}
}
