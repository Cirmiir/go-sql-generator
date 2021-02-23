# go-sql-generator

Simple sql stored procedure generator. 
The resulting stored procedure can support select, update, delete, insert the row. The store procedure is generated based on table structure.

The template can be modified for each database driver.

# example of usage

mssql
  `main.exe "server=localhost;user id=user;password=password;database=test" mssql simple -s -u -d -i`

mysql
  `main.exe "user:password@/test" mysql simple -s -u -d -i`

Table:
```
CREATE TABLE simple2(
	name nvarchar(max) NULL,
	ID int IDENTITY(1,1) NOT NULL,
PRIMARY KEY CLUSTERED 
(
	ID ASC
)WITH (PAD_INDEX = OFF, STATISTICS_NORECOMPUTE = OFF, IGNORE_DUP_KEY = OFF, ALLOW_ROW_LOCKS = ON, ALLOW_PAGE_LOCKS = ON) ON [PRIMARY]
) ON [PRIMARY] TEXTIMAGE_ON [PRIMARY]

```

Result:

```
CREATE PROCEDURE sp_simple_select
  @name nvarchar(max) = NULL,
  @ID int = NULL
AS
BEGIN
SELECT
        name,
        ID
FROM simple
 WHERE ((ID = @ID OR @ID IS NULL))
END
CREATE PROCEDURE sp_simple_delete
  @ID int = NULL
AS
BEGIN
DELETE FROM simple
 WHERE ((ID = @ID OR @ID IS NULL))
END
CREATE PROCEDURE sp_simple_insert
  @name nvarchar(max) = NULL
AS
BEGIN
INSERT INTO simple (name)
VALUES(@name)
END
CREATE PROCEDURE sp_simple_update
  @name nvarchar(max) = NULL,
  @ID int = NULL
AS
BEGIN
UPDATE simple SET
        name = @name
 WHERE ((ID = @ID OR @ID IS NULL))
END
```