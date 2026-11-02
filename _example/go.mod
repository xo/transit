module github.com/xo/transit/_example

go 1.27.1

require (
	github.com/xo/transit v0.7.0
	github.com/xo/transit/grammars/clickhouse v0.7.0
	github.com/xo/transit/grammars/cql v0.7.0
	github.com/xo/transit/grammars/duckdb v0.7.0
	github.com/xo/transit/grammars/mysql v0.7.0
	github.com/xo/transit/grammars/oracle v0.7.0
	github.com/xo/transit/grammars/postgres v0.7.0
	github.com/xo/transit/grammars/sql v0.7.0
	github.com/xo/transit/grammars/sqlite3 v0.7.0
	github.com/xo/transit/grammars/sqlserver v0.7.0
	github.com/xo/transit/grammars/trino v0.7.0
	github.com/xo/transit/grammars/usql v0.7.0
	github.com/xo/transit/styles v0.7.0
)

replace github.com/xo/transit => ..

replace github.com/xo/transit/grammars/clickhouse => ../grammars/clickhouse

replace github.com/xo/transit/grammars/cql => ../grammars/cql

replace github.com/xo/transit/grammars/duckdb => ../grammars/duckdb

replace github.com/xo/transit/grammars/mysql => ../grammars/mysql

replace github.com/xo/transit/grammars/oracle => ../grammars/oracle

replace github.com/xo/transit/grammars/postgres => ../grammars/postgres

replace github.com/xo/transit/grammars/sql => ../grammars/sql

replace github.com/xo/transit/grammars/sqlite3 => ../grammars/sqlite3

replace github.com/xo/transit/grammars/sqlserver => ../grammars/sqlserver

replace github.com/xo/transit/grammars/trino => ../grammars/trino

replace github.com/xo/transit/grammars/usql => ../grammars/usql

replace github.com/xo/transit/styles => ../styles
