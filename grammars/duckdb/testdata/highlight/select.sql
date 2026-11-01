FROM tbl t SELECT t.* EXCLUDE (a), COLUMNS('x.*') WHERE b > 1 QUALIFY row_number() OVER w = 1 WINDOW w AS (ORDER BY c);
-- <- keyword
--   ^ type
--       ^ variable
--         ^ keyword
--                    ^ keyword
--                                 ^ function.call
--                                                            ^ keyword
--                                                                                      ^ type
--                                                                                            ^ keyword
SELECT * FROM a ASOF JOIN b USING (k) POSITIONAL JOIN c, read_csv('f.csv', header := true) USING SAMPLE 10%;
--              ^ keyword
--                          ^ keyword
--                                 ^ variable.member
--                                    ^ keyword
--                                                       ^ function.call
--                                                                         ^ variable.parameter
--                                                                                ^ operator
--                                                                                               ^ keyword
WITH RECURSIVE r(n) USING KEY (n) AS (SELECT 1) SELECT n FROM r UNION ALL BY NAME FROM 'data.parquet';
-- <- keyword
--   ^ keyword
--             ^ type
--                        ^ keyword
--                                                              ^ keyword
--                                                                           ^ keyword
--                                                                                     ^ string
PIVOT cities ON year USING sum(population) GROUP BY country ORDER BY ALL LIMIT 10;
-- <- keyword
--    ^ type
--           ^ keyword
--                   ^ keyword
--                         ^ function.call
--                                         ^ keyword
--                                                                   ^ keyword
--                                                                       ^ keyword
UNPIVOT monthly_sales ON jan, feb INTO NAME month VALUE sales;
-- <- keyword
--      ^ type
--                                ^ keyword
--                                     ^ keyword
--                                                ^ keyword
