// The external scanner of the PL/pgSQL grammar.
//
// The scanner reads the comments, the dollar-quoted strings and the pieces
// of SQL in a body of PL/pgSQL. A comment is -- to the end of the line, or /* ... */, which can
// hold other block comments. A piece of SQL is one token, which ends where
// its place in the grammar ends, as PostgreSQL reads it: at a semicolon, or
// at a word such as THEN or LOOP, or at a character such as a comma, when
// the word or the character is outside parentheses, brackets and CASE
// expressions. Each external token of the grammar is one place, with its own
// ends. The scanner reads past the strings, the quoted identifiers, the
// dollar-quoted strings and the comments in the SQL, so that an end in them
// does not end the token. A token does not end with a comment or with white
// space. The scanner keeps no state.

#include "tree_sitter/parser.h"

#include <stdbool.h>
#include <stdint.h>
#include <string.h>

// The external tokens, in the order of externals in grammar.js.
enum TokenType {
  COMMENT,
  SQL_STATEMENT,
  SQL_SEMICOLON,
  SQL_RETURN,
  SQL_QUERY,
  SQL_THEN,
  SQL_WHEN,
  SQL_LOOP,
  SQL_RANGE,
  SQL_BY,
  SQL_LIST,
  SQL_LIST_LOOP,
  SQL_ARGUMENT,
  SQL_SUBSCRIPT,
  DOLLAR_QUOTED_STRING,
  ERROR_SENTINEL,
};

// The ends of a token, besides the semicolon, which ends each token.
enum {
  END_THEN = 1 << 0,
  END_WHEN = 1 << 1,
  END_LOOP = 1 << 2,
  END_RANGE = 1 << 3,
  END_BY = 1 << 4,
  END_COMMA = 1 << 5,
  END_INTO = 1 << 6,
  END_USING = 1 << 7,
  END_PAREN = 1 << 8,
  END_BRACKET = 1 << 9,
};

// ENDS holds the ends of each token of SQL.
static const uint16_t ENDS[] = {
    [COMMENT] = 0,
    [SQL_STATEMENT] = 0,
    [SQL_SEMICOLON] = 0,
    [SQL_RETURN] = 0,
    [SQL_QUERY] = 0,
    [SQL_THEN] = END_THEN,
    [SQL_WHEN] = END_WHEN,
    [SQL_LOOP] = END_LOOP,
    [SQL_RANGE] = END_RANGE | END_LOOP,
    [SQL_BY] = END_BY | END_LOOP,
    [SQL_LIST] = END_COMMA | END_INTO | END_USING,
    [SQL_LIST_LOOP] = END_COMMA | END_USING | END_LOOP,
    [SQL_ARGUMENT] = END_COMMA | END_PAREN,
    [SQL_SUBSCRIPT] = END_BRACKET,
};

// COMMANDS holds the first words of the SQL commands that a body can hold.
// The commands of transactions, cursors and dynamic SQL are statements of
// PL/pgSQL, so they are not in the list.
static const char *const COMMANDS[] = {
    "alter",   "analyse",  "analyze", "call",     "checkpoint", "cluster",  "comment",
    "copy",    "create",   "deallocate", "delete", "discard",   "do",       "drop",
    "explain", "grant",    "import",  "insert",   "listen",     "load",     "lock",
    "merge",   "notify",   "prepare", "reassign", "refresh",    "reindex",  "release",
    "repack",  "reset",    "revoke",  "savepoint", "security",  "select",   "set",
    "show",    "table",    "truncate", "unlisten", "update",    "vacuum",   "values",
    "with",
};

// MAX_WORD is the longest word that the scanner compares, in bytes.
#define MAX_WORD 16

// MAX_TAG is the longest tag of a dollar-quoted string that the scanner
// reads, in characters.
#define MAX_TAG 64

// A word of the SQL, in lower case. length is MAX_WORD + 1 when the word is
// longer than MAX_WORD or holds a character outside ASCII, so that it
// matches no word that the scanner compares.
typedef struct {
  char text[MAX_WORD + 1];
  unsigned length;
} Word;

static inline void advance(TSLexer *lexer) { lexer->advance(lexer, false); }

static inline void skip(TSLexer *lexer) { lexer->advance(lexer, true); }

// is_space reports whether c is white space.
static inline bool is_space(int32_t c) {
  return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v';
}

// is_digit reports whether c is a decimal digit.
static inline bool is_digit(int32_t c) { return c >= '0' && c <= '9'; }

// is_word_start reports whether c can start a word: a letter, an
// underscore or a character outside ASCII.
static inline bool is_word_start(int32_t c) {
  return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c >= 0x80;
}

// is_word_char reports whether c can be in a word after its first
// character.
static inline bool is_word_char(int32_t c) { return is_word_start(c) || is_digit(c) || c == '$'; }

// is_tag_char reports whether c can be in the tag of a dollar-quoted
// string.
static inline bool is_tag_char(int32_t c) { return is_word_start(c) || is_digit(c); }

