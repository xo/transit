package usql

// This file is a port of src/scanner.c of the usql grammar of transit, in
// grammars/usql (D101, D102, D108, D112).
//
// The scanner reads every token of the grammar. It finds the end of a
// statement, the strings, the dollar quotes, the comments and the depth of
// the parentheses. It splits the name of a meta command into its base name
// and its modifiers, with the commands of usql. It ends the arguments of a
// meta command at the end of the line or at the next backslash outside
// quotes. The client lexer of PostgreSQL, src/bin/psql/psqlscan.l, is the
// reference for the rules.
//
// The scanner tells the contexts apart by the valid tokens. commandEnd is
// valid only inside the arguments of a meta command, the option tokens only
// inside a list of options, and variableName only after the sigil of a
// variable. Everywhere else, the text is SQL.
//
// The options of the dialect change some tokens. The C scanner reads them
// from the macro USQL_OPTIONS when it is compiled, and the Go scanner reads
// them from its field options.
//
// With the option BeginEndBlocks, the scanner keeps a stored program in one
// statement (D112). It reads the words of the SQL text, outside strings,
// quoted identifiers and comments. After CREATE ... PROCEDURE, FUNCTION,
// TRIGGER or EVENT, it counts BEGIN and END, and a ; inside the body does
// not end the statement. The words that it reads are in keywords below.
//
// The C scanner is the reference (D42). The test module compares the two
// scanners on every corpus input, with each set of options that it tests.

import (
	"encoding/binary"
	"math"
	"strings"

	"github.com/xo/transit/internal/abi"
)

// The external tokens, in the order of externals in grammar.json. They are
// the C enum TokenType.
const (
	sqlText = iota
	stringToken
	quotedIdentifier
	dollarString
	comment
	semicolon
	backslash
	backslashOptions
	commandBase
	shellCommandName
	separatorCommandName
	modifier
	commandEnd
	word
	pipe
	shellCommand
	backtickOpen
	backtickText
	backtickClose
	optionOpen
	optionClose
	optionEquals
	optionWord
	sigilPlain
	sigilSingleQuote
	sigilDoubleQuote
	sigilBrace
	variableName
	closeSingleQuote
	closeDoubleQuote
	closeBrace
	errorSentinel
)

// context is the context of a text token: SQL, an argument of a meta
// command, or a word in a list of options. It is the C enum Context.
type context int

const (
	contextSQL context = iota
	contextArgument
	contextOption
)

// maxNameLength is MAX_NAME_LENGTH, the longest command name that the
// scanner splits. A longer name is not a command of usql, so it is one whole
// name.
const maxNameLength = 32

// maxTagLength is MAX_TAG_LENGTH, the longest tag of a dollar quote, as usql
// allows it.
const maxTagLength = 128

// maxWordLength is MAX_WORD_LENGTH, the longest keyword that the scanner
// reads in SQL text.
const maxWordLength = 16

// serializedSize is SERIALIZED_SIZE, the number of bytes that Serialize
// writes.
const serializedSize = 9

// block is the C enum Block, the part of a statement that the scanner is
// in, with the option BeginEndBlocks.
type block uint8

const (
	// blockStart is before the first word of the statement.
	blockStart block = iota
	// blockNone is a statement that is not a stored program, or the rest of
	// a stored program after the END of its body.
	blockNone
	// blockCreate is after CREATE, before the kind of the object.
	blockCreate
	// blockDefiner is after DEFINER in blockCreate, where a word can name
	// the user.
	blockDefiner
	// blockHeader is after PROCEDURE or FUNCTION, where IS or AS can start
	// the declarations of PL/SQL.
	blockHeader
	// blockRoutine is in a stored program before its BEGIN. A ; ends the
	// statement.
	blockRoutine
	// blockDeclare is in the declarations of a stored program before its
	// BEGIN. A ; does not end the statement.
	blockDeclare
	// blockBody is inside the body, from BEGIN to its END.
	blockBody
)

// pending is the C enum Pending, a word whose meaning depends on the next
// word.
type pending uint8

const (
	pendingNone pending = iota
	// pendingBegin is after BEGIN, which starts a transaction when WORK,
	// TRANSACTION, TRAN, DISTRIBUTED or ; comes next, and else a block.
	pendingBegin
	// pendingEnd is after END, which ends a statement when IF, LOOP, WHILE
	// or REPEAT comes next, and else a block.
	pendingEnd
	// pendingAs is after IS or AS in blockHeader, which starts declarations
	// when a word that does not start a statement comes next.
	pendingAs
)

// keyword is the C enum Keyword, a word that the scanner reads in SQL text,
// or kwNone for any other word. The order is the order of keywords.
type keyword int

