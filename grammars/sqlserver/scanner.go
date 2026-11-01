package sqlserver

import "github.com/xo/transit/internal/abi"

// The external tokens, the C enum TokenType, in the order of externals in
// grammar.js. goLine is GO of C, because go is a keyword of Go, and
// endStatement is END of C.
const (
	comment = iota
	goLine
	endStatement
	errorSentinel
)

// scanner is a port of src/scanner.c of the grammar. It reads three tokens.
// comment is -- to the end of the line, or /* ... */, where a block comment
// can hold another block comment, as in /* a /* b */ c */. goLine is the
// word GO, in any case, alone on its line with an optional count, as in GO
// 5. It ends a batch in sqlcmd and in SQL Server Management Studio. Only
// white space can come before it on the line, and only white space, the
// count and a comment after it. The token holds the word only, and the
// grammar reads the count.
//
// endStatement is the end of a statement that has no semicolon. It holds no
// characters. The scanner gives it where the grammar can end a statement,
// when the next word starts a statement, as in SELECT 1 PRINT 'a', or when
// the next word is ELSE or END, or the next line is GO, or a label comes
// next, or the input ends. Then the parser does not need each word that
// starts a statement as a lookahead after each expression, and those parse
// states stay small. FETCH and WITH can also continue a statement, so the
// scanner reads the words after them. FETCH starts a statement unless FIRST
// or NEXT comes after it with no FROM, as in OFFSET 5 ROWS FETCH NEXT 5
// ROWS ONLY. WITH starts a statement when a common table expression comes
// after it, as in WITH c (a) AS, or XMLNAMESPACES. The scanner also gives
// endStatement before WITH CHECK OPTION, which ends the query of a view, so
// that the parse states after the query of a view are the states after a
// query.
//
// errorSentinel is a token that the grammar never uses. When it is valid,
// the parser recovers from an error, and the scanner gives no endStatement.
type scanner struct{}

// newScanner is tree_sitter_sqlserver_external_scanner_create.
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

// isBlank is is_blank. It reports whether c is white space that does not
// end a line.
func isBlank(c int32) bool { return c == ' ' || c == '\t' || c == '\f' || c == '\v' }

// isNewline is is_newline. It reports whether c ends a line.
func isNewline(c int32) bool { return c == '\n' || c == '\r' }

