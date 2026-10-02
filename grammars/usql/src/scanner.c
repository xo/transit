// The external scanner of the usql grammar (D101, D102, D108).
//
// The scanner reads every token of the grammar. It finds the end of a
// statement, the strings, the dollar quotes, the comments and the depth of
// the parentheses. It splits the name of a meta command into its base name
// and its modifiers, with the commands of usql. It ends the arguments of a
// meta command at the end of the line or at the next backslash outside
// quotes. The client lexer of PostgreSQL, src/bin/psql/psqlscan.l, is the
// reference for the rules.
//
// The scanner tells the contexts apart by the valid tokens. COMMAND_END is
// valid only inside the arguments of a meta command, the option tokens only
// inside a list of options, and VARIABLE_NAME only after the sigil of a
// variable. Everywhere else, the text is SQL.
//
// The options of the dialect change some tokens. USQL_OPTIONS gives them as
// a set of the flags below. A build can set it, as the test module does
// with -DUSQL_OPTIONS=<n>, to build the scanner of other options.

#include "tree_sitter/alloc.h"
#include "tree_sitter/parser.h"

#include <stdbool.h>
#include <stdint.h>
#include <string.h>

// The flags of the options of a dialect. They are the fields of the type
// Options of the Go package, in the same order.
#define USQL_DOLLAR_QUOTES 1   // $tag$ ... $tag$ is a string
#define USQL_BLOCK_COMMENTS 2  // /* ... */ is a comment
#define USQL_SLASH_COMMENTS 4  // // starts a comment
#define USQL_HASH_COMMENTS 8   // # starts a comment
#define USQL_BACKTICKS 16      // `...` is a quoted identifier

// The options of the scanner. The default is dollar quotes and block
// comments, the options of PostgreSQL. The upstream tool and the golden
// harness build the scanner with the default.
#ifndef USQL_OPTIONS
#define USQL_OPTIONS (USQL_DOLLAR_QUOTES | USQL_BLOCK_COMMENTS)
#endif

// The external tokens, in the order of externals in grammar.js.
enum TokenType {
  SQL_TEXT,
  STRING,
  QUOTED_IDENTIFIER,
  DOLLAR_STRING,
  COMMENT,
  SEMICOLON,
  BACKSLASH,
  BACKSLASH_OPTIONS,
  COMMAND_BASE,
  SHELL_COMMAND_NAME,
  SEPARATOR_COMMAND_NAME,
  MODIFIER,
  COMMAND_END,
  WORD,
  PIPE,
  SHELL_COMMAND,
  BACKTICK_OPEN,
  BACKTICK_TEXT,
  BACKTICK_CLOSE,
  OPTION_OPEN,
  OPTION_CLOSE,
  OPTION_EQUALS,
  OPTION_WORD,
  SIGIL_PLAIN,
  SIGIL_SINGLE_QUOTE,
  SIGIL_DOUBLE_QUOTE,
  SIGIL_BRACE,
  VARIABLE_NAME,
  CLOSE_SINGLE_QUOTE,
  CLOSE_DOUBLE_QUOTE,
  CLOSE_BRACE,
  ERROR_SENTINEL,
};

// The context of a text token: SQL, an argument of a meta command, or a
// word in a list of options.
enum Context {
  CONTEXT_SQL,
  CONTEXT_ARGUMENT,
  CONTEXT_OPTION,
};

// The longest command name that the scanner splits. A longer name is not a
// command of usql, so it is one whole name.
#define MAX_NAME_LENGTH 32

// The longest tag of a dollar quote, as usql allows it.
#define MAX_TAG_LENGTH 128

// The number of bytes that tree_sitter_usql_external_scanner_serialize
// writes.
#define SERIALIZED_SIZE 6

// Scanner is the state of the scanner.
typedef struct {
  // depth is the depth of the parentheses in the statement.
  uint32_t depth;
  // base is the length of the base name of the meta command that
  // BACKSLASH starts, which COMMAND_BASE reads, or 0 when the name is one
  // whole name.
  uint8_t base;
  // command is 1 from the name of a meta command to its COMMAND_END. It
  // makes COMMAND_END, which reads no character, change the state.
  uint8_t command;
} Scanner;