const (
	kwNone keyword = iota
	kwAggregate
	kwAlter
	kwAs
	kwBegin
	kwCall
	kwCase
	kwCatch
	kwConstraint
	kwCreate
	kwDeclare
	kwDefiner
	kwDelete
	kwDistributed
	kwEditionable
	kwEnd
	kwEvent
	kwExec
	kwExecute
	kwExternal
	kwFunction
	kwIf
	kwInsert
	kwIs
	kwLanguage
	kwLoop
	kwMerge
	kwNoneditionable
	kwOr
	kwPrint
	kwProc
	kwProcedure
	kwRepeat
	kwReplace
	kwReturn
	kwSelect
	kwSet
	kwTemp
	kwTemporary
	kwTran
	kwTransaction
	kwTrigger
	kwTry
	kwUpdate
	kwValues
	kwWhile
	kwWith
	kwWork
)

// keywords holds the text of each keyword, in lower case.
var keywords = [...]string{
	"", "aggregate", "alter", "as", "begin", "call",
	"case", "catch", "constraint", "create", "declare", "definer",
	"delete", "distributed", "editionable", "end", "event", "exec",
	"execute", "external", "function", "if", "insert", "is",
	"language", "loop", "merge", "noneditionable", "or", "print",
	"proc", "procedure", "repeat", "replace", "return", "select",
	"set", "temp", "temporary", "tran", "transaction", "trigger",
	"try", "update", "values", "while", "with", "work",
}

// sqlWord is the C struct Word, the word of SQL text that the scanner
// reads, in lower case.
type sqlWord struct {
	// text holds the first maxWordLength characters of the word. A
	// character that is not ASCII is 1.
	text [maxWordLength]byte
	// n is the length of the word, or maxWordLength + 1 for a longer word.
	n int
	// spoiled is true for a word right after ., @, $, # or [, which is a
	// name and not a keyword.
	spoiled bool
}

// scanner is a port of src/scanner.c of the usql grammar of transit, in
// grammars/usql. It is the C struct Scanner, the state of the scanner, with
// the options that USQL_OPTIONS gives the C scanner.
type scanner struct {
	// options are the options of the dialect. Serialize does not write
	// them, because they do not change.
	options Options

	// depth is the depth of the parentheses in the statement.
	depth uint32
	// base is the length of the base name of the meta command that
	// backslash starts, which commandBase reads, or 0 when the name is one
	// whole name.
	base uint8
	// command is 1 from the name of a meta command to its commandEnd. It
	// makes commandEnd, which reads no character, change the state.
	command uint8
	// block is the block of the statement, pending its pending word, and
	// level the number of blocks of the body that are open. They stay 0
	// without the option BeginEndBlocks.
	block   block
	pending pending
	level   uint8
}

// command is the C struct Command, a meta command of usql, from
// metacmd/descs.go of usql.
type command struct {
	// base is the base name, without the backslash.
	base string
	// classes holds the letters that can follow the base name in any order,
	// each once, before the modifiers.
	classes string
	// modifiers holds the letters that can follow the base name, each once,
	// in their order.
	modifiers string
	// options is true when the command takes a list of options in
	// parentheses.
	options bool
}

