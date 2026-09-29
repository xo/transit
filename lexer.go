package transit

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"

	"github.com/xo/transit/internal/abi"
)

// This file ports lib/src/lexer.c, lib/src/lexer.h and lib/src/unicode.h,
// and the types of lib/include/tree_sitter/api.h that give the lexer its
// input: TSRange, TSInput, TSInputEncoding, TSLogType and TSLogger.
//
// The static functions of lexer.c with two underscores, such as
// ts_lexer__advance, are the function members of TSLexer, which a lex
// function and an external scanner call. In Go they are the methods of
// abi.LexerFuncs, so they are exported methods of the unexported type lexer.
// The other functions of lexer.c are for the parser, and they are unexported
// methods.
//
// These parts have no Go form. ts_lexer_delete frees memory.
// TSInputEncodingCustom and the member decode of TSInput are not in the API
// of docs/API.md, so the lexer decodes only the three encodings of Encoding.

// Range is a range of the text, in bytes and in points.
//
// Range is TSRange.
type Range struct {
	StartByte  int
	EndByte    int
	StartPoint Point
	EndPoint   Point
}

// textRange is TSRange inside the runtime, with the widths of C.
type textRange struct {
	startPoint point
	endPoint   point
	startByte  uint32
	endByte    uint32
}

// public returns the range as a Range of the exported API.
func (r textRange) public() Range {
	return Range{
		StartByte:  int(r.startByte),
		EndByte:    int(r.endByte),
		StartPoint: r.startPoint.public(),
		EndPoint:   r.endPoint.public(),
	}
}

// internal returns the Range as a textRange of the runtime. A value that
// does not fit in 32 bits is cut, as a conversion in C cuts it.
func (r Range) internal() textRange {
	return textRange{
		startPoint: r.StartPoint.internal(),
		endPoint:   r.EndPoint.internal(),
		startByte:  uint32(r.StartByte),
		endByte:    uint32(r.EndByte),
	}
}

// Input gives the text in chunks. ReadAt returns the text from a byte offset,
// which is at a point, and an empty slice at the end of the text. The parser
// keeps the slice only until it calls ReadAt again.
//
// Input is the member read of TSInput.
type Input interface {
	ReadAt(offset int, at Point) []byte
}

// Encoding is the encoding of the text.
//
// Encoding is TSInputEncoding.
type Encoding int

// The encodings of the text.
const (
	// EncodingUTF8 is TSInputEncodingUTF8.
	EncodingUTF8 Encoding = iota
	// EncodingUTF16LE is TSInputEncodingUTF16LE.
	EncodingUTF16LE
	// EncodingUTF16BE is TSInputEncodingUTF16BE.
	EncodingUTF16BE
)

// String returns the name of the encoding.
func (e Encoding) String() string {
	switch e {
	case EncodingUTF8:
		return "UTF-8"
	case EncodingUTF16LE:
		return "UTF-16LE"
	case EncodingUTF16BE:
		return "UTF-16BE"
	}
	return unknownName
}

// input is TSInput: the text and its encoding.
type input struct {
	read     Input
	encoding Encoding
}

// LogType says whether a log message comes from the parser or the lexer.
//
// LogType is TSLogType.
type LogType int

// The sources of a log message.
const (
	// LogParse is TSLogTypeParse.
	LogParse LogType = iota
	// LogLex is TSLogTypeLex.
	LogLex
)

// String returns the name of the source.
func (t LogType) String() string {
	switch t {
	case LogParse:
		return "parse"
	case LogLex:
		return "lex"
	}
	return unknownName
}

// logger is TSLogger. It is nil when the parser has no logger.
type logger func(LogType, string)

// decodeError is TS_DECODE_ERROR, the code point that a decoder returns for
// a sequence that is not valid.
const decodeError int32 = -1

// lead3T1Bits is U8_LEAD3_T1_BITS of ICU: for each lead byte of a 3-byte
// sequence, a bit for each valid top three bits of the first trail byte.
var lead3T1Bits = [16]byte{
	0x20, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30,
	0x30, 0x30, 0x30, 0x30, 0x30, 0x10, 0x30, 0x30,
}

