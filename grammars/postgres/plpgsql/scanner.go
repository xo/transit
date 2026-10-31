package plpgsql

import "github.com/xo/transit/internal/abi"

// The external tokens, the C enum TokenType, in the order of externals in
// grammar.js.
const (
	comment = iota
	sqlStatement
	sqlSemicolon
	sqlReturn
	sqlQuery
	sqlThen
	sqlWhen
	sqlLoop
	sqlRange
	sqlBy
	sqlList
	sqlListLoop
	sqlArgument
	sqlSubscript
	dollarQuotedString
	errorSentinel
)

// The ends of a token, besides the semicolon, which ends each token.
const (
	endThen = 1 << iota
	endWhen
	endLoop
	endRange
	endBy
	endComma
	endInto
	endUsing
	endParen
	endBracket
)

// ends is ENDS. It holds the ends of each token of SQL.
var ends = [...]uint16{
	comment:      0,
	sqlStatement: 0,
	sqlSemicolon: 0,
	sqlReturn:    0,
	sqlQuery:     0,
	sqlThen:      endThen,
	sqlWhen:      endWhen,
	sqlLoop:      endLoop,
	sqlRange:     endRange | endLoop,
	sqlBy:        endBy | endLoop,
	sqlList:      endComma | endInto | endUsing,
	sqlListLoop:  endComma | endUsing | endLoop,
	sqlArgument:  endComma | endParen,
	sqlSubscript: endBracket,
}

// commands is COMMANDS. It holds the first words of the SQL commands that a
// body can hold. The commands of transactions, cursors and dynamic SQL are
// statements of PL/pgSQL, so they are not in the list.
var commands = [...]string{
	"alter", "analyse", "analyze", "call", "checkpoint", "cluster", "comment",
	"copy", "create", "deallocate", "delete", "discard", "do", "drop",
	"explain", "grant", "import", "insert", "listen", "load", "lock",
	"merge", "notify", "prepare", "reassign", "refresh", "reindex", "release",
	"repack", "reset", "revoke", "savepoint", "security", "select", "set",
	"show", "table", "truncate", "unlisten", "update", "vacuum", "values",
	"with",
}

// maxWord is MAX_WORD, the longest word that the scanner compares, in
// bytes.
const maxWord = 16

// maxTag is MAX_TAG, the longest tag of a dollar-quoted string that the
// scanner reads, in characters.
const maxTag = 64

// word is Word. It is a word of the SQL, in lower case. length is
// maxWord + 1 when the word is longer than maxWord or holds a character
// outside ASCII, so that it matches no word that the scanner compares.
type word struct {
	text   [maxWord]byte
	length int
}

// scanner is a port of src/scanner.c of the grammar. It reads the comments,
// the dollar-quoted strings and the pieces of SQL in a body of PL/pgSQL. A
// comment is -- to the end of the line, or /* ... */, which can hold other
// block comments. A piece of SQL is one token, which ends where its place
// in the grammar ends, as PostgreSQL reads it: at a semicolon, or at a word
// such as THEN or LOOP, or at a character such as a comma, when the word or
// the character is outside parentheses, brackets and CASE expressions. Each
// external token of the grammar is one place, with its own ends. The
// scanner reads past the strings, the quoted identifiers, the dollar-quoted
// strings and the comments in the SQL, so that an end in them does not end
// the token. A token does not end with a comment or with white space. The
// scanner keeps no state.
type scanner struct{}

// newScanner is tree_sitter_plpgsql_external_scanner_create.
func newScanner() *scanner {
	return &scanner{}
}

// advance is advance.
func advance(lexer *abi.Lexer) { lexer.Advance(false) }

// skip is skip.
func skip(lexer *abi.Lexer) { lexer.Advance(true) }

// isSpace is is_space. It reports whether c is white space.
func isSpace(c int32) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// isDigit is is_digit. It reports whether c is a decimal digit.
func isDigit(c int32) bool { return c >= '0' && c <= '9' }

// isWordStart is is_word_start. It reports whether c can start a word: a
// letter, an underscore or a character outside ASCII.
func isWordStart(c int32) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c >= 0x80
}

// isWordChar is is_word_char. It reports whether c can be in a word after
// its first character.
func isWordChar(c int32) bool { return isWordStart(c) || isDigit(c) || c == '$' }

// isTagChar is is_tag_char. It reports whether c can be in the tag of a
// dollar-quoted string.
func isTagChar(c int32) bool { return isWordStart(c) || isDigit(c) }

