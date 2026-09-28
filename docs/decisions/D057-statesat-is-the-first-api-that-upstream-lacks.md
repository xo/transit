# D57. StatesAt is the first API that upstream does not have

Status: Decided.

Ken decided on 2026-09-29, answering question 55, that the first API that
upstream does not have (D28) is:

    func (p *Parser) StatesAt(ctx context.Context, src []byte, offset int, old *Tree) ([]StateID, error)

It parses the text up to the offset, and returns the parse state of each stack
version there, before the parser recovers from an error. A consumer lists the
symbols that can come next with the lookahead iterator of each state.

The working C example found the need. After `SELECT * FROM `, the parser
recovers from an error, the state at the cursor is 0, and the lookahead lists
375 symbols. `Node.NextParseState` also gives 0 for a token that the parser
lexed before a reduce. `docs/API.md` records both cases.

Phase 3 writes `StatesAt` in a file that ports no upstream file, and the test
module measures it against the cases of the C example.
