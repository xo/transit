# D51. A grammar is pinned by its commit

Status: Decided.

Ken decided on 2026-09-29, answering question 50:

1. The golden harness fetches the commit in `grammars/grammars.json`, and not
   the tag. A tag that moves changes nothing.
2. If a repository disappears, its entry and its hashes stay, marked
   unavailable, and the grammar leaves the count of the gate (D9). Ken decides
   whether another grammar replaces it.
3. A grammar that commits `src/grammar.json` is generated from that file. The
   harness runs `grammar.js` only for a grammar that commits no
   `grammar.json`, and then it installs the npm packages that `grammar.js`
   requires, at the versions in its `package.json`.
