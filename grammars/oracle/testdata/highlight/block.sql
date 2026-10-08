CREATE OR REPLACE PACKAGE BODY hr.names IS
-- <- keyword
    -- ^ keyword.operator
       -- ^ keyword
               -- ^ keyword
                       -- ^ keyword
                            -- ^ namespace
                               -- ^ module
                                     -- ^ keyword.operator
FUNCTION get_name (p_id IN NUMBER) RETURN VARCHAR2 IS
-- <- keyword.function
      -- ^ function
               -- ^ punctuation.bracket
                -- ^ variable.parameter
                     -- ^ keyword.operator
                        -- ^ type.builtin
                                -- ^ keyword.return
                                       -- ^ type.builtin
                                                -- ^ keyword.operator
  l_name employees.last_name%TYPE;
  -- <- variable
                         -- ^ type.qualifier
                              -- ^ punctuation.delimiter
BEGIN
-- <- keyword
  SELECT last_name INTO l_name FROM employees e WHERE e.id = p_id;
  -- <- keyword
      -- ^ variable
                -- ^ keyword
                            -- ^ keyword
                                 -- ^ type
                                             -- ^ keyword
                                                    -- ^ punctuation.delimiter
                                                        -- ^ operator
  IF l_name IS NULL AND p_id > 0 THEN
  -- <- keyword.conditional
         -- ^ keyword.operator
            -- ^ constant.builtin
                 -- ^ keyword.operator
                          -- ^ operator
                            -- ^ number
                              -- ^ keyword.conditional
    l_name := 'none';
        -- ^ operator
           -- ^ string
  END IF;
  -- <- keyword
  RETURN upper(l_name);
  -- <- keyword.return
      -- ^ function.call
END get_name;
-- <- keyword
END names;
