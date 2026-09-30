# D86. The layout of the fixture grammar modules

Status: Decided.

The fixture grammar modules of phase 4 raised these points of layout. Ken
decided on 2026-09-30:

1. The package `golang` of the grammar go (D77) lives in `grammars/go`, and
   its import path is `github.com/xo/transit/grammars/go`, as the package
   `golang` lives in `generate/backend/go` (D26).
2. In a module with two grammars, each package holds its own copy of
   `queries/` and of `testdata/`, because each package embeds its own
   queries. `LICENSE` and `tree-sitter.json` are in the folder of the module
   only.
3. A package copies the query files that its entry in `tree-sitter.json`
   lists, and no other. So php_only has no `injections-text.scm`.
4. The doc comment of a scanner names the tag and the commit of
   `grammars/grammars.json`. For php, the commit is on the branch
   `upstream_test_fixture`, and the comment names both.
5. `.gitignore` ignores `/tree-sitter`, which matches the checkout of
   upstream and a symbolic link to it in a git worktree.
