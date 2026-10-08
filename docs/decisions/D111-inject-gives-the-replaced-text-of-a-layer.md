# D111. inject gives the replaced text of a layer

Status: Decided.

To complete in a SQL statement, usql runs `StatesAt` with the grammar of the
dialect on the text of the statement, with the same placeholders that
`inject.WithReplacer` gives the layer (D101). `inject` did not give back that
text, so usql had to make the replacements again.

Ken decided on 2026-10-08, answering question 79, that transit adds an API
for it, an API that upstream does not have (D28). `docs/API.md` gives its
form.
