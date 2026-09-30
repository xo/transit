package cgrammar

import "github.com/xo/transit/grammars/c"

func init() {
	goPackages = append(goPackages, goPackage{"c", c.Language})
}
