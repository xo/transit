with r as (select 1) select r.a, count(*) over w from db.s.t as x
-- <- keyword
--   ^ type
--                          ^ type
--                            ^ variable.member
--                               ^ function.call
--                                        ^ keyword
--                                             ^ type
--                                                    ^ module
--                                                       ^ module
--                                                         ^ type
--                                                              ^ variable
  left join u on u.id = x.id qualify row_number() over (partition by a) = 1;
--^ keyword
--     ^ keyword
--          ^ type
--            ^ keyword
--                 ^ variable.member
--                           ^ keyword
--                                   ^ function.call
--                                                      ^ keyword
select v:name::string, cast(a as int), $1, @p, interval 1 day from @stage;
--     ^ variable.member
--       ^ variable.member
--             ^ type.builtin
--                     ^ function.builtin
--                               ^ type.builtin
--                                     ^ variable.parameter
--                                         ^ variable.parameter
--                                             ^ keyword
--                                                        ^ keyword
--                                                                 ^ string.special.path
select case when a then null else current_date end from t inner join u using (k);
--     ^ keyword.conditional
--          ^ keyword.conditional
--                      ^ constant.builtin
--                           ^ keyword.conditional
--                                ^ function.builtin
--                                             ^ keyword.conditional
--                                                        ^ keyword
--                                                                            ^ variable.member
select * from a union all select * from b; -- a comment
--              ^ keyword
--                    ^ keyword
--                                         ^ comment
