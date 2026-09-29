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
const char *rt_load(void *handle);

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
