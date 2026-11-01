package cgrammar

import "github.com/xo/transit/grammars/duckdb"

func init() {
	goPackages = append(goPackages, goPackage{"duckdb", duckdb.Language})
}