// read_word reads a word into w.
static void read_word(TSLexer *lexer, Word *w) {
  w->length = 0;
  while (is_word_char(lexer->lookahead)) {
    int32_t c = lexer->lookahead;
    if (w->length < MAX_WORD && c < 0x80) {
      w->text[w->length++] = (char)(c >= 'A' && c <= 'Z' ? c + ('a' - 'A') : c);
    } else {
      w->length = MAX_WORD + 1;
    }
    advance(lexer);
  }
  if (w->length <= MAX_WORD) {
    w->text[w->length] = 0;
  }
}

// word_is reports whether w is the word s, which is in lower case.
static bool word_is(const Word *w, const char *s) {
  return w->length <= MAX_WORD && strcmp(w->text, s) == 0;
}

// ends_at reports whether the word w ends a token with the ends ends.
static bool ends_at(uint16_t ends, const Word *w) {
  return ((ends & END_THEN) && word_is(w, "then")) || ((ends & END_WHEN) && word_is(w, "when")) ||
         ((ends & END_LOOP) && word_is(w, "loop")) || ((ends & END_BY) && word_is(w, "by")) ||
         ((ends & END_INTO) && word_is(w, "into")) || ((ends & END_USING) && word_is(w, "using"));
}

// refuses reports whether the token kind cannot start with the word w,
// because the grammar takes the word as a keyword in its place.
static bool refuses(int kind, const Word *w) {
  switch (kind) {
  case SQL_RETURN:
    return word_is(w, "next") || word_is(w, "query");
  case SQL_QUERY:
    return word_is(w, "execute");
  case SQL_RANGE:
    return word_is(w, "reverse") || word_is(w, "execute");
  case SQL_STATEMENT:
    for (unsigned i = 0; i < sizeof(COMMANDS) / sizeof(COMMANDS[0]); i++) {
      if (word_is(w, COMMANDS[i])) {
        return false;
      }
    }
    return true;
  default:
    return false;
  }
}

// skip_line_comment reads a comment to the end of the line. The -- of the
// comment is already read.
static void skip_line_comment(TSLexer *lexer) {
  while (!lexer->eof(lexer) && lexer->lookahead != '\n' && lexer->lookahead != '\r') {
    advance(lexer);
  }
}

