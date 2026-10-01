package main

// This file ports the parts of chroma v2.27.0 that resolve the entry of a
// token type: style.go, colour.go and the token types of types.go and
// tokentype_enumer.go. The command reads the XML files of chroma and does
// not import chroma (D65).

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// tokenType is a TokenType of chroma. A category is a range of 1000, and a
// subcategory a range of 100.
type tokenType int

// The token types that the resolution names.
const (
	background tokenType = -1
	text       tokenType = 8000
)

// tokenTypes holds the value of each token type of chroma by its name, as
// tokentype_enumer.go lists them.
var tokenTypes = map[string]tokenType{
	"Ignore":                   -14,
	"None":                     -13,
	"Other":                    -12,
	"Error":                    -11,
	"CodeLine":                 -10,
	"LineLink":                 -9,
	"LineTableTD":              -8,
	"LineTable":                -7,
	"LineHighlight":            -6,
	"LineNumbersTable":         -5,
	"LineNumbers":              -4,
	"Line":                     -3,
	"PreWrapper":               -2,
	"Background":               -1,
	"EOFType":                  0,
	"Keyword":                  1000,
	"KeywordConstant":          1001,
	"KeywordDeclaration":       1002,
	"KeywordNamespace":         1003,
	"KeywordPseudo":            1004,
	"KeywordReserved":          1005,
	"KeywordType":              1006,
	"Name":                     2000,
	"NameAttribute":            2001,
	"NameClass":                2002,
	"NameConstant":             2003,
	"NameDecorator":            2004,
	"NameEntity":               2005,
	"NameException":            2006,
	"NameKeyword":              2007,
	"NameLabel":                2008,
	"NameNamespace":            2009,
	"NameOperator":             2010,
	"NameOther":                2011,
	"NamePseudo":               2012,
	"NameProperty":             2013,
	"NameTag":                  2014,
	"NameBuiltin":              2100,
	"NameBuiltinPseudo":        2101,
	"NameVariable":             2200,
	"NameVariableAnonymous":    2201,
	"NameVariableClass":        2202,
	"NameVariableGlobal":       2203,
	"NameVariableInstance":     2204,
	"NameVariableMagic":        2205,
	"NameFunction":             2300,
	"NameFunctionMagic":        2301,
	"Literal":                  3000,
	"LiteralDate":              3001,
	"LiteralOther":             3002,
	"LiteralString":            3100,
	"LiteralStringAffix":       3101,
	"LiteralStringAtom":        3102,
	"LiteralStringBacktick":    3103,
	"LiteralStringBoolean":     3104,
	"LiteralStringChar":        3105,
	"LiteralStringDelimiter":   3106,
	"LiteralStringDoc":         3107,
	"LiteralStringDouble":      3108,
	"LiteralStringEscape":      3109,
	"LiteralStringHeredoc":     3110,
	"LiteralStringInterpol":    3111,
	"LiteralStringName":        3112,
	"LiteralStringOther":       3113,
	"LiteralStringRegex":       3114,
	"LiteralStringSingle":      3115,
	"LiteralStringSymbol":      3116,
	"LiteralNumber":            3200,
	"LiteralNumberBin":         3201,
	"LiteralNumberFloat":       3202,
	"LiteralNumberHex":         3203,
	"LiteralNumberInteger":     3204,
	"LiteralNumberIntegerLong": 3205,
	"LiteralNumberOct":         3206,
	"LiteralNumberByte":        3207,
	"Operator":                 4000,
	"OperatorWord":             4001,
	"OperatorReserved":         4002,
	"Punctuation":              5000,
	"Comment":                  6000,
	"CommentHashbang":          6001,
	"CommentMultiline":         6002,
	"CommentSingle":            6003,
	"CommentSpecial":           6004,
	"CommentPreproc":           6100,
	"CommentPreprocFile":       6101,
	"Generic":                  7000,
	"GenericDeleted":           7001,
	"GenericEmph":              7002,
	"GenericError":             7003,
	"GenericHeading":           7004,
	"GenericInserted":          7005,
	"GenericOutput":            7006,
	"GenericPrompt":            7007,
	"GenericStrong":            7008,
	"GenericSubheading":        7009,
	"GenericTraceback":         7010,
	"GenericUnderline":         7011,
	"Text":                     8000,
	"TextWhitespace":           8001,
	"TextSymbol":               8002,
	"TextPunctuation":          8003,
}

