# D36. CI comes with the first package, runs in tiers, and runs on linux/amd64

Status: Decided.

Ken decided on 2026-09-29, answering questions 16 and 34:

1. `.github/workflows/test.yml` and `.golangci.yml` come with the first Go
   package. The lint posture is the one of dburl: `default: all`, with each
   disabled linter and its reason.
2. CI runs in three tiers:
   - On each push: the test grammars, the tests of the root package, and
     json, c, python, javascript, the PostgreSQL grammar and one large
     grammar.
   - Each night: every grammar, at both ABI versions, with the corpus of each
     C output.
   - Each week: the long fuzz tests.
3. CI runs on linux/amd64 only, as in dbmeta.
