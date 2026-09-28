# D21. transit will support the languages of every dbmeta dialect

Status: Decided.

Ken decided on 2026-09-29 that usql supports SQL-like languages as well as
SQL, and that the languages of the dialects that dbmeta tests, verifies and
stages are to be supported in transit in time. A SQL-like language is a query
language that is not SQL, such as CQL, SQL++, Cypher, SurrealQL or SPARQL.

dbmeta listed its dialects on 2026-09-29, from `dbrun list --json all`.
"The dialects of dbmeta" in [`docs/CANDIDATES.md`](../CANDIDATES.md) holds the
list, the language of each, and the grammars that exist for each language.

## What this means

1. A dialect that dbmeta tests, verifies or stages needs a grammar for its
   language, in time. So does a hosted service in dbmeta's `hosted/` list.
2. A flavor, such as MariaDB of MySQL or ScyllaDB of Cassandra, uses the
   grammar of its dialect, unless its language differs enough to need its own.
3. Where no grammar exists, someone must write one. transit writes it, in
   `grammars/xo/` (D42).
4. "In time" means after the Go backend (D9). No phase waits for these
   grammars, except that the grammars that already exist join the golden
   set now (D23).

The languages of the staged products come from product knowledge. dbmeta has
not measured them.
