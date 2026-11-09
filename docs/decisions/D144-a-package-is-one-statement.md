# D144. A package and a type body are each one statement

Status: Decided, amends D138.

D138 point 5 says that the usql grammar does not learn the other PL/SQL
units, so that each `;` in them ends the statement. That text names more
units than really split. With the option `BeginEndBlocks` on, the scanner
already keeps these units in one statement:

1. A `CREATE FUNCTION` or a `CREATE PROCEDURE`, with its declarations after
   `IS` or `AS`.
2. A `CREATE TRIGGER`, up to the `END` of its body.

The scanner has no case for a package spec, a package body or a type body.
A `;` in one of them ends the statement. On 2026-11-09 the usql peer found
that `usql --dump` of the northwind fixture on Oracle loads the package
`HR.PAYROLL` invalid for this reason. Every function, procedure and trigger
of the dump loads valid.

Ken decided on 2026-11-10:

1. With `BeginEndBlocks` on, the usql grammar keeps each of these units in
   one statement, from `CREATE` to the `;` after the `END` that closes the
   unit:
   - `CREATE [OR REPLACE] [EDITIONABLE | NONEDITIONABLE] PACKAGE name`, the
     package spec.
   - `CREATE [OR REPLACE] [EDITIONABLE | NONEDITIONABLE] PACKAGE BODY name`.
   - `CREATE [OR REPLACE] [EDITIONABLE | NONEDITIONABLE] TYPE BODY name`.
2. A package spec holds declarations only. Its unit ends at `END [name];`.
3. A package body or a type body holds declarations and whole subprograms,
   each with its own `BEGIN ... END;`. A package body can end with its own
   `BEGIN` part, the code that runs when the package loads. The unit ends at
   the `END [name];` that closes the body itself, not at the `END;` of a
   subprogram.
4. A `CREATE TYPE` that is not `TYPE BODY` stays as it is. It holds no
   `BEGIN` and no `;` before its end.
5. Point 5 of D138 now names only the units that still split. With
   `BeginEndBlocks` on, no PL/SQL unit of this decision, of D112 or of D128
   needs `\;`. A `;` that is escaped as `\;` still stays in the statement.
6. With `BeginEndBlocks` off, nothing changes.
