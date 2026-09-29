package cgrammar

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/xo/transit/generate"
	"github.com/xo/transit/generate/backend/c"
)

// buildLocks holds a *sync.Mutex for each output of a build, so that two
// tests that build the same library do not write the same files at once.
var buildLocks sync.Map

// lockBuild locks the build of the output out, and returns the unlock.
func lockBuild(out string) func() {
	m, _ := buildLocks.LoadOrStore(out, &sync.Mutex{})
	mu, _ := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// ErrMissing is the error of a build whose input is not on this machine:
// the checkout of upstream or a grammar of the cache. A test skips on it.
var ErrMissing = errors.New("missing")

// Root returns the root of the repository: the first folder above the
// working folder that holds the go.mod of github.com/xo/transit.
func Root() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("finding the working folder: %w", err)
	}
	for {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.HasPrefix(string(b), "module github.com/xo/transit\n") {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("finding the root of the repository: %w", ErrMissing)
		}
		dir = parent
	}
}

// CacheDir returns the folder of the cache of transit, as the golden
// harness names it: $XDG_CACHE_HOME/transit or ~/.cache/transit.
func CacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("finding the cache folder: %w", err)
	}
	return filepath.Join(dir, "transit"), nil
}

// hashFiles returns the hash of the names and the contents of files.
func hashFiles(files ...string) (string, error) {
	h := sha256.New()
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", f, err)
		}
		// a write to a hash never fails
		_, _ = fmt.Fprintf(h, "%s %d\n", filepath.Base(f), len(b))
		_, _ = h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// cc runs the C compiler in dir.
func cc(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "cc", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("running cc %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return nil
}

// BuildRuntime builds the shared library of the upstream C runtime from the
// checkout of upstream in the root of the repository, in the cache, and
// returns its path. A library of the same sources in the cache is used again.
func BuildRuntime(ctx context.Context, root, cache string) (string, error) {
	src := filepath.Join(root, "tree-sitter", "lib", "src")
	if _, err := os.Stat(filepath.Join(src, "lib.c")); err != nil {
		return "", fmt.Errorf("finding the checkout of upstream: %w", ErrMissing)
	}
	var files []string
	for _, dir := range []string{src, filepath.Join(src, "unicode"), filepath.Join(src, "portable"), filepath.Join(root, "tree-sitter", "lib", "include", "tree_sitter")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", dir, err)
		}
		for _, e := range entries {
			if !e.IsDir() && (strings.HasSuffix(e.Name(), ".c") || strings.HasSuffix(e.Name(), ".h")) {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	}
	key, err := hashFiles(files...)
	if err != nil {
		return "", err
	}
	out := filepath.Join(cache, "cgrammar", "runtime-"+key+".so")
	defer lockBuild(out)()
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", fmt.Errorf("making the folder of the runtime: %w", err)
	}
	tmp := out + ".tmp"
	if err := cc(ctx, src, "-shared", "-fPIC", "-O1", "-g",
		"-I", filepath.Join(root, "tree-sitter", "lib", "include"), "-I", src,
		"lib.c", "-o", tmp); err != nil {
		return "", err
	}
	return out, os.Rename(tmp, out)
}

// BuildGrammar builds the shared library of the grammar in the folder dir,
// which holds src/grammar.json and the scanner, in the cache, and returns
// its path. It generates parser.c at ABI 15 with the generator of transit,
// and compiles it with the headers of generate/templates and the scanner
// of the grammar. A library of the same inputs in the cache is used again.
func BuildGrammar(ctx context.Context, dir, cache string) (string, error) {
	src := filepath.Join(dir, "src")
	grammarJSON := filepath.Join(src, "grammar.json")
	if _, err := os.Stat(grammarJSON); err != nil {
		return "", fmt.Errorf("finding the grammar in %s: %w", dir, ErrMissing)
	}
	scanners, err := scannerFiles(src)
	if err != nil {
		return "", err
	}
	key, err := hashFiles(append([]string{grammarJSON}, scanners...)...)
	if err != nil {
		return "", err
	}
	build := filepath.Join(cache, "cgrammar", filepath.Base(dir)+"-"+key)
	out := filepath.Join(build, "grammar.so")
	defer lockBuild(out)()
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}

	b, err := os.ReadFile(grammarJSON)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", grammarJSON, err)
	}
	var diagnostics []generate.Diagnostic
	_, code, err := generate.ParserForGrammar(b, nil, generate.OptLevelMergeStates, c.Backend{}, &diagnostics)
	if err != nil {
		return "", fmt.Errorf("generating the parser of %s: %w", dir, err)
	}
	headers := filepath.Join(build, "tree_sitter")
	if err := os.MkdirAll(headers, 0o755); err != nil {
		return "", fmt.Errorf("making the build folder of %s: %w", dir, err)
	}
	for name, text := range map[string]string{
		"parser.h": generate.ParserHeader,
		"alloc.h":  generate.AllocHeader,
		"array.h":  generate.ArrayHeader,
	} {
		if err := os.WriteFile(filepath.Join(headers, name), []byte(text), 0o644); err != nil {
			return "", fmt.Errorf("writing %s: %w", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(build, "parser.c"), []byte(code), 0o644); err != nil {
		return "", fmt.Errorf("writing the parser of %s: %w", dir, err)
	}
	// The headers of the build folder come first, so that the scanner gets
	// the headers of the same version as parser.c.
	args := []string{"-shared", "-fPIC", "-O1", "-g", "-I", build, "-I", src, filepath.Join(build, "parser.c")}
	args = append(args, scanners...)
	tmp := out + ".tmp"
	args = append(args, "-o", tmp)
	if err := cc(ctx, build, args...); err != nil {
		return "", err
	}
	return out, os.Rename(tmp, out)
}

// scannerFiles returns the C files of the scanner in src: scanner.c, and
// any other C file but parser.c.
func scannerFiles(src string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != src {
			return filepath.SkipDir
		}
		if strings.HasSuffix(path, ".c") && filepath.Base(path) != "parser.c" {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("finding the scanner in %s: %w", src, err)
	}
	slices.Sort(out)
	return out, nil
}
