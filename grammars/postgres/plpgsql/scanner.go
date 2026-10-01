package plpgsql

import "github.com/xo/transit/internal/abi"

// The scanner of PL/pgSQL. PL/pgSQL holds the SQL of PostgreSQL in many
// places of its grammar, and each place ends the SQL at its own terminator.
// The condition of IF ends at THEN, the expression of WHILE and FOR ends at
// LOOP, the expression of a dynamic EXECUTE ends at INTO, USING or ;, and a
// normal SQL statement ends at a semicolon. The PL/pgSQL parser of
// PostgreSQL calls functions such as read_sql_expression() and
// read_sql_construct(), which get the terminators of the place.
//
// The scanner follows that model with several external tokens. The grammar
// chooses the token of the place, and the scanner reads the SQL as one token
// until it reaches a terminator of that token. The postgres grammar parses
// and highlights the token later, through an injection.

// The external tokens of the grammar, in the order of externals in
// grammar.json. They are enum TokenType.
const (
	sqlStatement = iota
	sqlUntilSemicolon
	sqlUntilThen
	sqlUntilWhen
	sqlUntilLoop
	sqlUntilAssignment
	sqlUntilRange
	sqlUntilByOrLoop
	sqlUntilIntoUsingOrSemicolon
	sqlUntilUsingOrSemicolon
	sqlUntilUsingOrLoop
	sqlUntilCommaOrSemicolon
	sqlUntilCommaUsingOrSemicolon
	sqlUntilCommaOrLoop
	sqlUntilCommaOrRparen
	sqlUntilFromOrInto
	sqlUntilSemicolonGuarded
)

// scanner is a port of plpgsql/src/scanner.c of tree-sitter-postgres at
// v1.2.4 (9b27ba5c8700f9bf808221a0f6d17fe6515da787). It keeps no state, so
// it serializes no bytes.
type scanner struct{}

// newScanner is tree_sitter_plpgsql_external_scanner_create.
func newScanner() *scanner {
	return &scanner{}
}

// Serialize is tree_sitter_plpgsql_external_scanner_serialize.
func (s *scanner) Serialize([]byte) int {
	return 0
}

// Deserialize is tree_sitter_plpgsql_external_scanner_deserialize.
func (s *scanner) Deserialize([]byte) {}

// skipWhitespace is skip_whitespace.
func skipWhitespace(lexer *abi.Lexer) {
	for lexer.Lookahead == ' ' || lexer.Lookahead == '\t' ||
		lexer.Lookahead == '\n' || lexer.Lookahead == '\r' {
		lexer.Advance(true)
	}
}

// The C scanner has its own functions for ASCII, so that a Wasm build of the
// grammar needs no function of the C library. Zed and other editors can
// give a Wasm module of a grammar no ctype function of the C library.

// isASCIIAlpha is is_ascii_alpha.
func isASCIIAlpha(c int32) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isASCIIDigit is is_ascii_digit.
func isASCIIDigit(c int32) bool {
	return c >= '0' && c <= '9'
}

// isASCIIAlnum is is_ascii_alnum.
func isASCIIAlnum(c int32) bool {
	return isASCIIAlpha(c) || isASCIIDigit(c)
}

// asciiToLower is ascii_tolower. It returns a C char, the low byte of the
// character.
func asciiToLower(c int32) byte {
	if c >= 'A' && c <= 'Z' {
		return byte(c + ('a' - 'A'))
	}
	return byte(c)
}

// isTagStartChar is is_tag_start_char.
func isTagStartChar(c int32) bool {
	return isASCIIAlpha(c) || c == '_' || c >= 0x80
}

// isTagChar is is_tag_char.
func isTagChar(c int32) bool {
	return isTagStartChar(c) || isASCIIDigit(c)
}

