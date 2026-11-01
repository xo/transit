package cgrammar

import "github.com/xo/transit/grammars/sqlite3"

func init() {
	goPackages = append(goPackages, goPackage{"sqlite3", sqlite3.Language})
}
