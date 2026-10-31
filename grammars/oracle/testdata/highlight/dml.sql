INSERT INTO hr.employees e (id, name) VALUES (1, DEFAULT) RETURNING id INTO :new_id;
-- <- keyword
--     ^ keyword
--          ^ module
--             ^ type
--                       ^ variable
--                          ^ variable.member
--                                    ^ keyword
--                                               ^ keyword
--                                                        ^ keyword
--                                                                          ^ variable.parameter
UPDATE employees e SET e.salary = 1 WHERE CURRENT OF emp_cur;
-- <- keyword
--     ^ type
--               ^ variable
--                 ^ keyword
--                       ^ variable.member
--                                        ^ keyword
--                                                   ^ variable
MERGE INTO t USING s ON (t.id = s.id) WHEN MATCHED THEN UPDATE SET t.a = s.a LOG ERRORS;
-- <- keyword
--         ^ type
--           ^ keyword
--                                         ^ keyword
--                                                                           ^ keyword
--                                                                               ^ keyword
COMMIT WORK; SAVEPOINT s1; ROLLBACK TO SAVEPOINT s1;
-- <- keyword
--     ^ keyword
--           ^ keyword
--                     ^ label
--                         ^ keyword
--                                               ^ label
ALTER SESSION SET nls_date_format = 'YYYY-MM-DD';
-- <- keyword
--    ^ keyword
--                ^ variable.builtin
--                                  ^ string
ALTER SYSTEM KILL SESSION '13,8' IMMEDIATE;
--    ^ keyword
--           ^ keyword
--                        ^ string
--                               ^ keyword
GRANT CREATE SESSION, UNLIMITED TABLESPACE TO app WITH ADMIN OPTION;
-- <- keyword
--    ^ keyword
--                    ^ keyword
--                              ^ keyword
--                                            ^ variable
--                                                     ^ keyword
COMMENT ON TABLE hr.employees IS 'staff';
-- <- keyword
--                  ^ type
--                            ^ keyword
--                               ^ string
