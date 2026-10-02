package cgrammar

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// buildMode is a build of the C runtime and the C grammars: the folder of
// the cache that holds it, the flags of the C compiler that set its
// optimization, and the C macros of a grammar, such as USQL_OPTIONS=1,
// which the compiler gets as -D flags.
type buildMode struct {
	folder  string
	flags   []string
	defines []string
}

var (
	// testBuild is the build of the tests that compare trees, with debug
	// information.
	testBuild = buildMode{folder: "cgrammar", flags: []string{"-O1", "-g"}}
	// o2Build is the build of the benchmarks, with -O2, as upstream builds a
	// grammar (D92).
	o2Build = buildMode{folder: "cgrammar-O2", flags: []string{"-O2"}}
)

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
	return buildRuntime(ctx, root, cache, testBuild)
}

// BuildRuntimeO2 is BuildRuntime for the benchmarks. It compiles with -O2,
// in a folder of the cache of its own (D92).
func BuildRuntimeO2(ctx context.Context, root, cache string) (string, error) {
	return buildRuntime(ctx, root, cache, o2Build)
}

// buildRuntime is BuildRuntime with the build mode.
func buildRuntime(ctx context.Context, root, cache string, mode buildMode) (string, error) {
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
	out := filepath.Join(cache, mode.folder, "runtime-"+key+".so")
	defer lockBuild(out)()
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", fmt.Errorf("making the folder of the runtime: %w", err)
	}
	tmp := out + ".tmp"
	args := append([]string{"-shared", "-fPIC"}, mode.flags...)
	args = append(args,
		"-I", filepath.Join(root, "tree-sitter", "lib", "include"), "-I", src,
		"lib.c", "-o", tmp)
	if err := cc(ctx, src, args...); err != nil {
		return "", err
	}
	return out, os.Rename(tmp, out)
}

// BuildGrammar builds the shared library of the grammar in the folder dir,
// which holds src/grammar.json and the scanner, in the cache, and returns
// its path. It generates parser.c at ABI 15 with the generator of transit,
// and compiles it with the headers of generate/templates and the scanner
// of the grammar. The version of the grammar is 0.0.0. A library of the same
// inputs in the cache is used again.
func BuildGrammar(ctx context.Context, dir, cache string) (string, error) {
	return buildGrammar(ctx, dir, cache, nil, testBuild)
}

// BuildGrammarVersion is BuildGrammar with the version of the nearest
// tree-sitter.json, as transit generate and the golden harness give it. The
// tables of the library are then the tables of the Go package of the
// grammar, with its metadata too.
func BuildGrammarVersion(ctx context.Context, dir, cache string) (string, error) {
	return buildGrammarVersion(ctx, dir, cache, testBuild)
}

// BuildGrammarVersionDefines is BuildGrammarVersion with the C macros
// defines, such as USQL_OPTIONS=1, which the compiler gets as -D flags. The
// macros are part of the name of the library in the cache, so each set of
// macros has a library of its own.
func BuildGrammarVersionDefines(ctx context.Context, dir, cache string, defines ...string) (string, error) {
	mode := testBuild
	mode.defines = defines
	return buildGrammarVersion(ctx, dir, cache, mode)
}

// BuildGrammarVersionO2 is BuildGrammarVersion for the benchmarks. It
// compiles with -O2, in a folder of the cache of its own (D92).
func BuildGrammarVersionO2(ctx context.Context, dir, cache string) (string, error) {
	return buildGrammarVersion(ctx, dir, cache, o2Build)
}

// buildGrammarVersion is BuildGrammarVersion with the build mode.
func buildGrammarVersion(ctx context.Context, dir, cache string, mode buildMode) (string, error) {
	version, err := generate.ReadGrammarVersion(dir)
	if err != nil {
		return "", fmt.Errorf("reading the version of the grammar in %s: %w", dir, err)
	}
	if version == nil {
		version = &generate.SemanticVersion{}
	}
	return buildGrammar(ctx, dir, cache, version, mode)
}

