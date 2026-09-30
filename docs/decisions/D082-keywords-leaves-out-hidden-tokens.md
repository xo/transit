# D82. Keywords leaves out hidden tokens

Status: Decided, amends D77.

The keyword lex table of a grammar can accept hidden tokens that the
generator makes from a pattern inside a rule, such as `cast_type_token3` of
php or `program_token1` of ruby. `SymbolForName` finds no hidden symbol, and
such a name tells a consumer nothing. Ken decided on 2026-09-30 that
`Keywords` of a generated grammar package leaves out each symbol that is not
visible. Every name that it returns is one that `SymbolForName` finds.
