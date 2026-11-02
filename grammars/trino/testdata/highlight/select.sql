WITH t AS (SELECT 1) SELECT DISTINCT a, count(*) AS c FROM cat.sch.tbl AS x WHERE a > 1;
-- <- keyword
--   ^ type
--                          ^ keyword
--                                      ^ function.call
--                                                  ^ variable
--                                                         ^ module
--                                                             ^ module
--                                                                 ^ type
--                                                                        ^ variable
SELECT * FROM a LEFT JOIN b ON a.id = b.id CROSS JOIN UNNEST(arr) WITH ORDINALITY AS u (v, n);
--     ^ punctuation.special
--              ^ keyword
--                   ^ keyword
--                                                    ^ keyword
--                                                                     ^ keyword
--                                                                                   ^ variable
--                                                                                      ^ variable.member
SELECT a FROM t GROUP BY ROLLUP (a) HAVING count(*) > 1 ORDER BY a NULLS LAST OFFSET 1 LIMIT 10;
--              ^ keyword
--                       ^ keyword
--                                  ^ keyword
--                                                                 ^ keyword
--                                                                            ^ keyword
--                                                                                     ^ keyword
SELECT * FROM TABLE(system.sequence(start => 1, stop => 5)) s TABLESAMPLE BERNOULLI (10);
--            ^ keyword
--                  ^ module
--                         ^ function.call
--                                  ^ variable.parameter
--                                        ^ operator
--                                                          ^ variable
--                                                                        ^ keyword
SELECT * FROM t MATCH_RECOGNIZE (MEASURES A.x AS ax PATTERN (A B+?) DEFINE B AS B.x > PREV(B.x));
--              ^ keyword
--                               ^ keyword
--                                               ^ variable
--                                                  ^ keyword
--                                                           ^ variable
--                                                              ^ operator
--                                                                  ^ keyword
--                                                                                    ^ function.call
WITH FUNCTION twice(x bigint) RETURNS bigint RETURN x * 2 SELECT twice(3);
--   ^ keyword
--            ^ function
--                  ^ variable.parameter
--                            ^ keyword
--                                           ^ keyword.return
--                                                               ^ function.call
WITH SESSION max_hours = 2 * 24 SELECT 1;
--           ^ variable.builtin
--                     ^ operator
--                       ^ number
--                         ^ operator
SELECT * FROM events FOR VERSION AS OF 100 + 5 e;
--                   ^ keyword
--                                     ^ number
--                                         ^ operator