// scanMode is ScanMode, the terminators of the token that the scanner reads.
type scanMode struct {
	symbol                      uint16
	stopSemicolon               bool
	stopThen                    bool
	stopWhen                    bool
	stopLoop                    bool
	stopInto                    bool
	stopUsing                   bool
	stopComma                   bool
	stopAssignment              bool
	stopRange                   bool
	stopBy                      bool
	stopFrom                    bool
	refusePLpgSQLStatementStart bool
	loopYieldsLoopToken         bool
	// refuseFirstReturnOpenKw and refuseFirstForInKw refuse to start the
	// token on a word that must be a keyword of PL/pgSQL in the place:
	// NEXT, QUERY or EXECUTE after RETURN, and EXECUTE after OPEN ... FOR
	// (refuseFirstReturnOpenKw), or EXECUTE or REVERSE after FOR ... IN
	// (refuseFirstForInKw).
	refuseFirstReturnOpenKw bool
	refuseFirstForInKw      bool
}

// modeFor is mode_for.
func modeFor(symbol uint16) scanMode {
	mode := scanMode{symbol: symbol}

	switch symbol {
	case sqlStatement:
		mode.stopSemicolon = true
		mode.refusePLpgSQLStatementStart = true
	case sqlUntilSemicolon:
		mode.stopSemicolon = true
	case sqlUntilThen:
		mode.stopSemicolon = true
		mode.stopThen = true
	case sqlUntilWhen:
		mode.stopSemicolon = true
		mode.stopWhen = true
	case sqlUntilLoop:
		mode.stopSemicolon = true
		mode.stopLoop = true
	case sqlUntilAssignment:
		mode.stopSemicolon = true
		mode.stopAssignment = true
	case sqlUntilRange:
		mode.stopSemicolon = true
		mode.stopRange = true
	case sqlUntilByOrLoop:
		mode.stopSemicolon = true
		mode.stopBy = true
		mode.stopLoop = true
	case sqlUntilIntoUsingOrSemicolon:
		mode.stopSemicolon = true
		mode.stopInto = true
		mode.stopUsing = true
	case sqlUntilUsingOrSemicolon:
		mode.stopSemicolon = true
		mode.stopUsing = true
	case sqlUntilUsingOrLoop:
		mode.stopSemicolon = true
		mode.stopUsing = true
		mode.stopLoop = true
	case sqlUntilCommaOrSemicolon:
		mode.stopSemicolon = true
		mode.stopComma = true
	case sqlUntilCommaUsingOrSemicolon:
		mode.stopSemicolon = true
		mode.stopComma = true
		mode.stopUsing = true
	case sqlUntilCommaOrLoop:
		mode.stopSemicolon = true
		mode.stopComma = true
		mode.stopLoop = true
	case sqlUntilCommaOrRparen:
		mode.stopSemicolon = true
		mode.stopComma = true
	case sqlUntilFromOrInto:
		mode.stopSemicolon = true
		mode.stopFrom = true
		mode.stopInto = true
	case sqlUntilSemicolonGuarded:
		mode.stopSemicolon = true
		mode.refuseFirstReturnOpenKw = true
	}

	return mode
}

// wordExecute is the word EXECUTE, which starts a statement of PL/pgSQL. The
// C scanner compares a word with it in three places.
const wordExecute = "execute"

// isPLpgSQLStatementStart is is_plpgsql_statement_start.
func isPLpgSQLStatementStart(word []byte) bool {
	switch string(word) {
	case "begin", "declare", "end", "exception", "if", "case", "loop",
		"while", "for", "foreach", "return", "raise", "assert", wordExecute,
		"perform", "call", "do", "get", "open", "fetch", "move", "close",
		"null", "exit", "continue", "commit", "rollback", "elsif", "elseif",
		"else", "when":
		return true
	}
	return false
}

// isSQLStatementStart is is_sql_statement_start.
func isSQLStatementStart(word []byte) bool {
	switch string(word) {
	case "select", "insert", "update", "delete", "merge", "with", "values",
		"create", "alter", "drop", "truncate", "grant", "revoke", "analyze",
		"analyse", "explain", "vacuum", "lock", "notify", "listen",
		"unlisten", "refresh", "reindex", "copy", "set", "reset", "show",
		"discard", "prepare", "deallocate":
		return true
	}
	return false
}