// buildGrammar is BuildGrammar with a version, or nil for none, and the
// build mode.
func buildGrammar(ctx context.Context, dir, cache string, version *generate.SemanticVersion, mode buildMode) (string, error) {
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
	name := filepath.Base(dir) + "-" + key
	if version != nil {
		name += fmt.Sprintf("-v%d.%d.%d", version.Major, version.Minor, version.Patch)
	}
	if len(mode.defines) > 0 {
		name += "-D" + strings.Join(mode.defines, "-D")
	}
	build := filepath.Join(cache, mode.folder, name)
	out := filepath.Join(build, "grammar.so")
	defer lockBuild(out)()
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}

	b, err := os.ReadFile(grammarJSON)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", grammarJSON, err)
	}
	if err := buildLibrary(ctx, b, version, build, src, scanners, out, mode); err != nil {
		return "", fmt.Errorf("building the grammar in %s: %w", dir, err)
	}
	return out, nil
}

// BuildGrammarJSON builds the shared library of a grammar from the text of
// its grammar.json, in the cache, and returns its path and the name of the
// grammar. It is generate_parser and get_test_language of the upstream
// tests, so it generates parser.c with the version 0.0.0, as they do.
// scannerDir is the folder of the scanner.c of the grammar, or "" for a
// grammar with no scanner. A library of the same inputs in the cache is used
// again.
func BuildGrammarJSON(ctx context.Context, grammarJSON []byte, scannerDir, cache string) (string, string, error) {
	var head struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(grammarJSON, &head); err != nil {
		return "", "", fmt.Errorf("reading the name of the grammar: %w", err)
	}
	var scanners []string
	if scannerDir != "" {
		if _, err := os.Stat(filepath.Join(scannerDir, "scanner.c")); err == nil {
			scanners = []string{filepath.Join(scannerDir, "scanner.c")}
		}
	}
	h := sha256.New()
	// a write to a hash never fails
	_, _ = h.Write(grammarJSON)
	key, err := hashFiles(scanners...)
	if err != nil {
		return "", "", err
	}
	_, _ = h.Write([]byte(key))
	build := filepath.Join(cache, testBuild.folder, "json-"+head.Name+"-"+hex.EncodeToString(h.Sum(nil))[:16])
	out := filepath.Join(build, "grammar.so")
	defer lockBuild(out)()
	if _, err := os.Stat(out); err == nil {
		return out, head.Name, nil
	}
	if err := buildLibrary(ctx, grammarJSON, &generate.SemanticVersion{}, build, scannerDir, scanners, out, testBuild); err != nil {
		return "", "", fmt.Errorf("building the grammar %s: %w", head.Name, err)
	}
	return out, head.Name, nil
}

// buildLibrary generates parser.c from a grammar.json at ABI 15 with the
// generator of transit, in the folder build, and compiles it to out with the
// headers of generate/templates and the scanners, with the flags of mode.
// src is the folder that the scanners include from, or "".
func buildLibrary(ctx context.Context, grammarJSON []byte, version *generate.SemanticVersion, build, src string, scanners []string, out string, mode buildMode) error {
	var diagnostics []generate.Diagnostic
	_, code, err := generate.ParserForGrammar(grammarJSON, version, generate.OptLevelMergeStates, c.Backend{}, &diagnostics)
	if err != nil {
		return fmt.Errorf("generating the parser: %w", err)
	}
	headers := filepath.Join(build, "tree_sitter")
	if err := os.MkdirAll(headers, 0o755); err != nil {
		return fmt.Errorf("making the build folder: %w", err)
	}
	for name, text := range map[string]string{
		"parser.h": generate.ParserHeader,
		"alloc.h":  generate.AllocHeader,
		"array.h":  generate.ArrayHeader,
	} {
		if err := os.WriteFile(filepath.Join(headers, name), []byte(text), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(build, "parser.c"), []byte(code), 0o644); err != nil {
		return fmt.Errorf("writing the parser: %w", err)
	}
	// The headers of the build folder come first, so that the scanner gets
	// the headers of the same version as parser.c.
	args := append([]string{"-shared", "-fPIC"}, mode.flags...)
	for _, d := range mode.defines {
		args = append(args, "-D"+d)
	}
	args = append(args, "-I", build)
	if src != "" {
		args = append(args, "-I", src)
	}
	args = append(args, filepath.Join(build, "parser.c"))
	args = append(args, scanners...)
	tmp := out + ".tmp"
	args = append(args, "-o", tmp)
	if err := cc(ctx, build, args...); err != nil {
		return err
	}
	if err := os.Rename(tmp, out); err != nil {
		return fmt.Errorf("renaming the library: %w", err)
	}
	return nil
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
