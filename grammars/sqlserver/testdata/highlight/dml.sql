INSERT INTO dbo.t (a, b) OUTPUT inserted.a INTO @log VALUES (1, DEFAULT);
-- <- keyword
--     ^ keyword
--          ^ module
--              ^ type
--                 ^ variable.member
--                       ^ keyword
--                              ^ type
--                                         ^ keyword
--                                              ^ variable.parameter
--                                                   ^ keyword
--                                                              ^ keyword
UPDATE TOP (5) t SET a += 1, c.WRITE(N'x', 0, NULL) FROM t JOIN u ON t.id = u.id WHERE CURRENT OF cur
-- <- keyword
--     ^ keyword
--               ^ keyword
--                     ^ operator
--                             ^ function.call
--                                                  ^ keyword
--                                                                               ^ keyword
--                                                                                     ^ keyword
--                                                                                             ^ keyword
--                                                                                                ^ variable
DELETE FROM t WHERE a = 1
-- <- keyword
--     ^ keyword
--          ^ type
MERGE t AS tgt USING s ON tgt.id = s.id WHEN NOT MATCHED BY TARGET THEN INSERT (id) VALUES (s.id) OUTPUT $action;
-- <- keyword
--         ^ variable
--             ^ keyword
--                                      ^ keyword
--                                               ^ keyword
--                                                          ^ keyword
--                                                                 ^ keyword
--                                                                                                       ^ variable.builtin
BULK INSERT t FROM 'f.csv' WITH (FORMAT = 'CSV')
-- <- keyword
--                 ^ string
--                               ^ keyword
TRUNCATE TABLE t
-- <- keyword
--       ^ keyword