// lead4T1Bits is U8_LEAD4_T1_BITS of ICU: for each top four bits of the
// first trail byte of a 4-byte sequence, a bit for each valid lead byte.
var lead4T1Bits = [16]byte{
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x1E, 0x0F, 0x0F, 0x0F, 0x00, 0x00, 0x00, 0x00,
}

// decodeUTF8 decodes the first character of s. It returns the number of
// bytes that it read and the code point, or decodeError for a sequence that
// is not valid. It returns what the ICU macro U8_NEXT returns, and not what
// unicode/utf8 returns, as docs/UPSTREAM.md says.
//
// decodeUTF8 is ts_decode_utf8, with U8_NEXT of ICU. U8_NEXT reads the first
// byte before it looks at the length, so C reads past an empty string. Go
// returns decodeError for it.
func decodeUTF8(s []byte) (uint32, int32) {
	length := uint32(len(s))
	if length == 0 {
		return 0, decodeError
	}
	i := uint32(0)
	c := int32(s[i])
	i++
	if c&0x80 == 0 {
		return i, c
	}
	var t byte
	// valid is true when the bytes before the last trail byte are valid.
	valid := false
	if i != length {
		if c >= 0xe0 {
			// fetch/validate/assemble all but last trail byte
			ok := false
			if c < 0xf0 {
				// U+0800..U+FFFF except surrogates
				c &= 0xf
				t = s[i]
				if lead3T1Bits[c]&(1<<(t>>5)) != 0 {
					t &= 0x3f
					ok = true
				}
			} else {
				// U+10000..U+10FFFF
				c -= 0xf0
				if c <= 4 {
					t = s[i]
					if lead4T1Bits[t>>4]&(1<<c) != 0 {
						c = c<<6 | int32(t&0x3f)
						i++
						if i != length {
							t = s[i] - 0x80
							ok = t <= 0x3f
						}
					}
				}
			}
			// valid second-to-last trail byte
			if ok {
				c = c<<6 | int32(t)
				i++
				valid = i != length
			}
		} else if c >= 0xc2 {
			// U+0080..U+07FF
			c &= 0x1f
			valid = true
		}
	}
	// last trail byte
	if valid {
		t = s[i] - 0x80
		if t <= 0x3f {
			c = c<<6 | int32(t)
			i++
			return i, c
		}
	}
	// ill-formed
	return i, decodeError
}

// u16SurrogateOffset is U16_SURROGATE_OFFSET of ICU.
const u16SurrogateOffset = (0xd800 << 10) + 0xdc00 - 0x10000

// decodeUTF16 decodes the first character of s, with the byte order of
// unit. It returns what U16_NEXT of ICU returns: a surrogate that has no
// pair is a code point of its own, and not an error.
func decodeUTF16(s []byte, unit func([]byte) uint16) (uint32, int32) {
	length := uint32(len(s))
	if length < 2 {
		return length, decodeError
	}
	i := uint32(0)
	// length is in bytes; U16_NEXT indexes into uint16_t*, so its length
	// parameter must be in code units (length / 2), not bytes.
	units := length / 2
	// U16_IS_LEAD and U16_IS_TRAIL mask a code unit with 0xfffffc00, and a
	// code unit has 16 bits, so the Go form masks it with 0xfc00.
	lead := unit(s[2*i:])
	c := int32(lead)
	i++
	if lead&0xfc00 == 0xd800 {
		if i != units {
			if c2 := unit(s[2*i:]); c2&0xfc00 == 0xdc00 {
				i++
				c = (c << 10) + int32(c2) - u16SurrogateOffset
			}
		}
	}
	return i * 2, c
}

// decodeUTF16LE is ts_decode_utf16_le, with U16_NEXT_LE.
func decodeUTF16LE(s []byte) (uint32, int32) {
	return decodeUTF16(s, binary.LittleEndian.Uint16)
}

// decodeUTF16BE is ts_decode_utf16_be, with U16_NEXT_BE.
func decodeUTF16BE(s []byte) (uint32, int32) {
	return decodeUTF16(s, binary.BigEndian.Uint16)
}

