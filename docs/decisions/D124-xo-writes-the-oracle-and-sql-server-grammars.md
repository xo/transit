# D124. xo writes the Oracle and SQL Server grammars

Status: Decided, amends D106.

The grammar of `grammars/oracle` comes from `andreasmaierde/tree-sitter-plsql`.
Its repository has not changed since 2023, and the repository has no corpus
test of it. The grammar of `grammars/sqlserver` comes from
`Crary-Systems/tree-sitter-tsql`. It has 132 rules, which cover a small part
of Transact-SQL.

Ken decided on 2026-10-31, answering questions 97 and 98:

1. xo writes the Oracle grammar from the SQL Language Reference and the
   PL/SQL Language Reference of Oracle Database 23ai. It covers SQL and
   PL/SQL.
2. xo writes the SQL Server grammar from the Transact-SQL reference of SQL
   Server 2025.
3. The modules and the packages keep their names, `oracle` and `sqlserver`,
   so that usql imports them as it does today.
4. Each grammar keeps its parse tables small, with the technique of D122
   point 4.
5. The grammars of `andreasmaierde/tree-sitter-plsql` and
   `Crary-Systems/tree-sitter-tsql` stay in the golden set, but they get no
   Go package.
