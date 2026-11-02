package cgrammar

import "github.com/xo/transit/grammars/trino"

func init() {
	goPackages = append(goPackages, goPackage{"trino", trino.Language})
}
