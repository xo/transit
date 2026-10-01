package styles_test

import (
	"fmt"
	"log"
	"slices"

	"github.com/xo/transit/styles"
)

// This example gets a bundled style, and finds the entry of some capture
// names of a highlight query. A name that the style gives, such as
// keyword.operator, has its own entry. A name that the style does not give,
// such as keyword.function.builtin, takes the entry of its longest prefix. A name
// with no prefix in the style takes the default entry without its
// background, because a line editor keeps the background of the terminal.
func ExampleGet() {
	style, ok := styles.Get("monokai")
	if !ok {
		log.Fatal("no style monokai")
	}
	for _, capture := range []string{"keyword", "keyword.operator", "keyword.function.builtin", "string", "no.such.name"} {
		fmt.Printf("%-25s %s\n", capture, style.Lookup(capture))
	}
	fmt.Println("background", style.Background())
	// Output:
	// keyword                   #66d9ef
	// keyword.operator          #f92672
	// keyword.function.builtin  #66d9ef
	// string                    #e6db74
	// no.such.name              #f8f8f2
	// background #272822
}

// This example lists the names of the bundled styles, which come sorted.
func ExampleNames() {
	names := styles.Names()
	fmt.Println(slices.Contains(names, "monokai"), slices.IsSorted(names))
	// Output:
	// true true
}

// This example reads a style file of a user. The file has the form of a
// bundled style. Each value is a style string, which ParseEntry reads.
func ExampleParse() {
	data := []byte(`{
  "name": "mine",
  "source": "made by hand",
  "default": "#d0d0d0 bg:#1c1c1c",
  "captures": {
    "comment": "italic #808080",
    "keyword": "bold #ansiblue",
    "string": "#ansi114",
    "error": "#ffffff bg:#d70000"
  }
}`)
	style, err := styles.Parse(data)
	if err != nil {
		log.Fatal(err)
	}
	for _, capture := range []string{"comment", "keyword.operator", "string.special.key", "error", "variable"} {
		fmt.Printf("%-18s %s\n", capture, style.Lookup(capture))
	}
	// Output:
	// comment            italic #808080
	// keyword.operator   bold #ansiblue
	// string.special.key #ansi114
	// error              #ffffff bg:#d70000
	// variable           #d0d0d0
}

// This example reduces an RGB color for a terminal of 256 colors.
func ExampleColor_To256() {
	e, err := styles.ParseEntry("#f92672")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(e.Fg, e.Fg.To256())
	// Output:
	// #f92672 #ansi197
}

// This example reduces an RGB color, and one of the 256 colors, for a
// terminal of 16 colors.
func ExampleColor_To16() {
	e, err := styles.ParseEntry("#f92672 bg:#ansi236")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(e.Fg, e.Fg.To16())
	fmt.Println(e.Bg, e.Bg.To16())
	// Output:
	// #f92672 #ansipurple
	// #ansi236 #ansiblack
}
