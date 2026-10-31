#variable_conflict use_column
-- <- keyword.directive
--                 ^ constant
<<outer>>
--^ label
DECLARE
-- <- keyword
quantity CONSTANT integer := 30;
-- <- variable
--       ^ keyword
--                ^ type
--                        ^ operator
row users%ROWTYPE;
--  ^ type
--        ^ keyword
c1 CURSOR (k int) FOR SELECT * FROM t WHERE id = k;
-- ^ keyword
--         ^ variable.parameter
--                    ^ keyword
BEGIN
-- <- keyword
SELECT * INTO STRICT row FROM users WHERE id = 1;
-- <- keyword
--                       ^ keyword
IF quantity > 1 THEN NULL; ELSIF found THEN RETURN; END IF;
-- <- keyword
--              ^ keyword
--                   ^ keyword
--                         ^ keyword
--                                          ^ keyword
--                                                  ^ keyword
<<l>> FOR i IN REVERSE 10..1 LOOP EXIT l WHEN i < 2; END LOOP l;
--^ label
--    ^ keyword
--             ^ keyword
--                       ^ operator
--                           ^ keyword
--                                ^ keyword
--                                     ^ label
--                                                            ^ label
RAISE NOTICE 'q %', quantity USING HINT = 'h';
-- <- keyword
--    ^ keyword
--           ^ string
--                                 ^ property
GET STACKED DIAGNOSTICS n = ROW_COUNT;
-- <- keyword
--                          ^ constant.builtin
OPEN c1(k := 1); FETCH c1 INTO row; CLOSE c1;
-- <- keyword
--      ^ variable.parameter
--               ^ keyword
--                                  ^ keyword
EXCEPTION WHEN division_by_zero THEN RAISE;
-- <- keyword
--             ^ constant
END outer;
--  ^ label
