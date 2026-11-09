create or replace table t (id int not null, s string comment 'x') cluster by (id);
-- <- keyword
--        ^ keyword
--                      ^ type
--                         ^ variable.member
--                            ^ type.builtin
--                                ^ keyword
--                                    ^ constant.builtin
--                                                   ^ keyword
--                                                                ^ keyword
insert into t (a) values (1);
-- <- keyword
--          ^ type
--             ^ variable.member
--                ^ keyword
merge into t using s on t.k = s.k when matched then update set a = s.a;
-- <- keyword
--                                     ^ keyword
--                                                  ^ keyword
--                                                             ^ variable.member
create function f(x int) returns int language sql as 'x + 1';
--     ^ keyword
--              ^ function
--                ^ variable.parameter
--                       ^ keyword
--                                   ^ keyword
--                                                   ^ string
create stage st url = 's3://b/';
--     ^ keyword
--           ^ type
--              ^ keyword
show tables like 'a%';
-- <- keyword
--   ^ keyword
--          ^ keyword
grant select on table t to role r;
-- <- keyword
--    ^ keyword
--                      ^ keyword
use warehouse w;
-- <- keyword
--  ^ keyword
--            ^ type
