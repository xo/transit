declare n int default 0;
-- <- keyword
--      ^ variable
--        ^ type.builtin
--            ^ keyword
--                    ^ number
begin
-- <- keyword
  if n > 1 then n := n - 1; elseif n = 0 then return 'zero'; else raise e; end if;
--^ keyword.conditional
--         ^ keyword.conditional
--                ^ operator
--                          ^ keyword.conditional
--                                            ^ keyword.return
--                          ^ keyword.conditional
--                                                                ^ keyword.exception
--                                                                         ^ keyword.conditional
--                                                                             ^ keyword.conditional
  outer: while n > 0 do leave outer; end while outer;
--^ label
--       ^ keyword.repeat
--                   ^ keyword
--                      ^ keyword.repeat
--                            ^ label
--                                             ^ label
  for r in (select 1) do continue; end for;
--^ keyword
--      ^ keyword
--                       ^ keyword.repeat
exception when other then return 1;
-- <- keyword.exception
--        ^ keyword.conditional
end;
-- <- keyword.conditional
from t |> where a > 1 |> extend a div 2 as b;
-- <- keyword
--   ^ type
--     ^ operator
--        ^ keyword
--                       ^ keyword
--                                ^ keyword
--                                         ^ variable
select * from emp start with boss is null connect by prior id = boss;
--                ^ keyword
--                                        ^ keyword
--                                                   ^ keyword
select * from events@v2 where a.id = b.id(+) for update;
--                  ^ string.special
--                                       ^ operator
--                                           ^ keyword
--                                               ^ keyword
select f(date "2024-01-01"), a "Alias" from t;
--       ^ type.builtin
--            ^ string
--                             ^ variable