// columnData is ColumnData.
type columnData struct {
	value uint32
	valid bool
}

// lexer is Lexer, the lexer of a parser. data is the TSLexer that a lex
// function and an external scanner read.
type lexer struct {
	data               abi.Lexer
	currentPosition    length
	tokenStartPosition length
	tokenEndPosition   length

	includedRanges []textRange
	chunk          []byte
	input          input
	logger         logger

	currentIncludedRangeIndex uint32
	chunkStart                uint32
	lookaheadSize             uint32
	didGetColumn              bool
	columnData                columnData

	debugBuffer [abi.SerializationBufferSize]byte
}

// byteOrderMark is BYTE_ORDER_MARK.
const byteOrderMark int32 = 0xFEFF

// defaultRange is DEFAULT_RANGE.
var defaultRange = textRange{
	startPoint: point{0, 0},
	endPoint:   point{math.MaxUint32, math.MaxUint32},
	startByte:  0,
	endByte:    math.MaxUint32,
}

// log is the macro LOG. It writes a message with the character to the
// logger, if the lexer has one.
func (l *lexer) log(message string, character int32) {
	if l.logger == nil {
		return
	}
	if 32 <= character && character < 127 {
		l.writeLog(fmt.Sprintf("%s character:'%c'", message, character))
	} else {
		l.writeLog(fmt.Sprintf("%s character:%d", message, character))
	}
}

// writeLog writes a message to the logger through debugBuffer, as snprintf
// writes it in C: at most TREE_SITTER_SERIALIZATION_BUFFER_SIZE - 1 bytes and
// a NUL byte.
func (l *lexer) writeLog(message string) {
	n := copy(l.debugBuffer[:len(l.debugBuffer)-1], message)
	l.debugBuffer[n] = 0
	l.logger(LogLex, string(l.debugBuffer[:n]))
}

// setColumnData sets the column data to the given value and marks it valid.
//
// setColumnData is ts_lexer__set_column_data.
func (l *lexer) setColumnData(val uint32) {
	l.columnData.valid = true
	l.columnData.value = val
}

// incrementColumnData increments the value of the column data. It does
// nothing if the column data is not valid.
//
// incrementColumnData is ts_lexer__increment_column_data.
func (l *lexer) incrementColumnData() {
	if l.columnData.valid {
		l.columnData.value++
	}
}

// invalidateColumnData marks the column data as invalid.
//
// invalidateColumnData is ts_lexer__invalidate_column_data.
func (l *lexer) invalidateColumnData() {
	l.columnData.valid = false
	l.columnData.value = 0
}

// EOF is ts_lexer__eof.
//
// Check if the lexer has reached EOF. This state is stored
// by setting the lexer's `current_included_range_index` such that
// it has consumed all of its available ranges.
func (l *lexer) EOF() bool {
	return l.currentIncludedRangeIndex == uint32(len(l.includedRanges))
}

// clearChunk is ts_lexer__clear_chunk.
//
// Clear the currently stored chunk of source code, because the lexer's
// position has changed.
func (l *lexer) clearChunk() {
	l.chunk = nil
	l.chunkStart = 0
}

// getChunk is ts_lexer__get_chunk.
//
// Call the lexer's input callback to obtain a new chunk of source code
// for the current position.
func (l *lexer) getChunk() {
	l.chunkStart = l.currentPosition.bytes
	l.chunk = l.input.read.ReadAt(int(l.currentPosition.bytes), l.currentPosition.extent.public())
	if len(l.chunk) == 0 {
		l.currentIncludedRangeIndex = uint32(len(l.includedRanges))
		l.chunk = nil
	}
}

// decode returns the decoder of the encoding of the input.
func (l *lexer) decode() func([]byte) (uint32, int32) {
	switch l.input.encoding {
	case EncodingUTF16LE:
		return decodeUTF16LE
	case EncodingUTF16BE:
		return decodeUTF16BE
	}
	return decodeUTF8
}

