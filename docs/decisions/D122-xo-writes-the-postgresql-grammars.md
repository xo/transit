# D122. xo writes the PostgreSQL grammars

Status: Decided, amends D106.

The grammar `gmr/tree-sitter-postgres` is made from the Bison grammar of
PostgreSQL. It keeps the keyword classes of that grammar, so 5,401 of its
16,991 parse states are large, and its package adds about 15 MB to a binary.
The other SQL grammars of the repository add 3 MB or less.

Ken decided on 2026-10-31, answering questions 95 and 96:

1. xo writes the PostgreSQL grammar, as it wrote MySQL (D42, D106). It
   follows the reference manual of PostgreSQL 19.
2. xo writes the PL/pgSQL grammar too, so that no part of the module
   `grammars/postgres` comes from `gmr/tree-sitter-postgres`.
3. The module and the packages keep their names, `postgres` and `plpgsql`,
   so that usql imports them as it does today.
4. The grammar keeps its parse tables small. One identifier token covers the
   keywords that are not reserved, with the `word` rule and the reserved
   word sets of tree-sitter, so that few states are large.
5. `gmr/tree-sitter-postgres` stays in the golden set, but it gets no Go
   package.