// Command is a meta command of usql, from metacmd/descs.go of usql.
typedef struct {
  // base is the base name, without the backslash.
  const char *base;
  // classes holds the letters that can follow the base name in any order,
  // each once, before the modifiers.
  const char *classes;
  // modifiers holds the letters that can follow the base name, each once,
  // in their order.
  const char *modifiers;
  // options is true when the command takes a list of options in
  // parentheses.
  bool options;
} Command;

// commands holds the meta commands of usql. A name in brackets in
// metacmd/descs.go, such as d[S+], gives the base name d and the modifiers
// S and +. usql accepts the classes tvmsE of \d in any order.
static const Command commands[] = {
    {"q", "", "", false},
    {"quit", "", "", false},
    {"copyright", "", "", false},
    {"drivers", "", "", false},
    {"?", "", "", false},
    {"c", "", "", false},
    {"connect", "", "", false},
    {"Z", "", "", false},
    {"disconnect", "", "", false},
    {"password", "", "", false},
    {"passwd", "", "", false},
    {"conninfo", "", "", false},
    {"g", "", "", true},
    {"go", "", "", true},
    {"G", "", "", true},
    {"ego", "", "", true},
    {"gx", "", "", true},
    {"gexec", "", "", false},
    {"gset", "", "", true},
    {"bind", "", "", false},
    {"timing", "", "", false},
    {"crosstab", "", "", true},
    {"crosstabview", "", "", true},
    {"xtab", "", "", true},
    {"chart", "", "", true},
    {"watch", "", "", true},
    {"e", "", "", false},
    {"edit", "", "", false},
    {"p", "", "", false},
    {"print", "", "", false},
    {"raw", "", "", false},
    {"exec", "", "", false},
    {"w", "", "", false},
    {"write", "", "", false},
    {"r", "", "", false},
    {"reset", "", "", false},
    {"d", "tvmsE", "S+", false},
    {"da", "", "S", false},
    {"dA", "", "+", false},
    {"dAc", "", "+", false},
    {"dAf", "", "+", false},
    {"dAo", "", "+", false},
    {"dAp", "", "+", false},
    {"db", "", "+", false},
    {"dc", "", "S+", false},
    {"dconfig", "", "+", false},
    {"dC", "", "+", false},
    {"dd", "", "S", false},
    {"dD", "", "S+", false},
    {"ddp", "", "", false},
    {"dE", "", "S+", false},
    {"des", "", "+", false},
    {"det", "", "+", false},
    {"deu", "", "+", false},
    {"dew", "", "+", false},
    {"df", "", "anptwS+", false},
    {"dF", "", "+", false},
    {"dFd", "", "+", false},
    {"dFp", "", "+", false},
    {"dFt", "", "+", false},
    {"dg", "", "S+", false},
    {"di", "", "S+", false},
    {"dl", "", "+", false},
    {"dL", "", "S+", false},
    {"dm", "", "S+", false},
    {"dn", "", "S+", false},
    {"do", "", "S+", false},
    {"dO", "", "S+", false},
    {"dp", "", "S", false},
    {"dP", "", "+", false},
    {"drds", "", "", false},
    {"drg", "", "S", false},
    {"dRp", "", "+", false},
    {"dRs", "", "+", false},
    {"ds", "", "S+", false},
    {"dt", "", "S+", false},
    {"dT", "", "S+", false},
    {"du", "", "S+", false},
    {"dv", "", "S+", false},
    {"dx", "", "+", false},
    {"dX", "", "", false},
    {"dy", "", "+", false},
    {"l", "", "+", false},
    {"lo_list", "", "+", false},
    {"z", "", "S", false},
    {"sf", "", "+", false},
    {"sv", "", "+", false},
    {"ss", "", "+", false},
    {"set", "", "", false},
    {"unset", "", "", false},
    {"pset", "", "", false},
    {"a", "", "", false},
    {"C", "", "", false},
    {"f", "", "", false},
    {"H", "", "", false},
    {"T", "", "", false},
    {"t", "", "", false},
    {"x", "", "", false},
    {"cset", "", "", false},
    {"prompt", "", "", false},
    {"echo", "", "", false},
    {"qecho", "", "", false},
    {"warn", "", "", false},
    {"o", "", "", false},
    {"out", "", "", false},
    {"copy", "", "", false},
    {"i", "", "", false},
    {"include", "", "", false},
    {"ir", "", "", false},
    {"include_relative", "", "", false},
    {"if", "", "", false},
    {"elif", "", "", false},
    {"else", "", "", false},
    {"endif", "", "", false},
    {"begin", "", "", false},
    {"commit", "", "", false},
    {"rollback", "", "", false},
    {"abort", "", "", false},
    {"cd", "", "", false},
    {"getenv", "", "", false},
    {"setenv", "", "", false},
};

