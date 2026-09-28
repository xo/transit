#include <stdio.h>
#include <string.h>
#include <tree_sitter/api.h>

// Declare the SQL language function from the grammar
const TSLanguage *tree_sitter_sql(void);

// Simple highlight group mapping: capture name -> group id
typedef struct {
  const char *capture_name;
  int group_id;
} HighlightMap;

int main() {
  // 1. Set up parser and language
  TSParser *parser = ts_parser_new();
  ts_parser_set_language(parser, tree_sitter_sql());

  // 2. SQL source to highlight
  const char *sql = "SELECT id, name FROM users WHERE age > 21;";

  // 3. Parse the SQL
  TSTree *tree = ts_parser_parse_string(parser, NULL, sql, strlen(sql));
  TSNode root = ts_tree_root_node(tree);

  // 4. Create a highlighting query from a .scm string.
  //    In practice, you'd load this from the grammar's highlights.scm file.
  const char *query_source = "(keyword) @keyword\n"
                             "(string) @string\n"
                             "(number) @number\n"
                             "(identifier) @variable\n"
                             "(function_call name: (identifier) @function)\n";

  uint32_t error_offset;
  TSQueryError error_type;
  TSQuery *query =
      ts_query_new(tree_sitter_sql(), query_source, strlen(query_source),
                   &error_offset, &error_type);

  if (error_type != TSQueryErrorNone) {
    fprintf(stderr, "Query error at %u\n", error_offset);
    return 1;
  }

  // 5. Execute the query against the root node to get captures
  TSQueryCursor *cursor = ts_query_cursor_new();
  ts_query_cursor_exec(cursor, query, root);

  TSQueryMatch match;
  const char *source = sql;

  printf("Highlighting spans for: %s\n\n", sql);

  while (ts_query_cursor_next_match(cursor, &match)) {
    for (uint32_t i = 0; i < match.capture_count; i++) {
      TSQueryCapture capture = match.captures[i];

      // Get the capture name (e.g., "keyword", "string")
      uint32_t name_len;
      const char *capture_name =
          ts_query_capture_name_for_id(query, capture.index, &name_len);

      // Get the byte range of the captured node
      uint32_t start = ts_node_start_byte(capture.node);
      uint32_t end = ts_node_end_byte(capture.node);

      // Extract the text
      int len = end - start;
      char *text = malloc(len + 1);
      memcpy(text, source + start, len);
      text[len] = '\0';

      printf("  @%s: \"%s\" [bytes %u-%u]\n", capture_name, text, start, end);

      free(text);
    }
  }

  // 6. Cleanup
  ts_query_cursor_delete(cursor);
  ts_query_delete(query);
  ts_tree_delete(tree);
  ts_parser_delete(parser);

  return 0;
}
