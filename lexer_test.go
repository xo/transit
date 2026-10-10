package transit

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDecodeUTF8Cases(t *testing.T) {
	tests := []struct {
		in   string
		n    uint32
		want int32
	}{
		{"a", 1, 'a'},
		{"\x00", 1, 0},
		{"éx", 2, 0xe9},
		{"€", 3, 0x20ac},
		{"\U0001d11e", 4, 0x1d11e},
		{"\U0010ffff", 4, 0x10ffff},
		// a trail byte, an overlong form and a byte that is never valid
		{"\x80", 1, decodeError},
		{"\xc0\x80", 1, decodeError},
		{"\xc1\xbf", 1, decodeError},
		{"\xff", 1, decodeError},
		// a surrogate
		{"\xed\xa0\x80", 1, decodeError},
		// past U+10FFFF
		{"\xf4\x90\x80\x80", 1, decodeError},
		{"\xf5\x80\x80\x80", 1, decodeError},
		// ICU consumes the maximal subpart of a sequence that ends early
		{"\xe2\x82", 2, decodeError},
		{"\xe2\x82x", 2, decodeError},
		{"\xf0\x9d\x84", 3, decodeError},
		{"\xf0\x9dx", 2, decodeError},
		{"\xc3", 1, decodeError},
		{"\xc3x", 1, decodeError},
		{"", 0, decodeError},
	}
	for _, test := range tests {
		n, got := decodeUTF8([]byte(test.in))
		if n != test.n || got != test.want {
			t.Errorf("decodeUTF8(%q) = %d, %d, want %d, %d", test.in, n, got, test.n, test.want)
		}
	}
}

// maximalSubpart decodes the first character of s by the table of
// well-formed UTF-8 in section 3.9 of the Unicode standard. For a sequence
// that is not well formed, it returns the length of its maximal subpart and
// decodeError, as ICU does.
func maximalSubpart(s []byte) (uint32, int32) {
	lead := s[0]
	if lead < 0x80 {
		return 1, int32(lead)
	}
	var n int
	lo, hi := byte(0x80), byte(0xbf)
	switch {
	case 0xc2 <= lead && lead <= 0xdf:
		n = 2
	case lead == 0xe0:
		n, lo = 3, 0xa0
	case 0xe1 <= lead && lead <= 0xec, lead == 0xee, lead == 0xef:
		n = 3
	case lead == 0xed:
		n, hi = 3, 0x9f
	case lead == 0xf0:
		n, lo = 4, 0x90
	case 0xf1 <= lead && lead <= 0xf3:
		n = 4
	case lead == 0xf4:
		n, hi = 4, 0x8f
	default:
		return 1, decodeError
	}
	for i := 1; i < n; i++ {
		if i >= len(s) || s[i] < lo || s[i] > hi {
			return uint32(i), decodeError
		}
		lo, hi = 0x80, 0xbf
	}
	r, _ := utf8.DecodeRune(s[:n])
	return uint32(n), r
}

func TestDecodeUTF8IsTheMaximalSubpart(t *testing.T) {
	check := func(s []byte) {
		n, got := decodeUTF8(s)
		wantN, want := maximalSubpart(s)
		if n != wantN || got != want {
			t.Fatalf("decodeUTF8(% x) = %d, %d, want %d, %d", s, n, got, wantN, want)
		}
	}
	buf := make([]byte, 4)
	for a := range 256 {
		buf[0] = byte(a)
		check(buf[:1])
		for b := range 256 {
			buf[1] = byte(b)
			check(buf[:2])
			for c := range 256 {
				buf[2] = byte(c)
				check(buf[:3])
			}
		}
	}
	// every lead byte of four bytes, with every second and third byte, and
	// the edges of the ranges for the last byte
	for a := 0xf0; a <= 0xf4; a++ {
		buf[0] = byte(a)
		for b := range 256 {
			buf[1] = byte(b)
			for c := range 256 {
				buf[2] = byte(c)
				for _, d := range []byte{0x00, 0x7f, 0x80, 0xbf, 0xc0, 0xff} {
					buf[3] = d
					check(buf)
				}
			}
		}
	}
}

