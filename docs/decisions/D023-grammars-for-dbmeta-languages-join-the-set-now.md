# D23. The grammars that exist for dbmeta's languages join the set now

Status: Decided.

Ken accepted on 2026-09-29 the recommendation of question 28. The grammars
that exist today for the languages of dbmeta's dialects (D21) join the golden
set now, although they have fewer stars than tier 3 asks for:

| Grammar | Language | Stars | Last push | License | Scanner |
| --- | --- | --- | --- | --- | --- |
| `taekwombo/tree-sitter-cypher` | Cypher, for Neo4j | 13 | 2026-08-14 | Unlicense | no |
| `surrealdb/surrealql-tree-sitter` | SurrealQL | 7 | 2026-09-13 | Apache-2.0 | yes |
| `GordianDziwis/tree-sitter-sparql` | SPARQL, for Fuseki and Virtuoso | 16 | 2025-10-15 | MIT | no |
| `bkegley/tree-sitter-graphql` | GraphQL, for Dgraph, TerminusDB and Weaviate | 32 | 2024-06-07 | MIT | no |
| `udovin/tree-sitter-yql` | YQL, for YDB | 0 | 2026-03-18 | none (D20) | no |

Each one commits `src/grammar.json`. A grammar that the upstream tool cannot
generate at the base commit is recorded in `docs/CANDIDATES.md`, and it does
not count toward the gate of D9.

The SQL grammars in "SQL" in `docs/CANDIDATES.md` were in the set already
(D18).
