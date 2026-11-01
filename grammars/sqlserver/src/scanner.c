// The external scanner of the SQL Server grammar.
//
// The scanner reads three tokens. COMMENT is -- to the end of the line, or
// /* ... */, where a block comment can hold another block comment, as in
// /* a /* b */ c */. GO is the word GO, in any case, alone on its line with
// an optional count, as in GO 5. It ends a batch in sqlcmd and in SQL
// Server Management Studio. Only white space can come before it on the
// line, and only white space, the count and a comment after it. The token
// holds the word only, and the grammar reads the count.
//
// END is the end of a statement that has no semicolon. It holds no
// characters. The scanner gives it where the grammar can end a statement,
// when the next word starts a statement, as in SELECT 1 PRINT 'a', or when
// the next word is ELSE or END, or the next line is GO, or a label comes
// next, or the input ends. Then the parser does not need each word that
// starts a statement as a lookahead after each expression, and those parse
// states stay small. FETCH and WITH can also continue a statement, so the
// scanner reads the words after them. FETCH starts a statement unless FIRST
// or NEXT comes after it with no FROM, as in OFFSET 5 ROWS FETCH NEXT 5
// ROWS ONLY. WITH starts a statement when a common table expression comes
// after it, as in WITH c (a) AS, or XMLNAMESPACES.
//
// ERROR_SENTINEL is a token that the grammar never uses. When it is valid,
// the parser recovers from an error, and the scanner gives no END.

#include "tree_sitter/parser.h"

#include <stdbool.h>
#include <stdint.h>
#include <string.h>

// The external tokens, in the order of externals in grammar.js.
enum TokenType {
  COMMENT,
  GO,
  END,
  ERROR_SENTINEL,
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

// is_go_rest reports whether the rest of the line after GO holds only white
// space, a count and a comment. The lookahead is the character after GO.
static bool is_go_rest(TSLexer *lexer) {
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
    return lexer->lookahead == '-';
  }
  return lexer->eof(lexer) || is_newline(lexer->lookahead);
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
  if (!is_go_rest(lexer)) {
    return false;
  }
  lexer->result_symbol = GO;
  return true;
}

// The words that start a statement, and ELSE and END, which end the
// statement before them.
static const char *const statement_words[] = {
  "ALTER", "BACKUP", "BEGIN", "BREAK", "BULK", "CHECKPOINT", "CLOSE", "COMMIT",
  "CONTINUE", "CREATE", "DBCC", "DEALLOCATE", "DECLARE", "DELETE", "DENY",
  "DROP", "ELSE", "END", "EXEC", "EXECUTE", "FETCH", "GOTO", "GRANT", "IF",
  "INSERT", "KILL", "MERGE", "OPEN", "PRINT", "RAISERROR", "READTEXT",
  "RECONFIGURE", "RESTORE", "RETURN", "REVERT", "REVOKE", "ROLLBACK", "SAVE",
  "SELECT", "SET", "SETUSER", "SHUTDOWN", "THROW", "TRUNCATE", "UPDATE",
  "UPDATETEXT", "USE", "WAITFOR", "WHILE", "WITH", "WRITETEXT",
};

// The longest word that the scanner compares, with room for the end of the
// string.
#define WORD_SIZE 16

// is_name_start reports whether c can start an identifier.
static inline bool is_name_start(int32_t c) {
  return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '#' || c >= 0x80;
}

// is_name_char reports whether c can be in an identifier.
static inline bool is_name_char(int32_t c) {
  return is_name_start(c) || (c >= '0' && c <= '9') || c == '@' || c == '$';
}

// skip_blank reads white space, with lines.
static void skip_blank(TSLexer *lexer) {
  while (is_space(lexer->lookahead)) {
    advance(lexer);
  }
}