static inline void advance(TSLexer *lexer) { lexer->advance(lexer, false); }

static inline void skip(TSLexer *lexer) { lexer->advance(lexer, true); }

// is_newline reports whether c ends a line.
static inline bool is_newline(int32_t c) { return c == '\n' || c == '\r'; }

// is_space reports whether c is white space.
static inline bool is_space(int32_t c) {
  return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v';
}

// is_variable_char reports whether c can be in the name of a variable, as
// variable_char of psqlscan.l: a letter, a digit, an underscore or any
// character that is not ASCII.
static inline bool is_variable_char(int32_t c) {
  return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' ||
         c >= 0x80;
}

// is_tag_start reports whether c can start the tag of a dollar quote, as
// dolq_start of psqlscan.l.
static inline bool is_tag_start(int32_t c) {
  return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c >= 0x80;
}

// is_identifier_char reports whether c can be in an identifier, as
// ident_cont of psqlscan.l. A dollar quote cannot start after one.
static inline bool is_identifier_char(int32_t c) { return is_variable_char(c) || c == '$'; }

// is_name_end reports whether the lookahead ends the name of a meta command:
// white space, a backslash, a control character or the end of the input.
static inline bool is_name_end(TSLexer *lexer) {
  int32_t c = lexer->lookahead;
  return lexer->eof(lexer) || is_space(c) || c == '\\' || c < 0x20 || c == 0x7f;
}

// valid_modifiers reports whether the n letters of rest are modifiers of
// the command cmd.
static bool valid_modifiers(const Command *cmd, const char *rest, unsigned n) {
  unsigned i = 0;
  unsigned seen = 0;
  for (; i < n; i++) {
    const char *p = strchr(cmd->classes, rest[i]);
    if (p == NULL || rest[i] == '\0') {
      break;
    }
    unsigned bit = 1u << (unsigned)(p - cmd->classes);
    if (seen & bit) {
      break;
    }
    seen |= bit;
  }
  const char *m = cmd->modifiers;
  for (; i < n; i++) {
    const char *p = strchr(m, rest[i]);
    if (p == NULL || rest[i] == '\0') {
      return false;
    }
    m = p + 1;
  }
  return true;
}

// split_name returns the command of the longest base name that starts the
// name of n letters, with modifiers after it, and sets *base to its length.
// It returns NULL when no command matches.
static const Command *split_name(const char *name, unsigned n, unsigned *base) {
  for (unsigned length = n; length > 0; length--) {
    for (unsigned i = 0; i < sizeof(commands) / sizeof(commands[0]); i++) {
      const Command *cmd = &commands[i];
      if (strlen(cmd->base) == length && strncmp(cmd->base, name, length) == 0 &&
          valid_modifiers(cmd, name + length, n - length)) {
        *base = length;
        return cmd;
      }
    }
  }
  return NULL;
}

