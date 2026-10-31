CREATE TABLE hr.t (id NUMBER CONSTRAINT t_pk PRIMARY KEY, name VARCHAR2(20) DEFAULT 'x' NOT NULL) TABLESPACE users;
-- <- keyword
--     ^ keyword
--           ^ module
--              ^ type
--                 ^ variable.member
--                    ^ type.builtin
--                           ^ keyword
--                                      ^ label
--                                           ^ keyword
--                                                        ^ variable.member
--                                                             ^ type.builtin
--                                                                          ^ keyword
--                                                                                  ^ string
--                                                                                      ^ keyword
--                                                                                                ^ keyword
--                                                                                                           ^ keyword
CREATE UNIQUE INDEX t_ix ON t (name DESC) LOCAL;
--     ^ keyword
--                  ^ type
--                       ^ keyword
--                          ^ type
--                             ^ variable.member
--                                  ^ keyword
--                                        ^ keyword
ALTER TABLE t ADD CONSTRAINT t_fk FOREIGN KEY (a) REFERENCES p (id) ON DELETE CASCADE;
-- <- keyword
--            ^ keyword
--                           ^ label
--                                ^ keyword
--                                            ^ punctuation.bracket
--                                                ^ keyword
--                                                           ^ type
--                                                                            ^ keyword
DROP TABLE IF EXISTS t CASCADE CONSTRAINTS PURGE;
-- <- keyword
--         ^ keyword
--                   ^ type
--                     ^ keyword
--                                         ^ keyword
CREATE OR REPLACE TYPE person_t AS OBJECT (name VARCHAR2(20), MEMBER FUNCTION get_name RETURN VARCHAR2);
--        ^ keyword
--                     ^ type
--                                 ^ keyword
--                                         ^ variable.member
--                                                            ^ keyword
--                                                                            ^ function
--                                                                                     ^ keyword
CREATE SEQUENCE s START WITH 1 NOCACHE;
--     ^ keyword
--              ^ type
--                ^ keyword
--                           ^ number
--                             ^ keyword
