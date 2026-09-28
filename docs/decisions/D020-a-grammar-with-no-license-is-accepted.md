# D20. A freely available grammar with no license is accepted

Status: Decided.

Ken decided on 2026-09-29 that a grammar that is freely available and names
no license can be in the set, as fair use. On that date, two grammars in the
set named no license: `tree-sitter-grammars/tree-sitter-move` and
`madskjeldgaard/tree-sitter-supercollider`. D23 added a third,
`udovin/tree-sitter-yql`.

The rule of tier 3 in [`docs/CANDIDATES.md`](../CANDIDATES.md) now takes a
grammar with a permissive license or with no license. A grammar under the GPL,
the LGPL or the MPL still needs Ken to name it.

In the golden stage, transit stores only a hash of each file of such a
grammar. In the Go stage, a grammar package copies files from the grammar,
as [`docs/GRAMMAR.md`](../GRAMMAR.md) says. The rule is the same in both stages:
a freely available grammar with no license is fair use. Nobody asks its
authors for a license.
