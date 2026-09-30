package cgrammar

import "github.com/xo/transit/grammars/java"

func init() {
	goPackages = append(goPackages, goPackage{"java", java.Language})
}
