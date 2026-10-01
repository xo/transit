# D93. The failure list of a package holds its own cases

Status: Decided, amends D88.

Ken decided on 2026-10-01:

1. The file `testdata/failing.txt` of a package holds only the failing cases
   that the package runs under D83. In a repository with two grammars, a
   case that the other package runs is not in the list.
2. The log line of the corpus test names `testdata/failing.txt` as the source
   of the list.
3. The golden harness writes each `failing.txt` at the end of every run,
   whichever set of grammars it ran, so no file is stale after a run.