// commands holds the meta commands of usql. A name in brackets in
// metacmd/descs.go, such as d[S+], gives the base name d and the modifiers
// S and +. usql accepts the classes tvmsE of \d in any order.
var commands = []command{
	{"q", "", "", false},
	{"quit", "", "", false},
	{"copyright", "", "", false},
	{"drivers", "", "", false},
	{"?", "", "", false},
	{"c", "", "", false},
	{"connect", "", "", false},
	{"Z", "", "", false},
	{"disconnect", "", "", false},
	{"password", "", "", false},
	{"passwd", "", "", false},
	{"conninfo", "", "", false},
	{"g", "", "", true},
	{"go", "", "", true},
	{"G", "", "", true},
	{"ego", "", "", true},
	{"gx", "", "", true},
	{"gexec", "", "", false},
	{"gset", "", "", true},
	{"bind", "", "", false},
	{"timing", "", "", false},
	{"crosstab", "", "", true},
	{"crosstabview", "", "", true},
	{"xtab", "", "", true},
	{"chart", "", "", true},
	{"watch", "", "", true},
	{"e", "", "", false},
	{"edit", "", "", false},
	{"p", "", "", false},
	{"print", "", "", false},
	{"raw", "", "", false},
	{"exec", "", "", false},
	{"w", "", "", false},
	{"write", "", "", false},
	{"r", "", "", false},
	{"reset", "", "", false},
	{"d", "tvmsE", "S+", false},
	{"da", "", "S", false},
	{"dA", "", "+", false},
	{"dAc", "", "+", false},
	{"dAf", "", "+", false},
	{"dAo", "", "+", false},
	{"dAp", "", "+", false},
	{"db", "", "+", false},
	{"dc", "", "S+", false},
	{"dconfig", "", "+", false},
	{"dC", "", "+", false},
	{"dd", "", "S", false},
	{"dD", "", "S+", false},
	{"ddp", "", "", false},
	{"dE", "", "S+", false},
	{"des", "", "+", false},
	{"det", "", "+", false},
	{"deu", "", "+", false},
	{"dew", "", "+", false},
	{"df", "", "anptwS+", false},
	{"dF", "", "+", false},
	{"dFd", "", "+", false},
	{"dFp", "", "+", false},
	{"dFt", "", "+", false},
	{"dg", "", "S+", false},
	{"di", "", "S+", false},
	{"dl", "", "+", false},
	{"dL", "", "S+", false},
	{"dm", "", "S+", false},
	{"dn", "", "S+", false},
	{"do", "", "S+", false},
	{"dO", "", "S+", false},
	{"dp", "", "S", false},
	{"dP", "", "+", false},
	{"drds", "", "", false},
	{"drg", "", "S", false},
	{"dRp", "", "+", false},
	{"dRs", "", "+", false},
	{"ds", "", "S+", false},
	{"dt", "", "S+", false},
	{"dT", "", "S+", false},
	{"du", "", "S+", false},
	{"dv", "", "S+", false},
	{"dx", "", "+", false},
	{"dX", "", "", false},
	{"dy", "", "+", false},
	{"l", "", "+", false},
	{"lo_list", "", "+", false},
	{"z", "", "S", false},
	{"sf", "", "+", false},
	{"sv", "", "+", false},
	{"ss", "", "+", false},
	{"set", "", "", false},
	{"unset", "", "", false},
	{"pset", "", "", false},
	{"a", "", "", false},
	{"C", "", "", false},
	{"f", "", "", false},
	{"H", "", "", false},
	{"T", "", "", false},
	{"t", "", "", false},
	{"x", "", "", false},
	{"cset", "", "", false},
	{"prompt", "", "", false},
	{"echo", "", "", false},
	{"qecho", "", "", false},
	{"warn", "", "", false},
	{"o", "", "", false},
	{"out", "", "", false},
	{"copy", "", "", false},
	{"i", "", "", false},
	{"include", "", "", false},
	{"ir", "", "", false},
	{"include_relative", "", "", false},
	{"if", "", "", false},
	{"elif", "", "", false},
	{"else", "", "", false},
	{"endif", "", "", false},
	{"begin", "", "", false},
	{"commit", "", "", false},
	{"rollback", "", "", false},
	{"abort", "", "", false},
	{"cd", "", "", false},
	{"getenv", "", "", false},
	{"setenv", "", "", false},
}

// advance is advance.
func advance(lexer *abi.Lexer) { lexer.Advance(false) }

// skip is skip.
func skip(lexer *abi.Lexer) { lexer.Advance(true) }

// isNewline is is_newline. It reports whether c ends a line.
func isNewline(c int32) bool { return c == '\n' || c == '\r' }

// isSpace is is_space. It reports whether c is white space.
func isSpace(c int32) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// isVariableChar is is_variable_char. It reports whether c can be in the
// name of a variable, as variable_char of psqlscan.l: a letter, a digit, an
// underscore or any character that is not ASCII.
func isVariableChar(c int32) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' ||
		c >= 0x80
}

// isTagStart is is_tag_start. It reports whether c can start the tag of a
// dollar quote, as dolq_start of psqlscan.l.
func isTagStart(c int32) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c >= 0x80
}

// isIdentifierChar is is_identifier_char. It reports whether c can be in an
// identifier, as ident_cont of psqlscan.l. A dollar quote cannot start after
// one.
func isIdentifierChar(c int32) bool { return isVariableChar(c) || c == '$' }

// isNameEnd is is_name_end. It reports whether the lookahead ends the name
// of a meta command: white space, a backslash, a control character or the
// end of the input.
func isNameEnd(lexer *abi.Lexer) bool {
	c := lexer.Lookahead
	return lexer.EOF() || isSpace(c) || c == '\\' || c < 0x20 || c == 0x7f
}

// validModifiers is valid_modifiers. It reports whether the letters of rest
// are modifiers of the command cmd.
func validModifiers(cmd *command, rest string) bool {
	i := 0
	seen := uint(0)
	for ; i < len(rest); i++ {
		p := strings.IndexByte(cmd.classes, rest[i])
		if p < 0 {
			break
		}
		bit := uint(1) << uint(p)
		if seen&bit != 0 {
			break
		}
		seen |= bit
	}
	m := cmd.modifiers
	for ; i < len(rest); i++ {
		p := strings.IndexByte(m, rest[i])
		if p < 0 {
			return false
		}
		m = m[p+1:]
	}
	return true
}

// splitName is split_name. It returns the command of the longest base name
// that starts name, with modifiers after it, and the length of the base
// name. It returns nil when no command matches.
func splitName(name string) (*command, int) {
	for length := len(name); length > 0; length-- {
		for i := range commands {
			cmd := &commands[i]
			if len(cmd.base) == length && cmd.base == name[:length] && validModifiers(cmd, name[length:]) {
				return cmd, length
			}
		}
	}
	return nil, 0
}