func TestDecodeUTF8OfEveryCodePoint(t *testing.T) {
	buf := make([]byte, 8)
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if 0xd800 <= r && r <= 0xdfff {
			continue
		}
		n := utf8.EncodeRune(buf, r)
		buf[n] = 'x'
		gotN, got := decodeUTF8(buf[:n+1])
		if gotN != uint32(n) || got != r {
			t.Fatalf("decodeUTF8 of U+%04X = %d, %d, want %d, %d", r, gotN, got, n, r)
		}
	}
}

func TestDecodeUTF16(t *testing.T) {
	tests := []struct {
		le   string
		n    uint32
		want int32
	}{
		{"a\x00", 2, 'a'},
		{"a\x00b\x00", 2, 'a'},
		{"\xac\x20", 2, 0x20ac},
		// a surrogate pair
		{"\x34\xd8\x1e\xdd", 4, 0x1d11e},
		// a lead surrogate with no trail is a code point of its own
		{"\x34\xd8", 2, 0xd834},
		{"\x34\xd8a\x00", 2, 0xd834},
		{"\x34\xd8\x1e", 2, 0xd834},
		// a trail surrogate alone
		{"\x1e\xdd", 2, 0xdd1e},
		// fewer than two bytes
		{"a", 1, decodeError},
		{"", 0, decodeError},
	}
	for _, test := range tests {
		n, got := decodeUTF16LE([]byte(test.le))
		if n != test.n || got != test.want {
			t.Errorf("decodeUTF16LE(%q) = %d, %d, want %d, %d", test.le, n, got, test.n, test.want)
		}
		be := []byte(test.le)
		for i := 0; i+1 < len(be); i += 2 {
			be[i], be[i+1] = be[i+1], be[i]
		}
		n, got = decodeUTF16BE(be)
		if n != test.n || got != test.want {
			t.Errorf("decodeUTF16BE(%q) = %d, %d, want %d, %d", be, n, got, test.n, test.want)
		}
	}
}

// chunkedInput is an Input that returns the text in chunks of a fixed size.
type chunkedInput struct {
	text      []byte
	chunkSize int
	reads     []int
}

func (in *chunkedInput) ReadAt(offset int, _ Point) []byte {
	in.reads = append(in.reads, offset)
	if offset >= len(in.text) {
		return nil
	}
	return in.text[offset:min(offset+in.chunkSize, len(in.text))]
}

// newTestLexer returns a lexer of text, read in chunks of chunkSize bytes.
func newTestLexer(text string, chunkSize int, encoding Encoding) (*lexer, *chunkedInput) {
	in := &chunkedInput{text: []byte(text), chunkSize: chunkSize}
	l := &lexer{}
	l.init()
	l.setInput(input{read: in, encoding: encoding})
	return l, in
}

// lexStep is the lookahead of the lexer at a position.
type lexStep struct {
	lookahead int32
	position  length
}

// lexAll starts the lexer and advances it to the end of the input, through
// the function members of its TSLexer, as a lex function does.
func lexAll(l *lexer) []lexStep {
	l.start()
	var steps []lexStep
	for !l.data.EOF() {
		steps = append(steps, lexStep{l.data.Lookahead, l.currentPosition})
		l.data.Advance(false)
	}
	return steps
}

func TestLexerAdvances(t *testing.T) {
	want := []lexStep{
		{'a', length{0, point{0, 0}}},
		{'b', length{1, point{0, 1}}},
		{'\n', length{2, point{0, 2}}},
		{'c', length{3, point{1, 0}}},
		{'d', length{4, point{1, 1}}},
	}
	for _, chunkSize := range []int{1, 2, 3, 100} {
		l, _ := newTestLexer("ab\ncd", chunkSize, EncodingUTF8)
		if got := lexAll(l); !slices.Equal(got, want) {
			t.Errorf("chunks of %d: the steps are %v, want %v", chunkSize, got, want)
		}
		if got := l.currentPosition; got != (length{5, point{1, 2}}) {
			t.Errorf("chunks of %d: the position at the end is %v", chunkSize, got)
		}
		if l.data.Lookahead != 0 || l.chunk != nil {
			t.Errorf("chunks of %d: at the end the lookahead is %d and the chunk %q", chunkSize, l.data.Lookahead, l.chunk)
		}
		// an advance at the end does nothing
		l.data.Advance(false)
		if got := l.currentPosition; got != (length{5, point{1, 2}}) {
			t.Errorf("chunks of %d: an advance at the end moved to %v", chunkSize, got)
		}
	}
}

