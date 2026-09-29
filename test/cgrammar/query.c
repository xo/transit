// The C side of the query tests of the package cgrammar: the query functions
// of the C runtime, which the package loads with dlopen, and two functions
// that write the result of a query as text, so that the test can compare it
// with the text of the Go runtime.
#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include "bridge.h"
#include "query.h"

typedef struct TSQuery TSQuery;
typedef struct TSQueryCursor TSQueryCursor;

// CQueryCapture and CQueryMatch have the layout of TSQueryCapture and
// TSQueryMatch of api.h.
typedef struct {
  CNode node;
  uint32_t index;
} CQueryCapture;

typedef struct {
  uint32_t id;
  uint16_t pattern_index;
  uint16_t capture_count;
  const CQueryCapture *captures;
} CQueryMatch;

static struct {
  TSQuery *(*query_new)(const TSLanguage *, const char *, uint32_t, uint32_t *, int *);
  void (*query_delete)(TSQuery *);
  const char *(*capture_name)(const TSQuery *, uint32_t, uint32_t *);
  uint32_t (*pattern_count)(const TSQuery *);
  bool (*is_rooted)(const TSQuery *, uint32_t);
  bool (*is_non_local)(const TSQuery *, uint32_t);
  bool (*is_guaranteed)(const TSQuery *, uint32_t);
  uint32_t (*start_byte)(const TSQuery *, uint32_t);
  uint32_t (*end_byte)(const TSQuery *, uint32_t);
  TSQueryCursor *(*cursor_new)(void);
  void (*cursor_delete)(TSQueryCursor *);
  void (*cursor_exec)(TSQueryCursor *, const TSQuery *, CNode);
  bool (*next_match)(TSQueryCursor *, CQueryMatch *);
  bool (*next_capture)(TSQueryCursor *, CQueryMatch *, uint32_t *);
  bool (*set_byte_range)(TSQueryCursor *, uint32_t, uint32_t);
  void (*set_match_limit)(TSQueryCursor *, uint32_t);
  bool (*did_exceed)(const TSQueryCursor *);
  void (*set_max_start_depth)(TSQueryCursor *, uint32_t);
} q;

#define Q_LOAD(member, name) \
  if (!(*(void **)&q.member = dlsym(handle, name))) return name;

const char *cq_load(void *handle) {
  Q_LOAD(query_new, "ts_query_new")
  Q_LOAD(query_delete, "ts_query_delete")
  Q_LOAD(capture_name, "ts_query_capture_name_for_id")
  Q_LOAD(pattern_count, "ts_query_pattern_count")
  Q_LOAD(is_rooted, "ts_query_is_pattern_rooted")
  Q_LOAD(is_non_local, "ts_query_is_pattern_non_local")
  Q_LOAD(is_guaranteed, "ts_query_is_pattern_guaranteed_at_step")
  Q_LOAD(start_byte, "ts_query_start_byte_for_pattern")
  Q_LOAD(end_byte, "ts_query_end_byte_for_pattern")
  Q_LOAD(cursor_new, "ts_query_cursor_new")
  Q_LOAD(cursor_delete, "ts_query_cursor_delete")
  Q_LOAD(cursor_exec, "ts_query_cursor_exec")
  Q_LOAD(next_match, "ts_query_cursor_next_match")
  Q_LOAD(next_capture, "ts_query_cursor_next_capture")
  Q_LOAD(set_byte_range, "ts_query_cursor_set_byte_range")
  Q_LOAD(set_match_limit, "ts_query_cursor_set_match_limit")
  Q_LOAD(did_exceed, "ts_query_cursor_did_exceed_match_limit")
  Q_LOAD(set_max_start_depth, "ts_query_cursor_set_max_start_depth")
  return NULL;
}

// put_capture writes a capture as its name, the symbol of its node and the
// byte range of its node.
static void put_capture(FILE *f, const TSQuery *query, const CQueryCapture *c) {
  uint32_t len;
  const char *name = q.capture_name(query, c->index, &len);
  fprintf(f, " %.*s=%u@%u-%u", (int)len, name, rt_node_symbol(c->node),
          rt_node_start_byte(c->node), rt_node_end_byte(c->node));
}

void *cq_new(const TSLanguage *lang, const char *src, uint32_t len, uint32_t *offset, int *type) {
  return q.query_new(lang, src, len, offset, type);
}

void cq_delete(void *query) {
  q.query_delete(query);
}

char *cq_describe(void *query, uint32_t len) {
  char *out = NULL;
  size_t size = 0;
  FILE *f = open_memstream(&out, &size);
  uint32_t n = q.pattern_count(query);
  for (uint32_t i = 0; i < n; i++) {
    fprintf(f, "pattern %u %u %u %d %d\n", i, q.start_byte(query, i), q.end_byte(query, i),
            q.is_rooted(query, i), q.is_non_local(query, i));
  }
  for (uint32_t b = 0; b < len + 2; b++) {
    fprintf(f, "%d", q.is_guaranteed(query, b));
  }
  fprintf(f, "\n");
  fclose(f);
  return out;
}

char *cq_run(void *query, CNode root, int captures,
             uint32_t start, uint32_t end, uint32_t limit, uint32_t depth) {
  char *out = NULL;
  size_t size = 0;
  FILE *f = open_memstream(&out, &size);
  TSQueryCursor *cursor = q.cursor_new();
  q.set_byte_range(cursor, start, end);
  q.set_match_limit(cursor, limit);
  q.set_max_start_depth(cursor, depth);
  q.cursor_exec(cursor, query, root);
  CQueryMatch m;
  uint32_t index;
  if (!captures) {
    while (q.next_match(cursor, &m)) {
      fprintf(f, "match %u:", m.pattern_index);
      for (uint16_t i = 0; i < m.capture_count; i++) put_capture(f, query, &m.captures[i]);
      fprintf(f, "\n");
    }
  } else {
    while (q.next_capture(cursor, &m, &index)) {
      fprintf(f, "capture %u %u:", m.pattern_index, index);
      put_capture(f, query, &m.captures[index]);
      fprintf(f, "\n");
    }
  }
  fprintf(f, "exceeded %d\n", q.did_exceed(cursor));
  q.cursor_delete(cursor);
  fclose(f);
  return out;
}