// scan_line_comment scans a comment to the end of the line.
static bool scan_line_comment(TSLexer *lexer) {
  while (!lexer->eof(lexer) && !is_newline(lexer->lookahead)) {
    advance(lexer);
  }
  lexer->mark_end(lexer);
  lexer->result_symbol = COMMENT;
  return true;
}

// scan_block_comment scans the rest of a block comment after its /*. The
// comment does not nest, and with no */ it runs to the end of the input.
static bool scan_block_comment(TSLexer *lexer) {
  while (!lexer->eof(lexer)) {
    if (lexer->lookahead == '*') {
      advance(lexer);
      if (lexer->lookahead == '/') {
        advance(lexer);
        break;
      }
      continue;
    }
    advance(lexer);
  }
  lexer->mark_end(lexer);
  lexer->result_symbol = COMMENT;
  return true;
}

// scan_quoted_rest scans the rest of a quoted text after its opening quote
// q, up to the closing quote. A doubled quote is a quote, and in a string in
// single quotes a backslash escapes the next character. With no closing
// quote, the text runs to the end of the line when line is true, and else
// to the end of the input.
static void scan_quoted_rest(TSLexer *lexer, int32_t q, bool line) {
  for (;;) {
    if (lexer->eof(lexer) || (line && is_newline(lexer->lookahead))) {
      break;
    }
    int32_t c = lexer->lookahead;
    if (c == '\\' && q == '\'') {
      advance(lexer);
      if (!lexer->eof(lexer) && !(line && is_newline(lexer->lookahead))) {
        advance(lexer);
      }
      continue;
    }
    advance(lexer);
    if (c == q) {
      if (lexer->lookahead != q) {
        break;
      }
      advance(lexer);
    }
  }
  lexer->mark_end(lexer);
}

// scan_quoted scans a quoted text from its opening quote, and returns the
// token symbol.
static bool scan_quoted(TSLexer *lexer, enum TokenType symbol, bool line) {
  int32_t q = lexer->lookahead;
  advance(lexer);
  scan_quoted_rest(lexer, q, line);
  lexer->result_symbol = symbol;
  return true;
}

// scan_dollar_rest scans the body of a dollar quote after its opening
// $tag$, up to the closing $tag$ or the end of the input.
static bool scan_dollar_rest(TSLexer *lexer, const int32_t *tag, unsigned n) {
  while (!lexer->eof(lexer)) {
    if (lexer->lookahead != '$') {
      advance(lexer);
      continue;
    }
    advance(lexer);
    unsigned i = 0;
    while (i < n && lexer->lookahead == tag[i] && !lexer->eof(lexer)) {
      advance(lexer);
      i++;
    }
    if (i == n && lexer->lookahead == '$') {
      advance(lexer);
      break;
    }
  }
  lexer->mark_end(lexer);
  lexer->result_symbol = DOLLAR_STRING;
  return true;
}

static bool scan_text(Scanner *s, TSLexer *lexer, int options, enum Context ctx, bool content,
                      int32_t prev);

// scan_variable_start scans from the colon of a variable. It returns the
// sigil when a valid variable follows. Else the colon is text in the
// context ctx. After :' or :" with no valid name and closing quote, the
// quote opens a string or a quoted identifier, and the token holds the colon
// too.
static bool scan_variable_start(Scanner *s, TSLexer *lexer, int options, enum Context ctx,
                                bool line) {
  advance(lexer);
  int32_t c = lexer->lookahead;
  if (c == ':') {
    // :: is a cast
    advance(lexer);
    lexer->mark_end(lexer);
    return scan_text(s, lexer, options, ctx, true, ':');
  }
  if (is_variable_char(c)) {
    lexer->mark_end(lexer);
    lexer->result_symbol = SIGIL_PLAIN;
    return true;
  }
  if (c == '\'' || c == '"') {
    advance(lexer);
    lexer->mark_end(lexer);
    unsigned n = 0;
    while (is_variable_char(lexer->lookahead) && !lexer->eof(lexer)) {
      advance(lexer);
      n++;
    }
    if (n > 0 && lexer->lookahead == c) {
      lexer->result_symbol = c == '\'' ? SIGIL_SINGLE_QUOTE : SIGIL_DOUBLE_QUOTE;
      return true;
    }
    scan_quoted_rest(lexer, c, line);
    lexer->result_symbol = c == '\'' ? STRING : QUOTED_IDENTIFIER;
    return true;
  }
  if (c == '{') {
    advance(lexer);
    int32_t last = '{';
    if (lexer->lookahead == '?') {
      advance(lexer);
      last = '?';
      lexer->mark_end(lexer);
      unsigned n = 0;
      while (is_variable_char(lexer->lookahead) && !lexer->eof(lexer)) {
        last = lexer->lookahead;
        advance(lexer);
        n++;
      }
      if (n > 0 && lexer->lookahead == '}') {
        lexer->result_symbol = SIGIL_BRACE;
        return true;
      }
    }
    // the characters are text
    lexer->mark_end(lexer);
    return scan_text(s, lexer, options, ctx, true, last);
  }
  lexer->mark_end(lexer);
  return scan_text(s, lexer, options, ctx, true, ':');
}