// tokenTypeString is TokenTypeString of tokentype_enumer.go. It takes the
// name as it is, or the name in lower case.
func tokenTypeString(s string) (tokenType, error) {
	if t, ok := tokenTypes[s]; ok {
		return t, nil
	}
	for name, t := range tokenTypes {
		if strings.EqualFold(name, s) {
			return t, nil
		}
	}
	return 0, fmt.Errorf("%s does not belong to TokenType values", s)
}

// category is TokenType.Category.
func (t tokenType) category() tokenType {
	return t / 1000 * 1000
}

// subCategory is TokenType.SubCategory.
func (t tokenType) subCategory() tokenType {
	return t / 100 * 100
}

// colour is a Colour of chroma: the RGB value plus 1, and 0 when it is not
// set.
type colour int32

// ansi2RGB is ANSI2RGB of colour.go.
var ansi2RGB = map[string]string{
	"#ansiblack":     "000000",
	"#ansidarkred":   "7f0000",
	"#ansidarkgreen": "007f00",
	"#ansibrown":     "7f7fe0",
	"#ansidarkblue":  "00007f",
	"#ansipurple":    "7f007f",
	"#ansiteal":      "007f7f",
	"#ansilightgray": "e5e5e5",
	"#ansidarkgray":  "555555",
	"#ansired":       "ff0000",
	"#ansigreen":     "00ff00",
	"#ansiyellow":    "ffff00",
	"#ansiblue":      "0000ff",
	"#ansifuchsia":   "ff00ff",
	"#ansiturquoise": "00ffff",
	"#ansiwhite":     "ffffff",
	"#black":         "000000",
	"#darkred":       "7f0000",
	"#darkgreen":     "007f00",
	"#brown":         "7f7fe0",
	"#darkblue":      "00007f",
	"#purple":        "7f007f",
	"#teal":          "007f7f",
	"#lightgray":     "e5e5e5",
	"#darkgray":      "555555",
	"#red":           "ff0000",
	"#green":         "00ff00",
	"#yellow":        "ffff00",
	"#blue":          "0000ff",
	"#fuchsia":       "ff00ff",
	"#turquoise":     "00ffff",
	"#white":         "ffffff",
}

// parseColour is ParseColour. It returns a colour that is not set when s is
// not a colour.
func parseColour(s string) colour {
	s = normaliseColour(s)
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0
	}
	return colour(n + 1)
}

// normaliseColour is normaliseColour of colour.go.
func normaliseColour(s string) string {
	if ansi, ok := ansi2RGB[s]; ok {
		return ansi
	}
	if strings.HasPrefix(s, "#") {
		s = s[1:]
		if len(s) == 3 {
			return s[0:1] + s[0:1] + s[1:2] + s[1:2] + s[2:3] + s[2:3]
		}
	}
	return s
}

// isSet is Colour.IsSet.
func (c colour) isSet() bool {
	return c != 0
}

// rgb returns the RGB value of a colour that is set.
func (c colour) rgb() uint32 {
	return uint32(c - 1)
}

// trilean is Trilean: a font that the entry passes on, turns on or turns
// off.
type trilean uint8

// The values of a trilean.
const (
	pass trilean = iota
	yes
	no
)

// entry is a StyleEntry of chroma.
type entry struct {
	colour     colour
	background colour
	border     colour
	bold       trilean
	italic     trilean
	underline  trilean
	noInherit  bool
}

// inherit is StyleEntry.Inherit. The ancestors come from the oldest to the
// newest.
func (e entry) inherit(ancestors ...entry) entry {
	out := e
	for _, ancestor := range slices.Backward(ancestors) {
		if out.noInherit {
			return out
		}
		if !out.colour.isSet() {
			out.colour = ancestor.colour
		}
		if !out.background.isSet() {
			out.background = ancestor.background
		}
		if !out.border.isSet() {
			out.border = ancestor.border
		}
		if out.bold == pass {
			out.bold = ancestor.bold
		}
		if out.italic == pass {
			out.italic = ancestor.italic
		}
		if out.underline == pass {
			out.underline = ancestor.underline
		}
	}
	return out
}

