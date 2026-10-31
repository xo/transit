CREATE FUNCTION f(a int) RETURNS int AS $$ BEGIN RETURN a + 1; END $$ LANGUAGE plpgsql;
-- <- keyword
--                                         ^ keyword
--                                               ^ keyword
--                                                             ^ keyword
--                                                                    ^ keyword
CREATE FUNCTION g() RETURNS int LANGUAGE sql AS $$ SELECT 1 $$;
--                                                 ^ keyword
--                                                        ^ number
DO $$ DECLARE r record; BEGIN INSERT INTO t VALUES (1); END $$;
--    ^ keyword
--              ^ type
--                      ^ keyword
--                            ^ keyword
--                                          ^ keyword
DO LANGUAGE plpgsql $body$ BEGIN PERFORM 1; END $body$;
--                               ^ keyword
CREATE FUNCTION h() RETURNS int AS $$ BEGIN END $$ LANGUAGE perl;
--                                    ^ string