// read_name reads an identifier, and writes it in upper case into word. If
// the identifier is not only letters and _, or is too long, word is empty.
// The lookahead is the first character of the identifier.
static void read_name(TSLexer *lexer, char *word) {
  unsigned n = 0;
  bool letters = true;
  while (is_name_char(lexer->lookahead)) {
    int32_t c = lexer->lookahead;
    if (c >= 'a' && c <= 'z') {
      c -= 'a' - 'A';
    }
    if (!((c >= 'A' && c <= 'Z') || c == '_')) {
      letters = false;
    } else if (n < WORD_SIZE - 1) {
      word[n] = (char)c;
    }
    n++;
    advance(lexer);
  }
  if (!letters || n >= WORD_SIZE) {
    n = 0;
  }
  word[n] = 0;
}

// read_word skips white space and reads the next identifier into word, as
// read_name does. If no identifier comes next, word is empty.
static void read_word(TSLexer *lexer, char *word) {
  skip_blank(lexer);
  word[0] = 0;
  if (is_name_start(lexer->lookahead)) {
    read_name(lexer, word);
  }
}

// is_cte reports whether a common table expression comes after WITH: a
// name, the names of its columns in parentheses, and AS, or XMLNAMESPACES.
static bool is_cte(TSLexer *lexer) {
  char word[WORD_SIZE];
  skip_blank(lexer);
  int32_t c = lexer->lookahead;
  if (c == '[' || c == '"') {
    int32_t end = c == '[' ? ']' : '"';
    advance(lexer);
    while (!lexer->eof(lexer) && lexer->lookahead != end) {
      advance(lexer);
    }
    advance(lexer);
  } else if (is_name_start(c)) {
    read_name(lexer, word);
    if (strcmp(word, "XMLNAMESPACES") == 0) {
      return true;
    }
  } else {
    return false;
  }
  skip_blank(lexer);
  if (lexer->lookahead == '(') {
    while (!lexer->eof(lexer) && lexer->lookahead != ')') {
      advance(lexer);
    }
    advance(lexer);
  }
  read_word(lexer, word);
  return strcmp(word, "AS") == 0;
}

// is_fetch_statement reports whether the words after FETCH make it the
// start of a statement and not the FETCH of a query, which FIRST or NEXT
// and a count follow. FETCH FIRST FROM and FETCH NEXT FROM start a
// statement.
static bool is_fetch_statement(TSLexer *lexer) {
  char word[WORD_SIZE];
  read_word(lexer, word);
  if (strcmp(word, "FIRST") == 0 || strcmp(word, "NEXT") == 0) {
    read_word(lexer, word);
    return strcmp(word, "FROM") == 0;
  }
  return true;
}

// scan_end reports whether a statement ends before the lookahead. The
// token holds no characters, so its end is already marked. newline and
// skipped tell whether the lookahead is the first character of its line,
// as for GO.
static bool scan_end(TSLexer *lexer, bool newline, uint32_t skipped) {
  lexer->result_symbol = END;
  if (lexer->eof(lexer)) {
    return true;
  }
  if (!is_name_start(lexer->lookahead)) {
    return false;
  }
  bool first = newline || lexer->get_column(lexer) == skipped;
  char word[WORD_SIZE];
  read_name(lexer, word);
  if (lexer->lookahead == ':') {
    advance(lexer);
    return lexer->lookahead != ':';
  }
  if (first && strcmp(word, "GO") == 0) {
    return is_go_rest(lexer);
  }
  for (size_t i = 0; i < sizeof(statement_words) / sizeof(statement_words[0]); i++) {
    if (strcmp(word, statement_words[i]) == 0) {
      if (strcmp(word, "FETCH") == 0) {
        return is_fetch_statement(lexer);
      }
      if (strcmp(word, "WITH") == 0) {
        return is_cte(lexer);
      }
      return true;
    }
  }
  return false;
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
    // line. When the scan starts at the G, an END before it read the white
    // space, and the END comes only before GO that is first on its line.
    if (!newline && lexer->get_column(lexer) != skipped && skipped != 0) {
      return false;
    }
    return scan_go(lexer);
  }
  if (valid_symbols[END] && !valid_symbols[ERROR_SENTINEL]) {
    lexer->mark_end(lexer);
    return scan_end(lexer, newline, skipped);
  }
  return false;
}