// parseStyleEntry is ParseStyleEntry.
func parseStyleEntry(s string) (entry, error) {
	var out entry
	for part := range strings.FieldsSeq(s) {
		switch {
		case part == "italic":
			out.italic = yes
		case part == "noitalic":
			out.italic = no
		case part == "bold":
			out.bold = yes
		case part == "nobold":
			out.bold = no
		case part == "underline":
			out.underline = yes
		case part == "nounderline":
			out.underline = no
		case part == "inherit":
			out.noInherit = false
		case part == "noinherit":
			out.noInherit = true
		case part == "bg:":
			out.background = 0
		case strings.HasPrefix(part, "bg:#"):
			out.background = parseColour(part[3:])
			if !out.background.isSet() {
				return entry{}, fmt.Errorf("invalid background colour %q", part)
			}
		case strings.HasPrefix(part, "border:#"):
			out.border = parseColour(part[7:])
			if !out.border.isSet() {
				return entry{}, fmt.Errorf("invalid border colour %q", part)
			}
		case strings.HasPrefix(part, "#"):
			out.colour = parseColour(part)
			if !out.colour.isSet() {
				return entry{}, fmt.Errorf("invalid colour %q", part)
			}
		default:
			return entry{}, fmt.Errorf("unknown style element %q", part)
		}
	}
	return out, nil
}

// style is a Style of chroma. The styles of the XML files have no parent.
type style struct {
	name    string
	entries map[tokenType]entry
}

// parseStyle reads an XML style of chroma, as Style.UnmarshalXML does.
func parseStyle(r io.Reader) (*style, error) {
	dec := xml.NewDecoder(r)
	var start xml.StartElement
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("reading the XML style: %w", err)
		}
		if el, ok := tok.(xml.StartElement); ok {
			start = el
			break
		}
	}
	if start.Name.Local != "style" {
		return nil, fmt.Errorf("reading the XML style: expected the element style, got %s", start.Name.Local)
	}
	s := &style{entries: map[tokenType]entry{}}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "name":
			s.name = attr.Value
		case "counterpart":
		default:
			return nil, fmt.Errorf("unexpected attribute %s", attr.Name.Local)
		}
	}
	if s.name == "" {
		return nil, errors.New("missing style name attribute")
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("reading the XML style %s: %w", s.name, err)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			if el.Name.Local != "entry" {
				return nil, fmt.Errorf("unexpected element %s", el.Name.Local)
			}
			var t tokenType
			var e entry
			for _, attr := range el.Attr {
				switch attr.Name.Local {
				case "type":
					if t, err = tokenTypeString(attr.Value); err != nil {
						return nil, err
					}
				case "style":
					if e, err = parseStyleEntry(attr.Value); err != nil {
						return nil, err
					}
				default:
					return nil, fmt.Errorf("unexpected attribute %s", attr.Name.Local)
				}
			}
			s.entries[t] = e
		case xml.EndElement:
			if el.Name.Local == start.Name.Local {
				return s, nil
			}
		}
	}
}

// resolve is Style.Get: the entry of a token type, which inherits from its
// subcategory, its category, Text and Background, in that order.
func (s *style) resolve(t tokenType) entry {
	return s.get(t).inherit(
		s.get(background),
		s.get(text),
		s.get(t.category()),
		s.get(t.subCategory()))
}

// get is Style.get, for a style with no parent. The command never resolves
// a type that chroma synthesises, such as LineNumbers, so get does not
// synthesise one.
func (s *style) get(t tokenType) entry {
	return s.entries[t]
}

// withoutBackground returns the style with the background of its
// Background entry taken out, so that an entry has a background only when
// it or one of the types that it inherits from other than Background sets
// one. The background of the style applies only when the consumer asks for
// it (D65).
//
// clearBackground of formatters/tty_indexed.go of chroma does this for a
// terminal, but it puts the cleared entry in a style whose parent is the
// first style. When the Background entry has only a background, the cleared
// entry is zero, the parent gives the background again, and each entry
// keeps it. withoutBackground leaves out that fault.
func withoutBackground(s *style) *style {
	entries := maps.Clone(s.entries)
	bg := entries[background]
	bg.background = 0
	entries[background] = bg
	return &style{name: s.name, entries: entries}
}
