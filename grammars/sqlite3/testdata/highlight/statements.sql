INSERT OR REPLACE INTO main.t AS x (a, b) VALUES (1, ?2) ON CONFLICT (a) DO UPDATE SET b = excluded.b RETURNING *;
-- <- keyword
--     ^ keyword
--        ^ keyword
--                     ^ module
--                          ^ type
--                                                   ^ variable.parameter
--                                                          ^ keyword
--                                                                       ^ keyword
--                                                                                     ^ variable.member
--                                                                                       ^ operator
--                                                                                         ^ type
--                                                                                                    ^ keyword
UPDATE t SET a = 1 FROM u WHERE t.id = u.id ORDER BY a LIMIT 1;
-- <- keyword
--           ^ variable.member
--                 ^ keyword
--                                          ^ keyword
DELETE FROM t WHERE a > 0 RETURNING a AS deleted;
-- <- keyword
--          ^ type
--                                       ^ variable
CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT COLLATE NOCASE NOT NULL, CONSTRAINT u UNIQUE (name)) STRICT;
-- <- keyword
--           ^ keyword
--                         ^ type
--                            ^ variable.member
--                               ^ type.builtin
--                                                   ^ keyword
--                                                                  ^ variable.member
--                                                                                    ^ variable.builtin
--                                                                                                                ^ label
--                                                                                                                                 ^ keyword
CREATE VIRTUAL TABLE ft USING fts5(title, body, tokenize = 'porter');
--     ^ keyword
--                   ^ type
--                      ^ keyword
--                            ^ function
--                                                         ^ string
CREATE TRIGGER tr AFTER UPDATE OF a ON t BEGIN SELECT RAISE(IGNORE); END;
--     ^ keyword
--             ^ type
--                ^ keyword
--                                       ^ keyword
--                                                                   ^ keyword
PRAGMA main.journal_mode = WAL;
-- <- keyword
--     ^ module
--          ^ variable.builtin
--                         ^ variable
ATTACH DATABASE 'x.db' AS aux; SAVEPOINT sp; ROLLBACK TO sp; VACUUM aux INTO 'y.db';
-- <- keyword
--                        ^ module
--                                       ^ label
--                                                       ^ label
--                                                           ^ keyword
--                                                                  ^ module
