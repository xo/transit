# D133. The ClickHouse grammar reads the costly forms of part 3

Status: Decided, amends D132.

The third part of the ClickHouse grammar of D127 left out four forms
because of their cost in parse states.

Ken decided on 2026-11-02 that the ClickHouse grammar reads them, although
each one costs parse states:

1. The statement `PARALLEL WITH`, at about 37 large states and 60 KB.
2. `AT TIME ZONE`, at about 112 states, 40 of them large.
3. `COMMENT` with its string as one token, at about 910 states, so that an
   alias with no `AS` can be `comment`.
4. `DESCRIBE` of a query with no parentheses, at more than 400 large states.
