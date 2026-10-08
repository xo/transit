# D107. A grammar module takes the name of its dialect

Status: Decided, amends D26 and D106, amended by D108 and D110.

Ken decided on 2026-10-01, answering questions 76 and 78, that the grammar
modules take the names of the dialects of dbmeta. The dbmeta agent gave the
names from dbmeta at commit `3a0ff0f` on the same day. A dialect name is the
dialect name of dburl.

1. A grammar whose language is the language of one dialect lives in the
   module `grammars/<dialect>`:

   | Grammar | Module |
   | --- | --- |
   | `gmr/tree-sitter-postgres`, `postgres` and `plpgsql` | `grammars/postgres`, with the packages `postgres` and `plpgsql` |
   | `Crary-Systems/tree-sitter-tsql`, `TSQL` | `grammars/sqlserver` |
   | `andreasmaierde/tree-sitter-plsql`, `plsql` | `grammars/oracle` |
   | `shotover/tree-sitter-cql`, `cql` | `grammars/cql` |
   | the MySQL grammar that xo writes, `mysql` | `grammars/mysql` |
   | `taekwombo/tree-sitter-cypher`, `cypher` | `grammars/neo4j` |
   | `surrealdb/surrealql-tree-sitter`, `surrealql` | `grammars/surrealdb` |
   | `udovin/tree-sitter-yql`, `yql` | `grammars/ydb` |

2. A grammar that no single dialect owns keeps the name of its language:
   `grammars/sql` for `DerekStride/tree-sitter-sql`, which many dialects
   use, and `grammars/sparql` and `grammars/graphql`, which no dialect of
   dbmeta names.
3. A package has the name of its folder, in lowercase, and the package of the
   folder `go` is `golang` (D77). The package of a grammar at the root of its
   module has the name of the module folder. Every package before this
   decision already had the name of its folder. The name of the grammar, which
   `Language.Name` gives and which the queries and the injections use, stays
   the name of upstream, such as `TSQL`.
4. The family `sqlite` of the usql grammar becomes `sqlite3`, the name of
   the dialect: the grammar `usql_sqlite3` and the package `usqlsqlite3`.
   The families `postgres`, `mysql` and `cql` already have the names of
   dialects, and `standard` and `plain` name groups of dialects.

Question 77, which of the two grammars of GoogleSQL gets the dialect name
`bigquery` and which `spanner`, is open. Until Ken answers it, neither of
`takegue/tree-sitter-sql-bigquery` and `kitagry/tree-sitter-bigquery` gets a
Go package.
