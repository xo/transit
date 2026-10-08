package corpus

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// This file reads testdata/failing.txt of a grammar package (D88). It ports
// no upstream file.

// FailingPath is the file of a grammar package that names the corpus cases
// that fail upstream (D88).
const FailingPath = "testdata/failing.txt"

// ReadFailing returns the names of testdata/failing.txt in the folder dir,
// or no names when the file does not exist. A file with no name, a blank
// line or no newline at its end is an error.
func ReadFailing(dir string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(dir, FailingPath))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("reading %s: %w", FailingPath, err)
	}
	text, ok := strings.CutSuffix(string(b), "\n")
	if !ok {
		return nil, fmt.Errorf("reading %s: the file does not end with a newline", FailingPath)
	}
	names := strings.Split(text, "\n")
	if slices.Contains(names, "") {
		return nil, fmt.Errorf("reading %s: the file holds a blank line", FailingPath)
	}
	return names, nil
}
