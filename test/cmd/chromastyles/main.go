// Command chromastyles writes the styles of chroma as style files of the
// module github.com/xo/transit/styles, and the list of capture names that
// the styles key on (D65).
//
// It measures the capture names first. It reads each highlight query of the
// grammars of grammars/grammars.json in the cache of the golden harness, and
// of the grammar modules under grammars/, and it writes the names to
// styles/captures.txt, one name on each line.
//
// Then it reads the XML file of each style of chroma, and it writes one
// JSON file for each style to styles/chroma/. For each capture name, it
// takes the token type of chroma that captureTypes gives for the name, and
// it resolves the entry of that token type with the rules of chroma. It does
// not import chroma. It copies the license of chroma to
// styles/licenses/chroma/COPYING.
//
// Run it from the test module:
//
//	cd test && go run ./cmd/chromastyles
//
// It needs chroma in the Go module cache, where go mod download puts it,
// and the grammars in the cache of the golden harness, where
// go run ./cmd/golden puts them.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// chromaVersion is the version of chroma whose styles the command converts.
const chromaVersion = "v2.27.0"

func main() {
	os.Exit(mainCode())
}

// mainCode runs the command and returns the exit code.
func mainCode() int {
	version := flag.String("version", chromaVersion, "the version of chroma")
	src := flag.String("src", "", "the folder styles of chroma (default: the folder of that version in the Go module cache)")
	cache := flag.String("cache", "", "the folder of the grammars of the golden harness (default: transit/grammars in the cache folder of the user)")
	out := flag.String("out", "", "the folder of the module styles (default: styles of the repository)")
	flag.Parse()
	if err := run(context.Background(), os.Stdout, *src, *cache, *out, *version); err != nil {
		fmt.Fprintln(os.Stderr, "chromastyles:", err)
		return 1
	}
	return 0
}

// run finds the folders that the flags leave empty, measures the capture
// names, converts the styles, and writes its progress to w.
func run(ctx context.Context, w io.Writer, src, cache, out, version string) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	if src == "" {
		if src, err = findSource(ctx, version); err != nil {
			return err
		}
	}
	if cache == "" {
		if cache, err = grammarCache(); err != nil {
			return err
		}
	}
	if out == "" {
		out = filepath.Join(root, "styles")
	}
	captures, err := measure(ctx, root, cache)
	if err != nil {
		return err
	}
	files, err := convert(src, version, captures)
	if err != nil {
		return err
	}
	return write(w, out, files)
}

// measure returns the capture names of the highlight queries of the grammars.
func measure(ctx context.Context, root, cache string) ([]string, error) {
	dirs, err := grammarFolders(ctx, root, cache)
	if err != nil {
		return nil, err
	}
	dirs = append(dirs, filepath.Join(root, "grammars"))
	files, err := highlightFiles(dirs)
	if err != nil {
		return nil, err
	}
	return captureNames(files)
}

// findSource returns the folder styles of chroma at a version, in the Go
// module cache.
func findSource(ctx context.Context, version string) (string, error) {
	b, err := exec.CommandContext(ctx, "go", "env", "GOMODCACHE").Output()
	if err != nil {
		return "", fmt.Errorf("finding the Go module cache: %w", err)
	}
	dir := filepath.Join(strings.TrimSpace(string(b)), "github.com", "alecthomas", "chroma", "v2@"+version, "styles")
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("finding chroma %s: %w. Run: go mod download github.com/alecthomas/chroma/v2@%s", version, err, version)
	}
	return dir, nil
}

// grammarCache returns the folder of the grammars of the golden harness.
func grammarCache() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("finding the cache folder: %w", err)
	}
	return filepath.Join(dir, "transit", "grammars"), nil
}

// write writes the files into out. A path of files is relative to out. It
// removes each JSON file of out/chroma that is not in files.
func write(w io.Writer, out string, files map[string][]byte) error {
	chroma := filepath.Join(out, "chroma")
	if err := os.MkdirAll(chroma, 0o755); err != nil {
		return fmt.Errorf("making the folder of the styles: %w", err)
	}
	entries, err := os.ReadDir(chroma)
	if err != nil {
		return fmt.Errorf("listing the styles: %w", err)
	}
	for _, e := range entries {
		rel := filepath.ToSlash(filepath.Join("chroma", e.Name()))
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || files[rel] != nil {
			continue
		}
		path := filepath.Join(chroma, e.Name())
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("removing a style that chroma no longer has: %w", err)
		}
		_, _ = fmt.Fprintf(w, "removed %s\n", path)
	}
	for _, rel := range sortedKeys(files) {
		path := filepath.Join(out, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("making the folder of %s: %w", rel, err)
		}
		if err := os.WriteFile(path, files[rel], 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", rel, err)
		}
		_, _ = fmt.Fprintf(w, "wrote %s, %d bytes\n", path, len(files[rel]))
	}
	return nil
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
			return "", fmt.Errorf("finding the root of the transit repository from %s: %w", dir, fs.ErrNotExist)
		}
		dir = parent
	}
}
