package cgrammar

import "github.com/xo/transit/grammars/neo4j"

func init() {
	goPackages = append(goPackages, goPackage{"cypher", neo4j.Language})
}
