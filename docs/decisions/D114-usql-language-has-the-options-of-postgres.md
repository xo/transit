# D114. usql.Language has the options of PostgreSQL

Status: Decided, amends D108.

Ken decided on 2026-10-08, answering question 83, that `usql.Language()`
keeps the options of PostgreSQL: dollar quotes and block comments. psql is
the reference of the usql grammar (D101). `usql.Options{}`, with no option on,
is a language of its own, which `usql.LanguageFor` gives.
