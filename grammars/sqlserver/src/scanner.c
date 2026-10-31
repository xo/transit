// The external scanner of the SQL Server grammar.
//
// The scanner reads two tokens. COMMENT is -- to the end of the line, or
// /* ... */, where a block comment can hold another block comment, as in
// /* a /* b */ c */. GO is the word GO, in any case, alone on its line with
// an optional count, as in GO 5. It ends a batch in sqlcmd and in SQL
// Server Management Studio. Only white space can come before it on the
// line, and only white space, the count and a comment after it. The token
// holds the word only, and the grammar reads the count.

#include "tree_sitter/parser.h"

#include <stdbool.h>
#include <stdint.h>

// The external tokens, in the order of externals in grammar.js.
enum TokenType {
  COMMENT,
  GO,
};

static inline void advance(TSLexer *lexer) { lexer->advance(lexer, false); }

static inline void skip(TSLexer *lexer) { lexer->advance(lexer, true); }

// is_space reports whether c is white space.
static inline bool is_space(int32_t c) {
  return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v';
}

// is_blank reports whether c is white space that does not end a line.
static inline bool is_blank(int32_t c) { return c == ' ' || c == '\t' || c == '\f' || c == '\v'; }

// is_newline reports whether c ends a line.
static inline bool is_newline(int32_t c) { return c == '\n' || c == '\r'; }

// scan_line_comment reads a comment to the end of the line. The -- of the
// comment is already read.
static bool scan_line_comment(TSLexer *lexer) {
  while (!lexer->eof(lexer) && !is_newline(lexer->lookahead)) {
    advance(lexer);
  }
  lexer->mark_end(lexer);
  lexer->result_symbol = COMMENT;
  return true;
}

// scan_block_comment reads a block comment to its end, or to the end of the
// input. The /* of the comment is already read. A /* in the comment opens
// another comment, which its own */ closes.
static bool scan_block_comment(TSLexer *lexer) {
  uint32_t depth = 1;
  while (!lexer->eof(lexer)) {
    if (lexer->lookahead == '*') {
      advance(lexer);
      if (lexer->lookahead == '/') {
        advance(lexer);
        depth--;
        if (depth == 0) {
          break;
        }
      }
    } else if (lexer->lookahead == '/') {
      advance(lexer);
      if (lexer->lookahead == '*') {
        advance(lexer);
        depth++;
      }
    } else {
      advance(lexer);
    }
  }
  lexer->mark_end(lexer);
  lexer->result_symbol = COMMENT;
  return true;
}

// scan_go reads the rest of a line with GO. The G is the lookahead, and
// only white space comes before it on its line.
static bool scan_go(TSLexer *lexer) {
  advance(lexer);
  if (lexer->lookahead != 'o' && lexer->lookahead != 'O') {
    return false;
  }
  advance(lexer);
  lexer->mark_end(lexer);
  while (is_blank(lexer->lookahead)) {
    advance(lexer);
  }
  while (lexer->lookahead >= '0' && lexer->lookahead <= '9') {
    advance(lexer);
  }
  while (is_blank(lexer->lookahead)) {
    advance(lexer);
  }
  if (lexer->lookahead == '-') {
    advance(lexer);
    if (lexer->lookahead != '-') {
      return false;
    }
  } else if (!lexer->eof(lexer) && !is_newline(lexer->lookahead)) {
    return false;
  }
  lexer->result_symbol = GO;
  return true;
}

void *tree_sitter_sqlserver_external_scanner_create(void) { return NULL; }

void tree_sitter_sqlserver_external_scanner_destroy(void *payload) { (void)payload; }

// The scanner has no state.
unsigned tree_sitter_sqlserver_external_scanner_serialize(void *payload, char *buffer) {
  (void)payload;
  (void)buffer;
  return 0;
}

void tree_sitter_sqlserver_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
  (void)payload;
  (void)buffer;
  (void)length;
}

bool tree_sitter_sqlserver_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
  (void)payload;
  // newline reports whether the white space before the token holds the end
  // of a line, and skipped counts its characters.
  bool newline = false;
  uint32_t skipped = 0;
  while (is_space(lexer->lookahead)) {
    if (is_newline(lexer->lookahead)) {
      newline = true;
    }
    skip(lexer);
    skipped++;
  }
  int32_t c = lexer->lookahead;
  if (c == '-' && valid_symbols[COMMENT]) {
    advance(lexer);
    if (lexer->lookahead != '-') {
      return false;
    }
    advance(lexer);
    return scan_line_comment(lexer);
  }
  if (c == '/' && valid_symbols[COMMENT]) {
    advance(lexer);
    if (lexer->lookahead != '*') {
      return false;
    }
    advance(lexer);
    return scan_block_comment(lexer);
  }
  if ((c == 'g' || c == 'G') && valid_symbols[GO]) {
    // GO is the first word of its line when the white space before it
    // holds the end of a line, or when the scan starts at the start of a
    // line.
    if (!newline && lexer->get_column(lexer) != skipped) {
      return false;
    }
    return scan_go(lexer);
  }
  return false;
}
