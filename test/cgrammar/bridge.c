#include <dlfcn.h>
#include <stdarg.h>
#include <stdio.h>
#include "bridge.h"
#include "_cgo_export.h"

static void bridge_advance(TSLexer *self, bool skip) {
  cgrammarAdvance((BridgeLexer *)self, skip);
}

static void bridge_mark_end(TSLexer *self) {
  cgrammarMarkEnd((BridgeLexer *)self);
}

static uint32_t bridge_get_column(TSLexer *self) {
  return cgrammarGetColumn((BridgeLexer *)self);
}

static bool bridge_is_at_included_range_start(const TSLexer *self) {
  return cgrammarIsAtIncludedRangeStart((BridgeLexer *)self);
}

static bool bridge_eof(const TSLexer *self) {
  return cgrammarEOF((BridgeLexer *)self);
}

// bridge_log formats the message as the C runtime does, into a buffer of
// TREE_SITTER_SERIALIZATION_BUFFER_SIZE bytes, and gives it to Go.
static void bridge_log(const TSLexer *self, const char *format, ...) {
  char buffer[TREE_SITTER_SERIALIZATION_BUFFER_SIZE];
  va_list args;
  va_start(args, format);
  vsnprintf(buffer, sizeof(buffer), format, args);
  va_end(args);
  cgrammarLog((BridgeLexer *)self, buffer);
}

void bridge_lexer_init(BridgeLexer *self, uintptr_t handle) {
  self->lexer.lookahead = 0;
  self->lexer.result_symbol = 0;
  self->lexer.advance = bridge_advance;
  self->lexer.mark_end = bridge_mark_end;
  self->lexer.get_column = bridge_get_column;
  self->lexer.is_at_included_range_start = bridge_is_at_included_range_start;
  self->lexer.eof = bridge_eof;
  self->lexer.log = bridge_log;
  self->handle = handle;
}

bool bridge_call_lex(bool (*fn)(TSLexer *, TSStateId), BridgeLexer *lexer, TSStateId state) {
  return fn(&lexer->lexer, state);
}

void *bridge_call_create(void *(*fn)(void)) {
  return fn();
}

void bridge_call_destroy(void (*fn)(void *), void *payload) {
  fn(payload);
}

bool bridge_call_scan(bool (*fn)(void *, TSLexer *, const bool *), void *payload, BridgeLexer *lexer, const bool *valid) {
  return fn(payload, &lexer->lexer, valid);
}

unsigned bridge_call_serialize(unsigned (*fn)(void *, char *), void *payload, char *buffer) {
  return fn(payload, buffer);
}

void bridge_call_deserialize(void (*fn)(void *, const char *, unsigned), void *payload, const char *buffer, unsigned length) {
  fn(payload, buffer, length);
}

void *bridge_dlopen(const char *path) {
  return dlopen(path, RTLD_NOW | RTLD_LOCAL);
}

const char *bridge_dlerror(void) {
  return dlerror();
}

void *bridge_dlsym(void *handle, const char *name) {
  return dlsym(handle, name);
}

const TSLanguage *bridge_call_language(void *fn) {
  return ((const TSLanguage *(*)(void))fn)();
}

// The functions of the C runtime. The runtime is a library of its own, so
// the package declares their types here, with CNode for TSNode.
static struct {
  void *(*parser_new)(void);
  void (*parser_delete)(void *);
  bool (*parser_set_language)(void *, const TSLanguage *);
  void *(*parser_parse_string)(void *, const void *, const char *, uint32_t);
  void (*tree_delete)(void *);
  void (*tree_edit)(void *, const CInputEdit *);
  CRange *(*tree_get_changed_ranges)(const void *, const void *, uint32_t *);
  CNode (*tree_root_node)(const void *);
  char *(*node_string)(CNode);
  uint32_t (*node_child_count)(CNode);
  CNode (*node_child)(CNode, uint32_t);
  uint16_t (*node_symbol)(CNode);
  uint16_t (*node_grammar_symbol)(CNode);
  uint32_t (*node_start_byte)(CNode);
  uint32_t (*node_end_byte)(CNode);
  CPoint (*node_start_point)(CNode);
  CPoint (*node_end_point)(CNode);
  bool (*node_is_named)(CNode);
  bool (*node_is_missing)(CNode);
  bool (*node_is_extra)(CNode);
  bool (*node_has_error)(CNode);
  bool (*node_has_changes)(CNode);
  uint16_t (*node_parse_state)(CNode);
  uint16_t (*node_next_parse_state)(CNode);
  const char *(*node_field_name_for_child)(CNode, uint32_t);
  uint32_t (*node_descendant_count)(CNode);
} rt;

