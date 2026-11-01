# D131. The choices of the ClickHouse grammar and of anonymous blocks

Status: Decided, amends D127 and D128.

The first part of the ClickHouse grammar of D127 left three choices open.
The documentation of ClickHouse, in the repository `clickhouse-docs`, has
the license CC BY-NC-SA 4.0, so the agent measured the grammar only on the
stateless tests of ClickHouse, which have the license Apache 2.0. The
grammar did not take a comma before `FROM`, and it reserved the words that
start a clause. The option `AnonymousBlocks` of D128 also left one choice
open.

Ken decided on 2026-11-02:

1. The work on the ClickHouse grammar can use the code examples of the
   documentation of ClickHouse to measure coverage and to write corpus
   cases, as fair use.
2. A corpus case does not copy a stateless test of ClickHouse or an example
   of the documentation. xo writes each case in its own words.
3. The ClickHouse grammar takes a comma after the last item of the list of a
   `SELECT`, before `FROM`, although this costs about 470 parse states.
4. The ClickHouse grammar does not reserve the words that start a clause,
   so a column can have a name such as `from`.
5. Inside an anonymous block of D128, a `BEGIN` that a `;`, `WORK` or
   `TRANSACTION` follows does not open a nested block.
