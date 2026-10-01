// The external scanner of the usql grammar of the family standard: block comments only, for SQL Server, Oracle, ClickHouse, Trino, DuckDB and the others.
// common/scanner.h holds the scanner, and USQL_FAMILY gives the family.

#include "../../common/scanner.h"

#define USQL_FAMILY (USQL_BLOCK)

void *tree_sitter_usql_standard_external_scanner_create(void) { return usql_create(); }

void tree_sitter_usql_standard_external_scanner_destroy(void *payload) { usql_destroy(payload); }

unsigned tree_sitter_usql_standard_external_scanner_serialize(void *payload, char *buffer) {
  return usql_serialize(payload, buffer);
}

void tree_sitter_usql_standard_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
  usql_deserialize(payload, buffer, length);
}

bool tree_sitter_usql_standard_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
  return usql_scan(payload, lexer, valid_symbols, USQL_FAMILY);
}