// scan_text scans a run of plain text in the context ctx: SQL text, a word
// of an argument, or a word of a list of options. content is true when the
// token already holds text, and prev is the last character of that text.
// The token ends at its last character that is not white space.
static bool scan_text(Scanner *s, TSLexer *lexer, int options, enum Context ctx, bool content,
                      int32_t prev) {
  for (;;) {
    if (lexer->eof(lexer)) {
      break;
    }
    int32_t c = lexer->lookahead;
    if (ctx != CONTEXT_SQL) {
      // an argument ends at white space, and a word at a quote
      if (is_space(c) || c == '\\' || c == '\'' || c == '"' || c == '`') {
        break;
      }
      if (ctx == CONTEXT_OPTION && (c == '=' || c == ')')) {
        break;
      }
    } else {
      if ((c == ';' && s->depth == 0) || c == '\'' || c == '"' ||
          (c == '`' && (options & USQL_BACKTICKS)) || (c == '#' && (options & USQL_HASH_COMMENTS))) {
        if (!content && c == '#') {
          return scan_line_comment(lexer);
        }
        break;
      }
      if (c == '\\') {
        // \; and \: put the character in the statement, as psql does
        advance(lexer);
        if (lexer->lookahead != ';' && lexer->lookahead != ':') {
          break;
        }
        prev = lexer->lookahead;
        advance(lexer);
        lexer->mark_end(lexer);
        content = true;
        continue;
      }
      if (c == '-' || c == '/') {
        advance(lexer);
        int32_t d = lexer->lookahead;
        bool line = (c == '-' && d == '-') || (c == '/' && d == '/' && (options & USQL_SLASH_COMMENTS));
        bool block = c == '/' && d == '*' && (options & USQL_BLOCK_COMMENTS);
        if (line || block) {
          if (content) {
            break;
          }
          advance(lexer);
          return line ? scan_line_comment(lexer) : scan_block_comment(lexer);
        }
        lexer->mark_end(lexer);
        content = true;
        prev = c;
        continue;
      }
      if (c == '$' && (options & USQL_DOLLAR_QUOTES) && !is_identifier_char(prev)) {
        advance(lexer);
        int32_t tag[MAX_TAG_LENGTH];
        unsigned n = 0;
        bool valid = true;
        if (is_tag_start(lexer->lookahead)) {
          while (is_variable_char(lexer->lookahead) && !lexer->eof(lexer)) {
            if (n == MAX_TAG_LENGTH) {
              valid = false;
              break;
            }
            tag[n++] = lexer->lookahead;
            advance(lexer);
          }
        }
        if (valid && lexer->lookahead == '$') {
          if (content) {
            break;
          }
          advance(lexer);
          return scan_dollar_rest(lexer, tag, n);
        }
        // not a dollar quote, so the characters are text
        lexer->mark_end(lexer);
        content = true;
        prev = n > 0 ? tag[n - 1] : '$';
        continue;
      }
    }
    if (c == ':') {
      if (!content) {
        return scan_variable_start(s, lexer, options, ctx, ctx != CONTEXT_SQL);
      }
      advance(lexer);
      int32_t d = lexer->lookahead;
      if (d == ':') {
        advance(lexer);
        lexer->mark_end(lexer);
        prev = ':';
        continue;
      }
      if (is_variable_char(d) || d == '\'' || d == '"' || d == '{') {
        break;
      }
      lexer->mark_end(lexer);
      prev = ':';
      continue;
    }
    if (ctx == CONTEXT_SQL) {
      if (c == '(') {
        s->depth++;
      } else if (c == ')' && s->depth > 0) {
        s->depth--;
      }
    }
    advance(lexer);
    if (!is_space(c)) {
      lexer->mark_end(lexer);
      content = true;
    }
    prev = c;
  }
  if (!content) {
    return false;
  }
  switch (ctx) {
  case CONTEXT_SQL:
    lexer->result_symbol = SQL_TEXT;
    break;
  case CONTEXT_ARGUMENT:
    lexer->result_symbol = WORD;
    break;
  case CONTEXT_OPTION:
    lexer->result_symbol = OPTION_WORD;
    break;
  }
  return true;
}