// isKeywordTerminator is is_keyword_terminator. The C function writes the
// symbol of the token through a pointer, and the Go function returns it.
func isKeywordTerminator(mode *scanMode, word []byte) (uint16, bool) {
	resultSymbol := mode.symbol
	if mode.stopLoop && string(word) == "loop" {
		if mode.loopYieldsLoopToken {
			resultSymbol = sqlUntilLoop
		}
		return resultSymbol, true
	}
	return resultSymbol, (mode.stopThen && string(word) == "then") ||
		(mode.stopWhen && string(word) == "when") ||
		(mode.stopInto && string(word) == "into") ||
		(mode.stopUsing && string(word) == "using") ||
		(mode.stopBy && string(word) == "by") ||
		(mode.stopFrom && string(word) == "from")
}

// finishToken is finish_token.
func finishToken(lexer *abi.Lexer, symbol uint16, hasContent bool) bool {
	if !hasContent {
		return false
	}
	lexer.ResultSymbol = symbol
	return true
}

// consumeSingleQuotedString is consume_single_quoted_string. The C function
// returns true on each path, and no caller reads it, so the Go function
// returns nothing.
func consumeSingleQuotedString(lexer *abi.Lexer) {
	lexer.Advance(false)
	for lexer.Lookahead != 0 {
		if lexer.Lookahead == '\'' {
			lexer.Advance(false)
			if lexer.Lookahead != '\'' {
				return
			}
			lexer.Advance(false)
		} else {
			lexer.Advance(false)
		}
	}
}

// consumeDoubleQuotedIdentifier is consume_double_quoted_identifier. It
// returns nothing, as consumeSingleQuotedString does.
func consumeDoubleQuotedIdentifier(lexer *abi.Lexer) {
	lexer.Advance(false)
	for lexer.Lookahead != 0 {
		if lexer.Lookahead == '"' {
			lexer.Advance(false)
			if lexer.Lookahead != '"' {
				return
			}
			lexer.Advance(false)
		} else {
			lexer.Advance(false)
		}
	}
}

// consumeDollarQuotedString is consume_dollar_quoted_string. It returns
// nothing, as consumeSingleQuotedString does.
//
// The C function keeps each character of the tag as a char, and it compares
// the char as an unsigned char with the lookahead. So the tag keeps the low
// byte of each character, and a character above 0xff never matches its
// byte.
func consumeDollarQuotedString(lexer *abi.Lexer) {
	lexer.Advance(false) // the opening $

	var tag [64]byte
	tagLen := 0
	if isTagStartChar(lexer.Lookahead) {
		for {
			if tagLen >= 63 {
				return
			}
			tag[tagLen] = byte(lexer.Lookahead)
			tagLen++
			lexer.Advance(false)
			if !isTagChar(lexer.Lookahead) {
				break
			}
		}
	}

	if lexer.Lookahead != '$' {
		return // It is not a dollar quote, and the scanner read a $ of SQL.
	}
	lexer.Advance(false)

	for lexer.Lookahead != 0 {
		if lexer.Lookahead == '$' {
			lexer.Advance(false)
			i := 0
			for i < tagLen && lexer.Lookahead == int32(tag[i]) {
				lexer.Advance(false)
				i++
			}
			if i == tagLen && lexer.Lookahead == '$' {
				lexer.Advance(false)
				return
			}
			continue
		}
		lexer.Advance(false)
	}
}

