#!/usr/bin/env bash
#
# build.sh builds and runs the working C example of transit (D10).
#
# It needs the upstream checkout in tree-sitter/ at the base commit (D4, D30),
# cargo, node, git and a C compiler. It builds the upstream tool, generates the
# SQL grammar of DerekStride/tree-sitter-sql with it, compiles example.c with
# the upstream runtime, and runs it.
#
# Downloads and build output go to the cache folder of transit, outside the
# repository: $XDG_CACHE_HOME/transit, or $HOME/.cache/transit.
#
# Usage, from any folder:
#
#   _samples/example/build.sh

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
TS=$ROOT/tree-sitter
CACHE=${XDG_CACHE_HOME:-$HOME/.cache}/transit
BUILD=$CACHE/example

# the base commit (D30), and the grammar at its release tag
BASE=dcdc8cc55e5dfedfc858080835f153999a29ec40
GRAMMAR_REPO=https://github.com/DerekStride/tree-sitter-sql.git
GRAMMAR_TAG=v0.3.11
GRAMMAR=$CACHE/grammars/DerekStride/tree-sitter-sql

# the upstream checkout must be at the base commit
if [ "$(git -C "$TS" rev-parse HEAD)" != "$BASE" ]; then
  echo "tree-sitter/ is not at the base commit $BASE" >&2
  exit 1
fi

# build the upstream tool at the base commit
CLI=$TS/target/release/tree-sitter
if [ ! -x "$CLI" ]; then
  (cd "$TS" && cargo build --release -p tree-sitter-cli)
fi

# fetch the grammar at its tag
if [ ! -d "$GRAMMAR/.git" ]; then
  mkdir -p "$(dirname "$GRAMMAR")"
  git clone -q "$GRAMMAR_REPO" "$GRAMMAR"
fi
git -C "$GRAMMAR" fetch -q --tags origin
git -C "$GRAMMAR" checkout -q "$GRAMMAR_TAG"

# generate the grammar at ABI 15. It commits no src/grammar.json, so the tool
# runs grammar.js, which imports only its own files (D17, D51).
(cd "$GRAMMAR" && "$CLI" generate --abi 15)

# compile the upstream runtime with the flags of its own Makefile, then the
# grammar and the example
mkdir -p "$BUILD"
cc -O2 -std=c11 -D_POSIX_C_SOURCE=200112L -D_DEFAULT_SOURCE -D_BSD_SOURCE -D_DARWIN_C_SOURCE \
  -I "$TS/lib/include" -I "$TS/lib/src" -c "$TS/lib/src/lib.c" -o "$BUILD/lib.o"
cc -O2 -std=c11 -I "$GRAMMAR/src" -c "$GRAMMAR/src/parser.c" -o "$BUILD/parser.o"
cc -O2 -std=c11 -I "$GRAMMAR/src" -c "$GRAMMAR/src/scanner.c" -o "$BUILD/scanner.o"
cc -O2 -std=c11 -Wall -Wextra -I "$TS/lib/include" \
  -c "$ROOT/_samples/example/example.c" -o "$BUILD/example.o"
cc -o "$BUILD/example" "$BUILD/example.o" "$BUILD/parser.o" "$BUILD/scanner.o" "$BUILD/lib.o"

echo "tree-sitter $BASE, $("$CLI" --version)"
echo "grammar $GRAMMAR_REPO $GRAMMAR_TAG $(git -C "$GRAMMAR" rev-parse HEAD)"
"$BUILD/example" "$GRAMMAR/queries/highlights.scm"
