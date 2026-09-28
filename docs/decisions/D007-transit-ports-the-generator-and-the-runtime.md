# D7. transit ports the generator and the runtime

Status: Decided, amended by D27 and D41.

Ken decided on 2026-09-29 that transit ports two parts of upstream
tree-sitter:

1. The parser generator, `crates/generate`.
2. The runtime, `lib/src` and `lib/include`, except `wasm_store.c` (D1). The
   runtime includes the query engine, `query.c`.

transit does not port the highlighter (`crates/highlight`), the tags
extractor (`crates/tags`), the loader, the web binding or the subcommands of
the upstream command line tool. A consumer that highlights code, such as
rline, does it on its own with the query engine (D6).

The upstream tests of the two parts are ported with them. A command that runs
the generator is part of the generator. D41 gives the command its
subcommands, and D27 adds the predicates and the injections.
