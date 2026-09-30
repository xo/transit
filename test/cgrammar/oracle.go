package cgrammar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OracleInput is the input of the Rust oracle of D71, in test/injectoracle.
type OracleInput struct {
	// Languages are the languages that the root and the injections can
	// name.
	Languages []OracleLanguage `json:"languages"`
	// Root is the name of the language of the root layer.
	Root string `json:"root"`
	// Source is the text.
	Source string `json:"source"`
}

// OracleLanguage is a language of the input of the oracle.
type OracleLanguage struct {
	// Name is the name that an injection gives for the language, such as
	// "javascript".
	Name string `json:"name"`
	// Library is the path of the shared library of the grammar.
	Library string `json:"library"`
	// Symbol is the function of the library that returns the language,
	// such as "tree_sitter_javascript".
	Symbol string `json:"symbol"`
	// Injections is the text of queries/injections.scm of the grammar, or
	// "".
	Injections string `json:"injections"`
}

// OracleLayer is a layer that the highlighter of upstream builds.
type OracleLayer struct {
	// Name is the name of the language of the layer.
	Name string `json:"name"`
	// Depth is 0 for the root layer.
	Depth int `json:"depth"`
	// Ranges are the included ranges of the layer. The root layer has one
	// range from 0 to the largest usize of Rust, with that value as its end
	// row and column.
	Ranges []OracleRange `json:"ranges"`
	// Tree is the S-expression of the root node of the tree of the layer.
	Tree string `json:"tree"`
}

// OracleRange is a range of a layer of the oracle. The fields are uint64, so
// that the end of the root range of upstream fits.
type OracleRange struct {
	StartByte  uint64      `json:"start_byte"`
	EndByte    uint64      `json:"end_byte"`
	StartPoint OraclePoint `json:"start_point"`
	EndPoint   OraclePoint `json:"end_point"`
}

// OraclePoint is a point of a range of the oracle. The JSON of the oracle
// holds it as the array [row, column].
type OraclePoint struct {
	Row    uint64
	Column uint64
}

// MarshalJSON writes the point as [row, column].
func (p OraclePoint) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal([2]uint64{p.Row, p.Column})
	if err != nil {
		return nil, fmt.Errorf("writing a point: %w", err)
	}
	return b, nil
}

// UnmarshalJSON reads the point from [row, column].
func (p *OraclePoint) UnmarshalJSON(b []byte) error {
	var a [2]uint64
	if err := json.Unmarshal(b, &a); err != nil {
		return fmt.Errorf("reading a point: %w", err)
	}
	p.Row, p.Column = a[0], a[1]
	return nil
}

// BuildOracle builds the Rust oracle of D71 in test/injectoracle with cargo,
// offline and with the versions of its Cargo.lock, and returns the path of
// the program. The build folder of cargo is in the cache, and cargo builds
// again only what changed. It returns an error that wraps ErrMissing when
// cargo is not installed or the checkout of upstream is missing.
func BuildOracle(ctx context.Context, root, cache string) (string, error) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		return "", fmt.Errorf("finding cargo: %w", errors.Join(ErrMissing, err))
	}
	for _, f := range []string{
		filepath.Join(root, "tree-sitter", "lib", "Cargo.toml"),
		filepath.Join(root, "tree-sitter", "crates", "highlight", "src", "highlight.rs"),
	} {
		if _, err := os.Stat(f); err != nil {
			return "", fmt.Errorf("finding the checkout of upstream: %w", ErrMissing)
		}
	}
	target := filepath.Join(cache, "injectoracle", "target")
	defer lockBuild(target)()
	cmd := exec.CommandContext(ctx, cargo, "build", "--release", "--offline", "--locked",
		"--manifest-path", filepath.Join(root, "test", "injectoracle", "Cargo.toml"),
		"--target-dir", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("building the oracle: %w: %s", err, out)
	}
	return filepath.Join(target, "release", "injectoracle"), nil
}

// RunOracle runs the oracle binary on the input, and returns the layers that
// it prints, sorted by the start of the first range and then by the depth,
// as D72 sorts them.
func RunOracle(ctx context.Context, binary string, in OracleInput) ([]OracleLayer, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("writing the input of the oracle: %w", err)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binary)
	cmd.Stdin = bytes.NewReader(b)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running the oracle: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var layers []OracleLayer
	if err := json.Unmarshal(stdout.Bytes(), &layers); err != nil {
		return nil, fmt.Errorf("reading the output of the oracle: %w", err)
	}
	return layers, nil
}
