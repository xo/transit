DECLARE @n int = 0, @t TABLE (id int PRIMARY KEY);
-- <- keyword
--      ^ variable.parameter
--         ^ type.builtin
--                     ^ keyword
--                            ^ variable.member
--                                   ^ keyword
SET NOCOUNT ON
-- <- keyword
--  ^ keyword
--          ^ keyword
BEGIN TRAN t1
-- <- keyword
--    ^ keyword
IF @n = 0 PRINT 'zero' ELSE PRINT 'other'
-- <- keyword.conditional
--        ^ keyword
--              ^ string
--                     ^ keyword.conditional
WHILE @n < 10 BEGIN SET @n += 1 IF @n = 5 BREAK END
-- <- keyword.repeat
--            ^ keyword
--                                        ^ keyword.repeat
--                                              ^ keyword
again:
-- <- label
IF @n < 20 GOTO again
--         ^ keyword
--              ^ label
BEGIN TRY EXEC dbo.p @a = 1, @b OUTPUT, x END TRY
-- <- keyword.exception
--    ^ keyword.exception
--        ^ keyword
--             ^ module
--                 ^ function.call
--                   ^ variable.parameter
--                              ^ keyword
--                                      ^ string
--                                        ^ keyword.exception
BEGIN CATCH THROW; END CATCH
--    ^ keyword.exception
--          ^ keyword.exception
--                 ^ keyword.exception
RAISERROR ('x', 16, 1) WITH NOWAIT
-- <- keyword.exception
--                          ^ keyword
DECLARE c CURSOR LOCAL FOR SELECT a FROM t
--      ^ variable
--        ^ keyword
--               ^ keyword
FETCH NEXT FROM c INTO @n
-- <- keyword
--    ^ keyword
--              ^ variable
COMMIT
-- <- keyword
USE master
-- <- keyword
--  ^ module
RETURN 1
-- <- keyword.return
