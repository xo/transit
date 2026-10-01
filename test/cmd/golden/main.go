// Command golden is the golden harness of transit (D58). It runs the upstream
// tree-sitter tool, at the base commit, on the grammars that test the transit
// generator, and it records what the tool writes. The C backend of transit
// must write the same files (D8).
//
// For each of the 68 test grammars of upstream, it keeps grammar.json, and
// parser.c and node-types.json at ABI 15, at ABI 14, and at ABI 15 with the
// merge of parse states off, in generate/testdata (D19, D40). For each real
// grammar, it keeps the SHA-256 of parser.c and node-types.json at ABI 14 and
// ABI 15 in grammars/grammars.json (D40). It runs the tool twice for each file
// and fails when the two runs differ. It reports which paths of render.rs each
// parser.c reaches. At the end of each run, it writes testdata/failing.txt of
// each grammar package under grammars/ from the record (D88).
//
// Run it from the test module:
//
//	cd test && go run ./cmd/golden
//
// It needs the upstream checkout in tree-sitter/ at the base commit (D4, D30),
// cargo, node and git. It keeps what it downloads in the cache folder of
// transit, $XDG_CACHE_HOME/transit.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
)

// baseCommit is the upstream commit that phases 1 to 5 port (D30).
const baseCommit = "dcdc8cc55e5dfedfc858080835f153999a29ec40"

func main() {
	os.Exit(mainCode())
}

// mainCode runs the harness and returns the exit code, so that the deferred
// calls run before the program exits.
func mainCode() int {
	set := flag.String("set", "tests,fixtures", "the grammars to run, from tests, fixtures, candidates and corpus, separated by commas")
	only := flag.String("only", "", "run only the grammars with these names, separated by commas")
	jobs := flag.Int("j", max(1, runtime.NumCPU()/4), "the number of grammars to generate at once")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Stdout, strings.Split(*set, ","), splitNames(*only), *jobs); err != nil {
		fmt.Fprintln(os.Stderr, "golden:", err)
		return 1
	}
	return 0
}

// splitNames splits a list of names that commas separate.
func splitNames(s string) map[string]bool {
	if s == "" {
		return nil
	}
	names := map[string]bool{}
	for n := range strings.SplitSeq(s, ",") {
		names[strings.TrimSpace(n)] = true
	}
	return names
}

// run runs the harness on the sets that it names, and writes its progress and
// its report to w.
func run(ctx context.Context, w io.Writer, sets []string, only map[string]bool, jobs int) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("finding the cache folder: %w", err)
	}
	h := &harness{
		root:  root,
		ts:    filepath.Join(root, "tree-sitter"),
		cache: filepath.Join(cache, "transit"),
		jobs:  jobs,
		only:  only,
		out:   w,
	}
	if err := h.checkUpstream(ctx); err != nil {
		return err
	}
	if err := h.buildTool(ctx); err != nil {
		return err
	}
	var report coverage
	// the report covers the grammars that were made, even when a set fails
	defer report.print(w)
	for _, s := range sets {
		switch strings.TrimSpace(s) {
		case "tests":
			if err := h.testGrammars(ctx, &report); err != nil {
				return err
			}
		case "fixtures":
			if err := h.fixtureGrammars(ctx, &report); err != nil {
				return err
			}
		case "candidates":
			if err := h.candidateGrammars(ctx, &report); err != nil {
				return err
			}
		case "corpus":
			if err := h.corpusGrammars(ctx); err != nil {
				return err
			}
		default:
			return fmt.Errorf("naming the set %q, which is not tests, fixtures, candidates or corpus", s)
		}
	}
	return h.writeFailing()
}

// harness holds what one run of the harness needs.
type harness struct {
	root  string // the root of the transit repository
	ts    string // the upstream checkout
	cache string // the cache folder of transit
	tool  string // the upstream tool
	jobs  int
	only  map[string]bool
	out   io.Writer // where the progress and the report go

	// the versions that the golden files record
	toolVersion string
	rustVersion string
}

// logf writes one line of progress. A progress line that fails to write is
// not worth stopping the run for, so logf drops the error.
func (h *harness) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(h.out, format, args...)
}

// wanted reports whether the run includes the grammar with a name.
func (h *harness) wanted(name string) bool {
	return h.only == nil || h.only[name]
}

// repoRoot returns the root of the transit repository, which holds the test
// module in test/.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("finding the working folder: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "test", "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "docs", "UPSTREAM.md")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("finding the root of the transit repository from %s", dir)
		}
		dir = parent
	}
}
