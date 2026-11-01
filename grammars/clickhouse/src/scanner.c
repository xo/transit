// The external scanner of the ClickHouse grammar.
//
// The scanner reads the comments and the heredoc strings. A comment is -- or
// // to the end of the line, # followed by a space or by ! to the end of the
// line, or /* ... */, which can hold other block comments, as ClickHouse
// reads them. A # with another character after it is not a comment. A
// heredoc string is $tag$ ... $tag$, where the tag is empty or has the form
// of an identifier without a dollar sign. The scanner gives the opening tag,
// the text and the closing tag as three tokens, so that an injection can
// take the text. It keeps the tag from the opening tag to the closing tag.
//
// The scanner also reads the data of INSERT after FORMAT and the name of
// the format, as in INSERT INTO t FORMAT CSV 1,2. The client of ClickHouse
// reads the data to the end of the line. The data starts after the spaces
// and one line feed after the name, so it can start on the next line. A
// semicolon at the end of the line, with only spaces after it, is not part
// of the data, because it ends the statement. The parser asks for the
// data only after the name of the format of INSERT.

#include "tree_sitter/alloc.h"
#include "tree_sitter/parser.h"

#include <stdbool.h>
#include <stdint.h>
#include <string.h>

// The external tokens, in the order of externals in grammar.js.
enum TokenType {
  COMMENT,
  HEREDOC_START,
  STRING_CONTENT,
  HEREDOC_END,
  INSERT_DATA,
};

// MAX_TAG is the longest tag that the scanner keeps, in characters. With
// the byte of the length, the state fits in the serialization buffer.
#define MAX_TAG 255

// The state of the scanner.
typedef struct {
  // open is 1 from the opening tag of a heredoc string to its
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

// is_blank reports whether c is white space that does not end a line.
static inline bool is_blank(int32_t c) { return c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v'; }

// is_tag_start reports whether c can start a tag: a letter, an underscore
// or a character outside ASCII.
static inline bool is_tag_start(int32_t c) {
  return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c >= 0x80;
}

// is_tag_char reports whether c can be in a tag after its first character.
static inline bool is_tag_char(int32_t c) { return is_tag_start(c) || (c >= '0' && c <= '9'); }

// scan_line_comment reads a comment to the end of the line. The start of
// the comment, --, // or # and a space or !, is already read.
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

// scan_start reads the opening tag of a heredoc string, and keeps its
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
  lexer->result_symbol = HEREDOC_START;
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

// scan_content reads the text of a heredoc string up to its closing
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
        if (!valid_symbols[HEREDOC_END]) {
          return false;
        }
        lexer->mark_end(lexer);
        s->open = 0;
        s->length = 0;
        lexer->result_symbol = HEREDOC_END;
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

// scan_insert_data reads the data of INSERT to the end of the line. It
// skips the spaces and one line feed before the data. It returns false
// when no data comes, and then it has read only white space, so that the
// caller can go on to read a comment.
static bool scan_insert_data(TSLexer *lexer) {
  while (is_blank(lexer->lookahead)) {
    skip(lexer);
  }
  if (lexer->lookahead == '\n') {
    skip(lexer);
    while (is_blank(lexer->lookahead)) {
      skip(lexer);
    }
  }
  if (lexer->lookahead == ';') {
    return false;
  }
  bool data = false;
  while (!lexer->eof(lexer) && lexer->lookahead != '\n') {
    if (lexer->lookahead == ';') {
      advance(lexer);
      while (is_blank(lexer->lookahead)) {
        advance(lexer);
      }
      if (lexer->eof(lexer) || lexer->lookahead == '\n') {
        break;
      }
      continue;
    }
    bool blank = is_blank(lexer->lookahead);
    advance(lexer);
    if (!blank) {
      lexer->mark_end(lexer);
      data = true;
    }
  }
  lexer->result_symbol = INSERT_DATA;
  return data;
}

void *tree_sitter_clickhouse_external_scanner_create(void) { return ts_calloc(1, sizeof(Scanner)); }

void tree_sitter_clickhouse_external_scanner_destroy(void *payload) { ts_free(payload); }

// The state is open and the length of the tag in two bytes, and then each
// character of the tag in four bytes, in the order of the platform.
unsigned tree_sitter_clickhouse_external_scanner_serialize(void *payload, char *buffer) {
  Scanner *s = (Scanner *)payload;
  buffer[0] = (char)s->open;
  buffer[1] = (char)s->length;
  memcpy(&buffer[2], s->tag, s->length * sizeof(int32_t));
  return 2 + s->length * sizeof(int32_t);
}

void tree_sitter_clickhouse_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
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

bool tree_sitter_clickhouse_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
  Scanner *s = (Scanner *)payload;
  if (s->open && (valid_symbols[STRING_CONTENT] || valid_symbols[HEREDOC_END])) {
    return scan_content(s, lexer, valid_symbols);
  }
  // In error recovery, each token is valid, and the scanner does not read
  // the data of INSERT.
  if (valid_symbols[INSERT_DATA] && !valid_symbols[HEREDOC_END] && scan_insert_data(lexer)) {
    return true;
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
    if (lexer->lookahead == '/') {
      advance(lexer);
      return scan_line_comment(lexer);
    }
    if (lexer->lookahead != '*') {
      return false;
    }
    advance(lexer);
    return scan_block_comment(lexer);
  }
  if (c == '#' && valid_symbols[COMMENT]) {
    advance(lexer);
    if (lexer->lookahead != ' ' && lexer->lookahead != '!') {
      return false;
    }
    advance(lexer);
    return scan_line_comment(lexer);
  }
  if (c == '$' && valid_symbols[HEREDOC_START]) {
    advance(lexer);
    return scan_start(s, lexer);
  }
  return false;
}
