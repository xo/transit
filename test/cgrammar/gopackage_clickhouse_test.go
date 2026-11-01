package cgrammar

import "github.com/xo/transit/grammars/clickhouse"

func init() {
	goPackages = append(goPackages, goPackage{"clickhouse", clickhouse.Language})
}
