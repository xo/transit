# D49. Local modules are wired with generated replace blocks

Status: Decided, amended by D65.

Ken decided on 2026-09-29, answering question 48, that each module in this
repository that imports another module of this repository, such as the test
module, the chromastyles module and the grammar modules, has a `replace`
block in its `go.mod` that points at the local folder of that module.

A script writes these blocks, as `gen.sh -m` does in resvg, and CI makes sure
that the script changes nothing. `go.work` is not committed.
