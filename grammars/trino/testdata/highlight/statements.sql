INSERT INTO lake.logs @ dev (msg) SELECT m FROM src;
-- <- keyword
--     ^ keyword
--          ^ module
--               ^ type
--                    ^ punctuation.special
--                           ^ variable.member
--                                ^ keyword
UPDATE stock SET qty = qty - 1 WHERE item = 'pen';
-- <- keyword
--     ^ type
--           ^ keyword
--               ^ variable.member
--                   ^ operator
--                                          ^ string
CREATE TABLE IF NOT EXISTS t (id bigint NOT NULL, ts timestamp(3) WITH (x = 1)) WITH (format = 'ORC');
-- <- keyword
--     ^ keyword
--           ^ keyword
--                         ^ type
--                            ^ variable.member
--                               ^ type.builtin
--                                          ^ constant.builtin
--                                                   ^ type.builtin
--                                                                      ^ variable.parameter
--                                                                                    ^ variable.parameter
CREATE SCHEMA hive.web AUTHORIZATION ROLE admins;
--     ^ keyword
--            ^ module
--                 ^ module
--                     ^ keyword
--                                   ^ keyword
--                                        ^ variable
SET SESSION hive.codec = 'ZSTD';
-- <- keyword
--  ^ keyword
--          ^ module
--               ^ variable.builtin
--                     ^ operator
CALL system.sync_partition_metadata('web', 'clicks', 'FULL');
-- <- keyword
--   ^ module
--          ^ function.call
--                                  ^ string
SHOW TABLES FROM hive.web LIKE 'c%';
-- <- keyword
--   ^ keyword
--               ^ module
--                    ^ module
--                        ^ keyword
--                             ^ string
GRANT SELECT ON TABLE orders TO USER alice WITH GRANT OPTION;
-- <- keyword
--    ^ keyword
--                    ^ type
--                              ^ keyword
--                                   ^ variable
--                                                    ^ keyword
GRANT SELECT ON catalog_view sales.v TO ROLE readers;
-- <- keyword
--              ^ keyword
--                           ^ module
--                                 ^ type
--                                   ^ keyword
--                                             ^ variable
ALTER catalog_view sales.v SET AUTHORIZATION owners;
-- <- keyword
--    ^ keyword
--                       ^ type
--                         ^ keyword
--                                           ^ variable
CREATE TABLE copy AS SELECT * FROM orders WITH NO DATA;
--                                        ^ keyword
--                                             ^ keyword
--                                                ^ keyword
