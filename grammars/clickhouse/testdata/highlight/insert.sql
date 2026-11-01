INSERT INTO db.orders (id, item) SETTINGS async_insert = 1 VALUES (1, 'pen');
-- <- keyword
--     ^ keyword
--          ^ module
--             ^ type
--                     ^ variable.member
--                               ^ keyword
--                                        ^ variable.builtin
--                                                         ^ keyword
--                                                                    ^ string
INSERT INTO visits FORMAT CSV 3,home
--                 ^ keyword
--                        ^ type.builtin
--                            ^ string.special
DELETE FROM s ON CLUSTER main WHERE x = 1; UPDATE a SET b = 2 WHERE c; TRUNCATE TABLE IF EXISTS t SYNC;
-- <- keyword
--               ^ keyword
--                       ^ module
--                                         ^ keyword
--                                                  ^ keyword
--                                                      ^ variable.member
--                                                        ^ operator
--                                                                     ^ keyword
--                                                                                                ^ keyword