// scanLineComment is scan_line_comment. It reads a comment to the end of
// the line. The -- of the comment is already read.
func scanLineComment(lexer *abi.Lexer) bool {
	for !lexer.EOF() && !isNewline(lexer.Lookahead) {
		advance(lexer)
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = comment
	return true
}

// scanBlockComment is scan_block_comment. It reads a block comment to its
// end, or to the end of the input. The /* of the comment is already read. A
// /* in the comment opens another comment, which its own */ closes.
func scanBlockComment(lexer *abi.Lexer) bool {
	depth := uint32(1)
	for !lexer.EOF() {
		switch lexer.Lookahead {
		case '*':
			advance(lexer)
			if lexer.Lookahead == '/' {
				advance(lexer)
				depth--
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
		if depth == 0 {
			break
		}
	}
	lexer.MarkEnd()
	lexer.ResultSymbol = comment
	return true
}

// isGoRest is is_go_rest. It reports whether the rest of the line after GO
// holds only white space, a count and a comment. The lookahead is the
// character after GO.
func isGoRest(lexer *abi.Lexer) bool {
	for isBlank(lexer.Lookahead) {
		advance(lexer)
	}
	for lexer.Lookahead >= '0' && lexer.Lookahead <= '9' {
		advance(lexer)
	}
	for isBlank(lexer.Lookahead) {
		advance(lexer)
	}
	if lexer.Lookahead == '-' {
		advance(lexer)
		return lexer.Lookahead == '-'
	}
	return lexer.EOF() || isNewline(lexer.Lookahead)
}

// scanGo is scan_go. It reads the rest of a line with GO. The G is the
// lookahead, and only white space comes before it on its line.
func scanGo(lexer *abi.Lexer) bool {
	advance(lexer)
	if lexer.Lookahead != 'o' && lexer.Lookahead != 'O' {
		return false
	}
	advance(lexer)
	lexer.MarkEnd()
	if !isGoRest(lexer) {
		return false
	}
	lexer.ResultSymbol = goLine
	return true
}

// statementWords is statement_words. It holds the words that start a
// statement, and ELSE and END, which end the statement before them.
var statementWords = map[string]bool{
	"ALTER": true, "BACKUP": true, "BEGIN": true, "BREAK": true, "BULK": true,
	"CHECKPOINT": true, "CLOSE": true, "COMMIT": true, "CONTINUE": true,
	"CREATE": true, "DBCC": true, "DEALLOCATE": true, "DECLARE": true,
	"DELETE": true, "DENY": true, "DISABLE": true, "DROP": true, "ELSE": true,
	"ENABLE": true, "END": true, "EXEC": true, "EXECUTE": true, "FETCH": true,
	"GET": true, "GOTO": true, "GRANT": true, "IF": true, "INSERT": true,
	"KILL": true, "MERGE": true, "OPEN": true, "PRINT": true,
	"RAISERROR": true, "READTEXT": true, "RECEIVE": true, "RECONFIGURE": true,
	"RESTORE": true, "RETURN": true, "REVERT": true, "REVOKE": true,
	"ROLLBACK": true, "SAVE": true, "SELECT": true, "SEND": true, "SET": true,
	"SETUSER": true, "SHUTDOWN": true, "THROW": true, "TRUNCATE": true,
	"UPDATE": true, "UPDATETEXT": true, "USE": true, "WAITFOR": true,
	"WHILE": true, "WITH": true, "WRITETEXT": true,
}

// wordSize is WORD_SIZE. It is the longest word that the scanner compares,
// with room for the end of the string of C.
const wordSize = 16

// isNameStart is is_name_start. It reports whether c can start an
// identifier.
func isNameStart(c int32) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '#' || c >= 0x80
}

// isNameChar is is_name_char. It reports whether c can be in an identifier.
func isNameChar(c int32) bool {
	return isNameStart(c) || (c >= '0' && c <= '9') || c == '@' || c == '$'
}

// skipBlank is skip_blank. It reads white space, with lines.
func skipBlank(lexer *abi.Lexer) {
	for isSpace(lexer.Lookahead) {
		advance(lexer)
	}
}

// readName is read_name. It reads an identifier, and returns it in upper
// case. If the identifier is not only letters and _, or is too long, the
// word is empty. The lookahead is the first character of the identifier.
func readName(lexer *abi.Lexer) string {
	var word [wordSize]byte
	n := 0
	letters := true
	for isNameChar(lexer.Lookahead) {
		c := lexer.Lookahead
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		if !((c >= 'A' && c <= 'Z') || c == '_') {
			letters = false
		} else if n < wordSize-1 {
			word[n] = byte(c)
		}
		n++
		advance(lexer)
	}
	if !letters || n >= wordSize {
		return ""
	}
	return string(word[:n])
}

// readWord is read_word. It skips white space and reads the next
// identifier, as readName does. If no identifier comes next, the word is
// empty.
func readWord(lexer *abi.Lexer) string {
	skipBlank(lexer)
	if isNameStart(lexer.Lookahead) {
		return readName(lexer)
	}
	return ""
}

// isCTE is is_cte. It reports whether a common table expression comes after
// WITH: a name, the names of its columns in parentheses, and AS, or
// XMLNAMESPACES. It also reports whether CHECK OPTION comes after WITH.
func isCTE(lexer *abi.Lexer) bool {
	skipBlank(lexer)
	c := lexer.Lookahead
	switch {
	case c == '[' || c == '"':
		end := c
		if c == '[' {
			end = ']'
		}
		advance(lexer)
		for !lexer.EOF() && lexer.Lookahead != end {
			advance(lexer)
		}
		advance(lexer)
	case isNameStart(c):
		switch readName(lexer) {
		case "XMLNAMESPACES":
			return true
		case "CHECK":
			return readWord(lexer) == "OPTION"
		}
	default:
		return false
	}
	skipBlank(lexer)
	if lexer.Lookahead == '(' {
		for !lexer.EOF() && lexer.Lookahead != ')' {
			advance(lexer)
		}
		advance(lexer)
	}
	return readWord(lexer) == "AS"
}

// isFetchStatement is is_fetch_statement. It reports whether the words
// after FETCH make it the start of a statement and not the FETCH of a
// query, which FIRST or NEXT and a count follow. FETCH FIRST FROM and FETCH
// NEXT FROM start a statement.
func isFetchStatement(lexer *abi.Lexer) bool {
	word := readWord(lexer)
	if word == "FIRST" || word == "NEXT" {
		return readWord(lexer) == "FROM"
	}
	return true
}

// scanEnd is scan_end. It reports whether a statement ends before the
// lookahead. The token holds no characters, so its end is already marked.
// newline and skipped tell whether the lookahead is the first character of
// its line, as for GO.
func scanEnd(lexer *abi.Lexer, newline bool, skipped uint32) bool {
	lexer.ResultSymbol = endStatement
	if lexer.EOF() {
		return true
	}
	if !isNameStart(lexer.Lookahead) {
		return false
	}
	first := newline || lexer.GetColumn() == skipped
	word := readName(lexer)
	if lexer.Lookahead == ':' {
		advance(lexer)
		return lexer.Lookahead != ':'
	}
	if first && word == "GO" {
		return isGoRest(lexer)
	}
	if !statementWords[word] {
		return false
	}
	switch word {
	case "FETCH":
		return isFetchStatement(lexer)
	case "WITH":
		return isCTE(lexer)
	}
	return true
}

// Serialize is tree_sitter_sqlserver_external_scanner_serialize. The
// scanner has no state.
func (s *scanner) Serialize([]byte) int {
	return 0
}

// Deserialize is tree_sitter_sqlserver_external_scanner_deserialize.
func (s *scanner) Deserialize([]byte) {}

// Scan is tree_sitter_sqlserver_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	// newline reports whether the white space before the token holds the
	// end of a line, and skipped counts its characters.
	newline := false
	var skipped uint32
	for isSpace(lexer.Lookahead) {
		if isNewline(lexer.Lookahead) {
			newline = true
		}
		skip(lexer)
		skipped++
	}
	c := lexer.Lookahead
	if c == '-' && validSymbols[comment] {
		advance(lexer)
		if lexer.Lookahead != '-' {
			return false
		}
		advance(lexer)
		return scanLineComment(lexer)
	}
	if c == '/' && validSymbols[comment] {
		advance(lexer)
		if lexer.Lookahead != '*' {
			return false
		}
		advance(lexer)
		return scanBlockComment(lexer)
	}
	if (c == 'g' || c == 'G') && validSymbols[goLine] {
		// GO is the first word of its line when the white space before it
		// holds the end of a line, or when the scan starts at the start of a
		// line. When the scan starts at the G, an END before it read the
		// white space, and the END comes only before GO that is first on its
		// line.
		if !newline && lexer.GetColumn() != skipped && skipped != 0 {
			return false
		}
		return scanGo(lexer)
	}
	if validSymbols[endStatement] && !validSymbols[errorSentinel] {
		lexer.MarkEnd()
		return scanEnd(lexer, newline, skipped)
	}
	return false
}