func TestLexerDecodesAcrossChunks(t *testing.T) {
	// the first chunk ends in the middle of the \u00e9, so the lexer reads a
	// chunk again from the start of the character
	l, in := newTestLexer("a\u00e9b", 2, EncodingUTF8)
	want := []lexStep{
		{'a', length{0, point{0, 0}}},
		{0xe9, length{1, point{0, 1}}},
		{'b', length{3, point{0, 3}}},
	}
	if got := lexAll(l); !slices.Equal(got, want) {
		t.Errorf("the steps are %v, want %v", got, want)
	}
	if !slices.Equal(in.reads, []int{0, 1, 3, 4}) {
		t.Errorf("the reads are %v, want 0, 1, 3 and 4", in.reads)
	}

	// An input that gives 2 bytes from each offset never gives a whole
	// character of 4 bytes, so the lexer reads each byte as an error, as in
	// C.
	l, _ = newTestLexer("\U0001d11eb", 2, EncodingUTF8)
	want = []lexStep{
		{decodeError, length{0, point{0, 0}}},
		{decodeError, length{1, point{0, 1}}},
		{decodeError, length{2, point{0, 2}}},
		{decodeError, length{3, point{0, 3}}},
		{'b', length{4, point{0, 4}}},
	}
	if got := lexAll(l); !slices.Equal(got, want) {
		t.Errorf("chunks of 2: the steps are %v, want %v", got, want)
	}
}

func TestLexerInvalidUTF8(t *testing.T) {
	l, _ := newTestLexer("\xffa\xe2\x82", 10, EncodingUTF8)
	want := []lexStep{
		{decodeError, length{0, point{0, 0}}},
		{'a', length{1, point{0, 1}}},
		// the lexer advances one byte past an invalid character
		{decodeError, length{2, point{0, 2}}},
		{decodeError, length{3, point{0, 3}}},
	}
	if got := lexAll(l); !slices.Equal(got, want) {
		t.Errorf("the steps are %v, want %v", got, want)
	}

	l, _ = newTestLexer("\xff", 10, EncodingUTF8)
	l.start()
	l.MarkEnd()
	// the decoder can read 4 more bytes after an invalid character
	if got := l.finish(0); got != 5 {
		t.Errorf("finish after an invalid character = %d, want 5", got)
	}
}

func TestLexerUTF16(t *testing.T) {
	// "a\n" and U+1D11E, in UTF-16LE and UTF-16BE
	le := "a\x00\n\x00\x34\xd8\x1e\xdd"
	be := "\x00a\x00\n\xd8\x34\xdd\x1e"
	want := []lexStep{
		{'a', length{0, point{0, 0}}},
		{'\n', length{2, point{0, 2}}},
		{0x1d11e, length{4, point{1, 0}}},
	}
	for _, test := range []struct {
		text     string
		encoding Encoding
	}{{le, EncodingUTF16LE}, {be, EncodingUTF16BE}} {
		l, _ := newTestLexer(test.text, 4, test.encoding)
		if got := lexAll(l); !slices.Equal(got, want) {
			t.Errorf("%v: the steps are %v, want %v", test.encoding, got, want)
		}
		if got := l.currentPosition; got != (length{8, point{1, 4}}) {
			t.Errorf("%v: the position at the end is %v", test.encoding, got)
		}

		// A chunk of 3 bytes splits the surrogate pair. U16_NEXT returns a
		// lead surrogate with no trail as a character, and not an error, so
		// the lexer does not read the chunk again, as in C.
		l, _ = newTestLexer(test.text, 3, test.encoding)
		split := append(slices.Clone(want[:2]),
			lexStep{0xd834, length{4, point{1, 0}}},
			lexStep{0xdd1e, length{6, point{1, 2}}})
		if got := lexAll(l); !slices.Equal(got, split) {
			t.Errorf("%v, chunks of 3: the steps are %v, want %v", test.encoding, got, split)
		}
	}
}

func TestLexerSkipsAByteOrderMark(t *testing.T) {
	l, _ := newTestLexer("\ufeffab", 10, EncodingUTF8)
	l.start()
	if l.data.Lookahead != 'a' {
		t.Fatalf("the lookahead after start is %d, want 'a'", l.data.Lookahead)
	}
	// the column of the point counts the bytes of the mark, and the column
	// of the lexer does not count the mark
	if got := l.tokenStartPosition; got != (length{3, point{0, 3}}) {
		t.Errorf("the token starts at %v", got)
	}
	if got := l.data.GetColumn(); got != 0 {
		t.Errorf("GetColumn() = %d, want 0", got)
	}
	if !l.didGetColumn {
		t.Error("GetColumn did not set didGetColumn")
	}
}