// scan_command_name scans the name of a meta command after its backslash.
// For \\ and \!, the token holds the whole name. Else the token is the
// backslash, and the scanner keeps the length of the base name for
// COMMAND_BASE.
static bool scan_command_name(Scanner *s, TSLexer *lexer, const bool *valid_symbols) {
  lexer->mark_end(lexer);
  s->depth = 0;
  if (lexer->lookahead == '\\') {
    advance(lexer);
    lexer->mark_end(lexer);
    lexer->result_symbol = SEPARATOR_COMMAND_NAME;
    return valid_symbols[SEPARATOR_COMMAND_NAME];
  }
  char name[MAX_NAME_LENGTH];
  unsigned n = 0;
  bool ascii = true;
  while (!is_name_end(lexer)) {
    if (n < MAX_NAME_LENGTH && lexer->lookahead < 0x80) {
      name[n] = (char)lexer->lookahead;
    } else {
      ascii = false;
    }
    n++;
    advance(lexer);
  }
  s->command = 1;
  if (ascii && n == 1 && name[0] == '!') {
    lexer->mark_end(lexer);
    lexer->result_symbol = SHELL_COMMAND_NAME;
    return valid_symbols[SHELL_COMMAND_NAME];
  }
  unsigned base = 0;
  const Command *cmd = NULL;
  if (ascii && n > 0) {
    cmd = split_name(name, n, &base);
  }
  s->base = cmd == NULL ? 0 : (uint8_t)base;
  lexer->result_symbol = cmd != NULL && cmd->options ? BACKSLASH_OPTIONS : BACKSLASH;
  return valid_symbols[lexer->result_symbol];
}

// scan_command_base scans the base name of a meta command after its
// backslash.
static bool scan_command_base(Scanner *s, TSLexer *lexer) {
  for (unsigned i = 0; (s->base == 0 || i < s->base) && !is_name_end(lexer); i++) {
    advance(lexer);
  }
  s->base = 0;
  lexer->mark_end(lexer);
  lexer->result_symbol = COMMAND_BASE;
  return true;
}

// scan_shell_command scans a shell command to the end of the line or to the
// next backslash outside quotes.
static bool scan_shell_command(TSLexer *lexer) {
  int32_t quote = 0;
  for (;;) {
    int32_t c = lexer->lookahead;
    if (lexer->eof(lexer) || is_newline(c) || (quote == 0 && c == '\\')) {
      break;
    }
    if (quote == 0 && (c == '\'' || c == '"')) {
      quote = c;
    } else if (c == quote) {
      quote = 0;
    }
    advance(lexer);
    if (!is_space(c)) {
      lexer->mark_end(lexer);
    }
  }
  lexer->result_symbol = SHELL_COMMAND;
  return true;
}

