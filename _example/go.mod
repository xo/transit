module github.com/xo/transit/_example

go 1.27.1

require (
	github.com/xo/transit v0.2.1
	github.com/xo/transit/grammars/cql v0.2.1
	github.com/xo/transit/grammars/mysql v0.2.1
	github.com/xo/transit/grammars/oracle v0.2.1
	github.com/xo/transit/grammars/postgres v0.2.1
	github.com/xo/transit/grammars/sql v0.2.1
	github.com/xo/transit/grammars/sqlserver v0.2.1
	github.com/xo/transit/grammars/usql v0.2.1
	github.com/xo/transit/styles v0.2.1
)

replace github.com/xo/transit => ..

replace github.com/xo/transit/grammars/cql => ../grammars/cql

replace github.com/xo/transit/grammars/mysql => ../grammars/mysql

replace github.com/xo/transit/grammars/oracle => ../grammars/oracle

replace github.com/xo/transit/grammars/postgres => ../grammars/postgres

replace github.com/xo/transit/grammars/sql => ../grammars/sql

replace github.com/xo/transit/grammars/sqlserver => ../grammars/sqlserver

replace github.com/xo/transit/grammars/usql => ../grammars/usql

replace github.com/xo/transit/styles => ../styles
