package cgrammar

import (
	"encoding/binary"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/xo/transit/grammars/html"
)

func init() {
	goPackages = append(goPackages, goPackage{"html", html.Language})
}

// htmlScannerInputs are inputs that reach each branch of the scanner of
// tree-sitter-html.
var htmlScannerInputs = []string{
	// void tags and implicit end tags
	"<br><img src=x><input><hr/>text",
	"<p>a<p>b<div>c</div><p>d",
	"<ul><li>a<li>b</ul>",
	"<dl><dt>a<dd>b<dt>c</dl>",
	"<table><tr><td>a<td>b<th>c<tr><td>d</table>",
	"<colgroup><col><col><span></colgroup>",
	"<ruby><rb>a<rt>b<rp>c<rtc>d</ruby>",
	"<select><optgroup><option>a<optgroup><option>b</select>",
	"<html><head><title>t</title><body><p>a",
	"<html><head>",
	"<html>",
	"<body>",
	// closing tags that do not close the topmost element
	"<div><span></div>",
	"<div><p><b></div></p>",
	"</foo>",
	"<div></span></div>",
	"<my-tag>x</my-tag><a:b>y</a:b><x-y><x-z></x-y>",
	"<custom></custom></custom>",
	"<div></>",
	"<div></",
	"<div><",
	// script, style and raw text
	"<script>var a = \"</scr\" + '</SCRIPT' < 1;</script>",
	"<STYLE>a { b: c } </stylex></style>",
	"<script type=module>abc",
	"<style>",
	"<script></script>",
	// comments
	"<!-- a -- b -->x<!-- c --->",
	"<!-- a ->",
	"<!-x>",
	"<!DOCTYPE html><html>",
	"<!--",
	// self-closing tags
	"<div/><br/><x-y/>",
	"<div / >",
	"/>",
	// whitespace, names outside ASCII, and bytes that are not UTF-8
	"\t\n <div>\u00a0<\u00e9l\u00e8ve>x</\u00e9l\u00e8ve></div>",
	"<div\u017f><scr\u0131pt></div>",
	"<script>a</\u017fcript></script>",
	"<\xff>a</\xff>",
	// states that do not fit the buffer: a name longer than 255 bytes, many
	// long custom names, and many tags
	"<" + strings.Repeat("a", 300) + ">x</" + strings.Repeat("a", 300) + ">",
	strings.Repeat("<"+strings.Repeat("x", 60)+">", 20) + "y",
	strings.Repeat("<b>", 1100) + "z",
	"",
}

// TestGoPackageHTMLScannerMatchesC compares the Go scanner of html with the
// C scanner, on every corpus input, and on the inputs of htmlScannerInputs.
func TestGoPackageHTMLScannerMatchesC(t *testing.T) {
	t.Parallel()
	g, examples := loadGoPackage(t, goPackage{"html", html.Language})
	var inputs [][]byte
	for _, e := range examples {
		inputs = append(inputs, e.Input)
	}
	for _, s := range htmlScannerInputs {
		inputs = append(inputs, []byte(s))
	}
	n, calls := compareScanners(t, g, html.Language(), inputs)
	if calls == 0 {
		t.Errorf("the scanner of html was not called on %d inputs", n)
	}
	t.Logf("%d inputs, %d calls", n, calls)
}

// randomHTMLState returns a random state of the scanner of tree-sitter-html
// in the form that serialize writes. The number of tags and the number of
// tags that follow are random, each type is a random byte or the type of a
// custom tag, and each name has random bytes. deserialize of C does not test
// the length of the buffer, so the state holds each byte that the C function
// reads, and some bytes more.
func randomHTMLState(r *rand.Rand) []byte {
	if r.IntN(20) == 0 {
		return nil
	}
	const custom = 126
	serialized := r.IntN(12)
	tagCount := r.IntN(serialized + 4)
	if r.IntN(20) == 0 {
		tagCount = r.IntN(70000)
	}
	buf := binary.LittleEndian.AppendUint16(nil, uint16(serialized))
	buf = binary.LittleEndian.AppendUint16(buf, uint16(tagCount))
	if tagCount > 0 {
		for range serialized {
			typ := byte(r.UintN(256))
			if r.IntN(3) == 0 {
				typ = custom
			}
			buf = append(buf, typ)
			if typ == custom {
				name := make([]byte, r.IntN(40))
				for i := range name {
					if r.IntN(8) != 0 {
						name[i] = byte('A' + r.UintN(26))
					}
				}
				buf = append(buf, byte(len(name)))
				buf = append(buf, name...)
			}
		}
	}
	for range r.IntN(3) {
		buf = append(buf, byte(r.UintN(256)))
	}
	return buf
}

// TestGoPackageHTMLScannerStatesMatchC gives random states to Deserialize
// of the Go scanner and of the C scanner, and compares what Serialize writes
// after it.
func TestGoPackageHTMLScannerStatesMatchC(t *testing.T) {
	t.Parallel()
	g, _ := loadGoPackage(t, goPackage{"html", html.Language})
	compareScannerStateSequence(t, g, html.Language(), randomHTMLState, 5000)
}
