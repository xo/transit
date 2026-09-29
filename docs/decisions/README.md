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
| [D13](D013-usql-input-is-parsed-by-a-usql-grammar-that-embeds-sql.md) | usql input is parsed by a usql grammar that embeds SQL | Decided |
| [D14](D014-chroma-is-used-only-for-its-styles.md) | chroma is used only for its styles | Decided, amended by D32 |
| [D15](D015-no-go-package-is-added-without-ken.md) | No Go package is added without Ken | Decided, amended by D32 |
| [D16](D016-the-grammar-set-covers-the-c-template-and-the-runtime.md) | The grammar set covers the C template and the runtime, and prefers upstream grammars | Decided |
| [D17](D017-transit-starts-from-grammar-json.md) | transit starts from grammar.json, and the upstream tool makes it | Decided |
| [D18](D018-the-grammar-set-is-the-three-tiers-and-the-sql-grammars.md) | The grammar set is the three tiers and the SQL grammars | Decided |
| [D19](D019-golden-files-are-made-at-abi-14-and-abi-15.md) | Golden files are made at ABI 14 and ABI 15 | Decided |
| [D20](D020-a-grammar-with-no-license-is-accepted.md) | A freely available grammar with no license is accepted | Decided |
| [D21](D021-transit-will-support-the-languages-of-every-dbmeta-dialect.md) | transit will support the languages of every dbmeta dialect | Decided |
| [D22](D022-transit-does-not-generate-abi-13.md) | transit does not generate ABI 13, and the runtime keeps upstream's ABI 13 branch | Decided |
| [D23](D023-grammars-for-dbmeta-languages-join-the-set-now.md) | The grammars that exist for dbmeta's languages join the set now | Decided |
| [D24](D024-the-port-and-the-generated-code-are-idiomatic-go.md) | The port and the generated code are idiomatic Go | Decided, amends D1 |
| [D25](D025-the-go-api-follows-the-rust-binding-in-go-idioms.md) | The Go API follows the Rust binding, in Go idioms | Decided |
| [D26](D026-the-packages-and-modules-of-transit.md) | The packages and modules of transit | Decided, amended by D48 |
| [D27](D027-predicates-and-injections-are-ported.md) | Query predicates and injections are ported | Decided, amends D7 |
| [D28](D028-transit-can-add-an-api-that-upstream-lacks.md) | transit can add an API that upstream does not have | Decided |
| [D29](D029-the-subtree-form-is-chosen-by-a-benchmark.md) | The Go form of a subtree is chosen by a benchmark | Decided |
| [D30](D030-the-base-commit-and-the-upstream-ledger.md) | The base commit, and how the gap to upstream stays small | Decided |
| [D31](D031-the-go-backend-output-is-chosen-by-measurement.md) | The form of the Go tables is chosen by measurement, and grammar packages embed their queries | Decided, amended by D47 |
| [D32](D032-a-chroma-module-maps-captures-to-token-types.md) | A chroma module maps capture names to chroma token types | Decided, amends D14 and D15, amended by D48 |
| [D33](D033-the-license-names-2026-and-upstream.md) | The license names 2026, and keeps the upstream notice | Decided |
| [D34](D034-gotreesitter-is-read-and-not-copied.md) | gotreesitter can be read, and no code is copied from it | Decided |
| [D35](D035-every-upstream-test-of-the-ported-parts-is-ported.md) | Every upstream test of the ported parts is ported | Decided |
| [D36](D036-ci-runs-in-tiers-on-linux-amd64.md) | CI comes with the first package, runs in tiers, and runs on linux/amd64 | Decided |
| [D37](D037-the-speed-targets.md) | The speed targets are proposed, and phase 3 confirms them | Decided, amended by D47 |
| [D38](D038-unicode-tables-are-an-input-of-the-generator.md) | The generator uses Go's Unicode tables, and takes them as an input | Decided. Amended by D60 |
| [D39](D039-scanner-character-functions-use-go-unicode.md) | Scanner character functions use Go's unicode package | Decided, amended by D46 |
| [D40](D040-golden-files-are-hashes.md) | Golden files are hashes, and the test grammars keep their files | Decided |
| [D41](D041-cmd-transit-generates-tests-parses-and-queries.md) | cmd/transit generates, tests, parses and queries | Decided, amends D7 |
| [D42](D042-xo-grammars-live-in-transit.md) | Grammars that xo writes live in transit | Decided |
| [D43](D043-modules-are-tagged-one-by-one.md) | Each module is tagged on its own | Decided, amended by D48 |
| [D44](D044-input-that-transit-cannot-trust.md) | Input that transit cannot trust | Decided |
| [D45](D045-go-grammars-are-compared-with-c-at-unicode-16.md) | Go grammars are compared with C grammars at the Unicode version of upstream | Decided. Amended by D60 |
| [D46](D046-scanner-character-functions-can-be-swapped-in-tests.md) | Scanner character functions can be swapped in tests | Decided, amends D39 |
| [D47](D047-phase-3-measures-with-a-prototype-go-lexer.md) | Phase 3 measures with a prototype of the Go output | Decided, amends D31 and D37 |
| [D48](D048-the-chroma-module-is-chromastyles.md) | The chroma module is chromastyles | Decided, amends D26, D32 and D43 |
| [D49](D049-local-modules-are-wired-with-generated-replace-blocks.md) | Local modules are wired with generated replace blocks | Decided |
| [D50](D050-one-unit-of-work-per-change.md) | One unit of work per change | Decided |
| [D51](D051-a-grammar-is-pinned-by-its-commit.md) | A grammar is pinned by its commit | Decided |
| [D52](D052-the-concurrency-guarantees-follow-upstream.md) | The concurrency guarantees follow upstream, in Go terms | Decided |
| [D53](D053-examples-and-two-sample-programs.md) | Examples, and two sample programs | Decided |
| [D54](D054-the-plan-is-ready-and-phase-1-starts.md) | The plan is ready, and phase 1 starts | Decided |
| [D55](D055-transit-adds-the-lua-match-predicate.md) | transit adds the #lua-match? predicate | Decided |
| [D56](D056-a-node-lookup-returns-a-bool.md) | A node lookup returns the node and a bool | Decided |
| [D57](D057-statesat-is-the-first-api-that-upstream-lacks.md) | StatesAt is the first API that upstream does not have | Decided |
| [D58](D058-the-golden-harness-is-a-command-of-the-test-module.md) | The golden harness is a command of the test module | Decided |
| [D59](D059-the-generator-ports-regex-syntax.md) | The generator ports the parser and the translator of regex-syntax | Decided |
| [D60](D060-the-generator-uses-the-unicode-tables-of-regex-syntax.md) | The generator uses the Unicode tables of regex-syntax | Decided. Amends D38 and D45 |
