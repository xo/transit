SELECT CAST(a AS VARCHAR(10)), a COLLATE NOCASE, NULL, CURRENT_TIMESTAMP, X'0A', ?1, :p FROM t;
--     ^ function.builtin
--               ^ type.builtin
--                       ^ number
--                               ^ keyword
--                                       ^ variable.builtin
--                                               ^ constant.builtin
--                                                     ^ function.builtin
--                                                                        ^ string.special
--                                                                               ^ variable.parameter
--                                                                                   ^ variable.parameter
SELECT CASE WHEN a IS NOT NULL THEN 1 ELSE 0 END, b NOT LIKE 'x' ESCAPE '\', c ->> '$.d' FROM t;
--     ^ keyword.conditional
--          ^ keyword.conditional
--                 ^ keyword
--                    ^ keyword
--                             ^ keyword.conditional
--                                    ^ keyword.conditional
--                                           ^ keyword.conditional
--                                                      ^ keyword
--                                                               ^ keyword
--                                                                             ^ operator
SELECT RAISE(ABORT, 'no'), EXISTS (SELECT 1), a BETWEEN 1 AND 2 -- the end
--     ^ keyword.exception
--           ^ keyword
--                         ^ keyword
--                                              ^ keyword
--                                                              ^ comment