// scanSQL is scan_sql. The C function does not read valid_symbols, so the
// Go function does not take it.
func scanSQL(lexer *abi.Lexer, mode scanMode) bool {
	skipWhitespace(lexer)

	if lexer.Lookahead == 0 {
		return false
	}

	depth := 0
	hasContent := false
	sawFirstWord := false
	disableAssignmentStop := false

	for lexer.Lookahead != 0 {
		if depth == 0 {
			if mode.stopSemicolon && lexer.Lookahead == ';' {
				lexer.MarkEnd()
				return finishToken(lexer, mode.symbol, hasContent)
			}

			if mode.stopComma && lexer.Lookahead == ',' {
				lexer.MarkEnd()
				return finishToken(lexer, mode.symbol, hasContent)
			}
		}

		if lexer.Lookahead == '(' || lexer.Lookahead == '[' {
			depth++
			lexer.Advance(false)
			hasContent = true
			continue
		}

		if lexer.Lookahead == ')' || lexer.Lookahead == ']' {
			if depth > 0 {
				depth--
				lexer.Advance(false)
				hasContent = true
				continue
			}
			lexer.MarkEnd()
			return finishToken(lexer, mode.symbol, hasContent)
		}

		if lexer.Lookahead == '\'' {
			consumeSingleQuotedString(lexer)
			hasContent = true
			continue
		}

		if lexer.Lookahead == '"' {
			consumeDoubleQuotedIdentifier(lexer)
			hasContent = true
			continue
		}

		if lexer.Lookahead == '$' {
			consumeDollarQuotedString(lexer)
			hasContent = true
			continue
		}

		if lexer.Lookahead == '-' {
			lexer.Advance(false)
			if lexer.Lookahead == '-' {
				if !hasContent {
					return false
				}
				for lexer.Lookahead != 0 && lexer.Lookahead != '\n' {
					lexer.Advance(false)
				}
				hasContent = true
				continue
			}
			hasContent = true
			continue
		}

		if lexer.Lookahead == '/' {
			lexer.Advance(false)
			if lexer.Lookahead == '*' {
				if !hasContent {
					return false
				}
				lexer.Advance(false)
				commentDepth := 1
				for lexer.Lookahead != 0 && commentDepth > 0 {
					switch lexer.Lookahead {
					case '/':
						lexer.Advance(false)
						if lexer.Lookahead == '*' {
							commentDepth++
							lexer.Advance(false)
						}
					case '*':
						lexer.Advance(false)
						if lexer.Lookahead == '/' {
							commentDepth--
							lexer.Advance(false)
						}
					default:
						lexer.Advance(false)
					}
				}
				hasContent = true
				continue
			}
			hasContent = true
			continue
		}

		if depth == 0 && !hasContent && lexer.Lookahead == '<' {
			lexer.MarkEnd()
			lexer.Advance(false)
			if lexer.Lookahead == '<' {
				return finishToken(lexer, mode.symbol, hasContent)
			}
			hasContent = true
			continue
		}

		if depth == 0 && lexer.Lookahead == ':' {
			lexer.MarkEnd()
			lexer.Advance(false)
			if lexer.Lookahead == '=' {
				if mode.stopAssignment && !disableAssignmentStop {
					return finishToken(lexer, mode.symbol, hasContent)
				}
				lexer.Advance(false)
				hasContent = true
				continue
			}
			if lexer.Lookahead == ':' {
				lexer.Advance(false)
				hasContent = true
				continue
			}
			hasContent = true
			continue
		}

		if depth == 0 && lexer.Lookahead == '=' {
			lexer.MarkEnd()
			if mode.stopAssignment && !disableAssignmentStop {
				return finishToken(lexer, mode.symbol, hasContent)
			}
			lexer.Advance(false)
			hasContent = true
			continue
		}

		if depth == 0 && lexer.Lookahead == '.' {
			lexer.MarkEnd()
			lexer.Advance(false)
			if lexer.Lookahead == '.' {
				if mode.stopRange {
					return finishToken(lexer, mode.symbol, hasContent)
				}
				lexer.Advance(false)
				hasContent = true
				continue
			}
			hasContent = true
			continue
		}

		if depth == 0 && isASCIIAlpha(lexer.Lookahead) {
			lexer.MarkEnd()

			var buf [32]byte
			n := 0
			for isASCIIAlnum(lexer.Lookahead) || lexer.Lookahead == '_' {
				if n < 31 {
					buf[n] = asciiToLower(lexer.Lookahead)
					n++
				}
				lexer.Advance(false)
			}
			// The C array word ends with a 0 after the n characters.
			word := buf[:n]

			if !sawFirstWord {
				sawFirstWord = true
				if mode.refusePLpgSQLStatementStart && isPLpgSQLStatementStart(word) {
					return false
				}
				if mode.refuseFirstReturnOpenKw &&
					(string(word) == "next" ||
						string(word) == "query" ||
						string(word) == wordExecute) {
					return false
				}
				if mode.refuseFirstForInKw &&
					(string(word) == wordExecute ||
						string(word) == "reverse") {
					return false
				}
				if mode.stopAssignment && isSQLStatementStart(word) {
					disableAssignmentStop = true
				}
			}

			if resultSymbol, ok := isKeywordTerminator(&mode, word); ok {
				return finishToken(lexer, resultSymbol, hasContent)
			}

			hasContent = true
			continue
		}

		if isASCIIAlpha(lexer.Lookahead) || lexer.Lookahead == '_' || lexer.Lookahead >= 0x80 {
			for isASCIIAlnum(lexer.Lookahead) || lexer.Lookahead == '_' ||
				lexer.Lookahead == '$' || lexer.Lookahead >= 0x80 {
				lexer.Advance(false)
			}
			hasContent = true
			continue
		}

		lexer.Advance(false)
		hasContent = true
	}

	lexer.MarkEnd()
	return finishToken(lexer, mode.symbol, hasContent)
}

