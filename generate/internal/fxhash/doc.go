// Package fxhash gives the generator the hash and the order of iteration of
// the hash sets that upstream tree-sitter uses. Upstream uses the hasher
// FxHasher of the Rust crate rustc-hash 2.1.3, in the hash sets of the Rust
// standard library. In some places the hash or the order of a set reaches the
// output of the generator, so the port must give the same values.
//
// The package ports these parts:
//
//   - The hasher FxHasher, from src/lib.rs of rustc-hash 2.1.3, for a target
//     with 64-bit pointers, in lib.go.
//   - The hash of a slice of u32 values, from library/core/src/hash/mod.rs
//     of rustc 1.97.1, in hash.go.
//   - The hash table of hashbrown 0.17.1, as the standard library of rustc
//     1.97.1 vendors it at library/vendor/hashbrown-0.17.1, on x86_64 with
//     SSE2 groups of 16 control bytes. Only the parts that insert u32 values
//     and iterate over them are ported, in map.go, raw.go, set.go, tag.go,
//     bitmask.go and sse2.go. A Go file has the name of the Rust file that it
//     ports.
//
// The package keeps the MIT licenses of the code that it ports: rustc-hash in
// LICENSE, hashbrown in LICENSE-HASHBROWN, and the Rust standard library in
// LICENSE-RUST.
package fxhash
