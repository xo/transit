SELECT e.last_name, COUNT(*) AS n FROM hr.employees e JOIN d ON e.d = d.id WHERE e.x(+) = 1 GROUP BY e.last_name;
-- <- keyword
--     ^ type
--       ^ variable.member
--                  ^ function.call
--                       ^ punctuation.bracket
--                           ^ keyword
--                              ^ variable
--                                ^ keyword
--                                     ^ module
--                                        ^ type
--                                                  ^ variable
--                                                    ^ keyword
--                                                           ^ keyword
--                                                                                  ^ operator
--                                                                                          ^ keyword
--                                                                                                ^ keyword
SELECT DATE '2026-10-31', INTERVAL '1' DAY, CAST(a AS VARCHAR2(10)), NULL, ROWNUM FROM dual;
--     ^ type.builtin
--          ^ string
--                        ^ type.builtin
--                                     ^ type.builtin
--                                          ^ function.call
--                                                    ^ type.builtin
--                                                             ^ number
--                                                                   ^ constant.builtin
--                                                                         ^ variable.builtin
--                                                                                     ^ type
SELECT dbms_lob.getlength(doc), q'[it's]' FROM t; -- a comment
--     ^ module
--              ^ function.call
--                              ^ string
--                                                ^ comment
WITH c AS (SELECT 1 x FROM dual) SELECT x FROM c ORDER BY x DESC NULLS LAST FETCH FIRST 1 ROW ONLY;
-- <- keyword
--   ^ type
--                                                               ^ keyword
--                                                                          ^ keyword
--                                                                                            ^ keyword
SELECT SUM(s) OVER (PARTITION BY d ORDER BY m ROWS UNBOUNDED PRECEDING) FROM t CONNECT BY PRIOR id = parent;
--     ^ function.call
--            ^ keyword
--                  ^ keyword
--                                                           ^ keyword
--                                                                             ^ keyword
--                                                                                        ^ keyword
