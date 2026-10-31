select 1 as n, 'two' as s from t where a = _v;
-- <- keyword
--     ^ number
--       ^ keyword
--          ^ variable
--             ^ string
--                        ^ keyword
--                             ^ type
--                               ^ keyword
--                                     ^ variable.member
--                                       ^ operator
--                                         ^ variable.member
--                                           ^ punctuation.delimiter
select _name, '_name', "_name", TRUE     from _tbl;
-- <- keyword
--     ^ variable.member
--            ^ string
--                     ^ variable.member
--                              ^ boolean
--                                       ^ keyword
--                                            ^ type
select * from "_tbl" where TRUE      and _id = 1;
--     ^ punctuation.special
--            ^ type
--                         ^ boolean
--                                   ^ keyword
--                                       ^ variable.member
