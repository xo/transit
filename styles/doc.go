// Package styles holds the styles of transit, which give a color and a font
// to each capture name of a highlight query (D65). rline and usql take their
// colors from it, so that the two draw the same code in the same colors.
//
// A style is one JSON file, and the package embeds the files. The files in
// chroma/ are the styles of chroma, which the command test/cmd/chromastyles
// of the test module converts. The files in themes/ are styles that a person
// makes by hand. A consumer can read a style file of its own with Parse.
//
// A style file has four keys:
//
//	{
//	  "name": "monokai",
//	  "source": "chroma v2.27.0 styles/monokai.xml (MIT), taken from Pygments (BSD-2-Clause)",
//	  "default": "#f8f8f2 bg:#272822",
//	  "captures": {
//	    "comment": "#75715e",
//	    "keyword": "#66d9ef",
//	    "keyword.import": "#f92672"
//	  }
//	}
//
// Each value is a style string, a subset of the style strings of chroma.
// ParseEntry describes the words. Lookup gives the entry of a capture name.
// It tries the whole name, then each shorter prefix by dots, and then the
// default entry. An entry is whole. It takes nothing from the entry of a
// prefix.
//
// The background of the default entry is the background of the style.
// Lookup does not give it, because a line editor usually keeps the
// background of the terminal. A consumer that draws the background uses
// Background for each entry that has no background of its own.
//
// A color keeps the kind that the file writes: one of the 16 ANSI colors,
// one of the 256 colors of a terminal, or an RGB color. The consumer chooses
// the color depth of its terminal, and it reduces a color with To256 or
// To16. The package does not look at the terminal.
package styles
