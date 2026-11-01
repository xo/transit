SELECT 1
-- <- keyword
GO 2
-- <- keyword.directive
-- ^ number
select 2
-- <- keyword
--     ^ number
go
-- <- keyword.directive
select 3;
-- <- keyword
--     ^ number
