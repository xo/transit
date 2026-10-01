package surrealdb

import (
	"fmt"
	"os"

	"github.com/xo/transit/internal/abi"
)

// dbgf is DBG. It writes the text to the standard error when the
// environment holds the variable TSDBG.
//
// The C macro writes nothing in a build for WebAssembly. The Go function
// writes in every build.
func dbgf(format string, args ...any) {
	if _, ok := os.LookupEnv("TSDBG"); ok {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

// External tokens emitted by the SurrealQL scanner.
// Must stay in sync with the `externals:` declaration in grammar.js.
//
// They are the C enum TokenType, in the order of externals in grammar.json.
const (
	jsFunctionBody = iota
	objectOpen
)

// ---------------------------------------------------------------------------
// JS function body scanner (kept from previous grammar)
// ---------------------------------------------------------------------------

// scanner is a port of src/scanner.c of surrealql-tree-sitter at master
// (329dcec0a057e4bc25d71e2c97e16c391573ecb0). It keeps no state, so it
// serializes no bytes.
//
// The C file also has tree_sitter_surrealql_external_scanner_reset, which
// does nothing and which the runtime never calls. It has no Go form.
type scanner struct{}

// newScanner is tree_sitter_surrealql_external_scanner_create.
func newScanner() *scanner {
	return &scanner{}
}

// Serialize is tree_sitter_surrealql_external_scanner_serialize.
func (s *scanner) Serialize([]byte) int {
	return 0
}

// Deserialize is tree_sitter_surrealql_external_scanner_deserialize.
func (s *scanner) Deserialize([]byte) {}

// advance is advance.
func advance(lexer *abi.Lexer) { lexer.Advance(false) }

// skip is skip.
func skip(lexer *abi.Lexer) { lexer.Advance(true) }

// scanJSString is scan_js_string.
//
// Scan a JS single-line string delimited by `quote`. Handles \-escapes.
func scanJSString(lexer *abi.Lexer, quote int32) {
	for lexer.Lookahead != 0 && lexer.Lookahead != '\n' {
		switch lexer.Lookahead {
		case '\\':
			advance(lexer)
			if lexer.Lookahead != 0 {
				advance(lexer)
			}
		case quote:
			advance(lexer)
			return
		default:
			advance(lexer)
		}
	}
}

// scanTemplateLiteral is scan_template_literal.
func scanTemplateLiteral(lexer *abi.Lexer) {
	for lexer.Lookahead != 0 {
		switch lexer.Lookahead {
		case '\\':
			advance(lexer)
			if lexer.Lookahead != 0 {
				advance(lexer)
			}
		case '`':
			advance(lexer)
			return
		case '$':
			advance(lexer)
			if lexer.Lookahead == '{' {
				advance(lexer)
				depth := 1
				for lexer.Lookahead != 0 && depth > 0 {
					c := lexer.Lookahead
					advance(lexer)
					switch c {
					case '{':
						depth++
					case '}':
						depth--
					case '\'', '"':
						scanJSString(lexer, c)
					case '`':
						scanTemplateLiteral(lexer)
					}
				}
			}
		default:
			advance(lexer)
		}
	}
}

// scanJSFunctionBody is scan_js_function_body.
//
// Scan a JS function body — the entire `{...}` including its braces.
// The grammar's `JavaScriptBlock` rule simply references this token. We keep
// the leading `{` requirement so the scanner does not consume regular
// SurrealQL during error recovery at other `{`-adjacent positions.
func scanJSFunctionBody(lexer *abi.Lexer) bool {
	for lexer.Lookahead == ' ' || lexer.Lookahead == '\t' ||
		lexer.Lookahead == '\n' || lexer.Lookahead == '\r' {
		skip(lexer)
	}

	if lexer.Lookahead != '{' {
		return false
	}
	advance(lexer)

	depth := 1
	for depth > 0 && lexer.Lookahead != 0 {
		c := lexer.Lookahead
		advance(lexer)

		switch c {
		case '{':
			depth++
		case '}':
			depth--
		case '\'':
			scanJSString(lexer, '\'')
		case '"':
			scanJSString(lexer, '"')
		case '`':
			scanTemplateLiteral(lexer)
		case '/':
			if lexer.Lookahead == '/' {
				advance(lexer)
				for lexer.Lookahead != 0 && lexer.Lookahead != '\n' {
					advance(lexer)
				}
			} else if lexer.Lookahead == '*' {
				advance(lexer)
				for lexer.Lookahead != 0 {
					if lexer.Lookahead == '*' {
						advance(lexer)
						if lexer.Lookahead == '/' {
							advance(lexer)
							break
						}
					} else {
						advance(lexer)
					}
				}
			}
		}
	}

	lexer.ResultSymbol = jsFunctionBody
	return true
}

// ---------------------------------------------------------------------------
// Object-open scanner — mirrors lezer's tokens.js `objectToken`.
//
// Emits OBJECT_OPEN when `{` is followed by either:
//   - another `{` is NOT next (otherwise it's a Block),
//   - immediately `}` (empty object), or
//   - an identifier/string key followed by `:` (looking past whitespace +
//     comments).
// ---------------------------------------------------------------------------

// isSpace is is_space.
func isSpace(c int32) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// isIDChar is is_id_char.
func isIDChar(c int32) bool {
	return c == '_' ||
		(c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9')
}

// skipObjWhitespace is skip_obj_whitespace.
//
// Skip whitespace and line comments after the opening `{`. Mirrors lezer's
// skipSpace in tokens.js (handles #, //, --).
func skipObjWhitespace(lexer *abi.Lexer) {
	for {
		c := lexer.Lookahead
		if isSpace(c) {
			skip(lexer)
			continue
		}
		if c == '#' {
			skip(lexer)
			for lexer.Lookahead != 0 && lexer.Lookahead != '\n' {
				skip(lexer)
			}
			continue
		}
		if (c == '/' || c == '-') && lexer.Lookahead != 0 {
			// Peek the next char: we need lookahead == lookahead too. tree-sitter
			// doesn't have a peek-2 API, so we eagerly advance and check.
			skip(lexer)
			if lexer.Lookahead == c {
				skip(lexer)
				for lexer.Lookahead != 0 && lexer.Lookahead != '\n' {
					skip(lexer)
				}
				continue
			}
			// Not a comment — we already consumed a char, so we cannot
			// back-track. Bail out — caller will treat next non-space char as
			// the object key (which is correct for our purposes since `/` and
			// `-` are not valid identifier starts).
			return
		}
		break
	}
}

// consumeObjKey is consume_obj_key.
//
// Try to consume an identifier-like or string key starting at the current
// lookahead position. Returns true if a key was consumed.
func consumeObjKey(lexer *abi.Lexer) bool {
	c := lexer.Lookahead
	if isIDChar(c) && !(c >= '0' && c <= '9') {
		// identifier
		for isIDChar(lexer.Lookahead) {
			skip(lexer)
		}
		return true
	}
	if c >= '0' && c <= '9' {
		// numeric key — used by some object literals; treat as identifier-like
		for isIDChar(lexer.Lookahead) {
			skip(lexer)
		}
		return true
	}
	if c == '\'' || c == '"' {
		quote := c
		skip(lexer)
		for lexer.Lookahead != 0 {
			if lexer.Lookahead == '\\' {
				skip(lexer)
				if lexer.Lookahead != 0 {
					skip(lexer)
				}
				continue
			}
			if lexer.Lookahead == quote {
				skip(lexer)
				return true
			}
			skip(lexer)
		}
		return false
	}
	return false
}

// scanObjectOpen is scan_object_open.
func scanObjectOpen(lexer *abi.Lexer) bool {
	// Skip any leading whitespace/comments that come before the `{`. Tree-sitter
	// sometimes invokes the external scanner before extras are processed, so
	// we need to be tolerant here. Using skip() so they don't extend the token.
	for {
		c := lexer.Lookahead
		if isSpace(c) {
			skip(lexer)
			continue
		}
		if c == '#' {
			skip(lexer)
			for lexer.Lookahead != 0 && lexer.Lookahead != '\n' {
				skip(lexer)
			}
			continue
		}
		break
	}

	if lexer.Lookahead != '{' {
		return false
	}
	// Consume the `{`. We must use advance() so the token includes it.
	advance(lexer)
	lexer.MarkEnd()

	// Skip whitespace and comments — we use skip() so they don't extend the
	// token, but we already marked the end past `{`.
	skipObjWhitespace(lexer)

	c := lexer.Lookahead

	if c == '{' {
		// Direct nested brace → this is a Block, not an Object.
		return false
	}
	if c == '}' {
		// Empty {} — treat as Object.
		lexer.ResultSymbol = objectOpen
		return true
	}
	// Otherwise look for `key:`
	if !consumeObjKey(lexer) {
		return false
	}
	skipObjWhitespace(lexer)
	if lexer.Lookahead == ':' {
		// `{ fn::foo(1); }` is a Block whose first statement is a namespaced
		// call, not an Object keyed `fn`. The token end is already marked past
		// the `{`, so advancing here to see the second `:` costs nothing.
		skip(lexer)
		if lexer.Lookahead == ':' {
			return false
		}
		lexer.ResultSymbol = objectOpen
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Dispatcher
// ---------------------------------------------------------------------------

// Scan is tree_sitter_surrealql_external_scanner_scan.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	la := '?'
	if lexer.Lookahead >= 32 && lexer.Lookahead < 127 {
		la = lexer.Lookahead
	}
	dbgf("scan: valid=[js=%d,obj=%d] la='%c'(%d)\n",
		boolInt(validSymbols[jsFunctionBody]), boolInt(validSymbols[objectOpen]),
		la, lexer.Lookahead)

	if validSymbols[objectOpen] {
		ok := scanObjectOpen(lexer)
		dbgf("  object_open => %d\n", boolInt(ok))
		if ok {
			return true
		}
	}

	if validSymbols[jsFunctionBody] {
		if scanJSFunctionBody(lexer) {
			return true
		}
	}

	return false
}

// boolInt is the C conversion of a bool to an int.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
