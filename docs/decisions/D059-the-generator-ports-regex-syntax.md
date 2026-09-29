# D59. The generator ports the parser and the translator of regex-syntax

Status: Decided.

Ken decided on 2026-09-29, answering question 56, that the generator reads
the pattern of a token with a Go port of the Rust crate `regex-syntax`, at the
version that upstream pins. On 2026-09-29 that version is 0.8.11.

Upstream reads a pattern in `pattern.rs` and `expand_tokens.rs`. The parser
of `regex-syntax` makes a syntax tree, and its translator turns the tree into
an intermediate form (the HIR) that holds classes of Unicode ranges. The
generator builds its NFA from that form. The port covers what the generator
calls: the parser, the syntax tree, the translator, the HIR, the error text
and the Unicode tables of the translator. It does not port a matcher, because
the generator matches no text.

Before the decision, Ken asked whether a Go package already does this.
Gemini and DeepSeek were asked, and each claim was checked on GitHub and on
the grammars on disk on 2026-09-29:

1. `github.com/dlclark/regexp2/v2`, which chroma uses at v2.7.1, exports a
   parser, but for the syntax of .NET. Its tree holds .NET sets, not ranges,
   and its error text is the text of .NET.
2. Go's `regexp/syntax` reads the syntax of RE2. It rejects 27 of the 1,344
   patterns in the 86 grammars on disk, for `\uXXXX` and `\p{XID_Start}`,
   and its error text is another text.
3. `github.com/wasilibs/go-re2` runs in WebAssembly (D1), and
   `github.com/grafana/regexp` reads the same syntax as Go's `regexp`.
4. No Go package ports `regex-syntax`.

Only the port gives the same NFA and the same error text as upstream, byte for
byte (D8).

The port follows these rules:

1. It is an internal package of `generate`, so the root module still imports
   only the Go standard library (D11). One Rust file becomes one Go file, as
   D24 says for upstream.
2. `regex-syntax` is under the MIT license or the Apache 2.0 license. The
   package keeps the MIT license of the crate, with its copyright line, in a
   `LICENSE` file of its own.
3. When upstream moves to a new version of `regex-syntax`, the port follows,
   as [`UPSTREAM.md`](../UPSTREAM.md) says.

`parse_grammar.rs` asks the Rust crate `regex` whether a pattern matches the
empty string, only to decide a warning. Phase 2 unit 1 used Go's `regexp`
there. The port replaces it.
