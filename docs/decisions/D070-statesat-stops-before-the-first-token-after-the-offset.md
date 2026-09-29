# D70. StatesAt stops before the first token that ends after the offset

Status: Decided.

D57 says that `StatesAt` parses the text up to the offset and gives the parse
state of each stack version there. Phase 3 wrote it in `states_at.go`, and
found that "up to the offset" needs these rules. Ken accepted them on
2026-09-30, answering question 61.

1. The lexer reads all of `src`, and not `src` cut at the offset. A cut text
   changes the tokens next to the cut, because the text ends there. With the
   cut text, the scanner of JavaScript inserts an automatic semicolon before
   `=`, and the scanner of Python gives a newline. Then the states did not
   accept the next token at many offsets of the corpus of JavaScript,
   TypeScript, PHP, bash and other fixture grammars. With all of `src`, they
   accept it at every offset.
2. A stack version stops before it reduces or detects an error on the first
   token that ends after the offset, or on the end of the text. A token that
   ends at the offset is handled. A token with no width at the offset is
   handled too, such as the `_concat` that the scanner of bash gives before a
   word.
3. When the offset is inside a word, the states are the states before the
   word. At the end of a word, the word ends at the offset and is handled. To
   complete a word, a consumer gives the offset of the start of the word.
4. With an old tree, the parser does not reuse a node that holds the last
   leaf before the offset, or the offset. A reused node there ends after the
   last token that the version must handle. The parser can reuse a node
   where it splits into two stack versions without the old tree, so the old
   tree can give fewer states.
5. The states come in the order of the stack versions, with no state twice.
   A version that recovers from an error before the offset gives the state
   that it has after the recovery.
6. `StatesAt` resets the parser before and after, so a parse that a context
   stopped does not go on.

The test module measures the rules on the 17 fixture grammars. It takes the
first 40 corpus inputs of each grammar, drops the inputs that have an error,
and gives `StatesAt` the start of the first 60 tokens of each input. At each
of these 13,664 offsets, a state that `StatesAt` gives accepts the token, with
the old tree and without it. After `SELECT * FROM `, the SQL grammar of the C
example gives state 9144, which accepts 14 symbols, as after `FROM` in
`SELECT id FROM u`. The tree has an error there, and state 0 accepts 408
symbols.
