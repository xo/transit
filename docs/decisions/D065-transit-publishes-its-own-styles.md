# D65. transit publishes its own styles as embedded JSON files

Status: Decided, supersedes D32 and D48, amends D14, D15, D26, D43 and D49, amended by D98 and D99.

Ken decided on 2026-09-29 that transit does not use chroma. transit publishes
its own styles as embedded JSON files, and a small parser reads them. At
Ken's request, the transit agent asked Gemini and DeepSeek for the shape of
the package, and Ken chose from the options that they gave.

## The package

The package is `github.com/xo/transit/styles`, in the root module. It imports
only the standard library, so the root module still requires nothing (D11). A
program that does not import the package does not get the styles. A change of
a style is released with the root module (D43).

usql has a package named `styles` of its own. Ken chose the plain name
anyway.

## The files

Each style is one JSON file in the package, and `//go:embed` holds them.
`encoding/json` reads the file. This is a style:

```json
{
  "name": "monokai",
  "source": "chroma styles/monokai.xml, MIT",
  "default": "#f8f8f2 bg:#272822",
  "captures": {
    "comment": "italic #75715e",
    "keyword": "bold #f92672",
    "keyword.function": "#66d9ef",
    "string": "#e6db74"
  }
}
```

1. A key is a capture name of a highlight query, such as `keyword.function`.
   It is not a chroma token type.
2. A value is a style string. The small parser of the package reads it. The
   grammar is a subset of the style strings of chroma: `bold`, `italic`,
   `underline` and their `no` forms, a color as `#rgb` or `#rrggbb`, a color
   of the 16 ANSI colors or of the 256 colors of a terminal, and `bg:` before
   the background color. A converted entry keeps the text of chroma.
3. `default` is the entry of text that no capture names. Its background is the
   background of the style.

## The lookup

A lookup of a capture name tries the whole name, and then each shorter
prefix: `keyword.function`, then `keyword`, then the default. This is the rule
of Neovim and Helix. An entry is whole. It does not take the fields that it
leaves out from the entry of its prefix. A style has no other form of
inheritance, and a style file has no field that names a parent.

A color keeps the kind that the file writes: an ANSI color, a color of the
256 colors, or an RGB color. The package gives a function that reduces a
color to 256 or to 16 colors. The consumer chooses the depth, and the package
does not look at the terminal.

The background of a style applies only when the consumer asks for it. A line
editor usually keeps the background of the terminal.

A consumer can load a style file of its own with the same parser.

## Where the styles come from

1. A command of the test module converts the styles of chroma, as
   `test/cmd/regextables` writes the Unicode tables (D60). It reads the XML
   files in `styles/` of a checkout of chroma at a pinned version. It does not
   import chroma. For each capture name, it takes the chroma token type that
   matches the name, and it resolves the entry of that type with the rules of
   chroma, with the inheritance of token types, `noinherit` and the
   background. So each entry that it writes is whole.
2. A person tunes a few styles by hand to use the finer captures, such as
   `variable.parameter`, that chroma has no token type for. The default styles
   of rline and usql are among them. A tuned style is a file of its own, and
   the command does not write it.
3. No one edits a file that the command writes. To change it, run the command
   again, as hard rule 7 says for the other generated files.
4. The files that the command writes keep the license of chroma (MIT), and of
   Pygments (BSD) where chroma took a style from Pygments. The package holds
   those license files.

## The names of the captures

The styles key on a list of capture names. The list is measured from
`queries/highlights.scm` of the grammars in `grammars/grammars.json`, and the
repository holds it. A test makes sure that each name in the list reaches an
entry of each bundled style through its prefixes. When a grammar joins the
set, its capture names join the list.

## What this changes

1. D32 and D48 no longer hold. transit has no module `chromastyles`, and
   nothing matches capture names to chroma token types at run time. The match
   lives only in the command that converts the styles.
2. D14 still holds for the tokens: they come from the tree and the highlight
   queries, and never from a chroma lexer. A consumer takes its colors from
   the package `styles`, and it does not need chroma. transit does not test
   with chroma.
3. The approval of chroma in D15 and D32 ends. No test and no module of
   transit imports chroma.
4. The table of D26 has the package `styles` in the root module, in place of
   the module `chroma`.
5. D43 has no tag for a chroma module, and D49 has no chromastyles module to
   wire.
