# D50. One unit of work per change

Status: Decided.

Ken decided on 2026-09-29, answering question 49, how coding agents split the
port:

1. A unit is one upstream file of the runtime, one Rust module of the
   generator, or one ported part such as the predicates or the injections,
   with its ported tests.
2. An agent claims a unit in `docs/BACKLOG.md` before it starts, so that two
   agents do not port the same unit.
3. One staged change covers one unit.
4. A phase ends when its tests pass. Ken reviews the results of each phase,
   and not each line of each unit.
