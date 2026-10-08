// The external scanner of the MySQL grammar.
//
// The scanner reads the comments: # and -- to the end of the line, and
// /* ... */. The -- starts a comment only before white space, a control
// character or the end of the input, as the manual says. It reads the start
// of a version comment, /*! or /*M! with the digits of a version after it,
// and its end, */. The text between the two is SQL, so the parser reads it as
// the rest of the input.

#include "tree_sitter/alloc.h"
#include "tree_sitter/parser.h"

#include <stdbool.h>
#include <stdint.h>

// The external tokens, in the order of externals in grammar.js.
enum TokenType {
  COMMENT,
  VERSION_COMMENT_START,
  VERSION_COMMENT_END,
};

// The state of the scanner.
typedef struct {
  // version is 1 inside a version comment, from its start to its end.
  uint8_t version;
} Scanner;

static inline void advance(TSLexer *lexer) { lexer->advance(lexer, false); }

static inline void skip(TSLexer *lexer) { lexer->advance(lexer, true); }

// is_space reports whether c is white space.
static inline bool is_space(int32_t c) {
  return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v';
}

// is_newline reports whether c ends a line.
static inline bool is_newline(int32_t c) { return c == '\n' || c == '\r'; }

// scan_line_comment reads a comment to the end of the line. The start of the
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
// input. The /* of the comment is already read.
static bool scan_block_comment(TSLexer *lexer) {
  while (!lexer->eof(lexer)) {
    if (lexer->lookahead == '*') {
      advance(lexer);
      if (lexer->lookahead == '/') {
        advance(lexer);
        break;
      }
    } else {
      advance(lexer);
    }
  }
  lexer->mark_end(lexer);
  lexer->result_symbol = COMMENT;
  return true;
}

// scan_version_start reads the digits of the version of a version comment.
// The /*! or /*M! of the comment is already read.
static bool scan_version_start(Scanner *s, TSLexer *lexer) {
  while (lexer->lookahead >= '0' && lexer->lookahead <= '9') {
    advance(lexer);
  }
  lexer->mark_end(lexer);
  s->version = 1;
  lexer->result_symbol = VERSION_COMMENT_START;
  return true;
}

// scan_comment reads a block comment or the start of a version comment. The
// /* is already read.
static bool scan_comment(Scanner *s, TSLexer *lexer, const bool *valid_symbols) {
  if (lexer->lookahead == '!' && valid_symbols[VERSION_COMMENT_START]) {
    advance(lexer);
    return scan_version_start(s, lexer);
  }
  if (lexer->lookahead == 'M' && valid_symbols[VERSION_COMMENT_START]) {
    advance(lexer);
    if (lexer->lookahead == '!') {
      advance(lexer);
      return scan_version_start(s, lexer);
    }
  }
  return scan_block_comment(lexer);
}

void *tree_sitter_mysql_external_scanner_create(void) { return ts_calloc(1, sizeof(Scanner)); }

void tree_sitter_mysql_external_scanner_destroy(void *payload) { ts_free(payload); }

// The state is version, in one byte.
unsigned tree_sitter_mysql_external_scanner_serialize(void *payload, char *buffer) {
  Scanner *s = (Scanner *)payload;
  buffer[0] = (char)s->version;
  return 1;
}

void tree_sitter_mysql_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
  Scanner *s = (Scanner *)payload;
  s->version = 0;
  if (length > 0) {
    s->version = buffer[0] != 0;
  }
}

bool tree_sitter_mysql_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
  Scanner *s = (Scanner *)payload;
  while (is_space(lexer->lookahead)) {
    skip(lexer);
  }
  int32_t c = lexer->lookahead;
  if (c == '#' && valid_symbols[COMMENT]) {
    advance(lexer);
    return scan_line_comment(lexer);
  }
  if (c == '-' && valid_symbols[COMMENT]) {
    advance(lexer);
    if (lexer->lookahead != '-') {
      return false;
    }
    advance(lexer);
    int32_t d = lexer->lookahead;
    if (lexer->eof(lexer) || d <= ' ' || d == 0x7f) {
      return scan_line_comment(lexer);
    }
    return false;
  }
  if (c == '/' && valid_symbols[COMMENT]) {
    advance(lexer);
    if (lexer->lookahead != '*') {
      return false;
    }
    advance(lexer);
    return scan_comment(s, lexer, valid_symbols);
  }
  if (c == '*' && s->version && valid_symbols[VERSION_COMMENT_END]) {
    advance(lexer);
    if (lexer->lookahead != '/') {
      return false;
    }
    advance(lexer);
    lexer->mark_end(lexer);
    s->version = 0;
    lexer->result_symbol = VERSION_COMMENT_END;
    return true;
  }
  return false;
}