// getLookahead is ts_lexer__get_lookahead.
//
// Decode the next unicode character in the current chunk of source code.
// This assumes that the lexer has already retrieved a chunk of source
// code that spans the current position.
func (l *lexer) getLookahead() {
	positionInChunk := l.currentPosition.bytes - l.chunkStart
	size := uint32(len(l.chunk)) - positionInChunk

	if size == 0 {
		l.lookaheadSize = 1
		l.data.Lookahead = '\x00'
		return
	}

	chunk := l.chunk[positionInChunk:]

	if l.input.encoding == EncodingUTF8 && chunk[0] < 0x80 {
		l.data.Lookahead = int32(chunk[0])
		l.lookaheadSize = 1
		return
	}

	decode := l.decode()

	l.lookaheadSize, l.data.Lookahead = decode(chunk[:size])

	// If this chunk ended in the middle of a multi-byte character,
	// try again with a fresh chunk.
	if l.data.Lookahead == decodeError && size < 4 {
		l.getChunk()
		chunk = l.chunk
		l.lookaheadSize, l.data.Lookahead = decode(chunk)
	}

	if l.data.Lookahead == decodeError {
		l.lookaheadSize = 1
	}
}

// gotoPosition is ts_lexer_goto. goto is a keyword of Go.
func (l *lexer) gotoPosition(position length) {
	if position.bytes != l.currentPosition.bytes {
		l.invalidateColumnData()
	}

	l.currentPosition = position

	// Move to the first valid position at or after the given position.
	foundIncludedRange := false
	for i := range l.includedRanges {
		includedRange := &l.includedRanges[i]
		if includedRange.endByte > l.currentPosition.bytes &&
			includedRange.endByte > includedRange.startByte {
			if includedRange.startByte >= l.currentPosition.bytes {
				l.currentPosition = length{
					bytes:  includedRange.startByte,
					extent: includedRange.startPoint,
				}
			}

			l.currentIncludedRangeIndex = uint32(i)
			foundIncludedRange = true
			break
		}
	}

	if foundIncludedRange {
		// If the current position is outside of the current chunk of text,
		// then clear out the current chunk of text.
		if l.chunk != nil && (l.currentPosition.bytes < l.chunkStart ||
			l.currentPosition.bytes >= l.chunkStart+uint32(len(l.chunk))) {
			l.clearChunk()
		}

		l.lookaheadSize = 0
		l.data.Lookahead = '\x00'
	} else {
		// If the given position is beyond any of included ranges, move to the EOF
		// state - past the end of the included ranges.
		l.currentIncludedRangeIndex = uint32(len(l.includedRanges))
		lastIncludedRange := &l.includedRanges[len(l.includedRanges)-1]
		l.currentPosition = length{
			bytes:  lastIncludedRange.endByte,
			extent: lastIncludedRange.endPoint,
		}
		l.clearChunk()
		l.lookaheadSize = 1
		l.data.Lookahead = '\x00'
	}
}

// doAdvance actually advances the lexer. It does not log anything. When skip
// is true, it marks the consumed code point as whitespace.
//
// doAdvance is ts_lexer__do_advance.
func (l *lexer) doAdvance(skip bool) {
	if l.lookaheadSize != 0 {
		if l.data.Lookahead == '\n' {
			l.currentPosition.extent.row++
			l.currentPosition.extent.column = 0
			l.setColumnData(0)
		} else {
			isBOM := l.currentPosition.bytes == 0 &&
				l.data.Lookahead == byteOrderMark
			if !isBOM {
				l.incrementColumnData()
			}
			l.currentPosition.extent.column += l.lookaheadSize
		}
		l.currentPosition.bytes += l.lookaheadSize
	}

	// currentRange is the included range at currentIncludedRangeIndex, or
	// nil past the last one, as the pointer current_range is in C.
	currentRange := &l.includedRanges[l.currentIncludedRangeIndex]
	for l.currentPosition.bytes >= currentRange.endByte ||
		currentRange.endByte == currentRange.startByte {
		if l.currentIncludedRangeIndex < uint32(len(l.includedRanges)) {
			l.currentIncludedRangeIndex++
		}
		if l.currentIncludedRangeIndex < uint32(len(l.includedRanges)) {
			currentRange = &l.includedRanges[l.currentIncludedRangeIndex]
			l.currentPosition = length{
				currentRange.startByte,
				currentRange.startPoint,
			}
		} else {
			currentRange = nil
			break
		}
	}

	if skip {
		l.tokenStartPosition = l.currentPosition
	}

	if currentRange != nil {
		if l.currentPosition.bytes < l.chunkStart ||
			l.currentPosition.bytes >= l.chunkStart+uint32(len(l.chunk)) {
			l.getChunk()
		}
		l.getLookahead()
	} else {
		l.clearChunk()
		l.data.Lookahead = '\x00'
		l.lookaheadSize = 1
	}
}

