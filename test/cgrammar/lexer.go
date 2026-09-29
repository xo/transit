package cgrammar

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"runtime"
	"runtime/cgo"
	"unsafe"

	"github.com/xo/transit/internal/abi"
)

// This file gives the Go runtime the lex functions and the external scanner
// of a C grammar. A C lex function reads a BridgeLexer, a TSLexer whose
// functions call the functions below, which call the lexer of the Go
// runtime. Before each call into C, the Go lookahead and result symbol go
// into the TSLexer, and after it the result symbol comes back.

// lexerOf returns the Go lexer of a BridgeLexer.
func lexerOf(b *C.BridgeLexer) *abi.Lexer {
	lexer, ok := cgo.Handle(b.handle).Value().(*abi.Lexer)
	if !ok {
		panic("cgrammar: the handle of a BridgeLexer is not a lexer")
	}
	return lexer
}

//export cgrammarAdvance
func cgrammarAdvance(b *C.BridgeLexer, skip C.bool) {
	lexer := lexerOf(b)
	lexer.Advance(bool(skip))
	b.lexer.lookahead = C.int32_t(lexer.Lookahead)
}

//export cgrammarMarkEnd
func cgrammarMarkEnd(b *C.BridgeLexer) {
	lexerOf(b).MarkEnd()
}

//export cgrammarGetColumn
func cgrammarGetColumn(b *C.BridgeLexer) C.uint32_t {
	lexer := lexerOf(b)
	column := lexer.GetColumn()
	b.lexer.lookahead = C.int32_t(lexer.Lookahead)
	return C.uint32_t(column)
}

//export cgrammarIsAtIncludedRangeStart
func cgrammarIsAtIncludedRangeStart(b *C.BridgeLexer) C.bool {
	return C.bool(lexerOf(b).IsAtIncludedRangeStart())
}

//export cgrammarEOF
func cgrammarEOF(b *C.BridgeLexer) C.bool {
	return C.bool(lexerOf(b).EOF())
}

//export cgrammarLog
func cgrammarLog(b *C.BridgeLexer, message *C.char) {
	lexerOf(b).Logf("%s", C.GoString(message))
}

// withBridge calls fn with a BridgeLexer for lexer, in C memory, and copies
// the result symbol back.
func withBridge(lexer *abi.Lexer, fn func(b *C.BridgeLexer) bool) bool {
	h := cgo.NewHandle(lexer)
	defer h.Delete()
	b := (*C.BridgeLexer)(C.malloc(C.size_t(unsafe.Sizeof(C.BridgeLexer{}))))
	defer C.free(unsafe.Pointer(b))
	C.bridge_lexer_init(b, C.uintptr_t(h))
	b.lexer.lookahead = C.int32_t(lexer.Lookahead)
	b.lexer.result_symbol = C.TSSymbol(lexer.ResultSymbol)
	found := fn(b)
	lexer.ResultSymbol = uint16(b.lexer.result_symbol)
	return found
}

// lexFunc returns the Go form of a C lex function, or nil for NULL.
func lexFunc(fn *[0]byte) abi.LexFunc {
	if fn == nil {
		return nil
	}
	return func(lexer *abi.Lexer, state uint16) bool {
		return withBridge(lexer, func(b *C.BridgeLexer) bool {
			return bool(C.bridge_call_lex(fn, b, C.TSStateId(state)))
		})
	}
}

// scanner is an external scanner of C, the payload that its create function
// returns.
type scanner struct {
	lang    *C.TSLanguage
	payload unsafe.Pointer
}

// scannerCreate returns the Go form of the create function of the external
// scanner of a language. The garbage collector calls the destroy function of
// C when it frees the scanner.
func scannerCreate(l *C.TSLanguage) func() abi.Scanner {
	return func() abi.Scanner {
		s := &scanner{lang: l, payload: C.bridge_call_create(l.external_scanner.create)}
		destroy := l.external_scanner.destroy
		if destroy != nil {
			runtime.AddCleanup(s, func(payload unsafe.Pointer) {
				C.bridge_call_destroy(destroy, payload)
			}, s.payload)
		}
		return s
	}
}

// Scan calls scan of the C scanner.
func (s *scanner) Scan(lexer *abi.Lexer, validSymbols []bool) bool {
	var valid *C.bool
	if len(validSymbols) > 0 {
		valid = (*C.bool)(unsafe.Pointer(&validSymbols[0]))
	}
	return withBridge(lexer, func(b *C.BridgeLexer) bool {
		return bool(C.bridge_call_scan(s.lang.external_scanner.scan, s.payload, b, valid))
	})
}

// Serialize calls serialize of the C scanner.
func (s *scanner) Serialize(buf []byte) int {
	return int(C.bridge_call_serialize(s.lang.external_scanner.serialize, s.payload, (*C.char)(unsafe.Pointer(&buf[0]))))
}

// Deserialize calls deserialize of the C scanner.
func (s *scanner) Deserialize(buf []byte) {
	var data *C.char
	if len(buf) > 0 {
		data = (*C.char)(unsafe.Pointer(&buf[0]))
	}
	C.bridge_call_deserialize(s.lang.external_scanner.deserialize, s.payload, data, C.unsigned(len(buf)))
}
