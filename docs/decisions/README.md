# Decisions

Every decision of this project is a file in this folder, named by its number
and its title. This table is the index. Find the number here, then open the
file.

Each file opens with its status. `Decided` means that Ken chose it.
`Proposed` means that an agent suggested it and Ken did not confirm it yet. A
decision that changes an earlier one says so in its status, as `Amends D3`,
and the earlier one says it back, as `Amended by D9`. Read the status before
the decision.

A bare number, such as D3, names a decision in this folder. A decision of
another repository names that repository, such as dbmeta D110.

A new decision gets the next number and a file of its own. Add its row here.
An open question is not a decision. It goes at the end of
[`../PLAN.md`](../PLAN.md) until Ken answers it (dbmeta D111).

| # | Decision | Status |
| --- | --- | --- |
| [D1](D001-transit-is-a-pure-go-port-of-tree-sitter.md) | transit is a pure Go port of tree-sitter | Decided, amended by D24 |
| [D2](D002-transit-is-set-up-for-agents-as-dbmeta-d110-says.md) | transit is set up for coding agents as dbmeta D110 says | Decided |
| [D3](D003-the-plan-and-the-rules-come-before-any-code.md) | The plan and the rules come before any code | Decided |
| [D4](D004-the-upstream-checkout-is-in-the-root-and-git-ignores-it.md) | The upstream checkout is in the root, and git ignores it | Decided |
| [D5](D005-each-decision-is-a-file-of-its-own.md) | Each decision is a file of its own | Decided |
| [D6](D006-transit-serves-rline-and-usql-and-depends-on-neither.md) | transit serves rline and usql, and depends on neither | Decided |
| [D7](D007-transit-ports-the-generator-and-the-runtime.md) | transit ports the generator and the runtime | Decided, amended by D27 and D41 |
| [D8](D008-the-generator-has-pluggable-backends-c-first-then-go.md) | The generator has pluggable backends, C first, then Go | Decided |
| [D9](D009-the-go-backend-waits-for-50-correct-grammars.md) | The Go backend waits for 50 correct grammars | Decided |
| [D10](D010-the-target-go-api-comes-from-a-working-c-example.md) | The target Go API comes from a working C example | Decided |
| [D11](D011-rline-imports-the-transit-runtime.md) | rline imports the transit runtime | Decided |
| [D12](D012-the-runtime-is-ported-beside-the-generator.md) | The runtime is ported beside the generator, and a cgo test module tests it | Decided |
| [D13](D013-usql-input-is-parsed-by-a-usql-grammar-that-embeds-sql.md) | usql input is parsed by a usql grammar that embeds SQL | Decided, amended by D101 |
| [D14](D014-chroma-is-used-only-for-its-styles.md) | chroma is used only for its styles | Decided, amended by D32 and D65 |
| [D15](D015-no-go-package-is-added-without-ken.md) | No Go package is added without Ken | Decided, amended by D32 and D65 |
| [D16](D016-the-grammar-set-covers-the-c-template-and-the-runtime.md) | The grammar set covers the C template and the runtime, and prefers upstream grammars | Decided |
| [D17](D017-transit-starts-from-grammar-json.md) | transit starts from grammar.json, and the upstream tool makes it | Decided |
| [D18](D018-the-grammar-set-is-the-three-tiers-and-the-sql-grammars.md) | The grammar set is the three tiers and the SQL grammars | Decided |
| [D19](D019-golden-files-are-made-at-abi-14-and-abi-15.md) | Golden files are made at ABI 14 and ABI 15 | Decided |
| [D20](D020-a-grammar-with-no-license-is-accepted.md) | A freely available grammar with no license is accepted | Decided |
| [D21](D021-transit-will-support-the-languages-of-every-dbmeta-dialect.md) | transit will support the languages of every dbmeta dialect | Decided |
| [D22](D022-transit-does-not-generate-abi-13.md) | transit does not generate ABI 13, and the runtime keeps upstream's ABI 13 branch | Decided |
| [D23](D023-grammars-for-dbmeta-languages-join-the-set-now.md) | The grammars that exist for dbmeta's languages join the set now | Decided |
| [D24](D024-the-port-and-the-generated-code-are-idiomatic-go.md) | The port and the generated code are idiomatic Go | Decided, amends D1, amended by D64 |
| [D25](D025-the-go-api-follows-the-rust-binding-in-go-idioms.md) | The Go API follows the Rust binding, in Go idioms | Decided |
| [D26](D026-the-packages-and-modules-of-transit.md) | The packages and modules of transit | Decided, amended by D48, D65 and D99 |
| [D27](D027-predicates-and-injections-are-ported.md) | Query predicates and injections are ported | Decided, amends D7 |
| [D28](D028-transit-can-add-an-api-that-upstream-lacks.md) | transit can add an API that upstream does not have | Decided |
| [D29](D029-the-subtree-form-is-chosen-by-a-benchmark.md) | The Go form of a subtree is chosen by a benchmark | Decided |
| [D30](D030-the-base-commit-and-the-upstream-ledger.md) | The base commit, and how the gap to upstream stays small | Decided |
| [D31](D031-the-go-backend-output-is-chosen-by-measurement.md) | The form of the Go tables is chosen by measurement, and grammar packages embed their queries | Decided, amended by D47 |
| [D32](D032-a-chroma-module-maps-captures-to-token-types.md) | A chroma module maps capture names to chroma token types | Decided, amends D14 and D15, amended by D48, superseded by D65 |
| [D33](D033-the-license-names-2026-and-upstream.md) | The license names 2026, and keeps the upstream notice | Decided |
| [D34](D034-gotreesitter-is-read-and-not-copied.md) | gotreesitter can be read, and no code is copied from it | Decided |
| [D35](D035-every-upstream-test-of-the-ported-parts-is-ported.md) | Every upstream test of the ported parts is ported | Decided |
| [D36](D036-ci-runs-in-tiers-on-linux-amd64.md) | CI comes with the first package, runs in tiers, and runs on linux/amd64 | Decided |
| [D37](D037-the-speed-targets.md) | The speed targets are proposed, and phase 3 confirms them | Decided, amended by D47, D91 and D97 |
| [D38](D038-unicode-tables-are-an-input-of-the-generator.md) | The generator uses Go's Unicode tables, and takes them as an input | Decided, amended by D60 |
| [D39](D039-scanner-character-functions-use-go-unicode.md) | Scanner character functions use Go's unicode package | Decided, amended by D46 |
| [D40](D040-golden-files-are-hashes.md) | Golden files are hashes, and the test grammars keep their files | Decided |
| [D41](D041-cmd-transit-generates-tests-parses-and-queries.md) | cmd/transit generates, tests, parses and queries | Decided, amends D7 |
| [D42](D042-xo-grammars-live-in-transit.md) | Grammars that xo writes live in transit | Decided, amended by D104 |
| [D43](D043-modules-are-tagged-one-by-one.md) | Each module is tagged on its own | Decided, amended by D48, D65 and D99 |
| [D44](D044-input-that-transit-cannot-trust.md) | Input that transit cannot trust | Decided |
| [D45](D045-go-grammars-are-compared-with-c-at-unicode-16.md) | Go grammars are compared with C grammars at the Unicode version of upstream | Decided, amended by D60 |
| [D46](D046-scanner-character-functions-can-be-swapped-in-tests.md) | Scanner character functions can be swapped in tests | Decided, amends D39 |
| [D47](D047-phase-3-measures-with-a-prototype-go-lexer.md) | Phase 3 measures with a prototype of the Go output | Decided, amends D31 and D37 |
| [D48](D048-the-chroma-module-is-chromastyles.md) | The chroma module is chromastyles | Decided, amends D26, D32 and D43, superseded by D65 |
| [D49](D049-local-modules-are-wired-with-generated-replace-blocks.md) | Local modules are wired with generated replace blocks | Decided, amended by D65 |
| [D50](D050-one-unit-of-work-per-change.md) | One unit of work per change | Decided, amended by D61 |
| [D51](D051-a-grammar-is-pinned-by-its-commit.md) | A grammar is pinned by its commit | Decided |
| [D52](D052-the-concurrency-guarantees-follow-upstream.md) | The concurrency guarantees follow upstream, in Go terms | Decided |
| [D53](D053-examples-and-two-sample-programs.md) | Examples, and two sample programs | Decided |
| [D54](D054-the-plan-is-ready-and-phase-1-starts.md) | The plan is ready, and phase 1 starts | Decided |
| [D55](D055-transit-adds-the-lua-match-predicate.md) | transit adds the #lua-match? predicate | Decided, amended by D69 |
| [D56](D056-a-node-lookup-returns-a-bool.md) | A node lookup returns the node and a bool | Decided |
| [D57](D057-statesat-is-the-first-api-that-upstream-lacks.md) | StatesAt is the first API that upstream does not have | Decided |
| [D58](D058-the-golden-harness-is-a-command-of-the-test-module.md) | The golden harness is a command of the test module | Decided |
| [D59](D059-the-generator-ports-regex-syntax.md) | The generator ports the parser and the translator of regex-syntax | Decided |
| [D60](D060-the-generator-uses-the-unicode-tables-of-regex-syntax.md) | The generator uses the Unicode tables of regex-syntax | Decided, amends D38 and D45 |
| [D61](D061-a-staged-change-can-hold-several-units.md) | A staged change can hold several units | Decided, amends D50 |
| [D62](D062-a-subtree-is-a-pointer-from-a-chunk.md) | A subtree is a pointer to a node from a chunk | Decided, amended by D64 |
| [D63](D063-a-language-is-built-from-internal-tables.md) | A language is built from tables of an internal type | Decided |
| [D64](D064-a-go-subtree-keeps-the-count-and-the-inline-flag.md) | A Go subtree keeps the reference count and the inline flag of C | Decided, amends D24 and D62 |
| [D65](D065-transit-publishes-its-own-styles.md) | transit publishes its own styles as embedded JSON files | Decided, supersedes D32 and D48, amends D14, D15, D26, D43 and D49, amended by D98 and D99 |
| [D66](D066-a-json-error-keeps-the-text-of-go.md) | An error of grammar.json keeps the text of Go | Decided |
| [D67](D067-the-generator-logs-with-slog.md) | The generator logs with log/slog | Decided |
| [D68](D068-node-equal-is-ts-node-eq.md) | Node.Equal is ts_node_eq, and == also compares the position | Decided |
| [D69](D069-the-neovim-dialect-waits-for-the-tier-1-grammars.md) | The Neovim dialect of queries waits for the tier 1 grammars | Decided, amends D55 |
| [D70](D070-statesat-stops-before-the-first-token-after-the-offset.md) | StatesAt stops before the first token that ends after the offset | Decided |
| [D71](D071-inject-takes-utf-8-and-a-rust-oracle-tests-it.md) | The package inject takes UTF-8, and a Rust oracle tests it | Decided |
| [D72](D072-the-api-of-inject-gives-every-layer-at-once.md) | The API of inject gives every layer at once | Decided |
| [D73](D073-the-measurements-of-the-prototype.md) | The measurements of the prototype of phase 3 | Decided |
| [D74](D074-the-go-backend-writes-literal-tables-and-a-lexer-as-data.md) | The Go backend writes literal tables and a lexer as data | Decided |
| [D75](D075-phase-4-starts-before-the-review-of-api-md.md) | Phase 4 starts before the review of API.md | Decided |
| [D76](D076-nodetype-holds-node-types-json.md) | NodeType holds node-types.json | Decided |
| [D77](D077-the-rules-of-a-generated-grammar-package.md) | The rules of a generated grammar package | Decided, amended by D82 |
| [D78](D078-a-large-character-set-of-surrogates-is-an-error.md) | A large character set of surrogates is an error | Decided |
| [D79](D079-the-tests-of-a-grammar-module.md) | The tests of a grammar module | Decided, amended by D80, D83 and D88 |
| [D80](D080-the-highlight-test-follows-the-upstream-highlighter.md) | The highlight test follows the upstream highlighter | Decided, amends D79, amended by D84 |
| [D81](D081-the-ruby-scanner-checks-the-room-of-its-state.md) | The ruby scanner checks the room of its state | Decided |
| [D82](D082-keywords-leaves-out-hidden-tokens.md) | Keywords leaves out hidden tokens | Decided, amends D77 |
| [D83](D083-a-corpus-case-runs-in-the-package-of-its-grammar.md) | A corpus case runs in the package of its grammar | Decided, amends D79 |
| [D84](D084-a-module-copies-the-queries-of-another-grammar-that-it-lists.md) | A module copies the queries of another grammar that it lists | Decided, amends D80 |
| [D85](D085-the-rules-of-a-scanner-port.md) | The rules of a scanner port | Decided, amended by D96 |
| [D86](D086-the-layout-of-the-fixture-grammar-modules.md) | The layout of the fixture grammar modules | Decided |
| [D87](D087-the-test-module-runs-with-a-timeout-of-an-hour.md) | The test module runs with a timeout of an hour | Decided |
| [D88](D088-a-grammar-module-holds-its-list-of-upstream-failures.md) | A grammar module holds its list of upstream failures | Decided, amends D79, amended by D93 |
| [D89](D089-querymatch-remove-has-a-go-form.md) | QueryMatch.Remove has a Go form | Decided, amended by D94 |
| [D90](D090-the-folder-of-a-package-comes-from-its-path.md) | The folder of a package comes from its path | Decided |
| [D91](D091-tree-close-returns-the-nodes-to-a-free-list.md) | Tree.Close returns the nodes to a free list | Decided, amends D37, amended by D97 |
| [D92](D092-the-benchmarks-of-phase-4.md) | The benchmarks of phase 4 | Decided |
| [D93](D093-the-failure-list-of-a-package-holds-its-own-cases.md) | The failure list of a package holds its own cases | Decided, amends D88 |
| [D94](D094-remove-on-a-zero-match-does-nothing.md) | Remove on a zero match does nothing | Decided, amends D89 |
| [D95](D095-the-tests-of-phase-4-run-on-each-grammar-package.md) | The tests of phase 4 run on each grammar package | Decided |
| [D96](D096-a-scanner-keeps-its-slices-in-deserialize.md) | A scanner keeps its slices in Deserialize | Decided, amends D85 |
| [D97](D097-target-3-counts-the-chunks-of-new-nodes.md) | Target 3 counts the chunks of new nodes | Decided, amends D37 and D91 |
| [D98](D098-the-styles-that-transit-bundles.md) | The styles that transit bundles | Decided, amends D65, amended by D100 |
| [D99](D099-the-package-styles-is-a-module-of-its-own.md) | The package styles is a module of its own | Decided, amends D26, D43 and D65 |
| [D100](D100-three-choices-for-the-hand-made-styles.md) | Three choices for the hand-made styles | Decided, amends D98 |
| [D101](D101-the-usql-grammar-has-one-grammar-for-each-family-of-dialects.md) | The usql grammar has one grammar for each family of dialects | Decided, amends D13, amended by D105 |
| [D102](D102-the-shape-of-the-usql-grammar.md) | The shape of the usql grammar | Decided, amended by D105 |
| [D103](D103-ken-accepts-the-target-api.md) | Ken accepts the target API | Decided |
| [D104](D104-a-grammar-that-xo-writes-lives-in-grammars.md) | A grammar that xo writes lives in grammars | Decided, amends D42 |
| [D105](D105-ken-accepts-the-usql-grammar.md) | Ken accepts the usql grammar | Decided, amends D101 and D102 |
