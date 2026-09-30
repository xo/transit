module github.com/xo/transit/test

go 1.27.1

require (
	github.com/xo/transit v0.0.0-00010101000000-000000000000
	github.com/xo/transit/grammars/json v0.0.0-00010101000000-000000000000
)

replace github.com/xo/transit => ..

replace github.com/xo/transit/grammars/json => ../grammars/json
