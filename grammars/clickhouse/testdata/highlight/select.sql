WITH 1 AS one, t AS (SELECT 1) SELECT DISTINCT a, count() AS c FROM db.tbl AS x FINAL SAMPLE 0.1;
-- <- keyword
--        ^ variable
--             ^ type
--                                    ^ keyword
--                                                ^ function.call
--                                                           ^ variable
--                                                                  ^ module
--                                                                     ^ type
--                                                                            ^ variable
--                                                                              ^ keyword
--                                                                                    ^ keyword
SELECT * EXCEPT (a) APPLY(toString), COLUMNS('^x') FROM t ARRAY JOIN arr GLOBAL ANY LEFT JOIN u USING id;
--     ^ punctuation.special
--       ^ keyword
--                  ^ keyword
--                                   ^ function.builtin
--                                                        ^ keyword
--                                                                       ^ keyword
--                                                                              ^ keyword
--                                                                                  ^ keyword
--                                                                                              ^ keyword
--                                                                                                    ^ variable.member
SELECT a FROM t PREWHERE b WHERE c GROUP BY a WITH TOTALS HAVING d QUALIFY e ORDER BY a WITH FILL STEP 1;
--              ^ keyword
--                                 ^ keyword
--                                                 ^ keyword
--                                                                 ^ keyword
--                                                                                           ^ keyword
--                                                                                                ^ keyword
SELECT a FROM t LIMIT 1 BY a LIMIT 10 SETTINGS max_threads = 8 FORMAT JSONEachRow;
--              ^ keyword
--                      ^ keyword
--                                    ^ keyword
--                                             ^ variable.builtin
--                                                         ^ operator
--                                                             ^ keyword
--                                                                    ^ type.builtin
SELECT TOP 5 a FROM numbers(10) AS n UNION ALL SELECT 1 INTO OUTFILE 'out.csv';
--     ^ keyword
--                  ^ function.call
--                                 ^ variable
--                                   ^ keyword
--                                                           ^ keyword
--                                                                   ^ string
EXPLAIN PLAN actions = 1 SELECT a FROM t WINDOW w AS (ORDER BY a);
-- <- keyword
--      ^ keyword
--           ^ variable.builtin
--                                       ^ keyword
--                                              ^ type
