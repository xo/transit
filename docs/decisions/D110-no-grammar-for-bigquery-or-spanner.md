# D110. No grammar for bigquery or spanner

Status: Decided, amends D107.

Two grammars of the set parse GoogleSQL, the language of the dialects
`bigquery` and `spanner`: `takegue/tree-sitter-sql-bigquery` and
`kitagry/tree-sitter-bigquery`. Question 77 asked which one gets the module
`grammars/bigquery`.

Ken decided on 2026-10-08, answering question 77, that transit makes no
grammar for `bigquery` or `spanner`. Neither grammar gets a Go package. Both
stay in the golden set, as `m-novikov/tree-sitter-sql` does (D106).