#define RT_LOAD(member, name) \
  if (!(*(void **)&rt.member = dlsym(handle, name))) return name;

const char *rt_load(void *handle) {
  RT_LOAD(parser_new, "ts_parser_new")
  RT_LOAD(parser_delete, "ts_parser_delete")
  RT_LOAD(parser_set_language, "ts_parser_set_language")
  RT_LOAD(parser_parse_string, "ts_parser_parse_string")
  RT_LOAD(tree_delete, "ts_tree_delete")
  RT_LOAD(tree_edit, "ts_tree_edit")
  RT_LOAD(tree_get_changed_ranges, "ts_tree_get_changed_ranges")
  RT_LOAD(tree_root_node, "ts_tree_root_node")
  RT_LOAD(node_string, "ts_node_string")
  RT_LOAD(node_child_count, "ts_node_child_count")
  RT_LOAD(node_child, "ts_node_child")
  RT_LOAD(node_symbol, "ts_node_symbol")
  RT_LOAD(node_grammar_symbol, "ts_node_grammar_symbol")
  RT_LOAD(node_start_byte, "ts_node_start_byte")
  RT_LOAD(node_end_byte, "ts_node_end_byte")
  RT_LOAD(node_start_point, "ts_node_start_point")
  RT_LOAD(node_end_point, "ts_node_end_point")
  RT_LOAD(node_is_named, "ts_node_is_named")
  RT_LOAD(node_is_missing, "ts_node_is_missing")
  RT_LOAD(node_is_extra, "ts_node_is_extra")
  RT_LOAD(node_has_error, "ts_node_has_error")
  RT_LOAD(node_has_changes, "ts_node_has_changes")
  RT_LOAD(node_parse_state, "ts_node_parse_state")
  RT_LOAD(node_next_parse_state, "ts_node_next_parse_state")
  RT_LOAD(node_field_name_for_child, "ts_node_field_name_for_child")
  RT_LOAD(node_descendant_count, "ts_node_descendant_count")
  return NULL;
}

void *rt_parser_new(void) { return rt.parser_new(); }
void rt_parser_delete(void *parser) { rt.parser_delete(parser); }
bool rt_parser_set_language(void *parser, const TSLanguage *language) { return rt.parser_set_language(parser, language); }
void *rt_parser_parse_string(void *parser, const void *old_tree, const char *string, uint32_t length) { return rt.parser_parse_string(parser, old_tree, string, length); }
void rt_tree_delete(void *tree) { rt.tree_delete(tree); }
void rt_tree_edit(void *tree, const CInputEdit *edit) { rt.tree_edit(tree, edit); }
CRange *rt_tree_get_changed_ranges(const void *old_tree, const void *new_tree, uint32_t *length) { return rt.tree_get_changed_ranges(old_tree, new_tree, length); }
CNode rt_tree_root_node(const void *tree) { return rt.tree_root_node(tree); }
char *rt_node_string(CNode node) { return rt.node_string(node); }
uint32_t rt_node_child_count(CNode node) { return rt.node_child_count(node); }
CNode rt_node_child(CNode node, uint32_t index) { return rt.node_child(node, index); }
uint16_t rt_node_symbol(CNode node) { return rt.node_symbol(node); }
uint16_t rt_node_grammar_symbol(CNode node) { return rt.node_grammar_symbol(node); }
uint32_t rt_node_start_byte(CNode node) { return rt.node_start_byte(node); }
uint32_t rt_node_end_byte(CNode node) { return rt.node_end_byte(node); }
CPoint rt_node_start_point(CNode node) { return rt.node_start_point(node); }
CPoint rt_node_end_point(CNode node) { return rt.node_end_point(node); }
bool rt_node_is_named(CNode node) { return rt.node_is_named(node); }
bool rt_node_is_missing(CNode node) { return rt.node_is_missing(node); }
bool rt_node_is_extra(CNode node) { return rt.node_is_extra(node); }
bool rt_node_has_error(CNode node) { return rt.node_has_error(node); }
bool rt_node_has_changes(CNode node) { return rt.node_has_changes(node); }
uint16_t rt_node_parse_state(CNode node) { return rt.node_parse_state(node); }
uint16_t rt_node_next_parse_state(CNode node) { return rt.node_next_parse_state(node); }
const char *rt_node_field_name_for_child(CNode node, uint32_t index) { return rt.node_field_name_for_child(node, index); }
uint32_t rt_node_descendant_count(CNode node) { return rt.node_descendant_count(node); }