// readWord is read_word. It reads a word into w.
func readWord(lexer *abi.Lexer, w *word) {
	w.length = 0
	for isWordChar(lexer.Lookahead) {
		c := lexer.Lookahead
		if w.length < maxWord && c < 0x80 {
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			w.text[w.length] = byte(c)
			w.length++
		} else {
			w.length = maxWord + 1
		}
		advance(lexer)
	}
}

// matches is word_is. It reports whether w is the word s, which is in lower
// case.
func (w *word) matches(s string) bool {
	return w.length <= maxWord && string(w.text[:w.length]) == s
}

// endsAt is ends_at. It reports whether the word w ends a token with the
// ends e.
func endsAt(e uint16, w *word) bool {
	return (e&endThen != 0 && w.matches("then")) || (e&endWhen != 0 && w.matches("when")) ||
		(e&endLoop != 0 && w.matches("loop")) || (e&endBy != 0 && w.matches("by")) ||
		(e&endInto != 0 && w.matches("into")) || (e&endUsing != 0 && w.matches("using"))
}

// refuses is refuses. It reports whether the token kind cannot start with
// the word w, because the grammar takes the word as a keyword in its place.
func refuses(kind int, w *word) bool {
	switch kind {
	case sqlReturn:
		return w.matches("next") || w.matches("query")
	case sqlQuery:
		return w.matches("execute")
	case sqlRange:
		return w.matches("reverse") || w.matches("execute")
	case sqlStatement:
		for _, c := range commands {
			if w.matches(c) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// skipLineComment is skip_line_comment. It reads a comment to the end of
// the line. The -- of the comment is already read.
func skipLineComment(lexer *abi.Lexer) {
	for !lexer.EOF() && lexer.Lookahead != '\n' && lexer.Lookahead != '\r' {
		advance(lexer)
	}
}

// skipBlockComment is skip_block_comment. It reads a block comment to its
// end, or to the end of the input. The /* of the comment is already read. A
// block comment inside it must end before it ends.
func skipBlockComment(lexer *abi.Lexer) {
	depth := uint(1)
	for !lexer.EOF() {
		switch lexer.Lookahead {
		case '*':
			advance(lexer)
			if lexer.Lookahead == '/' {
				advance(lexer)
				depth--
				if depth == 0 {
					return
				}
			}
		case '/':
			advance(lexer)
			if lexer.Lookahead == '*' {
				advance(lexer)
				depth++
			}
		default:
			advance(lexer)
		}
	}
}

// skipQuoted is skip_quoted. It reads a string or a quoted identifier to
// its closing quote, or to the end of the input. A quote is doubled in it.
// With escapes, a backslash escapes the character after it, as in E'...'.
// The opening quote is not read yet.
func skipQuoted(lexer *abi.Lexer, quote int32, escapes bool) {
	advance(lexer)
	for !lexer.EOF() {
		c := lexer.Lookahead
		advance(lexer)
		switch {
		case c == '\\' && escapes:
			if !lexer.EOF() {
				advance(lexer)
			}
		case c == quote:
			if lexer.Lookahead != quote {
				return
			}
			advance(lexer)
		}
	}
}

// skipDollar is skip_dollar. It reads what starts with a dollar sign: a
// parameter such as $1, or a dollar-quoted string to its closing tag or to
// the end of the input. The $ is already read. When no $ ends the tag, the
// scanner reads only the characters of the tag. skipDollar reports whether
// it read a dollar-quoted string.
func skipDollar(lexer *abi.Lexer) bool {
	if isDigit(lexer.Lookahead) {
		for isDigit(lexer.Lookahead) {
			advance(lexer)
		}
		return false
	}
	var tag [maxTag]int32
	length := 0
	if isWordStart(lexer.Lookahead) {
		for isTagChar(lexer.Lookahead) {
			if length == maxTag {
				return false
			}
			tag[length] = lexer.Lookahead
			length++
			advance(lexer)
		}
	}
	if lexer.Lookahead != '$' {
		return false
	}
	advance(lexer)
	for !lexer.EOF() {
		if lexer.Lookahead != '$' {
			advance(lexer)
			continue
		}
		advance(lexer)
		i := 0
		for i < length && lexer.Lookahead == tag[i] {
			advance(lexer)
			i++
		}
		if i == length && lexer.Lookahead == '$' {
			advance(lexer)
			return true
		}
	}
	return true
}

// scanSQL is scan_sql. It reads a token of SQL of the kind kind. count is
// the number of the parts of the token that are already read. The token
// ends after the last part that is not a comment, before its end.
func scanSQL(lexer *abi.Lexer, kind int, count uint) bool {
	e := ends[kind]
	depth := uint(0)
	cases := uint(0)
loop:
	for {
		for isSpace(lexer.Lookahead) {
			advance(lexer)
		}
		if lexer.EOF() {
			break
		}
		c := lexer.Lookahead
		if c == ';' {
			break
		}
		if isWordStart(c) {
			var w word
			readWord(lexer, &w)
			if depth == 0 && cases == 0 && endsAt(e, &w) {
				break
			}
			if count == 0 && refuses(kind, &w) {
				return false
			}
			switch {
			case w.matches("case"):
				cases++
			case w.matches("end") && cases > 0:
				cases--
			case w.matches("e") && lexer.Lookahead == '\'':
				skipQuoted(lexer, '\'', true)
			}
			count++
			lexer.MarkEnd()
			// An assignment to a variable can start with the name of a
			// command, and an argument of a cursor can start with the name
			// of its parameter. Then the grammar reads the name.
			if count == 1 && (kind == sqlStatement || kind == sqlArgument) {
				for isSpace(lexer.Lookahead) {
					advance(lexer)
				}
				if kind == sqlStatement &&
					(lexer.Lookahead == '=' || lexer.Lookahead == '[' || lexer.Lookahead == '.') {
					return false
				}
				if lexer.Lookahead == ':' || (kind == sqlArgument && lexer.Lookahead == '=') {
					first := lexer.Lookahead
					advance(lexer)
					if (first == ':' && lexer.Lookahead == '=') || (first == '=' && lexer.Lookahead == '>') {
						return false
					}
					count++
					lexer.MarkEnd()
				}
			}
			continue
		}
		if kind == sqlStatement && count == 0 {
			return false
		}
		switch {
		case c == '-' || c == '/':
			advance(lexer)
			if c == '-' && lexer.Lookahead == '-' {
				advance(lexer)
				skipLineComment(lexer)
				continue
			}
			if c == '/' && lexer.Lookahead == '*' {
				advance(lexer)
				skipBlockComment(lexer)
				continue
			}
		case c == '\'' || c == '"':
			skipQuoted(lexer, c, false)
		case c == '$':
			advance(lexer)
			skipDollar(lexer)
		case isDigit(c):
			for isTagChar(lexer.Lookahead) {
				advance(lexer)
			}
		case c == '.':
			advance(lexer)
			if lexer.Lookahead == '.' {
				if depth == 0 && cases == 0 && e&endRange != 0 {
					break loop
				}
				advance(lexer)
			}
		case c == '(' || c == '[':
			depth++
			advance(lexer)
		case c == ')' || c == ']':
			end := uint16(endBracket)
			if c == ')' {
				end = endParen
			}
			if depth == 0 && e&end != 0 {
				break loop
			}
			if depth > 0 {
				depth--
			}
			advance(lexer)
		case c == ',':
			if depth == 0 && e&endComma != 0 {
				break loop
			}
			advance(lexer)
		default:
			advance(lexer)
		}
		count++
		lexer.MarkEnd()
	}
	lexer.ResultSymbol = uint16(kind)
	return count > 0
}

// Serialize is tree_sitter_plpgsql_external_scanner_serialize. The scanner
// keeps no state, so it writes no bytes.
func (*scanner) Serialize([]byte) int { return 0 }

// Deserialize is tree_sitter_plpgsql_external_scanner_deserialize.
func (*scanner) Deserialize([]byte) {}

// Scan is tree_sitter_plpgsql_external_scanner_scan.
func (*scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	kind := -1
	if !validSymbols[errorSentinel] {
		for k := sqlStatement; k <= sqlSubscript; k++ {
			if validSymbols[k] {
				kind = k
				break
			}
		}
	}
	for isSpace(lexer.Lookahead) {
		skip(lexer)
	}
	c := lexer.Lookahead
	if c == '-' || c == '/' {
		advance(lexer)
		if (c == '-' && lexer.Lookahead == '-') || (c == '/' && lexer.Lookahead == '*') {
			if !validSymbols[comment] {
				return false
			}
			advance(lexer)
			if c == '-' {
				skipLineComment(lexer)
			} else {
				skipBlockComment(lexer)
			}
			lexer.MarkEnd()
			lexer.ResultSymbol = comment
			return true
		}
		if kind < 0 || kind == sqlStatement {
			return false
		}
		lexer.MarkEnd()
		return scanSQL(lexer, kind, 1)
	}
	if kind < 0 {
		if c != '$' || !validSymbols[dollarQuotedString] || validSymbols[errorSentinel] {
			return false
		}
		advance(lexer)
		if !skipDollar(lexer) {
			return false
		}
		lexer.MarkEnd()
		lexer.ResultSymbol = dollarQuotedString
		return true
	}
	return scanSQL(lexer, kind, 0)
}
