CREATE OR REPLACE PROCEDURE hr.p (a IN NUMBER, b OUT t.c%TYPE) AUTHID DEFINER IS
--                ^ keyword
--                          ^ module
--                            ^ punctuation.delimiter
--                             ^ function
--                                ^ variable.parameter
--                                  ^ keyword
--                                     ^ type.builtin
--                                               ^ keyword
--                                                      ^ operator
--                                                       ^ type.builtin
--                                                             ^ keyword
--                                                                            ^ keyword
  n CONSTANT PLS_INTEGER := 0;
--^ variable
--  ^ keyword
--           ^ type.builtin
--                       ^ operator
--                          ^ number
  e EXCEPTION;
--^ constant
--  ^ keyword
  PRAGMA EXCEPTION_INIT(e, -20001);
--^ keyword
--       ^ keyword.directive
  TYPE tab_t IS TABLE OF NUMBER INDEX BY PLS_INTEGER;
--^ keyword
--     ^ type.definition
--              ^ keyword
--                              ^ keyword
  CURSOR c IS SELECT a FROM t;
--^ keyword
--       ^ variable
--            ^ keyword
BEGIN
-- <- keyword
  <<outer>>
--^ punctuation.special
--  ^ label
  FOR i IN 1..10 LOOP
--^ keyword
--    ^ variable
--      ^ keyword
--          ^ operator
--               ^ keyword
    EXIT outer WHEN c%NOTFOUND;
--  ^ keyword
--       ^ label
--             ^ keyword
--                    ^ attribute.builtin
  END LOOP outer;
--^ keyword
--    ^ keyword
--         ^ label
  IF a > 1 THEN dbms_output.put_line('x'); ELSIF a IS NULL THEN p; ELSE NULL; END IF;
--^ keyword
--         ^ keyword
--              ^ module
--                          ^ function.call
--                                         ^ keyword
--                                                              ^ function.call
--                                                                 ^ keyword
--                                                                      ^ keyword
  EXECUTE IMMEDIATE 'SELECT 1 FROM dual' INTO n;
--^ keyword
--        ^ keyword
--                  ^ string
--                                       ^ keyword
  $IF $$debug $THEN NULL; $END
--^ keyword.directive
--    ^ constant.macro
--            ^ keyword.directive
--                        ^ keyword.directive
EXCEPTION
-- <- keyword
  WHEN NO_DATA_FOUND OR e THEN RAISE e;
--^ keyword
--     ^ constant
--                   ^ keyword
--                      ^ constant
--                             ^ keyword
--                                   ^ constant
END p;
-- <- keyword
--  ^ function
--   ^ punctuation.delimiter
/
-- <- punctuation.delimiter
CREATE PACKAGE BODY pkg AS FUNCTION f RETURN NUMBER IS BEGIN RETURN 1; END f; END pkg;
--     ^ keyword
--             ^ keyword
--                  ^ module
--                         ^ keyword
--                                  ^ function
--                                    ^ keyword
--                                                                         ^ function
--                                                                                ^ module
CREATE TRIGGER trg BEFORE INSERT ON t FOR EACH ROW BEGIN :new.id := s.NEXTVAL; END;
--     ^ keyword
--             ^ function
--                 ^ keyword
--                        ^ keyword
--                                  ^ type
--                                        ^ keyword
--                                                       ^ variable.parameter
--                                                                    ^ variable.member