// skip_block_comment reads a block comment to its end, or to the end of the
// input. The /* of the comment is already read. A block comment inside it
// must end before it ends.
static void skip_block_comment(TSLexer *lexer) {
  unsigned depth = 1;
  while (!lexer->eof(lexer)) {
    if (lexer->lookahead == '*') {
      advance(lexer);
      if (lexer->lookahead == '/') {
        advance(lexer);
        depth--;
        if (depth == 0) {
          return;
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
}

// skip_quoted reads a string or a quoted identifier to its closing quote,
// or to the end of the input. A quote is doubled in it. With escapes, a
// backslash escapes the character after it, as in E'...'. The opening
// quote is not read yet.
static void skip_quoted(TSLexer *lexer, int32_t quote, bool escapes) {
  advance(lexer);
  while (!lexer->eof(lexer)) {
    int32_t c = lexer->lookahead;
    advance(lexer);
    if (c == '\\' && escapes) {
      if (!lexer->eof(lexer)) {
        advance(lexer);
      }
    } else if (c == quote) {
      if (lexer->lookahead != quote) {
        return;
      }
      advance(lexer);
    }
  }
}

// skip_dollar reads what starts with a dollar sign: a parameter such as $1,
// or a dollar-quoted string to its closing tag or to the end of the input.
// The $ is already read. When no $ ends the tag, the scanner reads only the
// characters of the tag. skip_dollar reports whether it read a
// dollar-quoted string.
static bool skip_dollar(TSLexer *lexer) {
  if (is_digit(lexer->lookahead)) {
    while (is_digit(lexer->lookahead)) {
      advance(lexer);
    }
    return false;
  }
  int32_t tag[MAX_TAG];
  unsigned length = 0;
  if (is_word_start(lexer->lookahead)) {
    while (is_tag_char(lexer->lookahead)) {
      if (length == MAX_TAG) {
        return false;
      }
      tag[length++] = lexer->lookahead;
      advance(lexer);
    }
  }
  if (lexer->lookahead != '$') {
    return false;
  }
  advance(lexer);
  while (!lexer->eof(lexer)) {
    if (lexer->lookahead != '$') {
      advance(lexer);
      continue;
    }
    advance(lexer);
    unsigned i = 0;
    while (i < length && lexer->lookahead == tag[i]) {
      advance(lexer);
      i++;
    }
    if (i == length && lexer->lookahead == '$') {
      advance(lexer);
      return true;
    }
  }
  return true;
}

// scan_sql reads a token of SQL of the kind kind. count is the number of
// the parts of the token that are already read. The token ends after the
// last part that is not a comment, before its end.
static bool scan_sql(TSLexer *lexer, int kind, unsigned count) {
  uint16_t ends = ENDS[kind];
  unsigned depth = 0;
  unsigned cases = 0;
  for (;;) {
    while (is_space(lexer->lookahead)) {
      advance(lexer);
    }
    if (lexer->eof(lexer)) {
      break;
    }
    int32_t c = lexer->lookahead;
    if (c == ';') {
      break;
    }
    if (is_word_start(c)) {
      Word w;
      read_word(lexer, &w);
      if (depth == 0 && cases == 0 && ends_at(ends, &w)) {
        break;
      }
      if (count == 0 && refuses(kind, &w)) {
        return false;
      }
      if (word_is(&w, "case")) {
        cases++;
      } else if (word_is(&w, "end") && cases > 0) {
        cases--;
      } else if (word_is(&w, "e") && lexer->lookahead == '\'') {
        skip_quoted(lexer, '\'', true);
      }
      count++;
      lexer->mark_end(lexer);
      // An assignment to a variable can start with the name of a command,
      // and an argument of a cursor can start with the name of its
      // parameter. Then the grammar reads the name.
      if (count == 1 && (kind == SQL_STATEMENT || kind == SQL_ARGUMENT)) {
        while (is_space(lexer->lookahead)) {
          advance(lexer);
        }
        if (kind == SQL_STATEMENT &&
            (lexer->lookahead == '=' || lexer->lookahead == '[' || lexer->lookahead == '.')) {
          return false;
        }
        if (lexer->lookahead == ':' || (kind == SQL_ARGUMENT && lexer->lookahead == '=')) {
          int32_t first = lexer->lookahead;
          advance(lexer);
          if ((first == ':' && lexer->lookahead == '=') || (first == '=' && lexer->lookahead == '>')) {
            return false;
          }
          count++;
          lexer->mark_end(lexer);
        }
      }
      continue;
    }
    if (kind == SQL_STATEMENT && count == 0) {
      return false;
    }
    if (c == '-' || c == '/') {
      advance(lexer);
      if (c == '-' && lexer->lookahead == '-') {
        advance(lexer);
        skip_line_comment(lexer);
        continue;
      }
      if (c == '/' && lexer->lookahead == '*') {
        advance(lexer);
        skip_block_comment(lexer);
        continue;
      }
    } else if (c == '\'' || c == '"') {
      skip_quoted(lexer, c, false);
    } else if (c == '$') {
      advance(lexer);
      skip_dollar(lexer);
    } else if (is_digit(c)) {
      while (is_tag_char(lexer->lookahead)) {
        advance(lexer);
      }
    } else if (c == '.') {
      advance(lexer);
      if (lexer->lookahead == '.') {
        if (depth == 0 && cases == 0 && (ends & END_RANGE)) {
          break;
        }
        advance(lexer);
      }
    } else if (c == '(' || c == '[') {
      depth++;
      advance(lexer);
    } else if (c == ')' || c == ']') {
      if (depth == 0 && (ends & (c == ')' ? END_PAREN : END_BRACKET))) {
        break;
      }
      if (depth > 0) {
        depth--;
      }
      advance(lexer);
    } else if (c == ',') {
      if (depth == 0 && (ends & END_COMMA)) {
        break;
      }
      advance(lexer);
    } else {
      advance(lexer);
    }
    count++;
    lexer->mark_end(lexer);
  }
  lexer->result_symbol = kind;
  return count > 0;
}

void *tree_sitter_plpgsql_external_scanner_create(void) { return NULL; }

void tree_sitter_plpgsql_external_scanner_destroy(void *payload) { (void)payload; }

unsigned tree_sitter_plpgsql_external_scanner_serialize(void *payload, char *buffer) {
  (void)payload;
  (void)buffer;
  return 0;
}

void tree_sitter_plpgsql_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
  (void)payload;
  (void)buffer;
  (void)length;
}

bool tree_sitter_plpgsql_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
  (void)payload;
  int kind = -1;
  if (!valid_symbols[ERROR_SENTINEL]) {
    for (int k = SQL_STATEMENT; k <= SQL_SUBSCRIPT; k++) {
      if (valid_symbols[k]) {
        kind = k;
        break;
      }
    }
  }
  while (is_space(lexer->lookahead)) {
    skip(lexer);
  }
  int32_t c = lexer->lookahead;
  if (c == '-' || c == '/') {
    advance(lexer);
    if ((c == '-' && lexer->lookahead == '-') || (c == '/' && lexer->lookahead == '*')) {
      if (!valid_symbols[COMMENT]) {
        return false;
      }
      advance(lexer);
      if (c == '-') {
        skip_line_comment(lexer);
      } else {
        skip_block_comment(lexer);
      }
      lexer->mark_end(lexer);
      lexer->result_symbol = COMMENT;
      return true;
    }
    if (kind < 0 || kind == SQL_STATEMENT) {
      return false;
    }
    lexer->mark_end(lexer);
    return scan_sql(lexer, kind, 1);
  }
  if (kind < 0) {
    if (c != '$' || !valid_symbols[DOLLAR_QUOTED_STRING] || valid_symbols[ERROR_SENTINEL]) {
      return false;
    }
    advance(lexer);
    if (!skip_dollar(lexer)) {
      return false;
    }
    lexer->mark_end(lexer);
    lexer->result_symbol = DOLLAR_QUOTED_STRING;
    return true;
  }
  return scan_sql(lexer, kind, 0);
}
