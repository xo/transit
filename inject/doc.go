// Package inject finds the injections of a text and parses the layer of each
// (D27). An injection is a range of the text that another grammar parses,
// such as a <script> element of HTML, or a SQL statement in the input of
// usql. A layer is one tree of one language over the ranges of its
// injections.
//
// The package ports the part of crates/highlight of upstream tree-sitter
// that finds injections and parses their layers. The part that highlights
// is not ported. Upstream has no public API for this part, so D72 sets the
// API: NewConfig compiles the injection query of a language, and
// Config.Layers gives every layer of a text at once. The text is UTF-8
// (D71).
//
// WithReplacer, Layer.Text and Layer.StatesAt are APIs that upstream does
// not have (D28, D101, D111). They let a consumer such as usql replace the
// variables of its input before a SQL layer is parsed, and complete in that
// layer.
//
// The examples of Layers and WithReplacer are in the grammar package
// github.com/xo/transit/grammars/html, because they need the grammars of
// HTML and JavaScript.
package inject
