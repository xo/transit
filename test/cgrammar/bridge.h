// The C side of the package cgrammar: a TSLexer that calls the lexer of the
// Go runtime, calls of the C function pointers of a TSLanguage, and the
// functions of the upstream C runtime, which the package loads with dlopen.
#ifndef CGRAMMAR_BRIDGE_H_
#define CGRAMMAR_BRIDGE_H_

#include <stdbool.h>
#include <stdint.h>
#include "parser.h"

// BridgeLexer is a TSLexer whose functions call the Go lexer that handle
// names.
typedef struct {
  TSLexer lexer;
  uintptr_t handle;
} BridgeLexer;

void bridge_lexer_init(BridgeLexer *self, uintptr_t handle);

bool bridge_call_lex(bool (*fn)(TSLexer *, TSStateId), BridgeLexer *lexer, TSStateId state);

// bridge_lex_states runs the lex function fn in each state below states,
// from the offset pos of a text of length bytes, with a lexer in C that
// does not call Go. lookahead and size hold the character and its size in
// bytes at each offset of the text, and 0 and 0 at the end. For each state,
// it writes to out 1 or 0 for whether fn found a token, the result symbol,
// the number of the calls of the lexer and each call: 1 for advance, 2 for
// advance with skip, and -1-p for mark_end at the offset p. It returns the
// number of values that it wrote, or -1 when out holds fewer than cap
// values.
int64_t bridge_lex_states(bool (*fn)(TSLexer *, TSStateId), uint32_t states, const int32_t *lookahead, const uint32_t *size, uint32_t length, uint32_t pos, int32_t *out, uint32_t cap);
void *bridge_call_create(void *(*fn)(void));
void bridge_call_destroy(void (*fn)(void *), void *payload);
bool bridge_call_scan(bool (*fn)(void *, TSLexer *, const bool *), void *payload, BridgeLexer *lexer, const bool *valid);
unsigned bridge_call_serialize(unsigned (*fn)(void *, char *), void *payload, char *buffer);
void bridge_call_deserialize(void (*fn)(void *, const char *, unsigned), void *payload, const char *buffer, unsigned length);

void *bridge_dlopen(const char *path);
const char *bridge_dlerror(void);
void *bridge_dlsym(void *handle, const char *name);
const TSLanguage *bridge_call_language(void *fn);

// CNode and CPoint have the layout of TSNode and TSPoint of api.h.
typedef struct {
  uint32_t context[4];
  const void *id;
  const void *tree;
} CNode;

typedef struct {
  uint32_t row;
  uint32_t column;
} CPoint;

typedef struct {
  uint32_t start_byte;
  uint32_t old_end_byte;
  uint32_t new_end_byte;
  CPoint start_point;
  CPoint old_end_point;
  CPoint new_end_point;
} CInputEdit;

typedef struct {
  CPoint start_point;
  CPoint end_point;
  uint32_t start_byte;
  uint32_t end_byte;
} CRange;

// rt_load finds the functions of the C runtime in a library that dlopen
// opened. It returns the name of a function that it cannot find, or NULL.
// rt_o2_load does the same for the runtime of the benchmarks, which is built
// with -O2.
const char *rt_load(void *handle);
const char *rt_o2_load(void *handle);

// The functions of a SpeedSession. Each calls the runtime of the benchmarks
// when o2 is true, and the runtime of the tests otherwise.
void *sp_parser_new(bool o2);
void sp_parser_delete(bool o2, void *parser);
bool sp_parser_set_language(bool o2, void *parser, const TSLanguage *language);
void *sp_parser_parse_string(bool o2, void *parser, const void *old_tree, const char *string, uint32_t length);
void sp_tree_delete(bool o2, void *tree);
void sp_tree_edit(bool o2, void *tree, const CInputEdit *edit);

void *rt_parser_new(void);
void rt_parser_delete(void *parser);
bool rt_parser_set_language(void *parser, const TSLanguage *language);
void *rt_parser_parse_string(void *parser, const void *old_tree, const char *string, uint32_t length);
void rt_tree_delete(void *tree);
void rt_tree_edit(void *tree, const CInputEdit *edit);
CRange *rt_tree_get_changed_ranges(const void *old_tree, const void *new_tree, uint32_t *length);
CNode rt_tree_root_node(const void *tree);
char *rt_node_string(CNode node);
uint32_t rt_node_child_count(CNode node);
CNode rt_node_child(CNode node, uint32_t index);
uint16_t rt_node_symbol(CNode node);
uint16_t rt_node_grammar_symbol(CNode node);
uint32_t rt_node_start_byte(CNode node);
uint32_t rt_node_end_byte(CNode node);
CPoint rt_node_start_point(CNode node);
CPoint rt_node_end_point(CNode node);
bool rt_node_is_named(CNode node);
bool rt_node_is_missing(CNode node);
bool rt_node_is_extra(CNode node);
bool rt_node_has_error(CNode node);
bool rt_node_has_changes(CNode node);
uint16_t rt_node_parse_state(CNode node);
uint16_t rt_node_next_parse_state(CNode node);
const char *rt_node_field_name_for_child(CNode node, uint32_t index);
uint32_t rt_node_descendant_count(CNode node);

#endif  // CGRAMMAR_BRIDGE_H_
