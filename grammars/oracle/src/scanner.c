// The external scanner of the Oracle grammar.
//
// The scanner reads three tokens. COMMENT is -- to the end of the line, or
// /* ... */. Q_STRING is a string with the alternative quoting of Oracle,
// q'[...]' or nq'[...]', in any case. Its quote delimiter is any character
// but white space, and the brackets [, {, ( and < close with ], }, ) and >.
// SLASH is a / alone on its line, which ends a statement or a PL/SQL unit in
// a script. Only white space can come before it and after it on the line.

#include "tree_sitter/parser.h"

#include <stdbool.h>
#include <stdint.h>

// The external tokens, in the order of externals in grammar.js.
enum TokenType {
  COMMENT,
  Q_STRING,
  SLASH,
};

static inline void advance(TSLexer *lexer) { lexer->advance(lexer, false); }

static inline void skip(TSLexer *lexer) { lexer->advance(lexer, true); }

// is_space reports whether c is white space.
static inline bool is_space(int32_t c) {
  return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v';
}

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

// scan_q_string reads a string with the alternative quoting to its end, or
// to the end of the input. The q' of the string is already read.
static bool scan_q_string(TSLexer *lexer) {
  int32_t open = lexer->lookahead;
  if (lexer->eof(lexer) || is_space(open)) {
    return false;
  }
  int32_t close = open;
  switch (open) {
    case '[':
      close = ']';
      break;
    case '{':
      close = '}';
      break;
    case '(':
      close = ')';
      break;
    case '<':
      close = '>';
      break;
  }
  advance(lexer);
  while (!lexer->eof(lexer)) {
    if (lexer->lookahead == close) {
      advance(lexer);
      if (lexer->lookahead == '\'') {
        advance(lexer);
        break;
      }
    } else {
      advance(lexer);
    }
  }
  lexer->mark_end(lexer);
  lexer->result_symbol = Q_STRING;
  return true;
}

// scan_slash reads a / alone on its line. The / is already read, and
// line_start reports whether only white space comes before it on its line.
static bool scan_slash(TSLexer *lexer, bool line_start) {
  if (!line_start) {
    return false;
  }
  lexer->mark_end(lexer);
  while (lexer->lookahead == ' ' || lexer->lookahead == '\t' || lexer->lookahead == '\f' ||
         lexer->lookahead == '\v') {
    advance(lexer);
  }
  if (!lexer->eof(lexer) && !is_newline(lexer->lookahead)) {
    return false;
  }
  lexer->result_symbol = SLASH;
  return true;
}

void *tree_sitter_oracle_external_scanner_create(void) { return NULL; }

void tree_sitter_oracle_external_scanner_destroy(void *payload) { (void)payload; }

// The scanner has no state.
unsigned tree_sitter_oracle_external_scanner_serialize(void *payload, char *buffer) {
  (void)payload;
  (void)buffer;
  return 0;
}

void tree_sitter_oracle_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
  (void)payload;
  (void)buffer;
  (void)length;
}

bool tree_sitter_oracle_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
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
  if (c == '/') {
    // The / is the first character of its line when the white space before
    // it holds the end of a line, or when the scan starts at the start of a
    // line.
    bool line_start = valid_symbols[SLASH] && (newline || lexer->get_column(lexer) == skipped);
    advance(lexer);
    if (lexer->lookahead == '*' && valid_symbols[COMMENT]) {
      advance(lexer);
      return scan_block_comment(lexer);
    }
    return scan_slash(lexer, line_start);
  }
  if (valid_symbols[Q_STRING]) {
    if (c == 'n' || c == 'N') {
      advance(lexer);
      c = lexer->lookahead;
      if (c != 'q' && c != 'Q') {
        return false;
      }
    }
    if (c == 'q' || c == 'Q') {
      advance(lexer);
      if (lexer->lookahead != '\'') {
        return false;
      }
      advance(lexer);
      return scan_q_string(lexer);
    }
  }
  return false;
}
