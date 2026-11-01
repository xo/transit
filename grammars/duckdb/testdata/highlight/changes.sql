INSERT OR REPLACE INTO tbl BY NAME SELECT 1 AS i ON CONFLICT (i) DO UPDATE SET j = EXCLUDED.j RETURNING *;
-- <- keyword
--        ^ keyword
--                     ^ type
--                            ^ keyword
--                                                  ^ keyword
--                                                            ^ variable.member
--                                                               ^ keyword
--                                                                         ^ keyword
--                                                                             ^ variable.member
--                                                                               ^ operator
--                                                                                 ^ type
--                                                                                            ^ keyword
UPDATE t AS u SET a = DEFAULT FROM s WHERE u.id = s.id;
-- <- keyword
--     ^ type
--          ^ variable
--                ^ variable.member
--                    ^ keyword
--                                 ^ type
DELETE FROM t USING s WHERE t.id = s.id; TRUNCATE TABLE t;
-- <- keyword
--            ^ keyword
--                                       ^ keyword
--                                                      ^ type
MERGE INTO t USING s ON t.id = s.id WHEN NOT MATCHED BY SOURCE THEN DELETE WHEN MATCHED THEN UPDATE SET *;
-- <- keyword
--                                  ^ keyword
--                                           ^ keyword
--                                                      ^ keyword
--                                                             ^ keyword
--                                                                                                      ^ punctuation.special
