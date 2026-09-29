#!/bin/sh
# gen.sh writes the files of the repository that a script makes.
#
#   ./gen.sh -m    write the replace block of each go.mod (D49)
#
# A module of this repository that requires another module of this
# repository gets a replace directive that points at the folder of that
# module, and a replace directive of a module that it no longer requires is
# dropped. CI runs ./gen.sh -m and fails when it changes a file.
set -eu

root=$(cd "$(dirname "$0")" && pwd)

usage() {
  echo "usage: $0 -m" >&2
  exit 2
}

# modules prints the folder and the path of each module of the repository,
# one module on each line.
modules() {
  find "$root" -name go.mod \
    -not -path "$root/tree-sitter/*" \
    -not -path "$root/.claude/*" \
    -not -path "$root/.git/*" | sort | while read -r gomod; do
    dir=$(dirname "$gomod")
    path=$(awk '$1 == "module" { print $2; exit }' "$gomod")
    echo "$dir $path"
  done
}

# requires prints the module paths that a go.mod requires.
requires() {
  (cd "$(dirname "$1")" && go mod edit -json) |
    awk '/"Require":/ { r = 1 } r && /"Path":/ { gsub(/[",]/, "", $2); print $2 } r && /^\t\]/ { r = 0 }'
}

# replaces prints the old module paths of the replace directives of a go.mod.
replaces() {
  (cd "$(dirname "$1")" && go mod edit -json) |
    awk '/"Replace":/ { r = 1 } r && /"Old":/ { o = 1 } r && o && /"Path":/ { gsub(/[",]/, "", $2); print $2; o = 0 } r && /^\t\]/ { r = 0 }'
}

replaceBlocks() {
  list=$(modules)
  echo "$list" | while read -r dir path; do
    gomod="$dir/go.mod"
    req=$(requires "$gomod")
    # drop each replace of a local module, then add the ones it needs
    for old in $(replaces "$gomod"); do
      if echo "$list" | awk -v p="$old" '$2 == p { found = 1 } END { exit !found }'; then
        (cd "$dir" && go mod edit -dropreplace="$old")
      fi
    done
    echo "$list" | while read -r otherDir otherPath; do
      if [ "$otherPath" = "$path" ]; then
        continue
      fi
      if echo "$req" | grep -qx "$otherPath"; then
        rel=$(realpath --relative-to="$dir" "$otherDir")
        case $rel in
          .*) ;;
          *) rel="./$rel" ;;
        esac
        (cd "$dir" && go mod edit -replace="$otherPath=$rel")
      fi
    done
  done
}

[ $# -eq 1 ] || usage
case $1 in
  -m) replaceBlocks ;;
  *) usage ;;
esac
