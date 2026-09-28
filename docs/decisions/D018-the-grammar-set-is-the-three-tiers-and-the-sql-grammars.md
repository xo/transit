# D18. The grammar set is the three tiers and the SQL grammars

Status: Decided.

Ken accepted on 2026-09-29 the grammar set of
[`docs/CANDIDATES.md`](../CANDIDATES.md), and the measurable gate for the Go
backend in "The gate for the Go backend" in [`docs/PLAN.md`](../PLAN.md). This
answers question 15 and gives D9 its measurable form.

The set is:

1. Tier 1: the grammars of the `tree-sitter` organization.
2. Tier 2: the grammars of the `tree-sitter-grammars` organization.
3. Tier 3: the most used and maintained grammars from other owners.
4. The SQL grammars that `CANDIDATES.md` names.
5. The 68 test grammars of upstream, which are always in the set.

Each tier has a rule, and `CANDIDATES.md` states it. In phase 1 the golden
harness measures each grammar. Ken then adds or removes a grammar by name.
