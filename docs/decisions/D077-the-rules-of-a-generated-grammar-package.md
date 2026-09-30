# D77. The rules of a generated grammar package

Status: Decided.

Ken decided on 2026-09-30 these rules of the Go backend and of the package
that it writes for a grammar (D26, D74):

1. The constants of the symbols and the fields come from the names of the C
   enums that render.rs makes, after its own suffixes for names that
   collide. The prefix `ts_builtin_sym_` becomes `BuiltinSym`,
   `anon_alias_sym_` becomes `AnonAliasSym`, `alias_sym_` becomes
   `AliasSym`, `anon_sym_` becomes `AnonSym`, `aux_sym_` becomes `AuxSym`,
   `sym_` becomes `Sym` and `field_` becomes `Field`. The rest of the name is
   split on `_`, an empty part is dropped, and each part gets an upper-case
   first letter. A name that is already taken gets `2`, `3` and so on. There
   is no list of initialisms, so `sym_json_value` becomes `SymJsonValue`.
2. `Keywords` returns the names of the symbols that the keyword lex table
   accepts, and the reserved words, sorted and with no name twice. For a
   keyword that a grammar writes as a pattern, such as a keyword of SQL in
   any case, the name is the rule name, such as `keyword_select`. The
   consumer maps it to the word.
3. The grammar `go` gets the package `golang`, as D26 names the package of
   the Go backend, because `go` is a keyword of Go. Any other grammar name
   that is not a Go package name stops the generator with an error.
4. A grammar module holds a copy of `tree-sitter.json` of the upstream
   repository. The generator reads the version of the grammar from it.
5. The interpreter of the lexer as data is one method of the runtime,
   `(*abi.LexTable).Lex` in `internal/abi`. A grammar package holds only the
   data of its lexer.
6. The backend exports `Tables`, which builds the tables of a grammar and
   writes no Go, so that the test module can compare the tables of every
   fixture grammar with C.
