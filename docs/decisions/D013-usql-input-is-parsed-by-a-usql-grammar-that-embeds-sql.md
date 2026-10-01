# D13. usql input is parsed by a usql grammar that embeds SQL

Status: Decided, amended by D101.

The input of usql is not only SQL. It holds backslash commands, such as `\d`
and `\g`, and variables, such as `:name`. A SQL grammar does not parse them.

Ken decided on 2026-09-29 that a grammar for usql input parses them, and that
it embeds SQL. The usql grammar parses the commands and the variables, and it
hands each SQL statement to a SQL grammar through an injection (a range of the
input that another grammar parses).

transit writes and owns the usql grammar, in `grammars/xo/` (D42).
