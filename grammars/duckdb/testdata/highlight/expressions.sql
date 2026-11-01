SELECT CAST(a AS INTEGER), TRY_CAST(b AS DATE), c::VARCHAR[], DATE '1992-03-22', INTERVAL 3 DAYS;
--     ^ function.builtin
--               ^ type.builtin
--                         ^ function.builtin
--                                       ^ type.builtin
--                                               ^ operator
--                                                 ^ type.builtin
--                                                            ^ type.builtin
--                                                                 ^ string
--                                                                               ^ keyword
--                                                                                        ^ number
--                                                                                          ^ keyword
SELECT [1, 2], {'a': 1, b: 2}, MAP {'k': 1}, l[1:2], s.a, $1, ?, $name, #1;
--     ^ punctuation.bracket
--              ^ string
--                      ^ property
--                             ^ keyword
--                                           ^ variable.member
--                                                        ^ variable.parameter
--                                                            ^ variable.parameter
--                                                               ^ variable.parameter
--                                                                      ^ variable.parameter
SELECT list_transform(l, x -> x + 1), list_filter(l, lambda y: y > 0), 'a' || 'b', s ILIKE 'x%';
--     ^ function.call
--                         ^ operator
--                                                   ^ keyword
--                                                          ^ variable.parameter
--                                                                         ^ operator
--                                                                                   ^ keyword
SELECT ('hello').upper(), count(*) FILTER (WHERE a > 1), sum(x) OVER (PARTITION BY y ORDER BY z), x IS NOT NULL;
--               ^ function.method.call
--                        ^ function.call
--                                 ^ keyword
--                                                              ^ keyword
--                                                                    ^ keyword
--                                                                                                  ^ keyword
--                                                                                                         ^ constant.builtin
SELECT CASE WHEN a THEN 1 ELSE 2 END, e'x\ny', $$text$$, X'CAFE', B'01', 1_000 -- note
--     ^ keyword.conditional
--          ^ keyword.conditional
--                 ^ keyword.conditional
--                        ^ keyword.conditional
--                               ^ keyword.conditional
--                                    ^ string
--                                             ^ string
--                                                       ^ string.special
--                                                                ^ string.special
--                                                                       ^ number
--                                                                             ^ comment
