INSERT INTO s.films AS f (code, board[1]) VALUES ($1, DEFAULT) ON CONFLICT (code) DO NOTHING RETURNING f.code;
-- <- keyword
--     ^ keyword
--          ^ module
--            ^ type
--                  ^ keyword
--                     ^ variable
--                        ^ variable.member
--                              ^ variable.member
--                                        ^ keyword
--                                                ^ variable.parameter
--                                                    ^ keyword
--                                                             ^ keyword
--                                                                ^ keyword
--                                                                          ^ variable.member
--                                                                                ^ keyword
--                                                                                   ^ keyword
--                                                                                           ^ keyword
--                                                                                                     ^ type
--                                                                                                       ^ variable.member
UPDATE t SET a = 1, b.c = DEFAULT WHERE CURRENT OF cur;
-- <- keyword
--     ^ type
--       ^ keyword
--           ^ variable.member
--                  ^ variable.member
--                    ^ variable.member
--                                      ^ keyword
--                                              ^ keyword
--                                                 ^ variable
DELETE FROM t USING u WHERE t.id = u.id;
-- <- keyword
--            ^ keyword
--                  ^ type
MERGE INTO w USING s ON s.id = w.id WHEN NOT MATCHED THEN INSERT VALUES (1) WHEN MATCHED THEN DELETE;
-- <- keyword
--                                  ^ keyword
--                                           ^ keyword
--                                                   ^ keyword
--                                                        ^ keyword
--                                                                                            ^ keyword
COPY t (a) FROM STDIN WITH (FORMAT csv, HEADER);
-- <- keyword
--      ^ variable.member
--         ^ keyword
--              ^ keyword
--                          ^ property
--                                 ^ variable
--                                      ^ property
BEGIN ISOLATION LEVEL SERIALIZABLE; SAVEPOINT sp; ROLLBACK TO sp; COMMIT;
-- <- keyword
--    ^ keyword
--                    ^ keyword
--                                  ^ keyword
--                                            ^ label
--                                                ^ keyword
--                                                            ^ label
--                                                                ^ keyword
PREPARE q (int) AS SELECT $1; EXECUTE q (2); DEALLOCATE q;
-- <- keyword
--      ^ function
--         ^ type.builtin
--                            ^ keyword
--                                    ^ function
--                                                      ^ function
SET LOCAL search_path TO public, "$user"; SET ROLE paul; SHOW ALL; RESET work_mem;
-- <- keyword
--  ^ keyword
--        ^ variable.builtin
--                    ^ keyword
--                                            ^ keyword
--                                                       ^ keyword
--                                                                       ^ variable.builtin
EXPLAIN (ANALYZE, COSTS off) SELECT 1; VACUUM FULL t; LOCK t IN ACCESS SHARE MODE;
-- <- keyword
--       ^ property
--                ^ property
--                                     ^ keyword
--                                            ^ keyword
--                                                    ^ keyword
--                                                              ^ keyword
--                                                                           ^ keyword