func TestLexerGetColumn(t *testing.T) {
	l, _ := newTestLexer("ab\nxéyz", 3, EncodingUTF8)
	l.reset(length{7, point{1, 4}})
	l.start()
	if l.data.Lookahead != 'z' {
		t.Fatalf("the lookahead at byte 7 is %d, want 'z'", l.data.Lookahead)
	}
	// the column counts characters, and the é has two bytes
	if got := l.data.GetColumn(); got != 3 {
		t.Errorf("GetColumn() = %d, want 3", got)
	}
	if got := l.currentPosition; got != (length{7, point{1, 4}}) {
		t.Errorf("GetColumn moved the lexer to %v", got)
	}
	if l.data.Lookahead != 'z' {
		t.Errorf("GetColumn changed the lookahead to %d", l.data.Lookahead)
	}
	// the column stays valid as the lexer advances on the line
	l.data.Advance(false)
	if got := l.data.GetColumn(); got != 4 {
		t.Errorf("GetColumn() after an advance = %d, want 4", got)
	}
}

func TestLexerIncludedRanges(t *testing.T) {
	l, _ := newTestLexer("abcdef", 10, EncodingUTF8)
	ranges := []textRange{
		{point{0, 1}, point{0, 3}, 1, 3},
		{point{0, 4}, point{0, 6}, 4, 6},
	}
	if !l.setIncludedRanges(ranges) {
		t.Fatal("setIncludedRanges returned false")
	}
	ranges[0].startByte = 0
	if got := l.getIncludedRanges()[0].startByte; got != 1 {
		t.Error("the lexer did not keep a copy of the ranges")
	}
	want := []lexStep{
		{'b', length{1, point{0, 1}}},
		{'c', length{2, point{0, 2}}},
		{'e', length{4, point{0, 4}}},
		{'f', length{5, point{0, 5}}},
	}
	l.start()
	var got []lexStep
	var starts []uint32
	for !l.EOF() {
		got = append(got, lexStep{l.data.Lookahead, l.currentPosition})
		if l.data.IsAtIncludedRangeStart() {
			starts = append(starts, l.currentPosition.bytes)
		}
		if l.currentPosition.bytes == 4 {
			// a token that ends at the start of a range ends at the end of
			// the range before it
			l.data.MarkEnd()
			if l.tokenEndPosition != (length{3, point{0, 3}}) {
				t.Errorf("MarkEnd at the start of a range = %v, want the end of the range before", l.tokenEndPosition)
			}
		}
		l.data.Advance(false)
	}
	if !slices.Equal(got, want) {
		t.Errorf("the steps are %v, want %v", got, want)
	}
	if !slices.Equal(starts, []uint32{1, 4}) {
		t.Errorf("the lexer is at the start of a range at %v, want 1 and 4", starts)
	}
	if l.data.IsAtIncludedRangeStart() {
		t.Error("IsAtIncludedRangeStart is true at the end")
	}

	// a position past the ranges goes to the end of the last one
	l.reset(length{6, point{0, 6}})
	if !l.EOF() || l.currentPosition != (length{6, point{0, 6}}) {
		t.Errorf("after a reset past the ranges, EOF is %t at %v", l.EOF(), l.currentPosition)
	}

	// ranges out of order are refused, and the lexer keeps its ranges
	bad := []textRange{{point{0, 3}, point{0, 5}, 3, 5}, {point{0, 1}, point{0, 2}, 1, 2}}
	if l.setIncludedRanges(bad) {
		t.Error("setIncludedRanges accepted ranges out of order")
	}
	if l.setIncludedRanges([]textRange{{point{0, 3}, point{0, 1}, 3, 1}}) {
		t.Error("setIncludedRanges accepted a range that ends before it starts")
	}
	if got := len(l.getIncludedRanges()); got != 2 {
		t.Errorf("the lexer has %d ranges after a refusal, want 2", got)
	}

	// no ranges is the whole text
	if !l.setIncludedRanges(nil) || !slices.Equal(l.getIncludedRanges(), []textRange{defaultRange}) {
		t.Errorf("setIncludedRanges(nil) gave %v", l.getIncludedRanges())
	}
}