// scanAssignmentOrStatement is scan_assignment_or_statement. It gives the
// token SQL_STATEMENT when it reaches ';' before an assignment operator, and
// SQL_UNTIL_ASSIGNMENT when it reaches ':=' or '=' first. The C function does
// not read valid_symbols, so the Go function does not take it.
func scanAssignmentOrStatement(lexer *abi.Lexer) bool {
	skipWhitespace(lexer)
	if lexer.Lookahead == 0 {
		return false
	}

	depth := 0
	hasContent := false
	sawFirstWord := false
	disableAssignmentStop := false

	for lexer.Lookahead != 0 {
		if depth == 0 && lexer.Lookahead == ';' {
			lexer.MarkEnd()
			if !hasContent {
				return false
			}
			lexer.ResultSymbol = sqlStatement
			return true
		}

		if lexer.Lookahead == '(' || lexer.Lookahead == '[' {
			depth++
			lexer.Advance(false)
			hasContent = true
			continue
		}

		if lexer.Lookahead == ')' || lexer.Lookahead == ']' {
			if depth > 0 {
				depth--
				lexer.Advance(false)
				hasContent = true
				continue
			}
			lexer.MarkEnd()
			if !hasContent {
				return false
			}
			lexer.ResultSymbol = sqlStatement
			return true
		}

		if lexer.Lookahead == '\'' {
			consumeSingleQuotedString(lexer)
			hasContent = true
			continue
		}
		if lexer.Lookahead == '"' {
			consumeDoubleQuotedIdentifier(lexer)
			hasContent = true
			continue
		}
		if lexer.Lookahead == '$' {
			consumeDollarQuotedString(lexer)
			hasContent = true
			continue
		}

		if lexer.Lookahead == '-' {
			lexer.Advance(false)
			if lexer.Lookahead == '-' {
				if !hasContent {
					return false
				}
				for lexer.Lookahead != 0 && lexer.Lookahead != '\n' {
					lexer.Advance(false)
				}
				hasContent = true
				continue
			}
			hasContent = true
			continue
		}

		if lexer.Lookahead == '/' {
			lexer.Advance(false)
			if lexer.Lookahead == '*' {
				if !hasContent {
					return false
				}
				lexer.Advance(false)
				commentDepth := 1
				for lexer.Lookahead != 0 && commentDepth > 0 {
					switch lexer.Lookahead {
					case '/':
						lexer.Advance(false)
						if lexer.Lookahead == '*' {
							commentDepth++
							lexer.Advance(false)
						}
					case '*':
						lexer.Advance(false)
						if lexer.Lookahead == '/' {
							commentDepth--
							lexer.Advance(false)
						}
					default:
						lexer.Advance(false)
					}
				}
				hasContent = true
				continue
			}
			hasContent = true
			continue
		}

		if depth == 0 && !hasContent && lexer.Lookahead == '<' {
			lexer.MarkEnd()
			lexer.Advance(false)
			if lexer.Lookahead == '<' {
				// hasContent is false here, so the C function returns false
				// before it sets the symbol.
				return false
			}
			hasContent = true
			continue
		}

		if depth == 0 && lexer.Lookahead == ':' {
			lexer.MarkEnd()
			lexer.Advance(false)
			if lexer.Lookahead == '=' {
				if !disableAssignmentStop {
					return finishToken(lexer, sqlUntilAssignment, hasContent)
				}
				lexer.Advance(false)
				hasContent = true
				continue
			}
			if lexer.Lookahead == ':' {
				lexer.Advance(false)
			}
			hasContent = true
			continue
		}

		if depth == 0 && lexer.Lookahead == '=' {
			lexer.MarkEnd()
			if !disableAssignmentStop {
				return finishToken(lexer, sqlUntilAssignment, hasContent)
			}
			lexer.Advance(false)
			hasContent = true
			continue
		}

		if depth == 0 && isASCIIAlpha(lexer.Lookahead) {
			lexer.MarkEnd()
			var buf [32]byte
			n := 0
			for isASCIIAlnum(lexer.Lookahead) || lexer.Lookahead == '_' {
				if n < 31 {
					buf[n] = asciiToLower(lexer.Lookahead)
					n++
				}
				lexer.Advance(false)
			}
			// The C array word ends with a 0 after the n characters.
			word := buf[:n]

			if !sawFirstWord {
				sawFirstWord = true
				if isPLpgSQLStatementStart(word) {
					return false
				}
				if isSQLStatementStart(word) {
					disableAssignmentStop = true
				}
			}

			hasContent = true
			continue
		}

		if isASCIIAlpha(lexer.Lookahead) || lexer.Lookahead == '_' || lexer.Lookahead >= 0x80 {
			for isASCIIAlnum(lexer.Lookahead) || lexer.Lookahead == '_' ||
				lexer.Lookahead == '$' || lexer.Lookahead >= 0x80 {
				lexer.Advance(false)
			}
			hasContent = true
			continue
		}

		lexer.Advance(false)
		hasContent = true
	}

	lexer.MarkEnd()
	if !hasContent {
		return false
	}
	lexer.ResultSymbol = sqlStatement
	return true
}

// Scan is tree_sitter_plpgsql_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	if validSymbols[sqlUntilAssignment] && validSymbols[sqlStatement] {
		return scanAssignmentOrStatement(lexer)
	}

	// FOR ... IN can start a range of integers (expr .. expr) or a query
	// (query LOOP). The scanner reads once, and it chooses the token by the
	// terminator that comes first. EXECUTE and REVERSE start the dynamic
	// form and the reverse form, so they must be keywords and not the start
	// of the token.
	if validSymbols[sqlUntilRange] && validSymbols[sqlUntilLoop] {
		mode := modeFor(sqlUntilRange)
		mode.stopLoop = true
		mode.loopYieldsLoopToken = true
		mode.refuseFirstForInKw = true
		return scanSQL(lexer, mode)
	}

	for symbol := uint16(sqlStatement); symbol <= sqlUntilSemicolonGuarded; symbol++ {
		if validSymbols[symbol] {
			return scanSQL(lexer, modeFor(symbol))
		}
	}

	return false
}
