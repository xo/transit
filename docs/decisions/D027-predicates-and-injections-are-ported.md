# D27. Query predicates and injections are ported

Status: Decided, amends D7.

Ken decided on 2026-09-29, answering questions 29 and 30, that transit ports
two parts that D7 left out:

1. The evaluation of query predicates, from `lib/binding_rust/lib.rs`: `eq?`,
   `not-eq?`, `any-eq?`, `any-not-eq?`, `match?`, `not-match?`, `any-match?`,
   `any-not-match?`, `any-of?`, `not-any-of?`, `is?`, `is-not?` and `set!`.
   It goes in the query code of the root package. `match?` uses the package
   `regexp`. Each difference from the Rust crate `regex` that a query finds
   is recorded as a decision.
2. The part of `crates/highlight` that finds the injected ranges and the
   language of each, and builds the tree of each layer. It goes in the
   package `inject` (D26). The part that highlights is not ported.

D71 says that `inject` takes UTF-8 and that a Rust oracle tests it, and D72
sets its API.

The C runtime parses predicates and does not evaluate them, and it has no
injection engine. rline needs the predicates, because most `highlights.scm`
files use them. The usql grammar needs injections (D13).

The Rust binding does not evaluate the predicates and the directives that
Neovim adds, such as `#lua-match?` and `#offset!`, and transit does not
either for now (D69). [`NEOVIM.md`](../NEOVIM.md) lists them.