// scan_arguments scans a token in the arguments of a meta command, or in a
// list of options.
static bool scan_arguments(Scanner *s, TSLexer *lexer, const bool *valid_symbols, int options) {
  while (!lexer->eof(lexer) && is_space(lexer->lookahead) && !is_newline(lexer->lookahead)) {
    skip(lexer);
  }
  int32_t c = lexer->lookahead;
  if (lexer->eof(lexer) || is_newline(c) || c == '\\') {
    if (!valid_symbols[COMMAND_END]) {
      return false;
    }
    lexer->mark_end(lexer);
    s->command = 0;
    lexer->result_symbol = COMMAND_END;
    return true;
  }
  if (valid_symbols[OPTION_CLOSE] && c == ')') {
    advance(lexer);
    lexer->mark_end(lexer);
    lexer->result_symbol = OPTION_CLOSE;
    return true;
  }
  if (valid_symbols[OPTION_EQUALS] && c == '=') {
    advance(lexer);
    lexer->mark_end(lexer);
    lexer->result_symbol = OPTION_EQUALS;
    return true;
  }
  if (valid_symbols[SHELL_COMMAND]) {
    return scan_shell_command(lexer);
  }
  if (valid_symbols[PIPE] && c == '|') {
    advance(lexer);
    lexer->mark_end(lexer);
    lexer->result_symbol = PIPE;
    return true;
  }
  if (valid_symbols[OPTION_OPEN] && c == '(') {
    advance(lexer);
    lexer->mark_end(lexer);
    lexer->result_symbol = OPTION_OPEN;
    return true;
  }
  if (valid_symbols[BACKTICK_OPEN] && c == '`') {
    advance(lexer);
    lexer->mark_end(lexer);
    lexer->result_symbol = BACKTICK_OPEN;
    return true;
  }
  if (valid_symbols[STRING] && c == '\'') {
    return scan_quoted(lexer, STRING, true);
  }
  if (valid_symbols[QUOTED_IDENTIFIER] && c == '"') {
    return scan_quoted(lexer, QUOTED_IDENTIFIER, true);
  }
  enum Context ctx = valid_symbols[OPTION_WORD] ? CONTEXT_OPTION : CONTEXT_ARGUMENT;
  if (ctx == CONTEXT_ARGUMENT && !valid_symbols[WORD]) {
    return false;
  }
  return scan_text(s, lexer, options, ctx, false, 0);
}

// scan_sql scans a token of SQL, or the name of a meta command.
static bool scan_sql(Scanner *s, TSLexer *lexer, const bool *valid_symbols, int options) {
  while (!lexer->eof(lexer) && is_space(lexer->lookahead)) {
    skip(lexer);
  }
  if (lexer->eof(lexer)) {
    return false;
  }
  int32_t c = lexer->lookahead;
  if (c == '\\') {
    advance(lexer);
    if (lexer->lookahead == ';' || lexer->lookahead == ':') {
      // \; and \: put the character in the statement, as psql does
      int32_t prev = lexer->lookahead;
      advance(lexer);
      lexer->mark_end(lexer);
      return scan_text(s, lexer, options, CONTEXT_SQL, true, prev);
    }
    return scan_command_name(s, lexer, valid_symbols);
  }
  if (c == ';' && s->depth == 0) {
    advance(lexer);
    lexer->mark_end(lexer);
    lexer->result_symbol = SEMICOLON;
    return true;
  }
  if (c == '\'') {
    return scan_quoted(lexer, STRING, false);
  }
  if (c == '"' || (c == '`' && (options & USQL_BACKTICKS))) {
    return scan_quoted(lexer, QUOTED_IDENTIFIER, false);
  }
  return scan_text(s, lexer, options, CONTEXT_SQL, false, 0);
}

void *tree_sitter_usql_external_scanner_create(void) { return ts_calloc(1, sizeof(Scanner)); }

void tree_sitter_usql_external_scanner_destroy(void *payload) { ts_free(payload); }

