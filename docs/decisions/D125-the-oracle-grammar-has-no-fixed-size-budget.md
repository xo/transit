# D125. The Oracle grammar has no fixed size budget

Status: Decided, amends D124.

D124 point 4 keeps the tables of the Oracle grammar small, and the work set a
target of about 3 MB added to a binary. The first part of the grammar already
adds 1.22 MB, because the parse states after an expression repeat for each
place where an expression can end.

Ken decided on 2026-11-01, answering questions 99 to 101:

1. The Oracle grammar has no fixed size budget. It covers the references
   fully, and each part reports what it costs. The technique of D122 point 4
   still holds.
2. The grammar does not read the substitution variables of SQL*Plus, `&name`
   and `&&name`. They are not SQL, and usql does not substitute them.
3. A word that only PL/SQL reserves joins the reserved set only where the
   measurements show that it saves parse states. `TYPE` stays a word that a
   column can have as its name.