// scanLineComment is scan_line_comment. It scans a comment to the end of the
// line.
func scanLineComment(lexer *abi.Lexer) bool {
	for !lexer.EOF() && !isNewline(lexer.Lookahead) {
		advance(lexer)
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = comment
	return true
}

// scanBlockComment is scan_block_comment. It scans the rest of a block
// comment after its /*. The comment does not nest, and with no */ it runs to
// the end of the input.
func scanBlockComment(lexer *abi.Lexer) bool {
	for !lexer.EOF() {
		if lexer.Lookahead == '*' {
			advance(lexer)
			if lexer.Lookahead == '/' {
				advance(lexer)
				break
			}
			continue
		}
		advance(lexer)
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = comment
	return true
}

// scanQuotedRest is scan_quoted_rest. It scans the rest of a quoted text
// after its opening quote q, up to the closing quote. A doubled quote is a
// quote, and in a string in single quotes a backslash escapes the next
// character. With no closing quote, the text runs to the end of the line
// when line is true, and else to the end of the input.
func scanQuotedRest(lexer *abi.Lexer, q int32, line bool) {
	for !lexer.EOF() && (!line || !isNewline(lexer.Lookahead)) {
		c := lexer.Lookahead
		if c == '\\' && q == '\'' {
			advance(lexer)
			if !lexer.EOF() && !(line && isNewline(lexer.Lookahead)) {
				advance(lexer)
			}
			continue
		}
		advance(lexer)
		if c == q {
			if lexer.Lookahead != q {
				break
			}
			advance(lexer)
		}
	}
	lexer.MarkEnd()
}

// scanQuoted is scan_quoted. It scans a quoted text from its opening quote,
// and returns the token symbol.
func scanQuoted(lexer *abi.Lexer, symbol uint16, line bool) bool {
	q := lexer.Lookahead
	advance(lexer)
	scanQuotedRest(lexer, q, line)
	lexer.ResultSymbol = symbol
	return true
}

// scanDollarRest is scan_dollar_rest. It scans the body of a dollar quote
// after its opening $tag$, up to the closing $tag$ or the end of the input.
func scanDollarRest(lexer *abi.Lexer, tag []int32) bool {
	for !lexer.EOF() {
		if lexer.Lookahead != '$' {
			advance(lexer)
			continue
		}
		advance(lexer)
		i := 0
		for i < len(tag) && lexer.Lookahead == tag[i] && !lexer.EOF() {
			advance(lexer)
			i++
		}
		if i == len(tag) && lexer.Lookahead == '$' {
			advance(lexer)
			break
		}
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = dollarString
	return true
}

// resetBlocks is reset_blocks. It resets the state of the blocks at the
// end of a statement.
func (s *scanner) resetBlocks() {
	s.block = blockStart
	s.pending = pendingNone
	s.level = 0
}

// openBlock is open_block. It opens a block of the body: the first BEGIN of
// the body, a BEGIN in it, or a CASE in it, which ends at END or END CASE.
func (s *scanner) openBlock() {
	if s.block != blockBody {
		s.block = blockBody
		s.level = 1
	} else if s.level < math.MaxUint8 {
		s.level++
	}
}

// closeBlock is close_block. It closes a block of the body. After the last
// one, the body ends.
func (s *scanner) closeBlock() {
	if s.block != blockBody {
		return
	}
	if s.level > 0 {
		s.level--
	}
	if s.level == 0 {
		s.block = blockNone
	}
}

// resolvePending is resolve_pending. It gives the pending word its meaning
// when a character that is not in a word comes after it: BEGIN does not
// start a block, END ends one, and AS does not start declarations.
func (s *scanner) resolvePending() {
	p := s.pending
	s.pending = pendingNone
	if p == pendingEnd {
		s.closeBlock()
	}
}

// endsStatement is ends_statement. It reports whether a ; outside
// parentheses ends the statement. It is not the end inside the
// declarations or the body of a stored program.
func (s *scanner) endsStatement(opts Options) bool {
	if !opts.BeginEndBlocks {
		return true
	}
	s.resolvePending()
	return s.block != blockDeclare && s.block != blockBody
}

// scanString is scan_string. It notes a string in SQL text. After IS or AS,
// a string is the body of the stored program, so no declarations follow.
func (s *scanner) scanString(opts Options) {
	if opts.BeginEndBlocks && s.pending == pendingAs {
		s.pending = pendingNone
		s.block = blockRoutine
	}
}

// keywordOf is keyword_of. It returns the keyword of the word w, or kwNone.
func keywordOf(w *sqlWord) keyword {
	if w.n > maxWordLength {
		return kwNone
	}
	for i := 1; i < len(keywords); i++ {
		if keywords[i] == string(w.text[:w.n]) {
			return keyword(i)
		}
	}
	return kwNone
}

// isStatementWord is is_statement_word. It reports whether kw starts a
// statement. After it, the header of a stored program ended, and IS or AS
// is a part of that statement.
func isStatementWord(kw keyword) bool {
	switch kw {
	case kwCall, kwDelete, kwExec, kwExecute, kwInsert, kwMerge, kwReplace,
		kwSelect, kwSet, kwUpdate, kwValues, kwWith:
		return true
	}
	return false
}

// scanKeyword is scan_keyword. It changes the state of the blocks for the
// word kw of SQL text.
func (s *scanner) scanKeyword(kw keyword) {
	p := s.pending
	s.pending = pendingNone
	switch p {
	case pendingNone:
	case pendingBegin:
		if kw == kwWork || kw == kwTransaction || kw == kwTran || kw == kwDistributed {
			return
		}
		s.openBlock()
	case pendingEnd:
		if kw == kwIf || kw == kwLoop || kw == kwWhile || kw == kwRepeat {
			return
		}
		s.closeBlock()
		if kw == kwNone || kw == kwCase || kw == kwTry || kw == kwCatch {
			// a label, or the end of CASE, BEGIN TRY or BEGIN CATCH
			return
		}
	case pendingAs:
		if isStatementWord(kw) || kw == kwReturn || kw == kwPrint || kw == kwIf ||
			kw == kwWhile || kw == kwLanguage || kw == kwExternal {
			s.block = blockRoutine
			return
		}
		if kw != kwBegin {
			s.block = blockDeclare
		}
	}
	switch s.block {
	case blockStart:
		if kw == kwCreate {
			s.block = blockCreate
		} else {
			s.block = blockNone
		}
	case blockNone:
	case blockDefiner:
		s.block = blockCreate
		if kw != kwProcedure && kw != kwProc && kw != kwFunction && kw != kwTrigger && kw != kwEvent {
			// the name of the user
			return
		}
		fallthrough
	case blockCreate:
		switch kw {
		case kwProcedure, kwProc, kwFunction:
			s.block = blockHeader
		case kwTrigger, kwEvent:
			s.block = blockRoutine
		case kwDefiner:
			s.block = blockDefiner
		case kwAggregate, kwAlter, kwConstraint, kwEditionable, kwNoneditionable,
			kwOr, kwReplace, kwTemp, kwTemporary:
		default:
			s.block = blockNone
		}
	case blockHeader:
		if kw == kwIs || kw == kwAs {
			if s.depth == 0 {
				s.pending = pendingAs
			}
			return
		}
		if isStatementWord(kw) {
			s.block = blockRoutine
			return
		}
		fallthrough
	case blockRoutine, blockDeclare:
		switch kw {
		case kwBegin:
			s.pending = pendingBegin
		case kwDeclare:
			s.block = blockDeclare
		}
	case blockBody:
		switch kw {
		case kwBegin:
			s.pending = pendingBegin
		case kwEnd:
			s.pending = pendingEnd
		case kwCase:
			s.openBlock()
		}
	}
}

// isWordChar is is_word_char. It reports whether c continues the word w: a
// letter, a digit, an underscore, a character that is not ASCII, or a $
// after the first character.
func isWordChar(w *sqlWord, c int32) bool {
	return isVariableChar(c) || (c == '$' && w.n > 0)
}

// addWordChar is add_word_char. It adds the character c to the word w.
func addWordChar(w *sqlWord, c int32) {
	if w.n < maxWordLength {
		lower := byte(1)
		if c >= 'A' && c <= 'Z' {
			lower = byte(c - 'A' + 'a')
		} else if c >= 0 && c < 0x80 {
			lower = byte(c)
		}
		w.text[w.n] = lower
	}
	if w.n <= maxWordLength {
		w.n++
	}
}

// endWord is end_word. It ends the word w at the character c, which is not
// in a word, and gives the word to scanKeyword.
func (s *scanner) endWord(w *sqlWord, c int32) {
	if w.n > 0 && !w.spoiled {
		s.scanKeyword(keywordOf(w))
	}
	w.n = 0
	w.spoiled = c == '.' || c == '@' || c == '$' || c == '#' || c == '['
}

// scanVariableStart is scan_variable_start. It scans from the colon of a
// variable. It returns the sigil when a valid variable follows. Else the
// colon is text in the context ctx. After :' or :" with no valid name and
// closing quote, the quote opens a string or a quoted identifier, and the
// token holds the colon too.
func (s *scanner) scanVariableStart(lexer *abi.Lexer, opts Options, ctx context, line bool) bool {
	advance(lexer)
	c := lexer.Lookahead
	if c == ':' {
		// :: is a cast
		advance(lexer)
		lexer.MarkEnd()
		return s.scanText(lexer, opts, ctx, true, ':')
	}
	if isVariableChar(c) {
		lexer.MarkEnd()
		lexer.ResultSymbol = sigilPlain
		return true
	}
	if c == '\'' || c == '"' {
		advance(lexer)
		lexer.MarkEnd()
		n := 0
		for isVariableChar(lexer.Lookahead) && !lexer.EOF() {
			advance(lexer)
			n++
		}
		if n > 0 && lexer.Lookahead == c {
			if c == '\'' {
				lexer.ResultSymbol = sigilSingleQuote
			} else {
				lexer.ResultSymbol = sigilDoubleQuote
			}
			return true
		}
		scanQuotedRest(lexer, c, line)
		if c == '\'' {
			lexer.ResultSymbol = stringToken
		} else {
			lexer.ResultSymbol = quotedIdentifier
		}
		return true
	}
	if c == '{' {
		advance(lexer)
		last := int32('{')
		if lexer.Lookahead == '?' {
			advance(lexer)
			last = '?'
			lexer.MarkEnd()
			n := 0
			for isVariableChar(lexer.Lookahead) && !lexer.EOF() {
				last = lexer.Lookahead
				advance(lexer)
				n++
			}
			if n > 0 && lexer.Lookahead == '}' {
				lexer.ResultSymbol = sigilBrace
				return true
			}
		}
		// the characters are text
		lexer.MarkEnd()
		return s.scanText(lexer, opts, ctx, true, last)
	}
	lexer.MarkEnd()
	return s.scanText(lexer, opts, ctx, true, ':')
}

// scanText is scan_text. It scans a run of plain text in the context ctx:
// SQL text, a word of an argument, or a word of a list of options. content
// is true when the token already holds text, and prev is the last character
// of that text. The token ends at its last character that is not white
// space. With the option BeginEndBlocks, scanText reads the words of SQL
// text.
func (s *scanner) scanText(lexer *abi.Lexer, opts Options, ctx context, content bool, prev int32) bool {
	blocks := ctx == contextSQL && opts.BeginEndBlocks
	var w sqlWord
	for !lexer.EOF() {
		c := lexer.Lookahead
		if blocks {
			if isWordChar(&w, c) {
				addWordChar(&w, c)
			} else {
				s.endWord(&w, c)
			}
		}
		if ctx != contextSQL {
			// an argument ends at white space, and a word at a quote
			if isSpace(c) || c == '\\' || c == '\'' || c == '"' || c == '`' {
				break
			}
			if ctx == contextOption && (c == '=' || c == ')') {
				break
			}
		} else {
			if (c == ';' && s.depth == 0 && s.endsStatement(opts)) || c == '\'' || c == '"' ||
				(c == '`' && opts.Backticks) || (c == '#' && opts.HashComments) {
				if !content && c == '#' {
					return scanLineComment(lexer)
				}
				break
			}
			if c == '\\' {
				// \; and \: put the character in the statement, as psql does
				advance(lexer)
				if lexer.Lookahead != ';' && lexer.Lookahead != ':' {
					break
				}
				prev = lexer.Lookahead
				advance(lexer)
				lexer.MarkEnd()
				content = true
				continue
			}
			if c == '-' || c == '/' {
				advance(lexer)
				d := lexer.Lookahead
				line := (c == '-' && d == '-') || (c == '/' && d == '/' && opts.SlashComments)
				block := c == '/' && d == '*' && opts.BlockComments
				if line || block {
					if content {
						break
					}
					advance(lexer)
					if line {
						return scanLineComment(lexer)
					}
					return scanBlockComment(lexer)
				}
				lexer.MarkEnd()
				content = true
				prev = c
				continue
			}
			if c == '$' && opts.DollarQuotes && !isIdentifierChar(prev) {
				advance(lexer)
				var tag [maxTagLength]int32
				n := 0
				valid := true
				last := int32('$')
				if isTagStart(lexer.Lookahead) {
					for isVariableChar(lexer.Lookahead) && !lexer.EOF() {
						if n == maxTagLength {
							valid = false
							break
						}
						tag[n] = lexer.Lookahead
						last = lexer.Lookahead
						n++
						advance(lexer)
					}
				}
				if valid && lexer.Lookahead == '$' {
					if content {
						break
					}
					advance(lexer)
					s.scanString(opts)
					return scanDollarRest(lexer, tag[:n])
				}
				// not a dollar quote, so the characters are text, and the
				// word after the $ is not a keyword
				lexer.MarkEnd()
				content = true
				prev = last
				continue
			}
		}
		if c == ':' {
			if !content {
				return s.scanVariableStart(lexer, opts, ctx, ctx != contextSQL)
			}
			advance(lexer)
			d := lexer.Lookahead
			if d == ':' {
				advance(lexer)
				lexer.MarkEnd()
				prev = ':'
				continue
			}
			if isVariableChar(d) || d == '\'' || d == '"' || d == '{' {
				break
			}
			lexer.MarkEnd()
			prev = ':'
			continue
		}
		if ctx == contextSQL {
			if c == '(' {
				s.depth++
			} else if c == ')' && s.depth > 0 {
				s.depth--
			}
		}
		if blocks && !isSpace(c) && w.n == 0 {
			s.resolvePending()
		}
		advance(lexer)
		if !isSpace(c) {
			lexer.MarkEnd()
			content = true
		}
		prev = c
	}
	if blocks {
		s.endWord(&w, 0)
	}
	if !content {
		return false
	}
	switch ctx {
	case contextSQL:
		lexer.ResultSymbol = sqlText
	case contextArgument:
		lexer.ResultSymbol = word
	case contextOption:
		lexer.ResultSymbol = optionWord
	}
	return true
}

// scanCommandName is scan_command_name. It scans the name of a meta command
// after its backslash. For \\ and \!, the token holds the whole name. Else
// the token is the backslash, and the scanner keeps the length of the base
// name for commandBase.
func (s *scanner) scanCommandName(lexer *abi.Lexer, validSymbols []bool) bool {
	lexer.MarkEnd()
	s.depth = 0
	s.resetBlocks()
	if lexer.Lookahead == '\\' {
		advance(lexer)
		lexer.MarkEnd()
		lexer.ResultSymbol = separatorCommandName
		return validSymbols[separatorCommandName]
	}
	var name [maxNameLength]byte
	n := 0
	ascii := true
	for !isNameEnd(lexer) {
		if n < maxNameLength && lexer.Lookahead < 0x80 {
			name[n] = byte(lexer.Lookahead)
		} else {
			ascii = false
		}
		n++
		advance(lexer)
	}
	s.command = 1
	if ascii && n == 1 && name[0] == '!' {
		lexer.MarkEnd()
		lexer.ResultSymbol = shellCommandName
		return validSymbols[shellCommandName]
	}
	base := 0
	var cmd *command
	if ascii && n > 0 {
		cmd, base = splitName(string(name[:n]))
	}
	if cmd == nil {
		s.base = 0
	} else {
		s.base = uint8(base)
	}
	if cmd != nil && cmd.options {
		lexer.ResultSymbol = backslashOptions
	} else {
		lexer.ResultSymbol = backslash
	}
	return validSymbols[lexer.ResultSymbol]
}

// scanCommandBase is scan_command_base. It scans the base name of a meta
// command after its backslash.
func (s *scanner) scanCommandBase(lexer *abi.Lexer) bool {
	for i := 0; (s.base == 0 || i < int(s.base)) && !isNameEnd(lexer); i++ {
		advance(lexer)
	}
	s.base = 0
	lexer.MarkEnd()
	lexer.ResultSymbol = commandBase
	return true
}

// scanShellCommand is scan_shell_command. It scans a shell command to the
// end of the line or to the next backslash outside quotes.
func scanShellCommand(lexer *abi.Lexer) bool {
	quote := int32(0)
	for {
		c := lexer.Lookahead
		if lexer.EOF() || isNewline(c) || (quote == 0 && c == '\\') {
			break
		}
		if quote == 0 && (c == '\'' || c == '"') {
			quote = c
		} else if c == quote {
			quote = 0
		}
		advance(lexer)
		if !isSpace(c) {
			lexer.MarkEnd()
		}
	}
	lexer.ResultSymbol = shellCommand
	return true
}

// scanArguments is scan_arguments. It scans a token in the arguments of a
// meta command, or in a list of options.
func (s *scanner) scanArguments(lexer *abi.Lexer, validSymbols []bool, opts Options) bool {
	for !lexer.EOF() && isSpace(lexer.Lookahead) && !isNewline(lexer.Lookahead) {
		skip(lexer)
	}
	c := lexer.Lookahead
	if lexer.EOF() || isNewline(c) || c == '\\' {
		if !validSymbols[commandEnd] {
			return false
		}
		lexer.MarkEnd()
		s.command = 0
		lexer.ResultSymbol = commandEnd
		return true
	}
	if validSymbols[optionClose] && c == ')' {
		advance(lexer)
		lexer.MarkEnd()
		lexer.ResultSymbol = optionClose
		return true
	}
	if validSymbols[optionEquals] && c == '=' {
		advance(lexer)
		lexer.MarkEnd()
		lexer.ResultSymbol = optionEquals
		return true
	}
	if validSymbols[shellCommand] {
		return scanShellCommand(lexer)
	}
	if validSymbols[pipe] && c == '|' {
		advance(lexer)
		lexer.MarkEnd()
		lexer.ResultSymbol = pipe
		return true
	}
	if validSymbols[optionOpen] && c == '(' {
		advance(lexer)
		lexer.MarkEnd()
		lexer.ResultSymbol = optionOpen
		return true
	}
	if validSymbols[backtickOpen] && c == '`' {
		advance(lexer)
		lexer.MarkEnd()
		lexer.ResultSymbol = backtickOpen
		return true
	}
	if validSymbols[stringToken] && c == '\'' {
		return scanQuoted(lexer, stringToken, true)
	}
	if validSymbols[quotedIdentifier] && c == '"' {
		return scanQuoted(lexer, quotedIdentifier, true)
	}
	ctx := contextArgument
	if validSymbols[optionWord] {
		ctx = contextOption
	}
	if ctx == contextArgument && !validSymbols[word] {
		return false
	}
	return s.scanText(lexer, opts, ctx, false, 0)
}

// scanSQL is scan_sql. It scans a token of SQL, or the name of a meta
// command.
func (s *scanner) scanSQL(lexer *abi.Lexer, validSymbols []bool, opts Options) bool {
	for !lexer.EOF() && isSpace(lexer.Lookahead) {
		skip(lexer)
	}
	if lexer.EOF() {
		return false
	}
	c := lexer.Lookahead
	if c == '\\' {
		advance(lexer)
		if lexer.Lookahead == ';' || lexer.Lookahead == ':' {
			// \; and \: put the character in the statement, as psql does
			prev := lexer.Lookahead
			advance(lexer)
			lexer.MarkEnd()
			return s.scanText(lexer, opts, contextSQL, true, prev)
		}
		return s.scanCommandName(lexer, validSymbols)
	}
	if c == ';' && s.depth == 0 && s.endsStatement(opts) {
		advance(lexer)
		lexer.MarkEnd()
		s.resetBlocks()
		lexer.ResultSymbol = semicolon
		return true
	}
	if c == '\'' {
		s.scanString(opts)
		return scanQuoted(lexer, stringToken, false)
	}
	if c == '"' || (c == '`' && opts.Backticks) {
		return scanQuoted(lexer, quotedIdentifier, false)
	}
	return s.scanText(lexer, opts, contextSQL, false, 0)
}

// newScanner returns a new scanner with the default options.
//
// newScanner is tree_sitter_usql_external_scanner_create.
func newScanner() *scanner { return &scanner{options: defaultOptions} }

// Serialize writes the depth in its first 4 bytes, in little-endian order,
// then base, command, block, pending and level.
//
// Serialize is tree_sitter_usql_external_scanner_serialize.
func (s *scanner) Serialize(buffer []byte) int {
	binary.LittleEndian.PutUint32(buffer, s.depth)
	buffer[4] = s.base
	buffer[5] = s.command
	buffer[6] = byte(s.block)
	buffer[7] = byte(s.pending)
	buffer[8] = s.level
	return serializedSize
}

// Deserialize reads the bytes that Serialize writes, or resets the scanner
// when there are none.
//
// Deserialize is tree_sitter_usql_external_scanner_deserialize.
func (s *scanner) Deserialize(buffer []byte) {
	s.depth = 0
	s.base = 0
	s.command = 0
	s.resetBlocks()
	if len(buffer) == serializedSize {
		s.depth = binary.LittleEndian.Uint32(buffer)
		s.base = buffer[4]
		s.command = buffer[5]
		s.block = block(buffer[6])
		s.pending = pending(buffer[7])
		s.level = buffer[8]
	}
}

// Scan scans one token with the options of the scanner.
//
// Scan is tree_sitter_usql_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	opts := s.options
	if validSymbols[errorSentinel] {
		// In the recovery from an error every token is valid, so the text is
		// read as SQL.
		return s.scanSQL(lexer, validSymbols, opts)
	}
	if validSymbols[variableName] {
		n := 0
		for isVariableChar(lexer.Lookahead) && !lexer.EOF() {
			advance(lexer)
			n++
		}
		lexer.MarkEnd()
		lexer.ResultSymbol = variableName
		return n > 0
	}
	if validSymbols[closeSingleQuote] || validSymbols[closeDoubleQuote] || validSymbols[closeBrace] {
		c := lexer.Lookahead
		switch {
		case validSymbols[closeSingleQuote] && c == '\'':
			lexer.ResultSymbol = closeSingleQuote
		case validSymbols[closeDoubleQuote] && c == '"':
			lexer.ResultSymbol = closeDoubleQuote
		case validSymbols[closeBrace] && c == '}':
			lexer.ResultSymbol = closeBrace
		default:
			return false
		}
		advance(lexer)
		lexer.MarkEnd()
		return true
	}
	if validSymbols[commandBase] && !isNameEnd(lexer) {
		return s.scanCommandBase(lexer)
	}
	if validSymbols[modifier] && !isNameEnd(lexer) {
		advance(lexer)
		lexer.MarkEnd()
		lexer.ResultSymbol = modifier
		return true
	}
	if validSymbols[backtickText] || validSymbols[backtickClose] {
		if validSymbols[backtickClose] && lexer.Lookahead == '`' {
			advance(lexer)
			lexer.MarkEnd()
			lexer.ResultSymbol = backtickClose
			return true
		}
		if !validSymbols[backtickText] {
			return false
		}
		n := 0
		for !lexer.EOF() && lexer.Lookahead != '`' && !isNewline(lexer.Lookahead) {
			advance(lexer)
			n++
		}
		lexer.MarkEnd()
		lexer.ResultSymbol = backtickText
		return n > 0
	}
	if validSymbols[commandEnd] || validSymbols[optionClose] || validSymbols[optionWord] ||
		validSymbols[optionEquals] {
		return s.scanArguments(lexer, validSymbols, opts)
	}
	return s.scanSQL(lexer, validSymbols, opts)
}
