package transit

import (
	"context"
	"errors"
	"testing"
)

// The test module tests the parser on the fixture grammars. These tests
// hold the checks of the input, which need no grammar.

func TestParserInputChecks(t *testing.T) {
	ctx := context.Background()
	in := &stringInput{text: []byte("a")}
	decode := func(b []byte) (rune, int) {
		return rune(b[0]), 1
	}

	p := NewParser()
	if _, err := p.ParseCustomEncoding(ctx, in, decode, nil); !errors.Is(err, ErrNoLanguage) {
		t.Errorf("ParseCustomEncoding with no language = %v, want %v", err, ErrNoLanguage)
	}

	// The test language has no lex function, so SetLanguage refuses it. The
	// checks of the input come before the parser lexes.
	p.language = testLanguage(15)
	tests := []struct {
		name  string
		parse func() (*Tree, error)
	}{
		{"ParseCustomEncoding with no decode", func() (*Tree, error) {
			return p.ParseCustomEncoding(ctx, in, nil, nil)
		}},
		{"ParseCustomEncoding with no input", func() (*Tree, error) {
			return p.ParseCustomEncoding(ctx, nil, decode, nil)
		}},
		{"ParseInput with the custom encoding", func() (*Tree, error) {
			return p.ParseInput(ctx, in, encodingCustom, nil)
		}},
		{"ParseInput with an unknown encoding", func() (*Tree, error) {
			return p.ParseInput(ctx, in, Encoding(-1), nil)
		}},
	}
	for _, test := range tests {
		if _, err := test.parse(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s = %v, want %v", test.name, err, ErrInvalidInput)
		}
	}
}