// tree_sitter_usql_external_scanner_serialize writes the depth in its first
// 4 bytes, then base and then command.
unsigned tree_sitter_usql_external_scanner_serialize(void *payload, char *buffer) {
  Scanner *s = (Scanner *)payload;
  memcpy(buffer, &s->depth, sizeof(s->depth));
  buffer[4] = (char)s->base;
  buffer[5] = (char)s->command;
  return SERIALIZED_SIZE;
}

// tree_sitter_usql_external_scanner_deserialize reads the bytes that
// tree_sitter_usql_external_scanner_serialize writes, or resets the scanner
// when there are none.
void tree_sitter_usql_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
  Scanner *s = (Scanner *)payload;
  s->depth = 0;
  s->base = 0;
  s->command = 0;
  if (length == SERIALIZED_SIZE) {
    memcpy(&s->depth, buffer, sizeof(s->depth));
    s->base = (uint8_t)buffer[4];
    s->command = (uint8_t)buffer[5];
  }
}

// tree_sitter_usql_external_scanner_scan scans one token with the options
// USQL_OPTIONS.
bool tree_sitter_usql_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
  Scanner *s = (Scanner *)payload;
  int options = USQL_OPTIONS;
  if (valid_symbols[ERROR_SENTINEL]) {
    // In the recovery from an error every token is valid, so the text is
    // read as SQL.
    return scan_sql(s, lexer, valid_symbols, options);
  }
  if (valid_symbols[VARIABLE_NAME]) {
    unsigned n = 0;
    while (is_variable_char(lexer->lookahead) && !lexer->eof(lexer)) {
      advance(lexer);
      n++;
    }
    lexer->mark_end(lexer);
    lexer->result_symbol = VARIABLE_NAME;
    return n > 0;
  }
  if (valid_symbols[CLOSE_SINGLE_QUOTE] || valid_symbols[CLOSE_DOUBLE_QUOTE] ||
      valid_symbols[CLOSE_BRACE]) {
    int32_t c = lexer->lookahead;
    if (valid_symbols[CLOSE_SINGLE_QUOTE] && c == '\'') {
      lexer->result_symbol = CLOSE_SINGLE_QUOTE;
    } else if (valid_symbols[CLOSE_DOUBLE_QUOTE] && c == '"') {
      lexer->result_symbol = CLOSE_DOUBLE_QUOTE;
    } else if (valid_symbols[CLOSE_BRACE] && c == '}') {
      lexer->result_symbol = CLOSE_BRACE;
    } else {
      return false;
    }
    advance(lexer);
    lexer->mark_end(lexer);
    return true;
  }
  if (valid_symbols[COMMAND_BASE] && !is_name_end(lexer)) {
    return scan_command_base(s, lexer);
  }
  if (valid_symbols[MODIFIER] && !is_name_end(lexer)) {
    advance(lexer);
    lexer->mark_end(lexer);
    lexer->result_symbol = MODIFIER;
    return true;
  }
  if (valid_symbols[BACKTICK_TEXT] || valid_symbols[BACKTICK_CLOSE]) {
    if (valid_symbols[BACKTICK_CLOSE] && lexer->lookahead == '`') {
      advance(lexer);
      lexer->mark_end(lexer);
      lexer->result_symbol = BACKTICK_CLOSE;
      return true;
    }
    if (!valid_symbols[BACKTICK_TEXT]) {
      return false;
    }
    unsigned n = 0;
    while (!lexer->eof(lexer) && lexer->lookahead != '`' && !is_newline(lexer->lookahead)) {
      advance(lexer);
      n++;
    }
    lexer->mark_end(lexer);
    lexer->result_symbol = BACKTICK_TEXT;
    return n > 0;
  }
  if (valid_symbols[COMMAND_END] || valid_symbols[OPTION_CLOSE] || valid_symbols[OPTION_WORD] ||
      valid_symbols[OPTION_EQUALS]) {
    return scan_arguments(s, lexer, valid_symbols, options);
  }
  return scan_sql(s, lexer, valid_symbols, options);
}
