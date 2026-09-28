# D14. chroma is used only for its styles

Status: Decided, amended by D32.

Ken decided on 2026-09-29 that a consumer of transit uses
[chroma](https://github.com/alecthomas/chroma) only for its styles. A style
is a set of colors for each kind of token. usql uses chroma styles today to
change its colors. The tokens and their kinds come from transit, through the
tree and the highlight queries of each grammar, and not from the lexers of
chroma.

transit tests with chroma too. The tests make sure that the captures of a
grammar can be drawn with a chroma style, as a consumer draws them.

Ken approves chroma for these tests (D15). The root module must not require it
(D11). D32 and D48 put the tests, and a package that matches capture names to
chroma token types, in the module `github.com/xo/transit/chromastyles`.
