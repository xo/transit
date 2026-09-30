package cgrammar

import golang "github.com/xo/transit/grammars/go"

func init() {
	goPackages = append(goPackages, goPackage{"go", golang.Language})
}
