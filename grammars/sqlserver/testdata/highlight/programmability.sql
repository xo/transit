CREATE PROCEDURE dbo.p @n int = 0 OUTPUT, @t dbo.tt READONLY WITH RECOMPILE AS
-- <- keyword
--     ^ keyword
--               ^ module
--                   ^ function
--                     ^ variable.parameter
--                        ^ type.builtin
--                              ^ number
--                                ^ keyword
--                                        ^ variable.parameter
--                                                  ^ keyword
--                                                                ^ keyword
--                                                                          ^ keyword
SELECT @n = COUNT(*) FROM @t
-- <- keyword
--          ^ function.call
GO
-- <- keyword.directive
CREATE OR ALTER FUNCTION dbo.f (@d date) RETURNS int WITH SCHEMABINDING AS BEGIN RETURN 1 END
--     ^ keyword
--              ^ keyword
--                           ^ function
--                              ^ variable.parameter
--                                 ^ type.builtin
--                                       ^ keyword
--                                               ^ type.builtin
--                                                        ^ keyword
--                                                                         ^ keyword
--                                                                               ^ keyword.return
--                                                                                        ^ keyword
GO
-- <- keyword.directive
CREATE FUNCTION dbo.g () RETURNS TABLE AS RETURN SELECT 1 AS n
--                  ^ function
--                               ^ keyword
--                                        ^ keyword.return
--                                               ^ keyword
GO
CREATE FUNCTION dbo.h (@s nvarchar(max)) RETURNS bigint AS EXTERNAL NAME a.c.m
--                  ^ function
--                                                         ^ keyword
--                                                                  ^ keyword
--                                                                       ^ module
--                                                                           ^ function
GO
CREATE TRIGGER dbo.tr ON dbo.t AFTER INSERT, UPDATE AS IF UPDATE(a) ROLLBACK
--     ^ keyword
--                 ^ function
--                    ^ keyword
--                           ^ type
--                             ^ keyword
--                                   ^ keyword
--                                           ^ keyword
--                                                     ^ keyword.conditional
--                                                        ^ function.call
--                                                                  ^ keyword
GO
CREATE TRIGGER safety ON DATABASE FOR DROP_TABLE AS PRINT 1
--             ^ function
--                       ^ keyword
--                                ^ keyword
--                                    ^ keyword
GO
DISABLE TRIGGER safety ON DATABASE
-- <- keyword
--              ^ function
BEGIN DIALOG @h FROM SERVICE s1 TO SERVICE 's2' ON CONTRACT c
-- <- keyword
--    ^ keyword
--           ^ variable.parameter
--                   ^ keyword
--                           ^ type
--                                         ^ string
--                                                 ^ keyword
--                                                          ^ type
SEND ON CONVERSATION @h MESSAGE TYPE m (@b)
-- <- keyword
--      ^ keyword
--                      ^ keyword
--                                   ^ type
WAITFOR (RECEIVE TOP (1) @b = message_body FROM q), TIMEOUT 1000
-- <- keyword
--       ^ keyword
--                            ^ variable.member
--                                              ^ type
--                                                  ^ keyword
END CONVERSATION @h WITH CLEANUP
-- <- keyword
--                       ^ keyword
BACKUP DATABASE d TO DISK = 'd.bak' WITH COMPRESSION
-- <- keyword
--     ^ keyword
--              ^ type
--                   ^ keyword
--                          ^ string
--                                       ^ keyword
DBCC CHECKDB (N'd') WITH NO_INFOMSGS
-- <- keyword
--   ^ function.builtin
--            ^ string
--                       ^ keyword
KILL 53 WITH STATUSONLY
-- <- keyword
--   ^ number
--           ^ keyword
