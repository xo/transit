COPY lineitem TO 'lineitem.csv' (FORMAT csv, HEADER);
-- <- keyword
--   ^ type
--            ^ keyword
--               ^ string
--                               ^ variable.parameter
--                                           ^ variable.parameter
ATTACH 'file.db' AS file_db (READ_ONLY); USE file_db; DETACH file_db;
-- <- keyword
--     ^ string
--                  ^ module
--                           ^ variable.parameter
--                                       ^ keyword
--                                           ^ module
--                                                    ^ keyword
--                                                           ^ module
SET GLOBAL threads = 4; RESET threads; PRAGMA explain_output = PHYSICAL_ONLY;
-- <- keyword
--  ^ keyword
--         ^ variable.builtin
--                   ^ number
--                      ^ keyword
--                            ^ variable.builtin
--                                     ^ keyword
--                                            ^ function.builtin
--                                                             ^ constant
INSTALL h3 FROM community; LOAD h3; EXPLAIN ANALYZE SELECT 1;
-- <- keyword
--      ^ module
--              ^ module
--                         ^ keyword
--                              ^ module
--                                  ^ keyword
--                                          ^ keyword
PREPARE q AS SELECT $1; EXECUTE q(42); BEGIN TRANSACTION; COMMIT;
-- <- keyword
--      ^ function
--                  ^ variable.parameter
--                      ^ keyword
--                              ^ function.call
--                                     ^ keyword
--                                           ^ keyword
--                                                        ^ keyword
COMMENT ON TABLE t IS 'nice'; SHOW TABLES; DESCRIBE t; SUMMARIZE t;
-- <- keyword
--               ^ type
--                 ^ keyword
--                            ^ keyword
--                                 ^ keyword
--                                         ^ keyword
--                                                     ^ keyword
