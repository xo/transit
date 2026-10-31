CREATE OR REPLACE FUNCTION app.add(a int, b int DEFAULT 1) RETURNS int AS $$ SELECT a + b $$ LANGUAGE sql IMMUTABLE;
-- <- keyword
--        ^ keyword
--                ^ keyword
--                         ^ module
--                             ^ function
--                                 ^ variable.parameter
--                                   ^ type.builtin
--                                              ^ keyword
--                                                         ^ keyword
--                                                                     ^ keyword
--                                                                        ^ string
--                                                                                           ^ keyword
--                                                                                                        ^ keyword
CREATE AGGREGATE avg (float8) (sfunc = float8_accum, sortop = >);
--     ^ keyword
--                             ^ property
--                                     ^ type.builtin
--                                                            ^ operator
CREATE TRIGGER check_update BEFORE UPDATE OF balance ON accounts FOR EACH ROW EXECUTE FUNCTION check_account();
--     ^ keyword
--             ^ label
--                          ^ keyword
--                                           ^ variable.member
--                                                      ^ type
--                                                                   ^ keyword
--                                                                            ^ keyword
--                                                                                             ^ function.call
CREATE EVENT TRIGGER abort_ddl ON ddl_command_start WHEN tag IN ('DROP TABLE') EXECUTE FUNCTION abort();
--     ^ keyword
--                   ^ label
--                                                       ^ variable.builtin
--                                                               ^ string
CREATE POLICY p ON accounts FOR SELECT TO managers USING (manager = current_user);
--     ^ keyword
--            ^ label
--                              ^ keyword
--                                                 ^ keyword
--                                                        ^ variable.member
GRANT SELECT (a), UPDATE ON TABLE films TO PUBLIC WITH GRANT OPTION;
-- <- keyword
--    ^ keyword
--            ^ variable.member
--                ^ keyword
--                                ^ type
--                                                           ^ keyword
CREATE ROLE miriam WITH LOGIN PASSWORD 'x' VALID UNTIL '2030-01-01';
--     ^ keyword
--                      ^ property
--                            ^ property
--                                         ^ keyword
--                                               ^ keyword
CREATE SERVER s FOREIGN DATA WRAPPER postgres_fdw OPTIONS (host 'foo', dbname 'foodb');
--     ^ keyword
--                           ^ keyword
--                                                ^ keyword
--                                                         ^ property
--                                                              ^ string
CREATE PUBLICATION p FOR TABLE users WHERE (active), TABLES IN SCHEMA s;
--     ^ keyword
--                             ^ type
--                                                   ^ keyword
