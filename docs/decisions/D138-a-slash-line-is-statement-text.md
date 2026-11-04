# D138. A line with only a slash is statement text

Status: Decided, supersedes D135 and D136, amends D128 and D137.

D135 and D136 made a line with only `/` a node `slash` of the usql grammar,
with the option `AnonymousBlocks` on. That form comes from SQL*Plus, the
client of Oracle. usql works as psql does, not as the client of each
database, and psql has no `/` terminator.

Ken decided on 2026-11-04, through the usql peer:

1. The usql grammar has no node `slash`. A line with only `/` is the text of
   a statement, with `AnonymousBlocks` on or off.
2. A statement ends only at a `;` outside parentheses, at a meta command or
   at the end of the input.
3. `AnonymousBlocks` stays. It keeps a PL/SQL block that starts with
   `DECLARE` or `BEGIN` in one statement, up to the `;` after its last
   `END`, as D128 says.
4. A highlight test can still name its options on its first line, as D137
   says.
5. The grammar does not learn the other PL/SQL units. A function or a
   procedure with declarations before `BEGIN`, a trigger with `DECLARE`, a
   package spec and a package body each hold a `;` that ends the statement.
   The person who writes the script escapes each such `;` as `\;`, as in
   psql and in earlier versions of usql. The scanner already keeps `\;` in
   the statement.

So the script `end;`, then a `/` line, then `select 1 from dual;` sends
`/` and `select 1 from dual` to Oracle as one statement, and Oracle refuses
it. psql does the same, and Ken accepts it.
