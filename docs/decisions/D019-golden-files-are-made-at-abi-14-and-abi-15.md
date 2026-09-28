# D19. Golden files are made at ABI 14 and ABI 15

Status: Decided.

Ken decided on 2026-09-29 that the golden harness makes each grammar at ABI
14 and at ABI 15. This answers question 26.

The upstream tool writes both, and `render.rs` writes a different form for
each. ABI 14 writes `TSLexMode` and no reserved words or supertypes. ABI 15
writes `TSLexerMode`, the reserved word sets and the supertype map. Both forms
test the C backend (D8).

The test grammars are also made with the merge of parse states off, as the
recommendation of question 26 proposed. The harness records the ABI version,
the version of the grammar and the merge setting beside each golden file
(D17).

transit does not generate ABI 13 (D22).
