# D28. transit can add an API that upstream does not have

Status: Decided.

Ken decided on 2026-09-29, answering question 31, that transit can add an API
that upstream does not have, under four conditions:

1. The API only adds. It changes no ported behavior.
2. It lives in files that port no upstream file, so a port of an upstream
   change never touches it.
3. Its name is not the name of an upstream function.
4. Each such API has a decision of its own.

The first one gives the parse states of each stack version at a byte offset,
so that usql can complete after the parser recovered from an error at the
cursor.
