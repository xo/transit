package cgrammar

import "github.com/xo/transit/grammars/sparql"

func init() {
	goPackages = append(goPackages, goPackage{"sparql", sparql.Language})
}
