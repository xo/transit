# D17. transit starts from grammar.json, and the upstream tool makes it

Status: Decided.

Ken decided on 2026-09-29 that transit uses the upstream tool as upstream
does: the tool evaluates `grammar.js` into `src/grammar.json`, and the
generator reads the JSON. The transit generator starts from `grammar.json`
and runs no JavaScript. This answers question 3.

The upstream source shows that this holds. `load_grammar_file` in
`crates/generate/src/generate.rs` runs `grammar.js` through `dsl.js`, or it
reads a `.json` file as it is. `generate_parser_in_directory` then parses the
JSON, and builds `parser.c` and `node-types.json` from it. Gemini and DeepSeek
agree.

## The other inputs

Three inputs besides `grammar.json` change `parser.c`:

1. The ABI version, from `--abi`: 14 or 15, and 15 by default.
2. The version of the grammar, from `metadata.version` in
   `tree-sitter.json`, in the folder of the grammar or a folder above it.
3. The merge of parse states, which `--disable-optimizations` turns off.

The transit generator takes the same three inputs. The golden harness records
all three beside each golden file.

## When a grammar commits no grammar.json

Some grammars commit no `grammar.json`, such as `DerekStride/tree-sitter-sql`.
The golden harness runs the upstream tool on `grammar.js`, and it keeps the
`grammar.json` that the tool writes. The transit generator reads that file.
