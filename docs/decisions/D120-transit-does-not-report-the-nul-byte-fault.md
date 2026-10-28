# D120. transit does not report the NUL byte fault upstream

Status: Decided.

Since 64aaee66 of upstream, `ts_language_symbol_for_name` in
`lib/src/language.c` compares `strlen(symbol_name)` with the length of the
name. If the name of a symbol holds a NUL byte, `strlen` stops at that byte,
and the function finds no symbol for the name. The Go grammar has the
anonymous token `"\u0000"`, and `SymbolForName("\x00", false)` finds no
symbol for it. transit ports the fault (hard rule 6), and the check of the
node types in `internal/grammartest` skips such a name.

Ken decided on 2026-10-28 that transit does not report the fault upstream.
The port keeps the fault, and the check keeps the skip.
