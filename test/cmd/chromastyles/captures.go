package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// record is the part of grammars/grammars.json that the command reads.
type record struct {
	Grammars []struct {
		Name       string `json:"name"`
		Repository string `json:"repository"`
		Commit     string `json:"commit"`
	} `json:"grammars"`
}

// grammarFolders returns the folder in the cache of the golden harness of
// each repository that grammars/grammars.json names. It makes sure that each
// folder is at the commit that the record names.
func grammarFolders(ctx context.Context, root, cache string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(root, "grammars", "grammars.json"))
	if err != nil {
		return nil, fmt.Errorf("reading the record of the grammars: %w", err)
	}
	var rec record
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, fmt.Errorf("decoding grammars/grammars.json: %w", err)
	}
	commits := map[string]string{}
	for _, g := range rec.Grammars {
		dir := filepath.Join(cache, cacheName(g.Repository))
		if c, ok := commits[dir]; ok && c != g.Commit {
			return nil, fmt.Errorf("grammars/grammars.json names %s at two commits, %s and %s", g.Repository, c, g.Commit)
		}
		commits[dir] = g.Commit
	}
	dirs := make([]string, 0, len(commits))
	for dir := range commits {
		dirs = append(dirs, dir)
	}
	slices.Sort(dirs)
	for _, dir := range dirs {
		out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD").Output()
		if err != nil {
			return nil, fmt.Errorf("reading the commit of %s: %w. Run the golden harness to fetch the grammars", dir, err)
		}
		if got := strings.TrimSpace(string(out)); got != commits[dir] {
			return nil, fmt.Errorf("%s is at the commit %s, not at the commit %s of grammars/grammars.json. Run the golden harness to fetch the grammars", dir, got, commits[dir])
		}
	}
	return dirs, nil
}

// cacheName is the name of the folder of a repository in the cache, as the
// golden harness names it: the owner and the name of the repository.
func cacheName(repo string) string {
	return filepath.Join(filepath.Base(filepath.Dir(repo)), filepath.Base(repo))
}

// highlightFiles returns each highlight query under the folders: each file
// whose name starts with "highlights" and ends in ".scm". It skips the
// folders node_modules and the folders whose name starts with a dot.
func highlightFiles(dirs []string) ([]string, error) {
	var files []string
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if path != dir && (name == "node_modules" || strings.HasPrefix(name, ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(name, "highlights") && strings.HasSuffix(name, ".scm") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("finding the highlight queries: %w", err)
		}
	}
	return files, nil
}

// captureNames returns the capture names of the queries, sorted and without
// repeats. It leaves out a name that starts with "_", which a query uses only
// in a predicate, by the convention of Neovim.
func captureNames(files []string) ([]string, error) {
	seen := map[string]bool{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading a highlight query: %w", err)
		}
		for _, name := range scanCaptures(string(b)) {
			if !strings.HasPrefix(name, "_") {
				seen[name] = true
			}
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	slices.Sort(names)
	return names, nil
}

// scanCaptures returns each capture name of a query, in order. It skips the
// strings and the comments of the query. A name has the characters of an
// identifier of the query parser of upstream: stream_is_ident_start and
// stream_scan_identifier of lib/src/query.c.
func scanCaptures(src string) []string {
	var names []string
	rs := []rune(src)
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '"':
			for i++; i < len(rs) && rs[i] != '"'; i++ {
				if rs[i] == '\\' {
					i++
				}
			}
		case ';':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
		case '@':
			if i+1 >= len(rs) || !isIdentStart(rs[i+1]) {
				continue
			}
			j := i + 2
			for j < len(rs) && (isIdentStart(rs[j]) || rs[j] == '.') {
				j++
			}
			names = append(names, string(rs[i+1:j]))
			i = j - 1
		}
	}
	return names
}

// isIdentStart is stream_is_ident_start.
func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}
