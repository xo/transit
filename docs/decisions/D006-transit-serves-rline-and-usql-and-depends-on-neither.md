# D6. transit serves rline and usql, and depends on neither

Status: Decided.

Ken decided on 2026-09-29 that the goal of transit is to serve two `xo`
projects with a fast, pure Go incremental parsing library:

1. [rline](https://github.com/xo/rline), a pure Go line editor. rline applies
   syntax highlighting on its own, from the tree that transit builds.
2. [usql](https://github.com/xo/usql), a command line client for SQL
   databases. usql gives tab completion that follows the context of the
   cursor. It uses the SQL dialects that dbmeta maintains, and it queries the
   database for the names that it offers.

## transit only gives parsing information

transit builds trees, updates them after an edit, and answers questions about
them: the node at an offset, the fields of a node, the captures of a query and
the symbols that the parser can accept next. transit does not highlight and it
does not complete. rline highlights, and usql completes.

## The dependency runs one way

transit does not import rline, usql or dbmeta. Both consumers import transit.
Where a type is useful to both consumers, transit defines it, and both use it.

## transit sets the design

rline and usql will change their implementations to follow transit. They take
the design from transit, and transit does not copy a design from them. So
transit gives two things to its consumers:

1. Helpful APIs in the code that the generator writes for a grammar.
2. Reference documents for the coding agents that work in each consumer:
   [`docs/RLINE.md`](../RLINE.md) and [`docs/USQL.md`](../USQL.md).
