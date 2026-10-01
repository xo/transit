package cgrammar

import "github.com/xo/transit/grammars/ydb"

func init() {
	goPackages = append(goPackages, goPackage{"yql", ydb.Language})
}
