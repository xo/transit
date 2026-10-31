// The external scanner of the PostgreSQL grammar.
//
// The scanner reads the comments and the dollar-quoted strings. A comment
// is -- to the end of the line, or /* ... */, which can hold other block
// comments, as the section "Comments" of the manual says. A dollar-quoted
// string is $tag$ ... $tag$, where the tag is empty or has the form of an
// identifier without a dollar sign. The scanner gives the opening tag, the
// text and the closing tag as three tokens, so that an injection can take
// the text. It keeps the tag from the opening tag to the closing tag.

#include "tree_sitter/alloc.h"
#include "tree_sitter/parser.h"

#include <stdbool.h>
#include <stdint.h>
#include <string.h>

// The external tokens, in the order of externals in grammar.js.
enum TokenType {
  COMMENT,
  DOLLAR_QUOTE_START,
  STRING_CONTENT,
  DOLLAR_QUOTE_END,
};

// MAX_TAG is the longest tag that the scanner keeps, in characters. With
// the byte of the length, the state fits in the serialization buffer.
#define MAX_TAG 255

// The state of the scanner.
typedef struct {
  // open is 1 from the opening tag of a dollar-quoted string to its
  // closing tag.
  uint8_t open;
  // length is the number of the characters of the tag.
  uint8_t length;
  // tag holds the characters of the tag.
  int32_t tag[MAX_TAG];
} Scanner;

static inline void advance(TSLexer *lexer) { lexer->advance(lexer, false); }

static inline void skip(TSLexer *lexer) { lexer->advance(lexer, true); }

// is_space reports whether c is white space.
static inline bool is_space(int32_t c) {
  return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v';
}

// is_tag_start reports whether c can start a tag: a letter, an underscore
// or a character outside ASCII.
static inline bool is_tag_start(int32_t c) {
  return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c >= 0x80;
}

// is_tag_char reports whether c can be in a tag after its first character.
static inline bool is_tag_char(int32_t c) { return is_tag_start(c) || (c >= '0' && c <= '9'); }

// scan_line_comment reads a comment to the end of the line. The -- of the
// comment is already read.
static bool scan_line_comment(TSLexer *lexer) {
  while (!lexer->eof(lexer) && lexer->lookahead != '\n' && lexer->lookahead != '\r') {
    advance(lexer);
  }
  lexer->mark_end(lexer);
  lexer->result_symbol = COMMENT;
  return true;
}

// scan_block_comment reads a block comment to its end, or to the end of the
// input. The /* of the comment is already read. A block comment inside it
// must end before it ends.
static bool scan_block_comment(TSLexer *lexer) {
  unsigned depth = 1;
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

// scan_start reads the opening tag of a dollar-quoted string, and keeps its
// tag. The $ of the tag is already read.
static bool scan_start(Scanner *s, TSLexer *lexer) {
  uint8_t length = 0;
  if (is_tag_start(lexer->lookahead)) {
    while (is_tag_char(lexer->lookahead)) {
      if (length == MAX_TAG) {
        return false;
      }
      s->tag[length++] = lexer->lookahead;
      advance(lexer);
    }
  }
  if (lexer->lookahead != '$') {
    return false;
  }
  advance(lexer);
  lexer->mark_end(lexer);
  s->open = 1;
  s->length = length;
  lexer->result_symbol = DOLLAR_QUOTE_START;
  return true;
}

// scan_end reports whether the closing tag comes next, and reads as much of
// it as matches. The $ before the tag is already read.
static bool scan_end(Scanner *s, TSLexer *lexer) {
  for (uint8_t i = 0; i < s->length; i++) {
    if (lexer->lookahead != s->tag[i]) {
      return false;
    }
    advance(lexer);
  }
  if (lexer->lookahead != '$') {
    return false;
  }
  advance(lexer);
  return true;
}

// scan_content reads the text of a dollar-quoted string up to its closing
// tag, or the closing tag when no text comes before it. The text goes to
// the end of the input when the closing tag does not come.
static bool scan_content(Scanner *s, TSLexer *lexer, const bool *valid_symbols) {
  bool text = false;
  while (!lexer->eof(lexer)) {
    if (lexer->lookahead == '$') {
      lexer->mark_end(lexer);
      advance(lexer);
      if (scan_end(s, lexer)) {
        if (text) {
          lexer->result_symbol = STRING_CONTENT;
          return valid_symbols[STRING_CONTENT];
        }
        if (!valid_symbols[DOLLAR_QUOTE_END]) {
          return false;
        }
        lexer->mark_end(lexer);
        s->open = 0;
        s->length = 0;
        lexer->result_symbol = DOLLAR_QUOTE_END;
        return true;
      }
    } else {
      advance(lexer);
    }
    text = true;
  }
  lexer->mark_end(lexer);
  lexer->result_symbol = STRING_CONTENT;
  return text && valid_symbols[STRING_CONTENT];
}

void *tree_sitter_postgres_external_scanner_create(void) { return ts_calloc(1, sizeof(Scanner)); }

void tree_sitter_postgres_external_scanner_destroy(void *payload) { ts_free(payload); }

// The state is open and the length of the tag in two bytes, and then each
// character of the tag in four bytes, in the order of the platform.
unsigned tree_sitter_postgres_external_scanner_serialize(void *payload, char *buffer) {
  Scanner *s = (Scanner *)payload;
  buffer[0] = (char)s->open;
  buffer[1] = (char)s->length;
  memcpy(&buffer[2], s->tag, s->length * sizeof(int32_t));
  return 2 + s->length * sizeof(int32_t);
}

void tree_sitter_postgres_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
  Scanner *s = (Scanner *)payload;
  s->open = 0;
  s->length = 0;
  if (length < 2) {
    return;
  }
  uint8_t n = (uint8_t)buffer[1];
  if (length < 2 + n * sizeof(int32_t)) {
    return;
  }
  s->open = buffer[0] != 0;
  s->length = n;
  memcpy(s->tag, &buffer[2], n * sizeof(int32_t));
}

bool tree_sitter_postgres_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
  Scanner *s = (Scanner *)payload;
  if (s->open && (valid_symbols[STRING_CONTENT] || valid_symbols[DOLLAR_QUOTE_END])) {
    return scan_content(s, lexer, valid_symbols);
  }
  while (is_space(lexer->lookahead)) {
    skip(lexer);
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
  if (c == '$' && valid_symbols[DOLLAR_QUOTE_START]) {
    advance(lexer);
    return scan_start(s, lexer);
  }
  return false;
}
