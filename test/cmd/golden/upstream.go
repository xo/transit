package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// variant is one way to run the upstream tool on a grammar (D17, D19).
type variant struct {
	name  string
	abi   int
	merge bool
}

var (
	abi15        = variant{name: "abi15", abi: 15, merge: true}
	abi14        = variant{name: "abi14", abi: 14, merge: true}
	abi15NoMerge = variant{name: "abi15-nomerge", abi: 15, merge: false}
)

// output is what one run of the upstream tool writes.
type output struct {
	grammarJSON []byte
	parserC     []byte
	nodeTypes   []byte
	err         string // the error text, when the tool rejects the grammar
}

// checkUpstream makes sure that the upstream checkout is at the base commit,
// and that docs/UPSTREAM.md names the same commit.
func (h *harness) checkUpstream(ctx context.Context) error {
	head, err := command(ctx, h.ts, "git", "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("reading the commit of tree-sitter/: %w", err)
	}
	if got := strings.TrimSpace(head); got != baseCommit {
		return fmt.Errorf("tree-sitter/ is at %s, and the base commit is %s (D30)", got, baseCommit)
	}
	doc, err := os.ReadFile(filepath.Join(h.root, "docs", "UPSTREAM.md"))
	if err != nil {
		return fmt.Errorf("reading docs/UPSTREAM.md: %w", err)
	}
	if !bytes.Contains(doc, []byte(baseCommit)) {
		return fmt.Errorf("docs/UPSTREAM.md does not name the base commit %s", baseCommit)
	}
	return nil
}

// buildTool builds the upstream tool at the base commit, and reads its version
// and the version of the Rust compiler.
func (h *harness) buildTool(ctx context.Context) error {
	h.tool = filepath.Join(h.ts, "target", "release", "tree-sitter")
	if _, err := os.Stat(h.tool); err != nil {
		h.logf("building the upstream tool\n")
		if _, err := command(ctx, h.ts, "cargo", "build", "--release", "-p", "tree-sitter-cli"); err != nil {
			return fmt.Errorf("building the upstream tool: %w", err)
		}
	}
	v, err := command(ctx, h.ts, h.tool, "--version")
	if err != nil {
		return fmt.Errorf("reading the version of the upstream tool: %w", err)
	}
	h.toolVersion = strings.TrimSpace(v)
	r, err := command(ctx, h.ts, "rustc", "--version")
	if err != nil {
		return fmt.Errorf("reading the version of rustc: %w", err)
	}
	h.rustVersion = strings.TrimSpace(r)
	return nil
}

// generate runs the upstream tool once. dir is a folder that the tool can
// write, and grammar is the path of grammar.js or grammar.json. The tool
// writes its output to out.
func (h *harness) generate(ctx context.Context, dir, grammar, out string, v variant) (output, error) {
	args := []string{"generate", grammar, "--abi", strconv.Itoa(v.abi), "-o", out}
	if !v.merge {
		args = append(args, "--disable-optimizations")
	}
	cmd := exec.CommandContext(ctx, h.tool, args...)
	cmd.Dir = dir
	// the environment can choose the ABI and the JavaScript runtime, so the
	// harness clears both
	cmd.Env = cleanEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	var o output
	if err := cmd.Run(); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			return o, fmt.Errorf("running the upstream tool on %s: %w", grammar, err)
		}
		o.err = strings.TrimSpace(stderr.String())
		return o, nil
	}
	var err error
	if o.grammarJSON, err = readIfExists(filepath.Join(out, "grammar.json")); err != nil {
		return o, err
	}
	if o.parserC, err = os.ReadFile(filepath.Join(out, "parser.c")); err != nil {
		return o, fmt.Errorf("reading parser.c of %s: %w", grammar, err)
	}
	if o.nodeTypes, err = os.ReadFile(filepath.Join(out, "node-types.json")); err != nil {
		return o, fmt.Errorf("reading node-types.json of %s: %w", grammar, err)
	}
	return o, nil
}

// generateTwice runs the upstream tool twice, each time into a new folder, and
// fails when the two runs differ. A difference is a fault of upstream, and it
// must not be blamed on the port.
func (h *harness) generateTwice(ctx context.Context, dir, grammar string, v variant) (output, error) {
	var runs [2]output
	for i := range runs {
		out, err := os.MkdirTemp("", "golden-")
		if err != nil {
			return output{}, fmt.Errorf("making a folder for the output: %w", err)
		}
		runs[i], err = h.generate(ctx, dir, grammar, out, v)
		_ = os.RemoveAll(out)
		if err != nil {
			return output{}, err
		}
	}
	a, b := runs[0], runs[1]
	if a.err != b.err || !bytes.Equal(a.parserC, b.parserC) || !bytes.Equal(a.nodeTypes, b.nodeTypes) {
		return output{}, fmt.Errorf("running the upstream tool twice on %s at %s gave two outputs", grammar, v.name)
	}
	return a, nil
}

// cleanEnv returns the environment without the variables that change the
// output of the upstream tool, and with NO_COLOR set, so that an error has no
// codes for the colors of a terminal.
func cleanEnv() []string {
	env := []string{"NO_COLOR=1"}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "TREE_SITTER_ABI_VERSION=") || strings.HasPrefix(e, "TREE_SITTER_JS_RUNTIME=") ||
			strings.HasPrefix(e, "NO_COLOR=") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// command runs a command in a folder and returns what it writes.
func command(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("running %s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// readIfExists reads a file, and returns nil when there is no file.
func readIfExists(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return b, nil
}

// sum returns the SHA-256 of a file, in hex.
func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
