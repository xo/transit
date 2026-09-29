# Queries written for Neovim

transit evaluates the query predicates that the Rust binding of upstream
evaluates (D27). Neovim adds predicates and directives of its own, and many
grammars ship queries that use them. transit does not support these now
(D69). This document says what that means, lists the grammars of the set
whose queries use them, and says what support would take.

A grammar in this list is not broken. transit parses its text, and its
queries compile and run. The only effect is that a pattern that uses a
Neovim predicate matches more often than it does in Neovim, and that a
Neovim directive does not change a capture.

## What transit evaluates

The Rust binding evaluates these predicates, and so does transit (D27):
`#eq?`, `#not-eq?`, `#any-eq?`, `#any-not-eq?`, `#match?`, `#not-match?`,
`#any-match?`, `#any-not-match?`, `#any-of?` and `#not-any-of?`. It keeps
`#set!` as a property setting and `#is?` and `#is-not?` as property
predicates.

The Rust binding keeps any other predicate or directive as a general
predicate. It does not evaluate a general predicate, so the pattern matches
whatever the text is. A consumer can read the general predicates of a
pattern with `GeneralPredicates` (docs/API.md). transit does the same as the
Rust binding.

## The names of the Neovim dialect

These names appear in the query files of the set. Neovim evaluates each one, and
transit keeps each one as a general predicate.

| Name | Kind | What Neovim does | What transit does |
| --- | --- | --- | --- |
| `#lua-match?`, `#not-lua-match?` | predicate | matches the text of a capture with a Lua pattern | the pattern matches whatever the text is |
| `#vim-match?` | predicate | matches the text with a regular expression of Vim | the same |
| `#has-ancestor?`, `#not-has-ancestor?` | predicate | tests the kinds of the ancestors of a node | the same |
| `#has-parent?`, `#not-has-parent?` | predicate | tests the kind of the parent of a node | the same |
| `#kind-eq?`, `#not-kind-eq?` | predicate | tests the kind of a node | the same |
| `#offset!` | directive | moves the start and the end of the range of a capture | the capture keeps its whole range |
| `#trim!` | directive | removes blank space from the ends of the range of a capture | the same |
| `#gsub!` | directive | replaces text in the text of a capture, with a Lua pattern | the text stays as it is |
| `#make-range!` | directive | makes one capture from the ranges of two nodes | the capture does not exist |

Three more names appear that neither the Rust binding nor this list covers.
`#strip!` and `#select-adjacent!` belong to the tags crate of upstream, which
transit does not port (D7). `#set-adjacent!`, in the tags query of go, and
`#set-language-from-grammar!`, in the injection query of test, are general
predicates too.

## The grammars that need Neovim support

These numbers were measured on 2026-09-30. The scan read each query file,
`*.scm`, in the folders `queries` and `nvim-queries` of each grammar of
`grammars/grammars.json`, in the cache of the golden harness. The tier is the
tier of [`CANDIDATES.md`](CANDIDATES.md), which marks each grammar of the
first table below with [N].

37 grammars use a Neovim predicate or directive, 155 times in 47 files. 32 of
them use one in a query file that transit reads: `highlights.scm`,
`injections.scm`, `locals.scm` or `tags.scm`. These 32 grammars need Neovim
support for their queries to give the result of Neovim:

