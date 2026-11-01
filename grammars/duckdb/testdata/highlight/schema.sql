CREATE OR REPLACE TABLE t (id INTEGER PRIMARY KEY, s VARCHAR DEFAULT 'x' NOT NULL, g AS (id * 2));
-- <- keyword
--        ^ keyword
--                      ^ type
--                         ^ variable.member
--                            ^ type.builtin
--                                    ^ keyword
--                                                           ^ keyword
--                                                                   ^ string
--                                                                           ^ constant.builtin
--                                                                                 ^ variable.member
ALTER TABLE t ADD COLUMN IF NOT EXISTS k INTEGER, RENAME COLUMN a TO b;
-- <- keyword
--            ^ keyword
--                ^ keyword
--                                     ^ variable.member
--                                                ^ keyword
--                                                              ^ variable.member
CREATE MACRO add_default(a, b := 5) AS a + b;
--     ^ keyword
--           ^ type
--                       ^ variable.parameter
--                          ^ variable.parameter
--                            ^ operator
CREATE SECRET s3_secret (TYPE s3, KEY_ID 'key', REGION = 'us');
--     ^ keyword
--                       ^ variable.parameter
--                                ^ variable.parameter
--                                       ^ string
--                                                     ^ operator
CREATE TYPE mood AS ENUM ('sad', 'happy'); DROP TABLE IF EXISTS t CASCADE;
--     ^ keyword
--          ^ type
--                  ^ keyword
--                                         ^ keyword
--                                                       ^ keyword
--                                                                ^ keyword
