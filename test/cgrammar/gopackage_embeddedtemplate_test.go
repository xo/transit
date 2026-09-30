package cgrammar

import "github.com/xo/transit/grammars/embeddedtemplate"

func init() {
	goPackages = append(goPackages, goPackage{"embedded_template", embeddedtemplate.Language})
}
