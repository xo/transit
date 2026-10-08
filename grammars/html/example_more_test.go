package html_test

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/xo/transit"
	"github.com/xo/transit/grammars/html"
	"github.com/xo/transit/grammars/javascript"
	"github.com/xo/transit/inject"
)

// configs returns the injection configurations of HTML and of JavaScript,
// with the injection query of each grammar.
func configs() (htmlConfig, jsConfig *inject.Config) {
	htmlQuery, err := html.Queries.ReadFile("queries/injections.scm")
	if err != nil {
		log.Fatal(err)
	}
	htmlConfig, err = inject.NewConfig(html.Language(), "html", string(htmlQuery))
	if err != nil {
		log.Fatal(err)
	}
	jsQuery, err := javascript.Queries.ReadFile("queries/injections.scm")
	if err != nil {
		log.Fatal(err)
	}
	jsConfig, err = inject.NewConfig(javascript.Language(), "javascript", string(jsQuery))
	if err != nil {
		log.Fatal(err)
	}
	return htmlConfig, jsConfig
}

// This example parses an HTML text with a script. The injection query of
// HTML says that the text of a script is JavaScript, so Layers gives a layer
// of HTML and a layer of JavaScript. The function lookup gives the
// configuration of each language name that an injection query names. It
// gives false for the names that the program does not know, such as css.
func Example_layers() {
	htmlConfig, jsConfig := configs()
	lookup := func(name string) (*inject.Config, bool) {
		switch name {
		case "html":
			return htmlConfig, true
		case "javascript":
			return jsConfig, true
		}
		return nil, false
	}
	src := []byte("<p>Hi</p><script>alert(1)</script>")
	layers, err := htmlConfig.Layers(context.Background(), transit.NewParser(), src, lookup)
	if err != nil {
		log.Fatal(err)
	}
	for _, l := range layers {
		root := l.Tree.RootNode()
		fmt.Printf("%d %s %q\n", l.Depth, l.Name, root.Text(src))
		fmt.Printf("  %s\n", root)
	}
	// Output:
	// 0 html "<p>Hi</p><script>alert(1)</script>"
	//   (document (element (start_tag (tag_name)) (text) (end_tag (tag_name))) (script_element (start_tag (tag_name)) (raw_text) (end_tag (tag_name))))
	// 1 javascript "alert(1)"
	//   (program (expression_statement (call_expression function: (identifier) arguments: (arguments (number)))))
}

// This example parses a template whose script holds a tag {{ name }} that
// the server replaces. The tag is not JavaScript, so the layer of the script
// has an error. A replacer gives the text of the script with spaces in
// place of the braces before the layer is parsed, and the layer has no
// error. The new text has
// the length of the old text, so each offset of the layer is the offset of
// the template.
func Example_replacer() {
	htmlConfig, jsConfig := configs()
	lookup := func(name string) (*inject.Config, bool) {
		return jsConfig, name == "javascript"
	}
	braces := func(name string, n transit.Node, src []byte) ([]byte, bool) {
		if name != "javascript" || n.Kind() != "raw_text" {
			return nil, false
		}
		text := src[n.StartByte():n.EndByte()]
		text = bytes.ReplaceAll(text, []byte("{{"), []byte("  "))
		text = bytes.ReplaceAll(text, []byte("}}"), []byte("  "))
		return text, true
	}
	src := []byte("<script>let user = {{ name }};</script>")
	for _, test := range []struct {
		name string
		opts []inject.Option
	}{
		{"without the replacer", nil},
		{"with the replacer", []inject.Option{inject.WithReplacer(braces)}},
	} {
		layers, err := htmlConfig.Layers(context.Background(), transit.NewParser(), src, lookup, test.opts...)
		if err != nil {
			log.Fatal(err)
		}
		js := layers[len(layers)-1]
		fmt.Printf("%s: %s\n", test.name, js.Tree.RootNode())
	}
	// Output:
	// without the replacer: (program (lexical_declaration (variable_declarator name: (identifier) value: (object (ERROR (object_pattern (shorthand_property_identifier_pattern)))))))
	// with the replacer: (program (lexical_declaration (variable_declarator name: (identifier) value: (identifier))))
}
