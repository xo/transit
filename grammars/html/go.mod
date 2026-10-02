module github.com/xo/transit/grammars/html

go 1.27.1

require (
	github.com/xo/transit v0.2.0
	github.com/xo/transit/grammars/javascript v0.2.0
)

replace github.com/xo/transit => ../..

replace github.com/xo/transit/grammars/javascript => ../javascript