// Advance is ts_lexer__advance.
//
// Advance to the next character in the source code, retrieving a new
// chunk of source code if needed.
func (l *lexer) Advance(skip bool) {
	if l.chunk == nil {
		return
	}

	if skip {
		l.log("skip", l.data.Lookahead)
	} else {
		l.log("consume", l.data.Lookahead)
	}

	nextPosition := l.currentPosition.bytes + 1
	currentRangeEnd := l.includedRanges[l.currentIncludedRangeIndex].endByte
	if l.input.encoding == EncodingUTF8 &&
		l.lookaheadSize == 1 &&
		l.data.Lookahead != '\n' &&
		nextPosition < currentRangeEnd &&
		nextPosition < l.chunkStart+uint32(len(l.chunk)) {
		nextByte := l.chunk[nextPosition-l.chunkStart]
		if nextByte < 0x80 {
			l.incrementColumnData()
			l.currentPosition.bytes++
			l.currentPosition.extent.column++
			if skip {
				l.tokenStartPosition = l.currentPosition
			}
			l.data.Lookahead = int32(nextByte)
			return
		}
	}

	l.doAdvance(skip)
}

// MarkEnd is ts_lexer__mark_end.
//
// Mark that a token match has completed. This can be called multiple
// times if a longer match is found later.
func (l *lexer) MarkEnd() {
	if !l.EOF() {
		// If the lexer is right at the beginning of included range,
		// then the token should be considered to end at the *end* of the
		// previous included range, rather than here.
		currentIncludedRange := &l.includedRanges[l.currentIncludedRangeIndex]
		if l.currentIncludedRangeIndex > 0 &&
			l.currentPosition.bytes == currentIncludedRange.startByte {
			previousIncludedRange := &l.includedRanges[l.currentIncludedRangeIndex-1]
			l.tokenEndPosition = length{
				previousIncludedRange.endByte,
				previousIncludedRange.endPoint,
			}
			return
		}
	}
	l.tokenEndPosition = l.currentPosition
}

// GetColumn is ts_lexer__get_column.
func (l *lexer) GetColumn() uint32 {
	l.didGetColumn = true

	if !l.columnData.valid {
		// Record current position
		goalByte := l.currentPosition.bytes

		// Back up to the beginning of the line
		startOfCol := length{
			l.currentPosition.bytes - l.currentPosition.extent.column,
			point{l.currentPosition.extent.row, 0},
		}
		l.gotoPosition(startOfCol)
		l.setColumnData(0)
		l.getChunk()

		if !l.EOF() {
			l.getLookahead()

			// Advance to the recorded position
			for l.currentPosition.bytes < goalByte && !l.EOF() && l.chunk != nil {
				l.doAdvance(false)
				if l.EOF() {
					break
				}
			}
		}
	}

	return l.columnData.value
}

// IsAtIncludedRangeStart is ts_lexer__is_at_included_range_start.
//
// Is the lexer at a boundary between two disjoint included ranges of
// source code? This is exposed as an API because some languages' external
// scanners need to perform custom actions at these boundaries.
func (l *lexer) IsAtIncludedRangeStart() bool {
	if l.currentIncludedRangeIndex < uint32(len(l.includedRanges)) {
		currentRange := &l.includedRanges[l.currentIncludedRangeIndex]
		return l.currentPosition.bytes == currentRange.startByte
	}
	return false
}

