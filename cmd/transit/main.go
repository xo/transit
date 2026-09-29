// Command transit is the command of transit (D41). It generates a parser
// from the grammar.json of a grammar:
//
//	transit generate [flags] [grammar path]
//
// The subcommands test, parse and query of D41 are not written yet.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/xo/transit/generate"
	"github.com/xo/transit/generate/backend/c"
)

// This file ports the parts of crates/cli/src/main.rs that run the
// generator: main, and the subcommand generate with Generate::run. It leaves
// out the flags that need a part of upstream that transit does not port: the
// JavaScript runtime (D17), the log, --report-states-for-rule, the JSON
// summary, and the deprecated --build. It adds --backend, because the
// generator has backends (D8).

// defaultGenerateABIVersion is the ABI version that generate writes when the
// flag --abi is not given.
//
// defaultGenerateABIVersion is DEFAULT_GENERATE_ABI_VERSION.
const defaultGenerateABIVersion = 15

// main runs the command, and exits with 1 on an error.
//
// main is main.
func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run runs the command with its arguments, writes the warnings and the
// error to stderr, and returns the exit code.
func run(args []string, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "generate" {
		eprintf(stderr, "usage: transit generate [flags] [grammar path]\n")
		return 2
	}
	if err := runGenerate(args[1:], stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		if !errors.Is(err, errUsage) {
			eprintf(stderr, "Error: %s\n", err)
		}
		return 1
	}
	return 0
}

// errUsage is the error of a bad flag, which the flag package reports
// itself.
var errUsage = errors.New("usage")

// generateOptions is the flags of the subcommand generate.
//
// generateOptions is Generate.
type generateOptions struct {
	grammarPath          string
	abiVersion           string
	noParser             bool
	output               string
	disableOptimizations bool
	backend              string
}

// runGenerate runs the subcommand generate.
//
// runGenerate is Generate::run.
func runGenerate(args []string, stderr io.Writer) error {
	opts, err := parseGenerateFlags(args, stderr)
	if err != nil {
		return err
	}
	abiVersion := defaultGenerateABIVersion
	switch {
	case opts.abiVersion == "latest":
		abiVersion = generate.LanguageVersion
	case opts.abiVersion != "":
		if abiVersion, err = strconv.Atoi(opts.abiVersion); err != nil {
			return errors.New("invalid abi version flag")
		}
	}
	var backend generate.Backend
	switch opts.backend {
	case "c":
		backend = c.Backend{}
	default:
		return fmt.Errorf("the backend %q does not exist. The backends are: c", opts.backend)
	}
	optimizations := generate.OptLevelMergeStates
	if opts.disableOptimizations {
		optimizations = 0
	}
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("finding the working folder: %w", err)
	}

	var diagnostics []generate.Diagnostic
	err = generate.ParserInDirectory(currentDir, opts.output, opts.grammarPath, abiVersion, !opts.noParser, optimizations, backend, &diagnostics)
	for _, d := range diagnostics {
		eprintf(stderr, "Warning: %s\n", d)
	}
	if err != nil {
		// upstream keeps only the text of the error, under a context
		return fmt.Errorf("%s", causedBy("Error when generating parser", err.Error()))
	}
	return nil
}

// causedBy writes an error with its context as the Debug of an anyhow error
// does: the context, a blank line, "Caused by:", and the cause with each line
// indented by four spaces.
func causedBy(context, cause string) string {
	var b strings.Builder
	b.WriteString(context + "\n\nCaused by:\n")
	for i, line := range strings.Split(strings.TrimSuffix(cause, "\n"), "\n") {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("    " + line)
	}
	return b.String()
}

// parseGenerateFlags reads the flags and the grammar path of generate. A
// flag can come before or after the path, as clap allows. The environment
// variable TREE_SITTER_ABI_VERSION gives --abi when the flag is not set.
func parseGenerateFlags(args []string, stderr io.Writer) (*generateOptions, error) {
	opts := &generateOptions{abiVersion: os.Getenv("TREE_SITTER_ABI_VERSION")}
	fs := flag.NewFlagSet("transit generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.abiVersion, "abi", opts.abiVersion, fmt.Sprintf("the language ABI version to generate (default %d); latest is %d", defaultGenerateABIVersion, generate.LanguageVersion))
	fs.BoolVar(&opts.noParser, "no-parser", false, "generate only node-types.json")
	fs.StringVar(&opts.output, "output", "", "the folder of the generated source files")
	fs.StringVar(&opts.output, "o", "", "the folder of the generated source files")
	fs.BoolVar(&opts.disableOptimizations, "disable-optimizations", false, "do not merge the parse states")
	fs.StringVar(&opts.backend, "backend", "c", "the backend that writes the parser")
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, err
			}
			return nil, errUsage
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		if opts.grammarPath != "" {
			eprintf(stderr, "transit generate: unexpected argument %q\n", args[0])
			return nil, errUsage
		}
		opts.grammarPath, args = args[0], args[1:]
	}
	return opts, nil
}

// eprintf writes a line to stderr. A line that fails to write is not worth an
// error of its own, as the logger of upstream does not report one either, so
// eprintf drops the error.
func eprintf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
