// The query functions of the C side of the package cgrammar.
#ifndef CGRAMMAR_QUERY_H_
#define CGRAMMAR_QUERY_H_

#include "bridge.h"

// cq_load finds the query functions of the C runtime in a library that
// dlopen opened. It returns the name of a function that it cannot find, or
// NULL.
const char *cq_load(void *handle);

// cq_new compiles a query, or it returns NULL and writes the offset and the
// type of the error.
void *cq_new(const TSLanguage *lang, const char *src, uint32_t len, uint32_t *offset, int *type);

// cq_delete frees a query of cq_new.
void cq_delete(void *query);

// cq_describe writes the start, the end, rooted and non-local of each
// pattern, and then guaranteed at each byte of a text of len bytes.
char *cq_describe(void *query, uint32_t len);

// cq_run runs a query on a node with the settings of a cursor, and writes
// each match, or each capture, and whether the cursor exceeded its limit.
char *cq_run(void *query, CNode root, int captures,
             uint32_t start, uint32_t end, uint32_t limit, uint32_t depth);

#endif  // CGRAMMAR_QUERY_H_