| Tier | Grammar | Query files | Names and uses |
| --- | --- | --- | --- |
| 1 | javascript | `queries/injections.scm` | `#offset!` 1 |
| 1 | julia | `queries/highlights.scm` | `#has-ancestor?` 1 |
| 2 | bitbake | `queries/highlights.scm`, `queries/indents.scm`, `queries/injections.scm` | `#lua-match?` 9 |
| 2 | cairo | `queries/highlights.scm` | `#lua-match?` 15 |
| 2 | diff | `queries/injections.scm` | `#offset!` 2 |
| 2 | firrtl | `queries/highlights.scm` | `#lua-match?` 1 |
| 2 | glsl | `queries/highlights.scm` | `#lua-match?` 1 |
| 2 | hare | `queries/highlights.scm` | `#lua-match?` 1 |
| 2 | ispc | `queries/highlights.scm` | `#lua-match?` 1 |
| 2 | kconfig | `queries/highlights.scm` | `#lua-match?` 1 |
| 2 | linkerscript | `queries/highlights.scm` | `#lua-match?` 2 |
| 2 | luau | `queries/highlights.scm`, `queries/injections.scm` | `#lua-match?` 5, `#offset!` 1 |
| 2 | objc | `queries/highlights.scm`, `queries/injections.scm` | `#offset!` 2, `#has-ancestor?` 1 |
| 2 | odin | `queries/highlights.scm` | `#lua-match?` 3, `#not-has-parent?` 2 |
| 2 | pony | `queries/highlights.scm` | `#lua-match?` 1 |
| 2 | query | `queries/query/highlights.scm` | `#lua-match?` 3 |
| 2 | re2c | `queries/highlights.scm`, `queries/injections.scm` | `#offset!` 2 |
| 2 | smali | `queries/highlights.scm` | `#lua-match?` 6 |
| 2 | squirrel | `queries/highlights.scm`, `queries/injections.scm`, `queries/locals.scm` | `#lua-match?` 5, `#not-has-ancestor?` 2, `#offset!` 2 |
| 2 | tablegen | `queries/highlights.scm`, `queries/injections.scm` | `#lua-match?` 3, `#offset!` 2 |
| 2 | thrift | `queries/highlights.scm` | `#lua-match?` 6 |
| 2 | uxntal | `queries/highlights.scm` | `#lua-match?` 2 |
| 2 | vue | `queries/html_tags/injections.scm` | `#not-lua-match?` 4, `#lua-match?` 3, `#offset!` 2, `#gsub!` 1 |
| 2 | zig | `queries/highlights.scm` | `#lua-match?` 3 |
| 3 | angular | `queries/highlights.scm` | `#lua-match?` 1 |
| 3 | blade | `queries/injections.scm` | `#has-ancestor?` 1 |
| 3 | cmake | `queries/highlights.scm` | `#lua-match?` 2 |
| 3 | fsharp | `queries/injections.scm` | `#offset!` 2 |
| 3 | heex | `queries/injections.scm` | `#offset!` 2 |
| 3 | http | `queries/injections.scm` | `#offset!` 2 |
| 3 | kotlin | `queries/highlights.scm` | `#lua-match?` 1 |
| 3 | perl | `queries/highlights.scm` | `#lua-match?` 1 |

Two of them are in tier 1:

1. julia uses `#has-ancestor?` once in `highlights.scm`. Without it,
   `begin` and `end` get the capture `@variable.builtin` everywhere, and
   not only inside an index expression.
2. javascript uses `#offset!` once in `injections.scm`. Without it, the
   range that a template string with the tag `hbs` injects into glimmer
   keeps its backticks.

The other 5 grammars use Neovim names only in query files that transit does
not read, such as `folds.scm`, `indents.scm` and `textobjects.scm`, or in a
folder that is only for Neovim. They need no Neovim support in transit:

| Tier | Grammar | Query files | Names and uses |
| --- | --- | --- | --- |
| 3 | just | `queries/just/folds.scm` | `#trim!` 1 |
| 3 | matlab | `queries/neovim/highlights.scm`, `queries/neovim/textobjects.scm` | `#make-range!` 17, `#lua-match?` 1 |
| 3 | rescript | `queries/textobjects.scm` | `#make-range!` 18 |
| 3 | swift | `queries/indents.scm` | `#not-kind-eq?` 1 |
| 3 | zsh | `nvim-queries/zsh/highlights.scm`, `nvim-queries/zsh/injections.scm` | `#lua-match?` 4, `#offset!` 4, `#not-lua-match?` 1 |

## What support would take

D69 defers the Neovim dialect until the work on the tier 1 grammars needs it.
Support has four parts, and each one can come on its own:

1. `#lua-match?` and `#not-lua-match?`. Port the matcher of LuaJIT 2.1 from
   `src/lib_string.c`, which Neovim runs (D69).
2. `#has-ancestor?`, `#has-parent?` and `#kind-eq?`, and their `not-` forms.
   Each one is a test of the kind of a node or of its ancestors, with the
   methods of `Node`.
3. `#offset!`, `#trim!` and `#gsub!`. Each one changes the range or the text
   of a capture. They matter most for injections, so they belong with the
   package `inject` (D27), and the API needs a place for the changed range.
4. `#make-range!` and `#vim-match?`. transit reads no text object query, and
   `#vim-match?` needs the regular expressions of Vim, so neither is planned.

[`BACKLOG.md`](BACKLOG.md) holds the item for this work, and the item that
keeps the tables above current.