func TestLexerStartAndFinish(t *testing.T) {
	l, _ := newTestLexer("abc", 10, EncodingUTF8)
	l.data.ResultSymbol = 9
	l.start()
	if l.data.ResultSymbol != 0 || !l.tokenEndPosition.isUndefined() {
		t.Errorf("start left the result %d and the end %v", l.data.ResultSymbol, l.tokenEndPosition)
	}
	l.data.Advance(false)
	l.data.Advance(false)
	// finish marks the end when the lex function did not
	if got := l.finish(0); got != 3 {
		t.Errorf("finish(0) = %d, want 3", got)
	}
	if l.tokenEndPosition != (length{2, point{0, 2}}) {
		t.Errorf("finish marked the end at %v", l.tokenEndPosition)
	}
	if got := l.finish(10); got != 10 {
		t.Errorf("finish(10) = %d, want 10", got)
	}

	// a token end before its start moves the start
	l.start()
	l.tokenEndPosition = length{1, point{0, 1}}
	l.finish(0)
	if l.tokenStartPosition != l.tokenEndPosition {
		t.Errorf("the start is %v and the end %v", l.tokenStartPosition, l.tokenEndPosition)
	}

	// markEnd is the same as MarkEnd
	l.markEnd()
	if l.tokenEndPosition != l.currentPosition {
		t.Errorf("markEnd set the end to %v, want %v", l.tokenEndPosition, l.currentPosition)
	}
}

func TestLexerSkip(t *testing.T) {
	l, _ := newTestLexer("  ab", 10, EncodingUTF8)
	l.start()
	l.data.Advance(true)
	l.data.Advance(true)
	if l.tokenStartPosition != (length{2, point{0, 2}}) {
		t.Errorf("after two skips the token starts at %v", l.tokenStartPosition)
	}
	l.data.Advance(false)
	if l.tokenStartPosition != (length{2, point{0, 2}}) {
		t.Errorf("a consume moved the token start to %v", l.tokenStartPosition)
	}
}

func TestLexerLogs(t *testing.T) {
	l, _ := newTestLexer("a\n", 10, EncodingUTF8)
	var messages []string
	l.logger = func(typ LogType, msg string) {
		if typ != LogLex {
			t.Errorf("the lexer logged with %v", typ)
		}
		messages = append(messages, msg)
	}
	l.start()
	l.data.Advance(false)
	l.data.Advance(true)
	l.data.Logf("%s=%d", "x", 7)
	want := []string{"consume character:'a'", "skip character:10", "x=7"}
	if !slices.Equal(messages, want) {
		t.Errorf("the messages are %q, want %q", messages, want)
	}

	// a message is cut at the size of the buffer, as snprintf cuts it
	messages = nil
	l.data.Logf("%s", strings.Repeat("y", 2000))
	if len(messages) != 1 || len(messages[0]) != 1023 {
		t.Errorf("a long message has %d bytes, want 1023", len(messages[0]))
	}
	if l.debugBuffer[1023] != 0 {
		t.Error("the buffer does not end with NUL")
	}

	// no logger, no message
	l.logger = nil
	l.data.Logf("z")
	l.log("consume", 'a')
}

func TestLexerStrings(t *testing.T) {
	for want, v := range map[string]interface{ String() string }{
		"UTF-8":    EncodingUTF8,
		"UTF-16LE": EncodingUTF16LE,
		"UTF-16BE": EncodingUTF16BE,
		"custom":   encodingCustom,
		"unknown":  Encoding(9),
		"parse":    LogParse,
		"lex":      LogLex,
	} {
		if got := v.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
	if got := LogType(9).String(); got != "unknown" {
		t.Errorf("LogType(9).String() = %q", got)
	}
}

func TestRangeConversion(t *testing.T) {
	r := textRange{point{1, 2}, point{3, 4}, 5, 6}
	if got := r.public(); got != (Range{StartByte: 5, EndByte: 6, StartPoint: Point{1, 2}, EndPoint: Point{3, 4}}) {
		t.Errorf("public() = %+v", got)
	}
	if got := r.public().internal(); got != r {
		t.Errorf("public().internal() = %+v, want %+v", got, r)
	}
}