// Logf is ts_lexer__log. The format is a format of the package fmt, because
// the scanner that calls it is Go.
func (l *lexer) Logf(format string, args ...any) {
	if l.logger != nil {
		l.writeLog(fmt.Sprintf(format, args...))
	}
}

// init is ts_lexer_init.
func (l *lexer) init() {
	*l = lexer{
		data: abi.Lexer{
			Lookahead:    0,
			ResultSymbol: 0,
		},
		chunk:                     nil,
		chunkStart:                0,
		currentPosition:           length{0, point{0, 0}},
		logger:                    nil,
		includedRanges:            nil,
		currentIncludedRangeIndex: 0,
		didGetColumn:              false,
		columnData: columnData{
			valid: false,
			value: 0,
		},
	}
	// The lexer's methods are stored as struct fields so that generated
	// parsers can call them without needing to be linked against this
	// library.
	l.data.Funcs = l
	l.setIncludedRanges(nil)
}

// setInput is ts_lexer_set_input.
func (l *lexer) setInput(in input) {
	l.input = in
	l.clearChunk()
	l.gotoPosition(l.currentPosition)
}

// reset is ts_lexer_reset.
//
// Move the lexer to the given position. This doesn't do any work
// if the parser is already at the given position.
func (l *lexer) reset(position length) {
	if position.bytes != l.currentPosition.bytes {
		l.gotoPosition(position)
	}
}

// start is ts_lexer_start.
func (l *lexer) start() {
	l.tokenStartPosition = l.currentPosition
	l.tokenEndPosition = lengthUndefined
	l.data.ResultSymbol = 0
	l.didGetColumn = false
	if !l.EOF() {
		if len(l.chunk) == 0 {
			l.getChunk()
		}
		if l.lookaheadSize == 0 {
			l.getLookahead()
		}
		if l.currentPosition.bytes == 0 {
			if l.data.Lookahead == byteOrderMark {
				l.Advance(true)
			}
			l.setColumnData(0)
		}
	}
}

// finish is ts_lexer_finish. The C function takes lookahead_end_byte as a
// pointer, and the Go function returns the new value.
func (l *lexer) finish(lookaheadEndByte uint32) uint32 {
	if l.tokenEndPosition.isUndefined() {
		l.MarkEnd()
	}

	// If the token ended at an included range boundary, then its end position
	// will have been reset to the end of the preceding range. Reset the start
	// position to match.
	if l.tokenEndPosition.bytes < l.tokenStartPosition.bytes {
		l.tokenStartPosition = l.tokenEndPosition
	}

	currentLookaheadEndByte := l.currentPosition.bytes + 1

	// In order to determine that a byte sequence is invalid UTF8 or UTF16,
	// the character decoding algorithm may have looked at the following byte.
	// Therefore, the next byte *after* the current (invalid) character
	// affects the interpretation of the current character.
	if l.data.Lookahead == decodeError {
		currentLookaheadEndByte += 4 // the maximum number of bytes read to identify an invalid code point
	}

	if currentLookaheadEndByte > lookaheadEndByte {
		lookaheadEndByte = currentLookaheadEndByte
	}
	return lookaheadEndByte
}

// markEnd is ts_lexer_mark_end.
func (l *lexer) markEnd() {
	l.MarkEnd()
}

// setIncludedRanges is ts_lexer_set_included_ranges. It keeps a copy of
// ranges.
func (l *lexer) setIncludedRanges(ranges []textRange) bool {
	if len(ranges) == 0 {
		ranges = []textRange{defaultRange}
	} else {
		previousByte := uint32(0)
		for i := range ranges {
			r := &ranges[i]
			if r.startByte < previousByte ||
				r.endByte < r.startByte {
				return false
			}
			previousByte = r.endByte
		}
	}

	l.includedRanges = slices.Clone(ranges)
	l.gotoPosition(l.currentPosition)
	return true
}

// getIncludedRanges is ts_lexer_included_ranges. It returns the ranges of
// the lexer, and not a copy.
func (l *lexer) getIncludedRanges() []textRange {
	return l.includedRanges
}
