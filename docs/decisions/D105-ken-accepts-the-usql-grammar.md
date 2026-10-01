# D105. Ken accepts the usql grammar

Status: Decided, amends D101 and D102, amended by D108.

Ken reviewed the usql grammar in `grammars/usql` on 2026-10-01 and accepted
it as it is. The agent that wrote it named ten choices that D101 and D102 do
not settle, and they hold as the grammar makes them:

1. The families are postgres, mysql, sqlite, standard, cql and plain. The
   grammars are `usql_<family>`, and the packages are `usql<family>`.
2. The node types are `statement`, `meta_command` with the field `name`,
   `modifier`, `word`, `string`, `quoted_identifier`, `dollar_string`,
   `variable` with the field `name`, `backtick_command`, `pipe`,
   `shell_command`, `option_list`, and `option` with the fields `key` and
   `value`.
3. No node wraps an argument. The parts of a word that are glued together
   are children of the meta command, one after another.
4. A string follows usql: `''` and a backslash escape work in every family.
5. A block comment does not nest, as in usql.
6. A dollar quote does not start right after a character of an identifier,
   as in psql.
7. A backslash starts a meta command at any depth of parentheses, as in
   psql.
8. Every meta command ends the statement node, so usql joins the buffer.
9. `\\` is a meta command with the name `\\`. `\;` and `\:` stay in the SQL
   text, and `\!ls` is a command that the scanner does not know.
10. An option list in parentheses follows only the commands that run the
    buffer. A `|` at the start of an argument of any command starts a pipe.

The option `inject.WithReplacer` of D101 is part of the API, in the form
that `docs/API.md` gives.
