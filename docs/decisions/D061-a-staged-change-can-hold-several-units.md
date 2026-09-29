# D61. A staged change can hold several units

Status: Decided. Amends D50.

Ken decided on 2026-09-29 that one staged change can hold several units, such
as the passes of `prepare_grammar/`. D50 said that one staged change covers
one unit.

Each unit keeps its own claim in `docs/BACKLOG.md`, and each unit keeps its
ported tests. The claims of a staged change are deleted when Ken commits it.
