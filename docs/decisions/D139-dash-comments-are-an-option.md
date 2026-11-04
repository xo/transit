# D139. A dash comment is an option of the usql grammar

Status: Decided, amends D108.

The usql grammar read `--` as the start of a comment in every dialect. In
Cypher, `--` is a relationship with no direction, as in `(p)--()`, and only
`//` starts a comment. So usql cut a Neo4j query at `--`. dbmeta found this
on 2026-11-04, with the movies fixture of Neo4j.

Ken decided on 2026-11-04, answering question 108:

1. `usql.Options` has a new field, `DashComments`. With it on, `--` starts a
   comment to the end of the line. With it off, `--` is the text of the
   statement.
2. The sense is positive, as `SlashComments` and `HashComments` are. So a
   caller that sets no option gets no dash comments. Each SQL dialect sets
   `DashComments`, and the Neo4j dialect does not.
3. The default build of `src/scanner.c`, with no `USQL_OPTIONS`, has dash
   comments, as it has dollar quotes and block comments.
4. The highlights follow the option, because the scanner makes the node
   `comment`.
